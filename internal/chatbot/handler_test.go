package chatbot

import (
	"context"
	"errors"
	"sync"
	"testing"
)

type recordedReply struct {
	root string
	text string
}

type fakeSender struct {
	mu      sync.Mutex
	replies []recordedReply
}

func (f *fakeSender) Reply(_ context.Context, root, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = append(f.replies, recordedReply{root: root, text: text})
	return nil
}

func (f *fakeSender) all() []recordedReply {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]recordedReply, len(f.replies))
	copy(out, f.replies)
	return out
}

type fakeEnqueuer struct {
	mu    sync.Mutex
	args  []SubmitArgs
	jobID string
	err   error
}

func (f *fakeEnqueuer) Submit(_ context.Context, args SubmitArgs) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.args = append(f.args, args)
	return f.jobID, f.err
}

func (f *fakeEnqueuer) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.args)
}

func noopProgress(string, string) {}

func TestHandleBoundChatEnqueuesAndAcks(t *testing.T) {
	snd := &fakeSender{}
	enq := &fakeEnqueuer{jobID: "job-1"}
	bindings := Bindings{{ChatID: "c1", Repo: "/r", Agent: "codex", Mode: "senate"}}
	h := NewHandler(bindings, enq, snd, noopProgress)

	h.Handle(context.Background(), IncomingMessage{
		MessageID: "m1", ChatID: "c1", Text: "do the thing",
		Mentioned: true, IsGroup: true,
	})

	if enq.count() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", enq.count())
	}
	got := enq.args[0]
	want := SubmitArgs{Repo: "/r", Prompt: "do the thing", Agent: "codex", Mode: "senate"}
	if got != want {
		t.Fatalf("SubmitArgs = %+v, want %+v", got, want)
	}
	if len(snd.all()) == 0 {
		t.Fatal("expected at least one reply (ack)")
	}
}

func TestHandleDefaultsModeToRage(t *testing.T) {
	enq := &fakeEnqueuer{jobID: "j"}
	h := NewHandler(Bindings{{ChatID: "c1", Repo: "/r"}}, enq, &fakeSender{}, noopProgress)
	h.Handle(context.Background(), IncomingMessage{MessageID: "m1", ChatID: "c1", Text: "x", Mentioned: true, IsGroup: true})
	if enq.count() != 1 || enq.args[0].Mode != "rage" {
		t.Fatalf("mode default = %q", enq.args[0].Mode)
	}
}

func TestHandleUnboundChatRepliesNoEnqueue(t *testing.T) {
	snd := &fakeSender{}
	enq := &fakeEnqueuer{}
	h := NewHandler(Bindings{}, enq, snd, noopProgress)

	h.Handle(context.Background(), IncomingMessage{
		MessageID: "m1", ChatID: "cX", Text: "hi", Mentioned: true, IsGroup: true,
	})

	if enq.count() != 0 {
		t.Fatalf("unbound chat must not enqueue, got %d", enq.count())
	}
	if len(snd.all()) != 1 {
		t.Fatalf("expected exactly one reply, got %d", len(snd.all()))
	}
}

func TestHandleDedup(t *testing.T) {
	enq := &fakeEnqueuer{jobID: "j"}
	h := NewHandler(Bindings{{ChatID: "c1", Repo: "/r"}}, enq, &fakeSender{}, noopProgress)
	msg := IncomingMessage{MessageID: "same", ChatID: "c1", Text: "x", Mentioned: true, IsGroup: true}
	h.Handle(context.Background(), msg)
	h.Handle(context.Background(), msg)
	if enq.count() != 1 {
		t.Fatalf("dedup failed: enqueued %d times", enq.count())
	}
}

func TestHandleIgnoresNonGroupAndEmpty(t *testing.T) {
	enq := &fakeEnqueuer{}
	h := NewHandler(Bindings{{ChatID: "c1", Repo: "/r"}}, enq, &fakeSender{}, noopProgress)
	cases := []IncomingMessage{
		{MessageID: "a", ChatID: "c1", Text: "x", Mentioned: true, IsGroup: false},
		{MessageID: "b", ChatID: "c1", Text: "", Mentioned: true, IsGroup: true},
		{MessageID: "c", ChatID: "c1", Text: "x", Mentioned: false, IsGroup: true},
	}
	for _, m := range cases {
		h.Handle(context.Background(), m)
	}
	if enq.count() != 0 {
		t.Fatalf("ignored messages enqueued %d", enq.count())
	}
}

func TestHandleSubmitErrorReplies(t *testing.T) {
	snd := &fakeSender{}
	enq := &fakeEnqueuer{err: errors.New("boom")}
	h := NewHandler(Bindings{{ChatID: "c1", Repo: "/r"}}, enq, snd, noopProgress)
	h.Handle(context.Background(), IncomingMessage{MessageID: "m1", ChatID: "c1", Text: "x", Mentioned: true, IsGroup: true})
	replies := snd.all()
	if len(replies) == 0 {
		t.Fatal("expected error reply")
	}
}
