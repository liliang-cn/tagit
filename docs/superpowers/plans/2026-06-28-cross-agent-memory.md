# Cross-Agent Persistent Memory Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a CortexDB-backed, key-free (lexical), cross-agent memory layer to TagIt, wired into rage runs (auto-inject + record) and exposed over MCP.

**Architecture:** A new `internal/memory` service module defines a `Memory` port (`Recall`/`Record`/`Note`). The default implementation wraps agent-go's `memory.Service` over a CortexDB store in lexical mode (nil LLM + nil embedder). `internal/run` recalls relevant memory before a rage run and records the outcome after. A thin MCP server exposes `memory_recall`/`memory_note`. Memory is advisory and best-effort — failures never fail a run.

**Tech Stack:** Go 1.25, `github.com/liliang-cn/agent-go/v2` (pkg/memory, pkg/store, pkg/domain), CortexDB (pure-Go SQLite, lexical FTS5), existing TagIt `internal/run` + event store.

**Spec:** `docs/superpowers/specs/2026-06-28-cross-agent-memory-design.md`

---

## Reference: verified agent-go API (do not re-derive)

```go
import (
    agmemory "github.com/liliang-cn/agent-go/v2/pkg/memory"
    agstore  "github.com/liliang-cn/agent-go/v2/pkg/store"
    agdomain "github.com/liliang-cn/agent-go/v2/pkg/domain"
)

st, err := agstore.NewCortexMemoryStore("/path/cortex.db") // returns *agstore.MemoryStore (a domain.MemoryStore)
svc := agmemory.NewService(st, nil /*llm*/, nil /*embedder*/, agmemory.DefaultConfig()) // lexical
err = svc.Add(ctx, &agdomain.Memory{                 // ID/CreatedAt auto-filled if empty
    ScopeID: repoKey, Type: agdomain.MemoryTypeObservation,
    Content: "...", Tags: []string{repoKey}, Importance: 0.5,
    Metadata: map[string]interface{}{"agent": "codex"},
})
hits, err := svc.Search(ctx, query, topK)            // []*agdomain.MemoryWithScore { *Memory; Score }
```

`agdomain.MemoryTypeObservation == "observation"` (episodic run records); `agdomain.MemoryTypeFact == "fact"` (durable notes).

---

## Task 1: Memory port, types, and no-op implementation

**Files:**
- Create: `internal/memory/memory.go`
- Create: `internal/memory/nop.go`
- Test: `internal/memory/memory_test.go`

- [ ] **Step 1: Write the failing test**

```go
// internal/memory/memory_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./internal/memory/...`
Expected: FAIL — `undefined: Memory`, `undefined: Nop`.

- [ ] **Step 3: Write the port and types**

```go
// internal/memory/memory.go
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

// Memory is TagIt's advisory, best-effort, cross-agent memory port.
type Memory interface {
	Recall(ctx context.Context, scope Scope, query string, limit int) (Recollection, error)
	Record(ctx context.Context, rec RunRecord) error
	Note(ctx context.Context, scope Scope, fact string, tags []string) error
}
```

```go
// internal/memory/nop.go
package memory

import "context"

type nopMemory struct{}

// Nop returns a Memory that stores nothing and recalls nothing. Used when memory
// is disabled or the backing engine fails to initialize.
func Nop() Memory { return nopMemory{} }

func (nopMemory) Recall(context.Context, Scope, string, int) (Recollection, error) {
	return Recollection{}, nil
}
func (nopMemory) Record(context.Context, RunRecord) error           { return nil }
func (nopMemory) Note(context.Context, Scope, string, []string) error { return nil }
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOWORK=off go test ./internal/memory/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/memory/memory.go internal/memory/nop.go internal/memory/memory_test.go
git commit -m "memory: add cross-agent memory port and no-op implementation"
```

---

## Task 2: agent-go backed implementation — Record + Recall round-trip (lexical)

**Files:**
- Modify: `go.mod` (add agent-go; bump `go` directive to 1.25)
- Create: `internal/memory/agentgo.go`
- Test: `internal/memory/agentgo_test.go`

- [ ] **Step 1: Add the dependency and bump Go**

Run:
```bash
GOWORK=off go get github.com/liliang-cn/agent-go/v2@v2.95.0
GOWORK=off go mod tidy
```
Edit `go.mod` so the directive reads `go 1.25.0` if `go get` did not already raise it. Verify `go 1.25` is the active toolchain: `go version` (Go auto-fetches 1.25 if needed).

- [ ] **Step 2: Write the failing test (lexical round-trip, no embedder, no network)**

```go
// internal/memory/agentgo_test.go
package memory

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestMemory(t *testing.T) Memory {
	t.Helper()
	mem, err := NewAgentGo(filepath.Join(t.TempDir(), "cortex.db"))
	if err != nil {
		t.Fatalf("NewAgentGo() error = %v", err)
	}
	return mem
}

func TestAgentGoRecordThenRecall(t *testing.T) {
	mem := newTestMemory(t)
	ctx := context.Background()
	scope := Scope{Repo: "/repo/alpha"}

	if err := mem.Record(ctx, RunRecord{
		Scope: scope, SessionID: "s1", TaskID: "t1", Agent: "codex", Mode: "rage",
		Prompt: "add input validation to the registration handler",
		ResultSummary: "added validation and tests", ChangedPaths: []string{"handler.go"},
		Verdict: "complete", Success: true, OccurredAt: time.Unix(1000, 0),
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}

	rec, err := mem.Recall(ctx, scope, "registration handler validation", 5)
	if err != nil {
		t.Fatalf("Recall() error = %v", err)
	}
	if len(rec.Episodes) == 0 {
		t.Fatalf("Recall() returned no episodes, want >=1")
	}
	if !strings.Contains(rec.ContextText, "validation") {
		t.Fatalf("ContextText = %q, want it to mention the past run", rec.ContextText)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `GOWORK=off go test ./internal/memory/ -run TestAgentGoRecordThenRecall -v`
Expected: FAIL — `undefined: NewAgentGo`.

- [ ] **Step 4: Implement the agent-go backed memory**

```go
// internal/memory/agentgo.go
package memory

import (
	"context"
	"fmt"
	"strings"
	"time"

	agdomain "github.com/liliang-cn/agent-go/v2/pkg/domain"
	agmemory "github.com/liliang-cn/agent-go/v2/pkg/memory"
	agstore "github.com/liliang-cn/agent-go/v2/pkg/store"
)

type agentGoMemory struct {
	svc *agmemory.Service
}

// NewAgentGo opens a CortexDB-backed memory at dbPath in lexical mode (no LLM,
// no embedder — no API key required).
func NewAgentGo(dbPath string) (Memory, error) {
	st, err := agstore.NewCortexMemoryStore(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open cortex memory store: %w", err)
	}
	svc := agmemory.NewService(st, nil, nil, agmemory.DefaultConfig())
	return &agentGoMemory{svc: svc}, nil
}

func (m *agentGoMemory) Record(ctx context.Context, rec RunRecord) error {
	content := renderRunRecord(rec)
	return m.svc.Add(ctx, &agdomain.Memory{
		ScopeID:    rec.Scope.Repo,
		Type:       agdomain.MemoryTypeObservation,
		Content:    content,
		Tags:       []string{rec.Scope.Repo},
		Importance: 0.5,
		Metadata: map[string]interface{}{
			"agent":   rec.Agent,
			"mode":    rec.Mode,
			"success": rec.Success,
			"kind":    "run",
		},
	})
}

func (m *agentGoMemory) Note(ctx context.Context, scope Scope, fact string, tags []string) error {
	fact = strings.TrimSpace(fact)
	if fact == "" {
		return nil
	}
	return m.svc.Add(ctx, &agdomain.Memory{
		ScopeID:    scope.Repo,
		Type:       agdomain.MemoryTypeFact,
		Content:    fact,
		Tags:       append([]string{scope.Repo}, tags...),
		Importance: 0.7,
		Metadata:   map[string]interface{}{"kind": "note"},
	})
}

func (m *agentGoMemory) Recall(ctx context.Context, scope Scope, query string, limit int) (Recollection, error) {
	if limit <= 0 {
		limit = 5
	}
	hits, err := m.svc.Search(ctx, query, limit*3) // over-fetch, then scope-filter
	if err != nil {
		return Recollection{}, fmt.Errorf("search memory: %w", err)
	}
	var out Recollection
	for _, h := range hits {
		if h == nil || h.Memory == nil || h.ScopeID != scope.Repo {
			continue
		}
		switch h.Type {
		case agdomain.MemoryTypeFact:
			out.Knowledge = append(out.Knowledge, Fact{Text: h.Content, Tags: h.Tags})
		default:
			out.Episodes = append(out.Episodes, Episode{
				Summary:    h.Content,
				Agent:      metaString(h.Metadata, "agent"),
				Mode:       metaString(h.Metadata, "mode"),
				Success:    metaBool(h.Metadata, "success"),
				OccurredAt: h.CreatedAt,
			})
		}
		if len(out.Episodes)+len(out.Knowledge) >= limit {
			break
		}
	}
	out.ContextText = renderRecollection(out)
	return out, nil
}

func renderRunRecord(rec RunRecord) string {
	var b strings.Builder
	status := "failed"
	if rec.Success {
		status = "succeeded"
	}
	fmt.Fprintf(&b, "Past run (%s, %s, %s): %s\n", rec.Agent, rec.Mode, status, rec.Prompt)
	if rec.ResultSummary != "" {
		fmt.Fprintf(&b, "Result: %s\n", rec.ResultSummary)
	}
	if len(rec.ChangedPaths) > 0 {
		fmt.Fprintf(&b, "Changed: %s\n", strings.Join(rec.ChangedPaths, ", "))
	}
	if rec.Verdict != "" {
		fmt.Fprintf(&b, "Verdict: %s\n", rec.Verdict)
	}
	return strings.TrimSpace(b.String())
}

func renderRecollection(r Recollection) string {
	if len(r.Episodes) == 0 && len(r.Knowledge) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Memory context (from past runs in this repo; advisory, may be stale):\n")
	for _, f := range r.Knowledge {
		fmt.Fprintf(&b, "- Known: %s\n", f.Text)
	}
	for _, e := range r.Episodes {
		fmt.Fprintf(&b, "- %s\n", strings.ReplaceAll(e.Summary, "\n", " "))
	}
	return strings.TrimSpace(b.String())
}

func metaString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func metaBool(m map[string]interface{}, key string) bool {
	v, _ := m[key].(bool)
	return v
}

var _ = time.Now
```

- [ ] **Step 5: Run test to verify it passes**

Run: `GOWORK=off go test ./internal/memory/ -run TestAgentGoRecordThenRecall -v`
Expected: PASS. If `Search` errors on the nil embedder, switch the search call to the store's text-only path per agent-go (`m.svc.Search` should fall back to FTS; if not, open the store via `agstore.NewCortexMemoryStore` and call its text search directly) — verify against `pkg/store/memory_cortexdb_test.go`.

- [ ] **Step 6: Remove the `var _ = time.Now` shim if unused, run vet, commit**

Run: `GOWORK=off go vet ./internal/memory/... && gofmt -l internal/memory`
```bash
git add go.mod go.sum internal/memory/agentgo.go internal/memory/agentgo_test.go
git commit -m "memory: add agent-go/CortexDB lexical backend with record+recall"
```

---

## Task 3: Cross-agent recall, Note, and empty-repo safety

**Files:**
- Test: `internal/memory/agentgo_test.go` (add cases)

- [ ] **Step 1: Write the failing tests**

```go
// append to internal/memory/agentgo_test.go
func TestAgentGoRecallIsCrossAgent(t *testing.T) {
	mem := newTestMemory(t)
	ctx := context.Background()
	scope := Scope{Repo: "/repo/beta"}

	if err := mem.Record(ctx, RunRecord{
		Scope: scope, Agent: "codex", Mode: "rage",
		Prompt: "refactor the payment module", Success: true, OccurredAt: time.Unix(1, 0),
	}); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	// A later run by a DIFFERENT agent (claude) must see codex's memory.
	rec, err := mem.Recall(ctx, scope, "payment module refactor", 5)
	if err != nil {
		t.Fatalf("Recall() error = %v", err)
	}
	if len(rec.Episodes) == 0 || rec.Episodes[0].Agent != "codex" {
		t.Fatalf("cross-agent recall failed: %#v", rec.Episodes)
	}
}

func TestAgentGoNoteThenRecall(t *testing.T) {
	mem := newTestMemory(t)
	ctx := context.Background()
	scope := Scope{Repo: "/repo/gamma"}

	if err := mem.Note(ctx, scope, "This repo builds with GOWORK=off", []string{"build"}); err != nil {
		t.Fatalf("Note() error = %v", err)
	}
	rec, err := mem.Recall(ctx, scope, "how to build GOWORK", 5)
	if err != nil {
		t.Fatalf("Recall() error = %v", err)
	}
	if len(rec.Knowledge) == 0 || !strings.Contains(rec.Knowledge[0].Text, "GOWORK") {
		t.Fatalf("note recall failed: %#v", rec.Knowledge)
	}
}

func TestAgentGoRecallEmptyRepoReturnsEmpty(t *testing.T) {
	mem := newTestMemory(t)
	rec, err := mem.Recall(context.Background(), Scope{Repo: "/repo/none"}, "anything", 5)
	if err != nil {
		t.Fatalf("Recall() error = %v", err)
	}
	if len(rec.Episodes) != 0 || len(rec.Knowledge) != 0 || rec.ContextText != "" {
		t.Fatalf("expected empty recollection, got %#v", rec)
	}
}
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `GOWORK=off go test ./internal/memory/ -v`
Expected: PASS (the Task 2 implementation already satisfies these — they pin cross-agent and note behavior). If `TestAgentGoRecallEmptyRepoReturnsEmpty` fails because Search returns other repos' rows, confirm the `h.ScopeID != scope.Repo` filter is present.

- [ ] **Step 3: Commit**

```bash
git add internal/memory/agentgo_test.go
git commit -m "memory: pin cross-agent recall, notes, and empty-repo safety"
```

---

## Task 4: Wire memory into rage runs (auto-inject + record) with audit events

**Files:**
- Modify: `internal/run/service.go` (add `Memory` field; recall+inject+record in `runRageDirect`)
- Modify: `internal/run/service.go` (`buildRageWorkerPrompt` — accept an optional memory block)
- Test: `internal/run/service_test.go` (fake Memory; assert inject + record)

- [ ] **Step 1: Write the failing test with a fake Memory**

```go
// append to internal/run/service_test.go
type fakeMemory struct {
	injected string
	recorded int
}

func (f *fakeMemory) Recall(_ context.Context, _ memory.Scope, _ string, _ int) (memory.Recollection, error) {
	return memory.Recollection{ContextText: "Memory context: previously added validation to handler.go"}, nil
}
func (f *fakeMemory) Record(_ context.Context, _ memory.RunRecord) error { f.recorded++; return nil }
func (f *fakeMemory) Note(_ context.Context, _ memory.Scope, _ string, _ []string) error { return nil }

func TestRageInjectsAndRecordsMemory(t *testing.T) {
	fm := &fakeMemory{}
	// Build the run.Service the same way existing rage tests do, then set memory:
	svc := newTestRageService(t) // existing helper used by rage tests
	svc.Memory = fm

	// Run a one-round rage task whose fake agent emits a foreman-complete review
	// (reuse the existing rage test scaffold/agent script).
	_ = runOneRoundRage(t, svc) // existing helper that drives a rage run to completion

	if fm.recorded == 0 {
		t.Fatalf("expected Record to be called after a rage run")
	}
	// Worker prompt should have carried the injected memory context.
	if got := lastWorkerPrompt(t, svc); !strings.Contains(got, "Memory context") {
		t.Fatalf("worker prompt missing injected memory: %q", got)
	}
}
```

Note: `newTestRageService`, `runOneRoundRage`, and `lastWorkerPrompt` mirror the helpers/patterns already in `service_test.go` (see `TestRunRageDirect`-style tests). If they do not exist as named, inline the existing rage test scaffold and capture the worker prompt via the fake supervisor already used there.

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./internal/run/ -run TestRageInjectsAndRecordsMemory -v`
Expected: FAIL — `svc.Memory` undefined.

- [ ] **Step 3: Add the Memory field and default it to Nop**

In `internal/run/service.go`, add to the `Service` struct:
```go
// Memory is the advisory cross-agent memory layer. Defaults to memory.Nop().
Memory memory.Memory
```
Import `"github.com/liliang-cn/tagit/internal/memory"`. In the `Service` constructor (`NewService`/wherever the struct is built), default it:
```go
if svc.Memory == nil {
	svc.Memory = memory.Nop()
}
```

- [ ] **Step 4: Recall + inject before the first worker prompt in `runRageDirect`**

Before the round loop builds `workerPrompt`, add:
```go
scope := memory.Scope{Repo: filepath.Clean(req.WorkingDir)}
memCtx := ""
if rec, err := s.recallMemory(ctx, scope, req.Prompt); err == nil {
	memCtx = rec.ContextText
}
```
Add a helper that is best-effort and bounded:
```go
func (s *Service) recallMemory(ctx context.Context, scope memory.Scope, query string) (memory.Recollection, error) {
	rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rec, err := s.Memory.Recall(rctx, scope, query, 5)
	if err != nil {
		log.Printf("[memory] recall failed scope=%s: %v", scope.Repo, err)
		return memory.Recollection{}, err
	}
	if rec.ContextText != "" {
		s.appendMemoryRecalledEvent(ctx, scope, len(rec.Episodes), len(rec.Knowledge))
	}
	return rec, nil
}
```
Change the first `workerPrompt` construction to pass `memCtx`:
```go
workerPrompt := buildRageWorkerPrompt(req.Prompt, "", "", 1, memCtx)
```
Leave subsequent-round `buildRageWorkerPrompt(...)` calls passing `""` for the memory block (recall once per run).

- [ ] **Step 5: Extend `buildRageWorkerPrompt` to prepend the memory block**

```go
func buildRageWorkerPrompt(originalPrompt, previousWorkerOutput, foremanInstruction string, round int, memoryContext string) string {
	var b strings.Builder
	if strings.TrimSpace(memoryContext) != "" {
		b.WriteString(memoryContext)
		b.WriteString("\n\n")
	}
	// ... existing body unchanged ...
	return b.String()
}
```
Update all call sites to pass the new trailing argument (`""` except the first round).

- [ ] **Step 6: Record after the loop completes**

After the rage loop finishes building `report`/result and you know success + changed paths, add (best-effort):
```go
s.recordMemory(ctx, scope, req, record, relatedArtifacts, runErr == nil)
```
With helper:
```go
func (s *Service) recordMemory(ctx context.Context, scope memory.Scope, req Request, record history.SessionRecord, artifacts []domain.ArtifactEnvelope, success bool) {
	rctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	rec := memory.RunRecord{
		Scope: scope, SessionID: record.ID, TaskID: record.TaskID,
		Agent: req.StarterAgent, Mode: req.Mode, Prompt: req.Prompt,
		ResultSummary: rageResultSummary(artifacts), Success: success,
	}
	if err := s.Memory.Record(rctx, rec); err != nil {
		log.Printf("[memory] record failed scope=%s: %v", scope.Repo, err)
		return
	}
	s.appendMemoryRecordedEvent(ctx, scope, rec)
}
```
Implement `rageResultSummary` to pull a short summary string from the final report artifact (reuse the report payload's summary field; fall back to the foreman verdict). Implement `appendMemoryRecalledEvent` / `appendMemoryRecordedEvent` mirroring existing `appendArtifactStoredEvent` (write an `events.Record` with types `"memory.recalled"` / `"memory.recorded"` carrying scope + counts). These reuse `s.events`/`s.appendSessionStateEvent` plumbing already in the file.

- [ ] **Step 7: Run the test (and the rage suite) to verify pass**

Run: `GOWORK=off go test ./internal/run/ -run 'Rage' -v`
Expected: PASS, including the existing rage tests (updated call sites compile).

- [ ] **Step 8: Commit**

```bash
git add internal/run/service.go internal/run/service_test.go
git commit -m "run: recall and record cross-agent memory around rage runs"
```

---

## Task 5: MCP server exposing memory_recall and memory_note

**Files:**
- Create: `internal/memory/mcp.go`
- Test: `internal/memory/mcp_test.go`

- [ ] **Step 1: Write the failing test (handlers driven by a fake Memory)**

```go
// internal/memory/mcp_test.go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `GOWORK=off go test ./internal/memory/ -run TestMCP -v`
Expected: FAIL — `undefined: NewToolHandlers`.

- [ ] **Step 3: Implement the tool handlers**

```go
// internal/memory/mcp.go
package memory

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolHandlers adapts the Memory port to MCP tool calls. Keep this transport-agnostic:
// it takes/returns JSON so it can be mounted on TagIt's existing MCP/gateway surface.
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

func (h *ToolHandlers) Note(ctx context.Context, raw json.RawMessage) (struct{ OK bool `json:"ok"` }, error) {
	var a noteArgs
	out := struct{ OK bool `json:"ok"` }{}
	if err := json.Unmarshal(raw, &a); err != nil {
		return out, fmt.Errorf("memory_note args: %w", err)
	}
	if a.Repo == "" || a.Fact == "" {
		return out, fmt.Errorf("memory_note: repo and fact are required")
	}
	if err := h.mem.Note(ctx, Scope{Repo: a.Repo}, a.Fact, a.Tags); err != nil {
		return out, err
	}
	out.OK = true
	return out, nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `GOWORK=off go test ./internal/memory/ -run TestMCP -v`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/memory/mcp.go internal/memory/mcp_test.go
git commit -m "memory: add MCP tool handlers for recall and note"
```

---

## Task 6: Construct real memory in the daemon and verify end-to-end build

**Files:**
- Modify: daemon/run wiring (where `run.Service` is built — `internal/app` or `internal/run` constructor)

- [ ] **Step 1: Wire the real backend at startup (best-effort)**

Where the daemon builds the `run.Service`, set its memory from `~/.tagit/memory/cortex.db`, falling back to Nop on error:
```go
memPath := filepath.Join(tagitpath.HomeDir(), "memory", "cortex.db")
if err := os.MkdirAll(filepath.Dir(memPath), 0o755); err == nil {
	if m, err := memory.NewAgentGo(memPath); err == nil {
		runService.Memory = m
	} else {
		log.Printf("[memory] disabled (init failed): %v", err)
	}
}
```

- [ ] **Step 2: Build and run the full suite**

Run: `GOWORK=off go build ./... && GOWORK=off go test -race -count=1 ./internal/memory/... ./internal/run/... ./internal/app/...`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "app: enable cross-agent memory backend for runs"
```

---

## Self-Review

- **Spec coverage:** interface (T1), agent-go lexical engine (T2), cross-agent + Note + empty-safety (T3), auto-inject + record + audit events (T4), MCP surface (T5), real wiring + degradation (T6). SKILL.md packaging and collab/senate wiring are deferred per spec scope — not in this plan.
- **No external key:** T2 uses `NewService(st, nil, nil, ...)` — no LLM, no embedder. Confirmed.
- **Type consistency:** `Memory`/`Scope`/`RunRecord`/`Recollection`/`Episode`/`Fact` defined T1, used identically T2–T6. `NewAgentGo`, `Nop`, `NewToolHandlers` names stable across tasks.
- **Degradation:** Nop fallback (T1/T6), best-effort helpers with timeouts and logging (T4), MCP nil-guard (T5).
- **Risk flagged in T2/T5:** if `Service.Search` does not fall back to FTS on nil embedder, switch to the store's text search (verify against `pkg/store/memory_cortexdb_test.go`); the `struct{ OK bool }` inline return in T5 may need a named type if the MCP mounting layer requires it.
