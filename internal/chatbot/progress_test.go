package chatbot

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/liliang-cn/roma/internal/events"
)

func TestStreamProgressPostsThrottledPhases(t *testing.T) {
	snd := &fakeSender{}
	ch := make(chan events.Record, 4)
	ch <- events.Record{Type: events.TypeRelayNodeStarted}
	ch <- events.Record{Type: events.TypeRuntimeStarted}
	close(ch)

	fixed := time.Unix(0, 0)
	streamProgress(context.Background(), snd, "root", ch, 0, func() time.Time { return fixed })

	replies := snd.all()
	if len(replies) < 1 {
		t.Fatalf("expected at least one reply, got %d", len(replies))
	}
	for i, r := range replies {
		if strings.TrimSpace(r.text) == "" {
			t.Fatalf("reply %d is empty", i)
		}
	}
}

func TestStreamProgressThrottleSkips(t *testing.T) {
	snd := &fakeSender{}
	ch := make(chan events.Record, 4)
	ch <- events.Record{Type: events.TypeRelayNodeStarted}
	ch <- events.Record{Type: events.TypeRuntimeStarted}
	close(ch)

	fixed := time.Unix(100, 0)
	streamProgress(context.Background(), snd, "root", ch, time.Hour, func() time.Time { return fixed })

	if got := len(snd.all()); got > 1 {
		t.Fatalf("throttle: expected at most 1 reply, got %d", got)
	}
}

func TestFinalSummary(t *testing.T) {
	cases := []struct {
		status string
		errMsg string
		want   []string
	}{
		{"succeeded", "", []string{"✅", "succeeded"}},
		{"failed", "boom", []string{"❌", "boom"}},
		{"running", "", []string{"running"}},
	}
	for _, c := range cases {
		got := finalSummary(c.status, c.errMsg)
		if got == "" {
			t.Fatalf("finalSummary(%q,%q) returned empty", c.status, c.errMsg)
		}
		for _, sub := range c.want {
			if !strings.Contains(got, sub) {
				t.Fatalf("finalSummary(%q,%q) = %q, want it to contain %q", c.status, c.errMsg, got, sub)
			}
		}
	}
}
