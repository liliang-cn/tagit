# Feishu @Tag Bot Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let a Feishu group `@mention` ROMA to run a coding task, with an ack, live progress, and the final result streamed back into the thread — over the SDK long connection (no public URL).

**Architecture:** New `internal/feishu` module. A long-connection `Bot` receives `im.message.receive_v1`, a pure `handler` decides-to-act (dedup, @-check, binding lookup) and submits a run via an `Enqueuer`, a `Sender` posts to the thread, and a `progress` streamer turns `StreamJobEvents` into throttled thread updates. Reuses the existing queue/run/memory stack. Disabled cleanly when `~/.roma/feishu.json` is absent.

**Tech Stack:** Go 1.25, `github.com/larksuite/oapi-sdk-go/v3` (v3.9.7: `ws`, `event/dispatcher`, `service/im/v1` as `larkim`, root as `lark`), existing `internal/api` (`Submit`, `StreamJobEvents`), `internal/queue`, `internal/events`.

**Spec:** `docs/superpowers/specs/2026-06-28-feishu-tag-bot-design.md`

---

## Reference: verified Feishu SDK v3.9.7 API (do not re-derive)

```go
import (
    lark "github.com/larksuite/oapi-sdk-go/v3"
    "github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
    larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
    larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// API client (for sending):
cli := lark.NewClient(appID, appSecret)
// reply into a thread (reply to the triggering message_id):
req := larkim.NewReplyMessageReqBuilder().
    MessageId(rootMessageID).
    Body(larkim.NewReplyMessageReqBodyBuilder().
        MsgType("text").
        Content(`{"text":"..."}`).   // text content is JSON-encoded
        Build()).
    Build()
resp, err := cli.Im.Message.Reply(ctx, req)   // resp.Success(), resp.Code, resp.Msg

// Long-connection event loop (no public URL, no verification token/encrypt key needed):
handler := dispatcher.NewEventDispatcher("", "").
    OnP2MessageReceiveV1(func(ctx context.Context, e *larkim.P2MessageReceiveV1) error { ... })
wsCli := larkws.NewClient(appID, appSecret,
    larkws.WithEventHandler(handler),
    larkws.WithAutoReconnect(true))
err = wsCli.Start(ctx)  // blocks; reconnects automatically
```

`*larkim.P2MessageReceiveV1` → `e.Event.Message`: `MessageId *string`, `ChatId *string`,
`ChatType *string` ("group"/"p2p"/"topic_group"), `MessageType *string` ("text"…),
`Content *string` (JSON, e.g. `{"text":"@_user_1 do X"}`), `Mentions []*larkim.MentionEvent`
(each has `Key *string` like `@_user_1`, `Name *string`, `Id *larkim.UserId`). `e.Event.Sender` is the sender.

Helper for pointers: `larkcore.StringValue(p)` or just guard nil and deref.

---

## File structure

- `internal/feishu/config.go` — `Config`, `Binding`, `Load(path)`; disabled when file missing.
- `internal/feishu/message.go` — `IncomingMessage` value + `parseTextContent` (extract plain text, strip mention tokens).
- `internal/feishu/sender.go` — `Sender` interface + `larkSender` (Feishu API impl).
- `internal/feishu/handler.go` — `Handler` (deps: `Enqueuer`, `Sender`, `Deduper`); `Handle(ctx, IncomingMessage)`.
- `internal/feishu/progress.go` — `streamProgress(ctx, ...)`: throttle StreamJobEvents → Sender; final formatting.
- `internal/feishu/bot.go` — `Bot`: ws wiring (dispatcher→handler), `Start/Stop`.
- `internal/feishu/*_test.go` — unit tests with fixtures/fakes (no live Feishu, no creds).
- `internal/app/daemon.go` — best-effort `Bot` startup when config present.

**Note on memory scope:** memory is already repo-scoped (sub-project A keys on
`WorkingDir`). Since each group binds to a distinct repo, per-group memory is
achieved via the repo. Threading `chat_id` into `Scope.Channel` is a deferred
fast-follow (only matters if two groups share one repo).

---

## Task 1: Config + Bindings

**Files:** Create `internal/feishu/config.go`; Test `internal/feishu/config_test.go`.

- [ ] **Step 1: Write failing tests**

```go
// internal/feishu/config_test.go
package feishu

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileDisabled(t *testing.T) {
	cfg, enabled, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if enabled || cfg != nil {
		t.Fatalf("missing file should be disabled; got enabled=%v cfg=%v", enabled, cfg)
	}
}

func TestLoadAndBindingLookup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "feishu.json")
	os.WriteFile(path, []byte(`{
	  "app_id":"cli_x","app_secret":"sec",
	  "bindings":[{"chat_id":"oc_1","repo":"/r","agent":"codex","mode":"rage"}]
	}`), 0o600)

	cfg, enabled, err := Load(path)
	if err != nil || !enabled {
		t.Fatalf("Load() enabled=%v err=%v", enabled, err)
	}
	if cfg.AppID != "cli_x" || cfg.AppSecret != "sec" {
		t.Fatalf("creds not parsed: %+v", cfg)
	}
	b, ok := cfg.BindingFor("oc_1")
	if !ok || b.Repo != "/r" || b.Agent != "codex" || b.Mode != "rage" {
		t.Fatalf("BindingFor() = %+v ok=%v", b, ok)
	}
	if _, ok := cfg.BindingFor("oc_unknown"); ok {
		t.Fatal("unknown chat must not resolve")
	}
}

func TestLoadMalformedErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	os.WriteFile(path, []byte("{not json"), 0o600)
	if _, _, err := Load(path); err == nil {
		t.Fatal("malformed config should error")
	}
}
```

- [ ] **Step 2: Run, verify FAIL** — `GOWORK=off go test ./internal/feishu/...` (undefined Load).

- [ ] **Step 3: Implement**

```go
// internal/feishu/config.go
package feishu

import (
	"encoding/json"
	"fmt"
	"os"
)

// Binding maps one Feishu group chat to a repo and run defaults.
type Binding struct {
	ChatID string `json:"chat_id"`
	Repo   string `json:"repo"`
	Agent  string `json:"agent,omitempty"`
	Mode   string `json:"mode,omitempty"`
}

// Config is the on-disk Feishu bot configuration (~/.roma/feishu.json).
type Config struct {
	AppID     string    `json:"app_id"`
	AppSecret string    `json:"app_secret"`
	Bindings  []Binding `json:"bindings"`
}

// Load reads the config. A missing file means the feature is disabled:
// it returns (nil, false, nil). A present-but-broken file returns an error.
func Load(path string) (*Config, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read feishu config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, false, fmt.Errorf("parse feishu config: %w", err)
	}
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return nil, false, fmt.Errorf("feishu config missing app_id/app_secret")
	}
	return &cfg, true, nil
}

// BindingFor returns the binding for a chat id.
func (c *Config) BindingFor(chatID string) (Binding, bool) {
	for _, b := range c.Bindings {
		if b.ChatID == chatID {
			return b, true
		}
	}
	return Binding{}, false
}
```

- [ ] **Step 4: Run, verify PASS** — `GOWORK=off go test ./internal/feishu/...`. gofmt/vet clean.
- [ ] **Step 5: Commit** — `git add internal/feishu/config.go internal/feishu/config_test.go && git commit -m "feishu: add config and per-group bindings"`

---

## Task 2: Incoming message parsing

**Files:** Create `internal/feishu/message.go`; Test `internal/feishu/message_test.go`.

- [ ] **Step 1: Failing tests**

```go
// internal/feishu/message_test.go
package feishu

import "testing"

func TestParseTextContentStripsMentions(t *testing.T) {
	got := parseTextContent(`{"text":"@_user_1  add input validation"}`)
	if got != "add input validation" {
		t.Fatalf("parseTextContent = %q", got)
	}
}

func TestParseTextContentPlain(t *testing.T) {
	if got := parseTextContent(`{"text":"hello world"}`); got != "hello world" {
		t.Fatalf("got %q", got)
	}
}

func TestParseTextContentNonText(t *testing.T) {
	if got := parseTextContent(`{"image_key":"img_x"}`); got != "" {
		t.Fatalf("non-text content should yield empty, got %q", got)
	}
}
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Implement**

```go
// internal/feishu/message.go
package feishu

import (
	"encoding/json"
	"regexp"
	"strings"
)

// IncomingMessage is the normalized form of a received Feishu message.
type IncomingMessage struct {
	MessageID string
	ChatID    string
	ChatType  string // "group" | "p2p" | "topic_group"
	Text      string // mention tokens stripped
	Mentioned bool   // the bot was @mentioned
}

var mentionToken = regexp.MustCompile(`@_user_\d+\s*`)

// parseTextContent extracts plain text from a Feishu text-message Content JSON
// ({"text":"..."}) and strips @mention placeholder tokens. Non-text → "".
func parseTextContent(contentJSON string) string {
	var c struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(contentJSON), &c); err != nil {
		return ""
	}
	return strings.TrimSpace(mentionToken.ReplaceAllString(c.Text, ""))
}
```

- [ ] **Step 4: Run, verify PASS.** gofmt/vet clean.
- [ ] **Step 5: Commit** — `git commit -m "feishu: parse text content and strip mention tokens"`

---

## Task 3: Sender (interface + Feishu API impl)

**Files:** Create `internal/feishu/sender.go`; Test `internal/feishu/sender_test.go`.

- [ ] **Step 1: Failing test** (verifies the text→Content JSON encoding helper; the API call itself is covered by manual integration)

```go
// internal/feishu/sender_test.go
package feishu

import "testing"

func TestTextContentJSONEscapes(t *testing.T) {
	got := textContentJSON(`he said "hi"` + "\n" + "next")
	want := `{"text":"he said \"hi\"\nnext"}`
	if got != want {
		t.Fatalf("textContentJSON = %s, want %s", got, want)
	}
}
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Implement**

```go
// internal/feishu/sender.go
package feishu

import (
	"context"
	"encoding/json"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

// Sender posts text into a Feishu thread, replying to the triggering message.
type Sender interface {
	Reply(ctx context.Context, rootMessageID, text string) error
}

type larkSender struct{ cli *lark.Client }

// NewSender builds a Feishu-API-backed Sender.
func NewSender(appID, appSecret string) Sender {
	return &larkSender{cli: lark.NewClient(appID, appSecret)}
}

func (s *larkSender) Reply(ctx context.Context, rootMessageID, text string) error {
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(rootMessageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType("text").
			Content(textContentJSON(text)).
			Build()).
		Build()
	resp, err := s.cli.Im.Message.Reply(ctx, req)
	if err != nil {
		return fmt.Errorf("feishu reply: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("feishu reply failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// textContentJSON encodes plain text into Feishu's text message content JSON.
func textContentJSON(text string) string {
	b, _ := json.Marshal(struct {
		Text string `json:"text"`
	}{Text: text})
	return string(b)
}
```

- [ ] **Step 4: Run, verify PASS.** `GOWORK=off go build ./internal/feishu/...` (pulls the SDK; run `GOWORK=off go get github.com/larksuite/oapi-sdk-go/v3@v3.9.7` + `go mod tidy` first if not already a dep). gofmt/vet clean.
- [ ] **Step 5: Commit** — `git add internal/feishu/sender.go internal/feishu/sender_test.go go.mod go.sum && git commit -m "feishu: add thread Sender over the official SDK"`

---

## Task 4: Handler (decide → ack → enqueue)

**Files:** Create `internal/feishu/handler.go`; Test `internal/feishu/handler_test.go`.

- [ ] **Step 1: Failing tests with fakes**

```go
// internal/feishu/handler_test.go
package feishu

import (
	"context"
	"testing"
)

type fakeSender struct{ replies []string }

func (f *fakeSender) Reply(_ context.Context, _ string, text string) error {
	f.replies = append(f.replies, text)
	return nil
}

type fakeEnqueuer struct {
	gotRepo, gotPrompt, gotAgent, gotMode string
	jobID                                 string
	called                                int
}

func (f *fakeEnqueuer) Submit(_ context.Context, a SubmitArgs) (string, error) {
	f.called++
	f.gotRepo, f.gotPrompt, f.gotAgent, f.gotMode = a.Repo, a.Prompt, a.Agent, a.Mode
	return f.jobID, nil
}

func newTestHandler(cfg *Config, snd Sender, enq Enqueuer) *Handler {
	return NewHandler(cfg, enq, snd, func(string) string { return "" }) // progress no-op
}

func TestHandleBoundChatEnqueuesAndAcks(t *testing.T) {
	cfg := &Config{Bindings: []Binding{{ChatID: "oc_1", Repo: "/r", Agent: "codex", Mode: "rage"}}}
	snd := &fakeSender{}
	enq := &fakeEnqueuer{jobID: "job_1"}
	h := newTestHandler(cfg, snd, enq)

	h.Handle(context.Background(), IncomingMessage{
		MessageID: "om_1", ChatID: "oc_1", ChatType: "group", Mentioned: true,
		Text: "add validation",
	})

	if enq.called != 1 || enq.gotRepo != "/r" || enq.gotPrompt != "add validation" || enq.gotAgent != "codex" {
		t.Fatalf("submit wrong: %+v", enq)
	}
	if len(snd.replies) == 0 {
		t.Fatal("expected an ack reply")
	}
}

func TestHandleUnboundChatRepliesAndSkips(t *testing.T) {
	cfg := &Config{}
	snd := &fakeSender{}
	enq := &fakeEnqueuer{}
	newTestHandler(cfg, snd, enq).Handle(context.Background(), IncomingMessage{
		MessageID: "om_2", ChatID: "oc_none", ChatType: "group", Mentioned: true, Text: "hi",
	})
	if enq.called != 0 {
		t.Fatal("unbound chat must not enqueue")
	}
	if len(snd.replies) == 0 {
		t.Fatal("unbound chat should get a 'not linked' reply")
	}
}

func TestHandleDedupesByMessageID(t *testing.T) {
	cfg := &Config{Bindings: []Binding{{ChatID: "oc_1", Repo: "/r"}}}
	enq := &fakeEnqueuer{jobID: "j"}
	h := newTestHandler(cfg, &fakeSender{}, enq)
	msg := IncomingMessage{MessageID: "dup", ChatID: "oc_1", ChatType: "group", Mentioned: true, Text: "x"}
	h.Handle(context.Background(), msg)
	h.Handle(context.Background(), msg)
	if enq.called != 1 {
		t.Fatalf("duplicate message_id ran %d times, want 1", enq.called)
	}
}

func TestHandleIgnoresNonGroupOrNoText(t *testing.T) {
	cfg := &Config{Bindings: []Binding{{ChatID: "oc_1", Repo: "/r"}}}
	enq := &fakeEnqueuer{}
	h := newTestHandler(cfg, &fakeSender{}, enq)
	h.Handle(context.Background(), IncomingMessage{MessageID: "a", ChatID: "oc_1", ChatType: "p2p", Mentioned: true, Text: "x"})
	h.Handle(context.Background(), IncomingMessage{MessageID: "b", ChatID: "oc_1", ChatType: "group", Mentioned: true, Text: ""})
	if enq.called != 0 {
		t.Fatalf("non-group / empty-text must not enqueue; called=%d", enq.called)
	}
}
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Implement**

```go
// internal/feishu/handler.go
package feishu

import (
	"context"
	"log"
	"sync"
)

// SubmitArgs is the run request the handler hands to the Enqueuer.
type SubmitArgs struct {
	Repo   string
	Prompt string
	Agent  string
	Mode   string
}

// Enqueuer submits a run and returns its job id. Implemented over api.Client.
type Enqueuer interface {
	Submit(ctx context.Context, args SubmitArgs) (jobID string, err error)
}

// ProgressFunc starts streaming progress for a job into its thread and returns
// nothing meaningful; it is launched in a goroutine. Provided by the Bot wiring.
type ProgressFunc func(jobID string)

// Handler turns a received message into an acked, deduped run submission.
type Handler struct {
	cfg      *Config
	enq      Enqueuer
	snd      Sender
	progress ProgressFunc

	mu   sync.Mutex
	seen map[string]struct{}
}

func NewHandler(cfg *Config, enq Enqueuer, snd Sender, progress ProgressFunc) *Handler {
	return &Handler{cfg: cfg, enq: enq, snd: snd, progress: progress, seen: map[string]struct{}{}}
}

func (h *Handler) Handle(ctx context.Context, msg IncomingMessage) {
	if msg.ChatType != "group" || !msg.Mentioned || msg.Text == "" {
		return
	}
	if h.dup(msg.MessageID) {
		return
	}
	binding, ok := h.cfg.BindingFor(msg.ChatID)
	if !ok {
		h.reply(ctx, msg.MessageID, "This group isn't linked to a repo yet. Ask an admin to add a binding in ~/.roma/feishu.json.")
		return
	}
	h.reply(ctx, msg.MessageID, "收到，开始干 🛠️")

	mode := binding.Mode
	if mode == "" {
		mode = "rage"
	}
	jobID, err := h.enq.Submit(ctx, SubmitArgs{
		Repo: binding.Repo, Prompt: msg.Text, Agent: binding.Agent, Mode: mode,
	})
	if err != nil {
		h.reply(ctx, msg.MessageID, "Failed to start the run: "+err.Error())
		return
	}
	if h.progress != nil {
		go h.progress(jobID)
	}
}

func (h *Handler) dup(id string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.seen[id]; ok {
		return true
	}
	h.seen[id] = struct{}{}
	return false
}

func (h *Handler) reply(ctx context.Context, rootID, text string) {
	if err := h.snd.Reply(ctx, rootID, text); err != nil {
		log.Printf("feishu: reply failed (ignored): %v", err)
	}
}
```

(Note: `ProgressFunc` signature in tests is `func(string) string` only to satisfy the no-op in `newTestHandler`; fix the test helper to `func(string){}` matching `ProgressFunc`. Use: `func newTestHandler(...) *Handler { return NewHandler(cfg, enq, snd, func(string) {}) }`.)

- [ ] **Step 4: Run, verify PASS** (correct the `newTestHandler` no-op to `func(string) {}`). gofmt/vet clean.
- [ ] **Step 5: Commit** — `git commit -m "feishu: add dedup + binding-aware message handler"`

---

## Task 5: Progress streamer

**Files:** Create `internal/feishu/progress.go`; Test `internal/feishu/progress_test.go`.

The streamer consumes `events.Record` from a channel (the Bot wires this to
`api.Client.StreamJobEvents`), throttles, and posts to the thread.

- [ ] **Step 1: Failing tests**

```go
// internal/feishu/progress_test.go
package feishu

import (
	"context"
	"testing"
	"time"

	"github.com/liliang-cn/roma/internal/events"
)

func TestStreamProgressThrottlesAndPostsFinal(t *testing.T) {
	snd := &fakeSender{}
	ch := make(chan events.Record, 8)
	// three quick events; throttle should coalesce, but terminal always posts.
	ch <- events.Record{Type: "node.started", Payload: map[string]any{"phase": "worker"}}
	ch <- events.Record{Type: "node.progress", Payload: map[string]any{"phase": "worker"}}
	ch <- events.Record{Type: "session.completed", Payload: map[string]any{"status": "succeeded"}}
	close(ch)

	streamProgress(context.Background(), snd, "om_root", ch,
		0, // throttle=0 in test → every event eligible, but final must appear
		func() time.Time { return time.Unix(0, 0) })

	if len(snd.replies) == 0 {
		t.Fatal("expected progress/final replies")
	}
	last := snd.replies[len(snd.replies)-1]
	if !contains(last, "succeeded") {
		t.Fatalf("final reply should mention status, got %q", last)
	}
}

func contains(s, sub string) bool { return len(s) >= len(sub) && (indexOf(s, sub) >= 0) }
func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Implement**

```go
// internal/feishu/progress.go
package feishu

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liliang-cn/roma/internal/events"
)

// streamProgress reads job events, posts throttled progress into the thread, and
// always posts a final summary on a terminal event. throttle is the minimum gap
// between non-terminal posts; now() is injectable for tests.
func streamProgress(ctx context.Context, snd Sender, rootMessageID string,
	ch <-chan events.Record, throttle time.Duration, now func() time.Time) {

	var last time.Time
	for rec := range ch {
		if terminal(rec.Type) {
			snd.Reply(ctx, rootMessageID, finalText(rec))
			return
		}
		if throttle > 0 && now().Sub(last) < throttle {
			continue
		}
		last = now()
		if line := progressText(rec); line != "" {
			snd.Reply(ctx, rootMessageID, line)
		}
	}
}

func terminal(t events.Type) bool {
	switch string(t) {
	case "session.completed", "session.failed", "job.completed", "job.failed":
		return true
	}
	return false
}

func progressText(rec events.Record) string {
	phase := payloadString(rec.Payload, "phase", "state")
	if phase == "" {
		return ""
	}
	return "… " + phase
}

func finalText(rec events.Record) string {
	status := payloadString(rec.Payload, "status", "state")
	if status == "" {
		status = strings.TrimPrefix(string(rec.Type), "session.")
	}
	files := payloadString(rec.Payload, "changed_files", "files")
	out := fmt.Sprintf("✅ Done — %s", status)
	if strings.Contains(status, "fail") {
		out = fmt.Sprintf("❌ %s", status)
	}
	if files != "" {
		out += "\nChanged: " + files
	}
	return out
}

func payloadString(p map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := p[k].(string); ok && v != "" {
			return v
		}
	}
	return ""
}
```

- [ ] **Step 4: Run, verify PASS.** gofmt/vet clean.
- [ ] **Step 5: Commit** — `git commit -m "feishu: stream throttled run progress into the thread"`

---

## Task 6: Bot wiring + daemon startup

**Files:** Create `internal/feishu/bot.go`; Modify `internal/app/daemon.go`; Test `internal/feishu/bot_test.go`.

This task wires the SDK long connection to the handler, adapts `api.Client` to
`Enqueuer`, and connects `progress` to `StreamJobEvents`. The SDK loop itself is
covered by manual integration; unit-test the event→IncomingMessage adapter.

- [ ] **Step 1: Failing test for the event adapter**

```go
// internal/feishu/bot_test.go
package feishu

import (
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func strptr(s string) *string { return &s }

func TestToIncomingMessage(t *testing.T) {
	ev := &larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{
		Message: &larkim.EventMessage{
			MessageId:   strptr("om_1"),
			ChatId:      strptr("oc_1"),
			ChatType:    strptr("group"),
			MessageType: strptr("text"),
			Content:     strptr(`{"text":"@_user_1 do X"}`),
			Mentions:    []*larkim.MentionEvent{{Key: strptr("@_user_1")}},
		},
	}}
	msg := toIncomingMessage(ev)
	if msg.MessageID != "om_1" || msg.ChatID != "oc_1" || msg.ChatType != "group" {
		t.Fatalf("bad ids: %+v", msg)
	}
	if !msg.Mentioned || msg.Text != "do X" {
		t.Fatalf("bad mention/text: %+v", msg)
	}
}
```

- [ ] **Step 2: Run, verify FAIL.**

- [ ] **Step 3: Implement bot.go**

```go
// internal/feishu/bot.go
package feishu

import (
	"context"
	"log"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// Bot runs the Feishu long-connection event loop and routes messages to a Handler.
type Bot struct {
	cfg     *Config
	handler *Handler
}

// NewBot builds a Bot. enq submits runs; progress streams a job to its thread.
func NewBot(cfg *Config, enq Enqueuer, progress ProgressFunc) *Bot {
	snd := NewSender(cfg.AppID, cfg.AppSecret)
	return &Bot{cfg: cfg, handler: NewHandler(cfg, enq, snd, progress)}
}

// Start blocks, running the long connection until ctx is cancelled. Reconnects automatically.
func (b *Bot) Start(ctx context.Context) error {
	d := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, e *larkim.P2MessageReceiveV1) error {
			b.handler.Handle(ctx, toIncomingMessage(e))
			return nil
		})
	cli := larkws.NewClient(b.cfg.AppID, b.cfg.AppSecret,
		larkws.WithEventHandler(d),
		larkws.WithAutoReconnect(true))
	log.Printf("feishu: starting long-connection bot (app=%s, bindings=%d)", b.cfg.AppID, len(b.cfg.Bindings))
	return cli.Start(ctx)
}

func toIncomingMessage(e *larkim.P2MessageReceiveV1) IncomingMessage {
	var m IncomingMessage
	if e == nil || e.Event == nil || e.Event.Message == nil {
		return m
	}
	msg := e.Event.Message
	m.MessageID = deref(msg.MessageId)
	m.ChatID = deref(msg.ChatId)
	m.ChatType = deref(msg.ChatType)
	m.Mentioned = len(msg.Mentions) > 0
	if deref(msg.MessageType) == "text" {
		m.Text = parseTextContent(deref(msg.Content))
	}
	return m
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
```

- [ ] **Step 4: Run, verify PASS.** gofmt/vet clean.

- [ ] **Step 5: Wire the Enqueuer + progress adapters and daemon startup**

Add an `api.Client`-backed Enqueuer and a progress adapter in `internal/feishu/adapter.go`:

```go
// internal/feishu/adapter.go
package feishu

import (
	"context"
	"time"

	"github.com/liliang-cn/roma/internal/api"
	"github.com/liliang-cn/roma/internal/events"
	"github.com/liliang-cn/roma/internal/run"
)

// apiEnqueuer adapts *api.Client to Enqueuer.
type apiEnqueuer struct{ client *api.Client }

func NewAPIEnqueuer(c *api.Client) Enqueuer { return &apiEnqueuer{client: c} }

func (e *apiEnqueuer) Submit(ctx context.Context, a SubmitArgs) (string, error) {
	resp, err := e.client.Submit(ctx, api.SubmitRequest{
		Prompt:       a.Prompt,
		Mode:         run.NormalizeMode(a.Mode),
		StarterAgent: a.Agent,
		WorkingDir:   a.Repo,
	})
	if err != nil {
		return "", err
	}
	return resp.JobID, nil
}

// NewProgressFunc returns a ProgressFunc that streams a job's events into rootMessageID's thread.
func NewProgressFunc(c *api.Client, snd Sender, rootMessageID string) ProgressFunc {
	return func(jobID string) {
		ch := make(chan events.Record, 32)
		go func() {
			_ = c.StreamJobEvents(context.Background(), jobID, ch)
			close(ch)
		}()
		streamProgress(context.Background(), snd, rootMessageID, ch, 5*time.Second, time.Now)
	}
}
```

IMPORTANT: `progress` needs the per-message `rootMessageID`, but `ProgressFunc`
only takes `jobID`. Resolve by having the Handler capture the rootID: change
`ProgressFunc` to `func(jobID, rootMessageID string)` and have the handler pass
`msg.MessageID`. Update `bot.go`/handler/tests accordingly (the Bot constructs the
progress closure from `api.Client` + its own `Sender`). Keep the Sender shared
between Handler and progress.

Concretely: `ProgressFunc = func(jobID, rootMessageID string)`; in `Handle`,
`go h.progress(jobID, msg.MessageID)`. In the Bot, build:
```go
progress := func(jobID, rootID string) {
	NewProgressFunc(apiClient, snd, rootID)(jobID)
}
```
where `snd` is the same `NewSender(cfg.AppID, cfg.AppSecret)` the Bot already made
(expose it from NewBot or reconstruct).

Then in `internal/app/daemon.go`, after the run service + api client exist, start the bot best-effort:
```go
feishuCfg, enabled, err := feishu.Load(filepath.Join(romapath.HomeDir(), "feishu.json"))
if err != nil {
	log.Printf("feishu: disabled (%v)", err)
} else if enabled {
	bot := feishu.NewBot(feishuCfg, feishu.NewAPIEnqueuer(apiClient), /* progress wired in NewBot */)
	go func() {
		if err := bot.Start(context.Background()); err != nil {
			log.Printf("feishu: bot stopped: %v", err)
		}
	}()
}
```
(Use the daemon's existing `api.Client` instance — the one the daemon already builds for self-calls; if none exists, construct `api.NewClientForControlDir(workDir, romapath.HomeDir())`.)

- [ ] **Step 6: Build, vet, test, commit**

Run: `GOWORK=off go build ./... && GOWORK=off go vet ./internal/feishu/... ./internal/app/... && GOWORK=off go test ./internal/feishu/...`
```bash
git add internal/feishu/ internal/app/daemon.go go.mod go.sum
git commit -m "feishu: wire long-connection bot, api enqueuer, progress, daemon startup"
```

---

## Task 7: Manual integration verification (no code)

- [ ] Publish the "tagit" app (版本管理与发布) so config takes effect.
- [ ] Ensure permissions: `im:message` (send) + `im:message.group_at_msg:readonly` (receive @), and event `im.message.receive_v1` via **long connection**.
- [ ] Write `~/.roma/feishu.json` with app_id/app_secret and one binding `{chat_id, repo}` (get chat_id from the group; e.g. via the bot logging received ChatIds, or 飞书 admin).
- [ ] Start `romad`; add the bot to the group; `@tagit do something small`.
- [ ] Confirm: ack appears in-thread, progress updates stream, final result posts. Capture the thread as evidence.

---

## Self-Review

- **Spec coverage:** long-connection transport (Task 6), per-group bindings (Task 1), @-detection + dedup + prompt extraction (Tasks 2,4,6), ack + enqueue reusing queue/run (Task 4 + adapter), real-time throttled progress + final (Task 5 + adapter), config-absent disable + daemon best-effort startup (Tasks 1,6), tests via fixtures/fakes without creds (all). Group-only/no-cards/no-multi-IM honored. `Scope.Channel=chat_id` is noted as a deferred fast-follow (memory works via repo scope meanwhile) — called out in File Structure, not silently dropped.
- **Placeholder scan:** none. The one cross-task wrinkle (ProgressFunc needs rootMessageID) is resolved explicitly in Task 6 Step 5 (signature `func(jobID, rootMessageID string)`); ensure Task 4's `ProgressFunc` type and `Handle` call use that 2-arg form when implementing (the Task 4 snippet shows 1-arg for the isolated test; reconcile to 2-arg — the test helper passes a 2-arg no-op `func(string, string){}`).
- **Type consistency:** `Config`/`Binding`/`BindingFor`, `IncomingMessage`, `Sender.Reply`, `Enqueuer.Submit(SubmitArgs)`, `Handler`/`NewHandler`, `streamProgress`, `Bot`/`NewBot`, `toIncomingMessage`, `NewAPIEnqueuer`/`NewProgressFunc` are consistent across tasks. **Action for implementer:** settle `ProgressFunc` as `func(jobID, rootMessageID string)` from Task 4 onward (update the Task 4 test no-op to `func(string, string) {}`).
```
