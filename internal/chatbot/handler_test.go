package chatbot

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
)

type recordedReply struct {
	chat string
	root string
	text string
}

type fakeSender struct {
	mu      sync.Mutex
	replies []recordedReply
}

func (f *fakeSender) Reply(_ context.Context, chat, root, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.replies = append(f.replies, recordedReply{chat: chat, root: root, text: text})
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

func noopProgress(string, string, string) {}

// fakeStore is an in-memory BindingStore for tests.
type fakeStore struct {
	mu       sync.Mutex
	m        map[string]Binding
	setErr   error
	delErr   error
	setCalls int
	delCalls int
}

func newFakeStore(seed ...Binding) *fakeStore {
	s := &fakeStore{m: make(map[string]Binding)}
	for _, b := range seed {
		s.m[b.ChatID] = b
	}
	return s
}

func (s *fakeStore) For(chatID string) (Binding, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, ok := s.m[chatID]
	return b, ok
}

func (s *fakeStore) Set(b Binding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.setCalls++
	if s.setErr != nil {
		return s.setErr
	}
	s.m[b.ChatID] = b
	return nil
}

func (s *fakeStore) Delete(chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.delCalls++
	if s.delErr != nil {
		return s.delErr
	}
	delete(s.m, chatID)
	return nil
}

func TestHandleBoundChatEnqueuesAndAcks(t *testing.T) {
	snd := &fakeSender{}
	enq := &fakeEnqueuer{jobID: "job-1"}
	store := newFakeStore(Binding{ChatID: "c1", Repo: "/r", Agent: "codex", Mode: "senate"})
	h := NewHandler(store, enq, snd, noopProgress)

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
	h := NewHandler(newFakeStore(Binding{ChatID: "c1", Repo: "/r"}), enq, &fakeSender{}, noopProgress)
	h.Handle(context.Background(), IncomingMessage{MessageID: "m1", ChatID: "c1", Text: "x", Mentioned: true, IsGroup: true})
	if enq.count() != 1 || enq.args[0].Mode != "rage" {
		t.Fatalf("mode default = %q", enq.args[0].Mode)
	}
}

func TestHandleUnboundChatRepliesNoEnqueue(t *testing.T) {
	snd := &fakeSender{}
	enq := &fakeEnqueuer{}
	h := NewHandler(newFakeStore(), enq, snd, noopProgress)

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
	h := NewHandler(newFakeStore(Binding{ChatID: "c1", Repo: "/r"}), enq, &fakeSender{}, noopProgress)
	msg := IncomingMessage{MessageID: "same", ChatID: "c1", Text: "x", Mentioned: true, IsGroup: true}
	h.Handle(context.Background(), msg)
	h.Handle(context.Background(), msg)
	if enq.count() != 1 {
		t.Fatalf("dedup failed: enqueued %d times", enq.count())
	}
}

func TestHandleIgnoresNonGroupAndEmpty(t *testing.T) {
	enq := &fakeEnqueuer{}
	h := NewHandler(newFakeStore(Binding{ChatID: "c1", Repo: "/r"}), enq, &fakeSender{}, noopProgress)
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

type fakeContextProvider struct{ transcript string }

func (f fakeContextProvider) RecentContext(_ context.Context, _, _ string) string {
	return f.transcript
}

func TestHandleWithContextProviderComposesPrompt(t *testing.T) {
	enq := &fakeEnqueuer{jobID: "j"}
	h := NewHandler(newFakeStore(Binding{ChatID: "c1", Repo: "/r"}), enq, &fakeSender{}, noopProgress)
	h.SetContextProvider(fakeContextProvider{transcript: "alice: hello\nbob: world"})

	h.Handle(context.Background(), IncomingMessage{
		MessageID: "m1", ChatID: "c1", Text: "fix the bug",
		Mentioned: true, IsGroup: true,
	})

	if enq.count() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", enq.count())
	}
	prompt := enq.args[0].Prompt
	if !strings.Contains(prompt, "alice: hello") || !strings.Contains(prompt, "bob: world") {
		t.Fatalf("prompt missing context: %q", prompt)
	}
	if !strings.Contains(prompt, "fix the bug") {
		t.Fatalf("prompt missing task: %q", prompt)
	}
}

func TestHandleWithoutContextProviderUsesText(t *testing.T) {
	enq := &fakeEnqueuer{jobID: "j"}
	h := NewHandler(newFakeStore(Binding{ChatID: "c1", Repo: "/r"}), enq, &fakeSender{}, noopProgress)

	h.Handle(context.Background(), IncomingMessage{
		MessageID: "m1", ChatID: "c1", Text: "fix the bug",
		Mentioned: true, IsGroup: true,
	})

	if enq.count() != 1 {
		t.Fatalf("expected 1 enqueue, got %d", enq.count())
	}
	if enq.args[0].Prompt != "fix the bug" {
		t.Fatalf("prompt = %q, want %q", enq.args[0].Prompt, "fix the bug")
	}
}

func TestComposePrompt(t *testing.T) {
	if got := composePrompt("", "task"); got != "task" {
		t.Fatalf("composePrompt empty context = %q, want %q", got, "task")
	}
	if got := composePrompt("   ", "task"); got != "task" {
		t.Fatalf("composePrompt blank context = %q, want %q", got, "task")
	}
	got := composePrompt("ctx line", "task")
	if !strings.Contains(got, "ctx line") || !strings.Contains(got, "task") {
		t.Fatalf("composePrompt = %q", got)
	}
}

func TestHandleSubmitErrorReplies(t *testing.T) {
	snd := &fakeSender{}
	enq := &fakeEnqueuer{err: errors.New("boom")}
	h := NewHandler(newFakeStore(Binding{ChatID: "c1", Repo: "/r"}), enq, snd, noopProgress)
	h.Handle(context.Background(), IncomingMessage{MessageID: "m1", ChatID: "c1", Text: "x", Mentioned: true, IsGroup: true})
	replies := snd.all()
	if len(replies) == 0 {
		t.Fatal("expected error reply")
	}
}
