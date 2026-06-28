package memory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestMemory(t *testing.T) Memory {
	t.Helper()
	mem, err := NewAgentGo(filepath.Join(t.TempDir(), "cortex.db"))
	if err != nil {
		t.Fatalf("NewAgentGo() error = %v", err)
	}
	return mem
}

func TestAgentGoRecordThenRecall(t *testing.T) {
	mem := newTestMemory(t)
	ctx := context.Background()
	scope := Scope{Repo: "/repo/alpha"}

	if err := mem.Record(ctx, RunRecord{
		Scope: scope, SessionID: "s1", TaskID: "t1", Agent: "codex", Mode: "rage",
		Prompt:        "add input validation to the registration handler",
		ResultSummary: "added validation and tests", ChangedPaths: []string{"handler.go"},
		Verdict: "complete", Success: true, OccurredAt: time.Unix(1000, 0),
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	rec, err := mem.Recall(ctx, scope, "registration handler validation", 5)
	if err != nil {
		t.Fatalf("Recall() error = %v", err)
	}
	if len(rec.Episodes) == 0 {
		t.Fatalf("Recall() returned no episodes, want >=1")
	}
	if !strings.Contains(rec.ContextText, "validation") {
		t.Fatalf("ContextText = %q, want it to mention the past run", rec.ContextText)
	}
}

func TestAgentGoScopeIsolation(t *testing.T) {
	mem := newTestMemory(t)
	ctx := context.Background()
	if err := mem.Record(ctx, RunRecord{Scope: Scope{Repo: "/repo/alpha"}, Agent: "codex", Mode: "rage",
		Prompt: "fix auth token expiry", Success: true}); err != nil {
		t.Fatalf("Record alpha: %v", err)
	}
	rec, err := mem.Recall(ctx, Scope{Repo: "/repo/beta"}, "auth token expiry", 5)
	if err != nil {
		t.Fatalf("Recall beta: %v", err)
	}
	if len(rec.Episodes) != 0 || len(rec.Knowledge) != 0 {
		t.Fatalf("scope isolation failed: beta saw alpha's memory: %+v", rec)
	}
}
