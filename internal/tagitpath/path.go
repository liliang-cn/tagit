package tagitpath

import (
	"os"
	"path/filepath"
	"strings"
)

// HomeDir returns the canonical TagIt home directory.
func HomeDir() string {
	if override := strings.TrimSpace(os.Getenv("TAGIT_HOME")); override != "" {
		return filepath.Clean(override)
	}
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return filepath.Clean(".tagit")
	}
	return filepath.Join(home, ".tagit")
}

// ControlDir returns the canonical TagIt control-plane directory.
func ControlDir() string {
	return HomeDir()
}

// ControlJoin returns a path rooted under the canonical TagIt control-plane directory.
func ControlJoin(elems ...string) string {
	parts := append([]string{ControlDir()}, elems...)
	return filepath.Join(parts...)
}

// WorkspaceStateDir returns the workspace-scoped TagIt execution directory.
func WorkspaceStateDir(workDir string) string {
	cleaned := filepath.Clean(strings.TrimSpace(workDir))
	if cleaned == "" || cleaned == "." {
		return filepath.Clean(".tagit")
	}
	if filepath.Base(cleaned) == ".tagit" {
		return cleaned
	}
	return filepath.Join(cleaned, ".tagit")
}

// WorkspaceJoin returns a path rooted under the workspace-scoped execution directory.
func WorkspaceJoin(workDir string, elems ...string) string {
	parts := append([]string{WorkspaceStateDir(workDir)}, elems...)
	return filepath.Join(parts...)
}

// StateDir is retained as an alias for workspace-scoped execution state.
func StateDir(workDir string) string {
	return WorkspaceStateDir(workDir)
}

// Join is retained as an alias for workspace-scoped execution paths.
func Join(workDir string, elems ...string) string {
	parts := append([]string{WorkspaceStateDir(workDir)}, elems...)
	return filepath.Join(parts...)
}
