package memory

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolHandlers adapts the Memory port to MCP tool calls. Transport-agnostic:
// JSON in, JSON-serializable out, so it can be mounted on ROMA's MCP/gateway surface.
type ToolHandlers struct{ mem Memory }

func NewToolHandlers(mem Memory) *ToolHandlers {
	if mem == nil {
		mem = Nop()
	}
	return &ToolHandlers{mem: mem}
}

type recallArgs struct {
	Repo  string `json:"repo"`
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

func (h *ToolHandlers) Recall(ctx context.Context, raw json.RawMessage) (Recollection, error) {
	var a recallArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return Recollection{}, fmt.Errorf("memory_recall args: %w", err)
	}
	if a.Repo == "" {
		return Recollection{}, fmt.Errorf("memory_recall: repo is required")
	}
	return h.mem.Recall(ctx, Scope{Repo: a.Repo}, a.Query, a.Limit)
}

type noteArgs struct {
	Repo string   `json:"repo"`
	Fact string   `json:"fact"`
	Tags []string `json:"tags"`
}

// NoteResult is the JSON-serializable result of the memory_note tool.
type NoteResult struct {
	OK bool `json:"ok"`
}

func (h *ToolHandlers) Note(ctx context.Context, raw json.RawMessage) (NoteResult, error) {
	var a noteArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return NoteResult{}, fmt.Errorf("memory_note args: %w", err)
	}
	if a.Repo == "" || a.Fact == "" {
		return NoteResult{}, fmt.Errorf("memory_note: repo and fact are required")
	}
	if err := h.mem.Note(ctx, Scope{Repo: a.Repo}, a.Fact, a.Tags); err != nil {
		return NoteResult{}, err
	}
	return NoteResult{OK: true}, nil
}
