package memory

import (
	"context"
	"encoding/json"
	"testing"
)

type recordingMemory struct {
	noted   string
	recalls int
}

func (r *recordingMemory) Recall(context.Context, Scope, string, int) (Recollection, error) {
	r.recalls++
	return Recollection{ContextText: "ctx", Knowledge: []Fact{{Text: "f"}}}, nil
}
func (r *recordingMemory) Record(context.Context, RunRecord) error { return nil }
func (r *recordingMemory) Note(_ context.Context, _ Scope, fact string, _ []string) error {
	r.noted = fact
	return nil
}

func TestMCPRecallTool(t *testing.T) {
	rm := &recordingMemory{}
	h := NewToolHandlers(rm)
	out, err := h.Recall(context.Background(), json.RawMessage(`{"repo":"/r","query":"q","limit":3}`))
	if err != nil {
		t.Fatalf("Recall() error = %v", err)
	}
	if rm.recalls != 1 || out.ContextText != "ctx" {
		t.Fatalf("unexpected recall result: %#v (recalls=%d)", out, rm.recalls)
	}
}

func TestMCPNoteTool(t *testing.T) {
	rm := &recordingMemory{}
	h := NewToolHandlers(rm)
	if _, err := h.Note(context.Background(), json.RawMessage(`{"repo":"/r","fact":"builds with GOWORK=off","tags":["build"]}`)); err != nil {
		t.Fatalf("Note() error = %v", err)
	}
	if rm.noted != "builds with GOWORK=off" {
		t.Fatalf("note not forwarded, got %q", rm.noted)
	}
}
