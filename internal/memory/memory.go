package memory

import (
	"context"
	"time"
)

// Scope addresses a memory partition. Memory is keyed by repo (and optionally a
// chat channel, populated by a later sub-project); never by agent — memory is shared.
type Scope struct {
	Repo    string
	Channel string
}

// RunRecord is the episodic memory of one completed run.
type RunRecord struct {
	Scope         Scope
	SessionID     string
	TaskID        string
	Agent         string // metadata only, NOT a key
	Mode          string
	Prompt        string
	ResultSummary string
	ChangedPaths  []string
	Verdict       string
	Success       bool
	OccurredAt    time.Time
}

// Episode is a recalled past run.
type Episode struct {
	Summary    string
	Agent      string
	Mode       string
	Success    bool
	OccurredAt time.Time
}

// Fact is a recalled durable note about the repo.
type Fact struct {
	Text string
	Tags []string
}

// Recollection is what Recall returns; ContextText is ready to inject into a prompt.
type Recollection struct {
	Episodes    []Episode
	Knowledge   []Fact
	ContextText string
}

// Memory is ROMA's advisory, best-effort, cross-agent memory port.
type Memory interface {
	Recall(ctx context.Context, scope Scope, query string, limit int) (Recollection, error)
	Record(ctx context.Context, rec RunRecord) error
	Note(ctx context.Context, scope Scope, fact string, tags []string) error
}
