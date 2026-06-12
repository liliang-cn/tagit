package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/liliang-cn/roma/internal/domain"
)

// ProfileAdapter launches coding agents from user-provided command/args profiles.
type ProfileAdapter struct{}

// Supports reports whether the profile is runnable.
func (ProfileAdapter) Supports(profile domain.AgentProfile) bool {
	return strings.TrimSpace(profile.Command) != ""
}

// BuildCommand builds a launch command from the profile's configured args.
func (ProfileAdapter) BuildCommand(ctx context.Context, req StartRequest) (*exec.Cmd, error) {
	if strings.TrimSpace(req.Profile.Command) == "" {
		return nil, fmt.Errorf("agent %q has no command configured", req.Profile.ID)
	}
	args, promptViaStdin, err := buildProfileArgs(req)
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, req.Profile.Command, args...)
	if promptViaStdin {
		command.Stdin = strings.NewReader(req.Prompt)
	}
	return command, nil
}

// RequiresPTY reports whether the profile should be launched in a PTY.
func (ProfileAdapter) RequiresPTY(profile domain.AgentProfile) bool {
	if profile.UsePTY {
		return true
	}
	if profile.Metadata == nil {
		return false
	}
	return truthy(profile.Metadata["pty"]) || truthy(profile.Metadata["use_pty"])
}

func buildProfileArgs(req StartRequest) ([]string, bool, error) {
	args := make([]string, 0, len(req.Profile.Args)+1)
	promptReferenced := false
	promptViaStdin := shouldUsePromptStdin(req)
	for _, arg := range req.Profile.Args {
		expanded, usedPrompt := expandProfileArg(arg, req, promptViaStdin)
		promptReferenced = promptReferenced || usedPrompt
		if strings.TrimSpace(expanded) == "" {
			continue
		}
		args = append(args, expanded)
	}
	if !promptReferenced && strings.TrimSpace(req.Prompt) != "" {
		if promptViaStdin {
			args = append(args, "-")
		} else {
			args = append(args, req.Prompt)
		}
	}
	args = maybeInjectCodexGitWriteAccess(req, args)
	return args, promptViaStdin, nil
}

func maybeInjectCodexGitWriteAccess(req StartRequest, args []string) []string {
	command := strings.ToLower(filepath.Base(strings.TrimSpace(req.Profile.Command)))
	if command != "codex" || strings.TrimSpace(req.WorkingDir) == "" {
		return args
	}
	out := append([]string{}, args...)
	for _, root := range gitWritableRoots(req.WorkingDir) {
		if hasAddDirArg(out, root) {
			continue
		}
		out = append(out, "--add-dir", root)
	}
	return out
}

func gitWritableRoots(workDir string) []string {
	gitPath := filepath.Clean(filepath.Join(workDir, ".git"))
	info, err := os.Stat(gitPath)
	if err != nil {
		return []string{gitPath}
	}
	if info.IsDir() {
		return []string{gitPath}
	}
	gitDir, ok := resolveGitDirFromPointer(workDir, gitPath)
	if !ok {
		return []string{gitPath}
	}
	roots := []string{gitDir}
	if commonDir, ok := resolveGitCommonDir(gitDir); ok {
		roots = append(roots, commonDir)
	}
	return uniqueCleanPaths(roots)
}

func resolveGitDirFromPointer(workDir, gitPath string) (string, bool) {
	data, err := os.ReadFile(gitPath)
	if err != nil {
		return "", false
	}
	line := strings.TrimSpace(string(data))
	if !strings.HasPrefix(strings.ToLower(line), "gitdir:") {
		return "", false
	}
	target := strings.TrimSpace(line[len("gitdir:"):])
	if target == "" {
		return "", false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(workDir, target)
	}
	return filepath.Clean(target), true
}

func resolveGitCommonDir(gitDir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(gitDir, "commondir"))
	if err != nil {
		return "", false
	}
	target := strings.TrimSpace(string(data))
	if target == "" {
		return "", false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(gitDir, target)
	}
	return filepath.Clean(target), true
}

func hasAddDirArg(args []string, root string) bool {
	cleanRoot := filepath.Clean(root)
	for i := 0; i < len(args)-1; i++ {
		if args[i] != "--add-dir" {
			continue
		}
		if filepath.Clean(args[i+1]) == cleanRoot {
			return true
		}
	}
	return false
}

func uniqueCleanPaths(paths []string) []string {
	out := make([]string, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		clean := filepath.Clean(path)
		if clean == "" {
			continue
		}
		if _, ok := seen[clean]; ok {
			continue
		}
		seen[clean] = struct{}{}
		out = append(out, clean)
	}
	return out
}

func expandProfileArg(arg string, req StartRequest, promptViaStdin bool) (string, bool) {
	replacements := map[string]string{
		"{prompt}":       req.Prompt,
		"{cwd}":          req.WorkingDir,
		"{working_dir}":  req.WorkingDir,
		"{session_id}":   req.SessionID,
		"{task_id}":      req.TaskID,
		"{execution_id}": req.ExecutionID,
	}
	expanded := arg
	promptReferenced := false
	for placeholder, value := range replacements {
		if strings.Contains(expanded, placeholder) {
			if placeholder == "{prompt}" {
				promptReferenced = true
				if promptViaStdin {
					value = "-"
				}
			}
			expanded = strings.ReplaceAll(expanded, placeholder, value)
		}
	}
	return expanded, promptReferenced
}

func shouldUsePromptStdin(req StartRequest) bool {
	return req.Profile.PromptTransport == domain.PromptTransportStdin && strings.TrimSpace(req.Prompt) != ""
}

func truthy(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// BuildDelegationPrompt augments the starter prompt with allowed delegate agents.
func BuildDelegationPrompt(prompt string, delegates []domain.AgentProfile) string {
	if len(delegates) == 0 {
		return prompt
	}

	names := make([]string, 0, len(delegates))
	for _, delegate := range delegates {
		names = append(names, delegate.DisplayName+" ("+delegate.ID+")")
	}

	return prompt + "\n\n" +
		"Available secondary coding agents for delegation if useful:\n" +
		"- " + strings.Join(names, "\n- ") + "\n" +
		"Use them only when they materially improve execution, and preserve clear task ownership."
}

// ValidateWorkingDir checks basic runtime launch preconditions.
func ValidateWorkingDir(workingDir string) error {
	if strings.TrimSpace(workingDir) == "" {
		return fmt.Errorf("working directory is required")
	}
	return nil
}
