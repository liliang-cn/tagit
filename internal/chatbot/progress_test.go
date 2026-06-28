package chatbot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/liliang-cn/roma/internal/events"
)

func TestStreamProgressPostsAndSummarizes(t *testing.T) {
	snd := &fakeSender{}
	ch := make(chan events.Record, 4)
	ch <- events.Record{Type: "node.started", Payload: map[string]any{"phase": "worker"}}
	ch <- events.Record{Type: "node.progress", Payload: map[string]any{"phase": "implementing"}}
	ch <- events.Record{Type: "session.completed", Payload: map[string]any{"status": "succeeded"}}
	close(ch)

	fixed := time.Unix(0, 0)
	streamProgress(context.Background(), snd, "root", ch, 0, func() time.Time { return fixed })

	replies := snd.all()
	if len(replies) < 1 {
		t.Fatalf("expected at least one reply, got %d", len(replies))
	}
	last := replies[len(replies)-1]
	if !strings.Contains(last.text, "succeeded") {
		t.Fatalf("last reply = %q, want it to contain 'succeeded'", last.text)
	}
}

func TestStreamProgressThrottles(t *testing.T) {
	snd := &fakeSender{}
	ch := make(chan events.Record, 4)
	ch <- events.Record{Type: "node.progress", Payload: map[string]any{"phase": "a"}}
	ch <- events.Record{Type: "node.progress", Payload: map[string]any{"phase": "b"}}
	close(ch)

	fixed := time.Unix(100, 0)
	streamProgress(context.Background(), snd, "root", ch, time.Minute, func() time.Time { return fixed })

	if got := len(snd.all()); got != 1 {
		t.Fatalf("throttle: expected 1 reply, got %d", got)
	}
}

func TestStreamProgressFailureSummary(t *testing.T) {
	snd := &fakeSender{}
	ch := make(chan events.Record, 2)
	ch <- events.Record{Type: "job.failed", Payload: map[string]any{"status": "error", "changed_files": []any{"a.go", "b.go"}}}
	close(ch)
	streamProgress(context.Background(), snd, "root", ch, 0, time.Now)
	replies := snd.all()
	if len(replies) != 1 {
		t.Fatalf("expected 1 reply, got %d", len(replies))
	}
	txt := replies[0].text
	if !strings.Contains(txt, "❌") || !strings.Contains(txt, "error") || !strings.Contains(txt, "a.go") {
		t.Fatalf("failure summary = %q", txt)
	}
}
