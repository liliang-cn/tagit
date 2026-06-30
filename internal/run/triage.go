package run

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/liliang-cn/tagit/internal/domain"
	"github.com/liliang-cn/tagit/internal/events"
	"github.com/liliang-cn/tagit/internal/history"
	"github.com/liliang-cn/tagit/internal/memory"
)

// triageTaskSentinel is the token the agent emits when it judges the message to
// be real repo work rather than conversation. Kept distinctive so it can't be
// confused with an ordinary chat answer.
const triageTaskSentinel = "__TAGIT_TASK__"

// triageTimeout bounds the read-only classification call so a wedged agent can't
// stall the run; on timeout we fall through to the normal task pipeline.
const triageTimeout = 45 * time.Second

// maybeChatReply lets the agent itself decide whether the prompt is real repo
// work or just conversation (a greeting, "are you there?", thanks, or a question
// answerable in a sentence or two). For conversation it answers directly and
// records a finished session, skipping the worktree/scheduler/merge-back
// pipeline entirely. It returns handled=false when the message is real work (or
// triage couldn't run), so the caller proceeds with the normal run.
//
// This lives in the run layer on purpose: every entry point (CLI, chat bots,
// ACP) goes through here, so the chat-vs-task decision is made once, not in each
// adapter.
func (s *Service) maybeChatReply(ctx context.Context, req Request, profile domain.AgentProfile, stdout io.Writer) (Result, bool, error) {
	// Recall cross-agent memory (CortexDB) so conversation can answer from what
	// the repo's past chats established (e.g. "who am I?"). Best-effort.
	scope := memory.Scope{Repo: filepath.Clean(req.WorkingDir)}
	memCtx := s.recallMemory(ctx, scope, req.Prompt)

	answer, isTask := triageWithAgent(ctx, profile, req.WorkingDir, req.Prompt, memCtx)
	if isTask || strings.TrimSpace(answer) == "" {
		return Result{}, false, nil
	}

	sessionID, taskID := reserveIDs("task", req.SessionID, req.TaskID)
	now := time.Now().UTC()
	record := history.SessionRecord{
		ID:         sessionID,
		TaskID:     taskID,
		Prompt:     req.Prompt,
		Starter:    profile.ID,
		WorkingDir: req.WorkingDir,
		Status:     "succeeded",
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if req.SessionID != "" && s.history != nil {
		if existing, err := s.history.Get(ctx, sessionID); err == nil {
			record.CreatedAt = existing.CreatedAt
		}
	}
	s.appendEvent(ctx, events.Record{
		ID:         "evt_" + sessionID + "_created",
		SessionID:  sessionID,
		TaskID:     taskID,
		Type:       events.TypeSessionCreated,
		ActorType:  events.ActorTypeSystem,
		OccurredAt: record.CreatedAt,
		Payload:    map[string]any{"starter": profile.ID, "conversational": true},
	})
	// The reply text rides on this event so streaming consumers (chat bots)
	// render the actual answer rather than a generic "done" line.
	s.appendEvent(ctx, events.Record{
		ID:         "evt_" + sessionID + "_reply",
		SessionID:  sessionID,
		TaskID:     taskID,
		Type:       events.TypeConversationReplied,
		ActorType:  events.ActorTypeAgent,
		OccurredAt: time.Now().UTC(),
		Payload:    map[string]any{"text": answer, "agent": profile.ID},
	})
	if s.history != nil {
		if err := s.history.Save(ctx, record); err != nil {
			return Result{}, false, fmt.Errorf("save conversational session: %w", err)
		}
	}
	s.appendSessionStateEvent(ctx, record)
	// Record this exchange into cross-agent memory so future chats (even in other
	// threads) can recall it. Best-effort; never fails the reply.
	s.recordMemory(ctx, memory.RunRecord{
		Scope:         scope,
		SessionID:     sessionID,
		TaskID:        taskID,
		Agent:         profile.ID,
		Mode:          "chat",
		Prompt:        req.Prompt,
		ResultSummary: answer,
		Success:       true,
		OccurredAt:    now,
	})
	if stdout != nil {
		_, _ = fmt.Fprintln(stdout, answer)
		_, _ = fmt.Fprintf(stdout, "session=%s task=%s status=%s\n", record.ID, record.TaskID, record.Status)
	}
	return Result{SessionID: sessionID, TaskID: taskID, Status: record.Status}, true, nil
}

// triageWithAgent runs the agent once, read-only, to classify the message. It
// returns (answer, isTask). On any failure it returns ("", true) so the caller
// never drops real work: when unsure, do the task. memoryContext is recalled
// cross-agent memory injected so the agent can answer from past conversations.
func triageWithAgent(ctx context.Context, profile domain.AgentProfile, workingDir, message, memoryContext string) (string, bool) {
	if profile.Command == "" {
		return "", true
	}
	out, err := runTriageAgent(ctx, profile, workingDir, triagePrompt(message, memoryContext, workingDir))
	if err != nil {
		return "", true
	}
	return interpretTriageOutput(out)
}

// interpretTriageOutput turns the agent's raw triage output into (answer,
// isTask). Empty output or the task sentinel means "treat as real work"; any
// other text is the conversational answer.
func interpretTriageOutput(out string) (answer string, isTask bool) {
	out = strings.TrimSpace(out)
	if out == "" || strings.Contains(out, triageTaskSentinel) {
		return "", true
	}
	return out, false
}

// runTriageAgent invokes the agent CLI in a minimal, read-only one-shot to get a
// plain-text answer. It deliberately does NOT use the profile's task args (which
// auto-approve edits) — triage must never touch the repo.
func runTriageAgent(ctx context.Context, profile domain.AgentProfile, workingDir, prompt string) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, triageTimeout)
	defer cancel()

	cmd := exec.CommandContext(cctx, profile.Command, triageArgs(profile, prompt)...)
	if workingDir != "" {
		cmd.Dir = workingDir
	}
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		// Some CLIs exit non-zero yet still printed a usable answer; keep it.
		if stdout.Len() == 0 {
			return "", err
		}
	}
	return stdout.String(), nil
}

// triageArgs builds read-only one-shot arguments for the given agent command.
// Coding CLIs converge on `-p <prompt>` for a single print-mode turn; codex uses
// `exec`. None of these enable auto-edit, so the run can't mutate files.
func triageArgs(profile domain.AgentProfile, prompt string) []string {
	switch strings.ToLower(filepath.Base(profile.Command)) {
	case "codex":
		return []string{"exec", "--skip-git-repo-check", prompt}
	default: // claude, gemini, copilot, and anything else print-capable
		return []string{"-p", prompt}
	}
}

// triagePrompt asks the agent to either flag the message as real work or answer
// it conversationally — in one shot, with no repo changes.
func triagePrompt(message, memoryContext, workingDir string) string {
	var mem string
	if strings.TrimSpace(memoryContext) != "" {
		mem = "\nWhat you remember from this repo's past conversations (use it to answer, e.g. who someone is):\n" +
			memoryContext + "\n"
	}
	var dir string
	if strings.TrimSpace(workingDir) != "" {
		dir = "\nThe repository you are bound to (your working directory) is: " + workingDir + "\n"
	}
	return "You are TagIt, a coding-agent assistant in a team chat. Someone messaged you.\n" +
		dir +
		mem +
		"\nThe message below may be preceded by background conversation context. Base your decision ONLY on what " +
		"the user is actually asking right now (the latest request), not on the surrounding chatter. A greeting, " +
		"a presence check (\"are you there?\", \"在吗?\"), thanks, or a question answerable in a sentence or two are " +
		"CONVERSATION even when the chat around them is technical.\n\n" +
		"Decide: does the latest request need real work on the code repository — writing or editing code, running " +
		"commands, investigating the codebase, producing a deliverable — or is it just conversation?\n\n" +
		"Reply with EXACTLY ONE of:\n" +
		"1. If it needs real repo work: output only this token, nothing else: " + triageTaskSentinel + "\n" +
		"2. Otherwise: a short, friendly reply (1-3 sentences) in the same language as the user. Use what you " +
		"remember above when relevant. Do not modify the repo.\n\n" +
		"Message:\n\"\"\"\n" + message + "\n\"\"\""
}
