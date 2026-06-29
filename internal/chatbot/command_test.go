package chatbot

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
)

var cmdMsgSeq int

// handle a command message and return the single reply text.
func runCmd(t *testing.T, h *Handler, snd *fakeSender, chatID, text string) string {
	t.Helper()
	cmdMsgSeq++
	before := len(snd.all())
	h.Handle(context.Background(), IncomingMessage{
		MessageID: fmt.Sprintf("msg-%d", cmdMsgSeq), // unique to avoid run-dedup
		ChatID:    chatID,
		Text:      text,
		Mentioned: true,
		IsGroup:   true,
	})
	replies := snd.all()
	if len(replies) != before+1 {
		t.Fatalf("expected exactly one reply for %q, got %d new", text, len(replies)-before)
	}
	return replies[len(replies)-1].text
}

func TestCommandHelp(t *testing.T) {
	snd := &fakeSender{}
	enq := &fakeEnqueuer{}
	h := NewHandler(newFakeStore(), enq, snd, noopProgress)
	got := runCmd(t, h, snd, "c1", "/help")
	for _, want := range []string{"/bind", "/agent", "/mode", "/status", "/unbind", "/help"} {
		if !strings.Contains(got, want) {
			t.Fatalf("/help missing %q: %q", want, got)
		}
	}
	if enq.count() != 0 {
		t.Fatalf("command must not enqueue, got %d", enq.count())
	}
}

func TestCommandUnknown(t *testing.T) {
	snd := &fakeSender{}
	h := NewHandler(newFakeStore(), &fakeEnqueuer{}, snd, noopProgress)
	got := runCmd(t, h, snd, "c1", "/wat now")
	if !strings.Contains(got, "Unknown command") {
		t.Fatalf("unknown command reply = %q", got)
	}
}

func TestCommandStatusUnboundAndBound(t *testing.T) {
	snd := &fakeSender{}
	store := newFakeStore()
	h := NewHandler(store, &fakeEnqueuer{}, snd, noopProgress)

	got := runCmd(t, h, snd, "c1", "/status")
	if !strings.Contains(got, "/bind") {
		t.Fatalf("unbound /status = %q", got)
	}

	_ = store.Set(Binding{ChatID: "c1", Repo: "/r", Agent: "codex", Mode: "senate"})
	got = runCmd(t, h, snd, "c1", "/status")
	for _, want := range []string{"/r", "codex", "senate"} {
		if !strings.Contains(got, want) {
			t.Fatalf("bound /status missing %q: %q", want, got)
		}
	}

	// defaults rendering: no agent/mode
	_ = store.Set(Binding{ChatID: "c2", Repo: "/r2"})
	got = runCmd(t, h, snd, "c2", "/status")
	if !strings.Contains(got, "(default)") || !strings.Contains(got, "rage") {
		t.Fatalf("default /status = %q", got)
	}
}

func TestCommandBindValidDir(t *testing.T) {
	snd := &fakeSender{}
	store := newFakeStore()
	h := NewHandler(store, &fakeEnqueuer{}, snd, noopProgress)
	dir := t.TempDir()

	got := runCmd(t, h, snd, "c1", "/bind "+dir)
	if !strings.Contains(got, "Linked") {
		t.Fatalf("/bind valid = %q", got)
	}
	b, ok := store.For("c1")
	if !ok || b.Repo != dir {
		t.Fatalf("binding not persisted: %+v ok=%v", b, ok)
	}
}

func TestCommandBindKeepsAgentMode(t *testing.T) {
	snd := &fakeSender{}
	store := newFakeStore(Binding{ChatID: "c1", Repo: "/old", Agent: "codex", Mode: "senate"})
	h := NewHandler(store, &fakeEnqueuer{}, snd, noopProgress)
	dir := t.TempDir()

	runCmd(t, h, snd, "c1", "/bind "+dir)
	b, _ := store.For("c1")
	if b.Repo != dir || b.Agent != "codex" || b.Mode != "senate" {
		t.Fatalf("/bind should keep agent/mode: %+v", b)
	}
}

func TestCommandBindEmptyAndBadPath(t *testing.T) {
	snd := &fakeSender{}
	h := NewHandler(newFakeStore(), &fakeEnqueuer{}, snd, noopProgress)

	if got := runCmd(t, h, snd, "c1", "/bind"); !strings.Contains(got, "Usage") {
		t.Fatalf("/bind empty = %q", got)
	}
	if got := runCmd(t, h, snd, "c1", "/bind /no/such/path/xyz123"); !strings.Contains(strings.ToLower(got), "can't use that path") {
		t.Fatalf("/bind bad path = %q", got)
	}
}

func TestCommandBindFileNotDir(t *testing.T) {
	snd := &fakeSender{}
	h := NewHandler(newFakeStore(), &fakeEnqueuer{}, snd, noopProgress)
	dir := t.TempDir()
	file := dir + "/f.txt"
	if err := os.WriteFile(file, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := runCmd(t, h, snd, "c1", "/bind "+file)
	if !strings.Contains(got, "Not a directory") {
		t.Fatalf("/bind file = %q", got)
	}
}

func TestCommandAgentBeforeAndAfterBind(t *testing.T) {
	snd := &fakeSender{}
	store := newFakeStore()
	h := NewHandler(store, &fakeEnqueuer{}, snd, noopProgress)

	got := runCmd(t, h, snd, "c1", "/agent codex")
	if !strings.Contains(got, "Bind a repo first") {
		t.Fatalf("/agent before bind = %q", got)
	}

	_ = store.Set(Binding{ChatID: "c1", Repo: "/r"})
	got = runCmd(t, h, snd, "c1", "/agent codex")
	if !strings.Contains(got, "Agent set to codex") {
		t.Fatalf("/agent after bind = %q", got)
	}
	if b, _ := store.For("c1"); b.Agent != "codex" {
		t.Fatalf("agent not persisted: %+v", b)
	}
}

func TestCommandModeValidAndInvalid(t *testing.T) {
	snd := &fakeSender{}
	store := newFakeStore(Binding{ChatID: "c1", Repo: "/r"})
	h := NewHandler(store, &fakeEnqueuer{}, snd, noopProgress)

	if got := runCmd(t, h, snd, "c1", "/mode nonsense"); !strings.Contains(got, "Usage") {
		t.Fatalf("/mode invalid = %q", got)
	}
	got := runCmd(t, h, snd, "c1", "/mode collab")
	if !strings.Contains(got, "Mode set to collab") {
		t.Fatalf("/mode valid = %q", got)
	}
	if b, _ := store.For("c1"); b.Mode != "collab" {
		t.Fatalf("mode not persisted: %+v", b)
	}
}

func TestCommandModeRequiresBind(t *testing.T) {
	snd := &fakeSender{}
	h := NewHandler(newFakeStore(), &fakeEnqueuer{}, snd, noopProgress)
	got := runCmd(t, h, snd, "c1", "/mode rage")
	if !strings.Contains(got, "Bind a repo first") {
		t.Fatalf("/mode before bind = %q", got)
	}
}

func TestCommandUnbind(t *testing.T) {
	snd := &fakeSender{}
	store := newFakeStore(Binding{ChatID: "c1", Repo: "/r"})
	h := NewHandler(store, &fakeEnqueuer{}, snd, noopProgress)

	got := runCmd(t, h, snd, "c1", "/unbind")
	if !strings.Contains(got, "Unlinked") {
		t.Fatalf("/unbind bound = %q", got)
	}
	if _, ok := store.For("c1"); ok {
		t.Fatal("binding should be deleted")
	}
	got = runCmd(t, h, snd, "c1", "/unbind")
	if !strings.Contains(got, "wasn't linked") {
		t.Fatalf("/unbind unbound = %q", got)
	}
}

func TestCommandCaseInsensitive(t *testing.T) {
	snd := &fakeSender{}
	h := NewHandler(newFakeStore(), &fakeEnqueuer{}, snd, noopProgress)
	got := runCmd(t, h, snd, "c1", "/HELP")
	if !strings.Contains(got, "/bind") {
		t.Fatalf("/HELP = %q", got)
	}
}

func TestCommandBothFormsBehaveIdentically(t *testing.T) {
	store := newFakeStore(Binding{ChatID: "c1", Repo: "/r", Agent: "codex", Mode: "senate"})
	h := NewHandler(store, &fakeEnqueuer{}, &fakeSender{}, noopProgress)

	withSlash := h.Command(context.Background(), "c1", "/status")
	noSlash := h.Command(context.Background(), "c1", "status")
	if withSlash != noSlash {
		t.Fatalf("status forms differ: %q vs %q", withSlash, noSlash)
	}
	for _, want := range []string{"/r", "codex", "senate"} {
		if !strings.Contains(noSlash, want) {
			t.Fatalf("status missing %q: %q", want, noSlash)
		}
	}

	if got := h.Command(context.Background(), "c1", "/help"); got != h.Command(context.Background(), "c1", "help") {
		t.Fatalf("help forms differ")
	}
}

func TestCommandBindNoSlashForm(t *testing.T) {
	store := newFakeStore()
	h := NewHandler(store, &fakeEnqueuer{}, &fakeSender{}, noopProgress)
	dir := t.TempDir()

	got := h.Command(context.Background(), "c1", "bind "+dir)
	if !strings.Contains(got, "Linked") {
		t.Fatalf("no-slash bind = %q", got)
	}
	if b, ok := store.For("c1"); !ok || b.Repo != dir {
		t.Fatalf("binding not persisted: %+v ok=%v", b, ok)
	}
}

func TestCommandDoesNotEnqueueButTaskDoes(t *testing.T) {
	snd := &fakeSender{}
	enq := &fakeEnqueuer{jobID: "j"}
	store := newFakeStore(Binding{ChatID: "c1", Repo: "/r"})
	h := NewHandler(store, enq, snd, noopProgress)

	// command path: no enqueue
	h.Handle(context.Background(), IncomingMessage{MessageID: "cmd", ChatID: "c1", Text: "/status", Mentioned: true, IsGroup: true})
	if enq.count() != 0 {
		t.Fatalf("command enqueued %d", enq.count())
	}
	// normal task: enqueues
	h.Handle(context.Background(), IncomingMessage{MessageID: "task", ChatID: "c1", Text: "do it", Mentioned: true, IsGroup: true})
	if enq.count() != 1 {
		t.Fatalf("task enqueue = %d", enq.count())
	}
}

func TestDedupLine(t *testing.T) {
	last := ""
	cases := []struct {
		line     string
		wantPost bool
		wantLast string
	}{
		{"📦 produced a result", true, "📦 produced a result"},
		{"📦 produced a result", false, "📦 produced a result"}, // consecutive identical collapses
		{"", false, "📦 produced a result"},                    // empty dropped, last unchanged
		{"🔧 working", true, "🔧 working"},
		{"📦 produced a result", true, "📦 produced a result"}, // not consecutive -> posts again
	}
	for i, c := range cases {
		var post bool
		last, post = dedupLine(last, c.line)
		if post != c.wantPost || last != c.wantLast {
			t.Fatalf("case %d dedupLine: post=%v last=%q, want post=%v last=%q", i, post, last, c.wantPost, c.wantLast)
		}
	}
}
