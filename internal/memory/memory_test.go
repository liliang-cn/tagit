package memory

import (
	"context"
	"testing"
)

func TestNopMemorySatisfiesInterfaceAndReturnsEmpty(t *testing.T) {
	var mem Memory = Nop()

	if err := mem.Record(context.Background(), RunRecord{Scope: Scope{Repo: "/r"}, Prompt: "p"}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if err := mem.Note(context.Background(), Scope{Repo: "/r"}, "fact", nil); err != nil {
		t.Fatalf("Note() error = %v", err)
	}
	rec, err := mem.Recall(context.Background(), Scope{Repo: "/r"}, "query", 5)
	if err != nil {
		t.Fatalf("Recall() error = %v", err)
	}
	if rec.ContextText != "" || len(rec.Episodes) != 0 || len(rec.Knowledge) != 0 {
		t.Fatalf("Recall() = %#v, want empty", rec)
	}
}
