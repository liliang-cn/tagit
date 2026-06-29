# Cross-Agent Persistent Memory — Design

Date: 2026-06-28
Status: Approved (brainstorming)
Sub-project: A (of the "open-source, self-hosted, multi-model, auditable @agent team assistant" effort)

## Purpose

Give TagIt a persistent memory that is **shared across all coding agents**: what
`codex` learned on a repo is recalled for `claude`, and vice versa. Memory makes
repeated runs in the same repo smarter without re-explaining context — the
"the more you use it, the more it understands" property of Claude Tag, but
self-hosted and model-agnostic.

## Hard constraints (from the user)

1. **No extra LLM / embedder API keys.** Recall uses CortexDB lexical full-text
   search (FTS5/BM25), which needs no embedder. Any "intelligence" (deciding what
   is worth remembering, summarizing) is done by the **coding agents themselves**,
   not a separate hosted LLM. A local embedder (e.g. Ollama) may be added later as
   an opt-in; it is **out of scope** for this sub-project.
2. **Exposed as an MCP server** (and optionally packaged as a skill) so the coding
   agents can call it as a tool. TagIt also injects recall in-process for agents
   that are not driven through MCP.

## Build constraints

- agent-go (`github.com/liliang-cn/agent-go/v2`, v2.95.0) declares `go 1.25.0`;
  TagIt is on `go 1.24.2`. Adding the dependency bumps TagIt's `go` directive to
  1.25 (Go's auto-toolchain fetches 1.25 transparently). CI matrices must use Go
  ≥ 1.25. agent-go is pure-Go SQLite (`modernc.org/sqlite`), no CGO — consistent
  with TagIt.

## Non-goals (YAGNI for this sub-project)

- Semantic/vector recall and embedder configuration (lexical only for now).
- The chat surface (Slack/Lark) — that is sub-project B; `Scope.Channel` is
  carried through the schema but not populated until B exists.
- Token-spend metering / governance — sub-project C.
- ambient/proactive behavior — sub-project D.

## Architecture

A new service-layer module `internal/memory`, peer to `sessions`/`taskstore`.

Dependency direction (respects `docs/backend-module-design.md` rules):

```
memory  ->  domain  +  agent-go (lexical mode)
run     ->  memory
```

`memory` must NOT depend on `scheduler`, `runtime`, or `policy`.

**Core principle — memory is a derived advisory layer, not a source of truth.**
The authoritative record stays in TagIt's event store. Every memory operation is
best-effort: failures and timeouts are logged and never fail a run.

### The port (interface)

```go
type Scope struct {
    Repo    string // repo / working-dir identity (primary key dimension)
    Channel string // optional chat channel (populated by sub-project B)
}

type RunRecord struct {
    Scope                 Scope
    SessionID, TaskID     string
    Agent, Mode           string   // Agent is metadata only, NOT a key -> cross-agent
    Prompt, ResultSummary string
    ChangedPaths          []string
    Verdict               string
    Success               bool
    OccurredAt            time.Time
}

type Episode struct { /* a recalled past RunRecord */ }
type Fact    struct { /* a recalled durable note */ }

type Recollection struct {
    Episodes    []Episode
    Knowledge   []Fact
    ContextText string // pre-rendered block, ready to prepend to an agent prompt
}

type Memory interface {
    Recall(ctx context.Context, scope Scope, query string, limit int) (Recollection, error)
    Record(ctx context.Context, rec RunRecord) error            // episodic, automatic, cheap
    Note(ctx context.Context, scope Scope, fact string, tags []string) error // durable knowledge
}
```

- No LLM-based `Distill`. Durable knowledge is captured by `Note`, called either by
  a coding agent through the MCP tool, or by TagIt extracting the foreman's verdict.
- Implementations:
  - `agentgoMemory` — default, wraps agent-go `memory.Service` (CortexDB, lexical).
  - `nopMemory` — used when memory is disabled or the engine fails to initialize.

### Engine: agent-go in lexical mode

- Backed by agent-go's memory store (CortexDB) under `~/.tagit/memory/cortex.db`
  (via `AGENTGO_HOME`/config `Home` pointed at TagIt's home), **no embedder
  configured** → CortexDB lexical FTS5 fallback.
- Episodic run records -> agent-go Memory API, keyed by `Scope.Repo` (agent in
  metadata). Durable notes -> Knowledge API, namespaced by `Scope.Repo`.
- `Recall` runs a lexical search over both stores for the query (the run prompt),
  merges, and renders `ContextText`.

## Data flow

### Recall + auto-inject (before a run)

In `internal/run` (start with **rage** mode, the verified path):

1. Compute `Scope{Repo: working-dir identity}`.
2. `rec, _ := mem.Recall(ctx, scope, prompt, K)` (bounded by a short timeout).
3. If `rec.ContextText` is non-empty, prepend it as a clearly delimited
   "Memory context (from past runs in this repo)" section in the worker prompt
   (extend `buildRageWorkerPrompt`). Coding agents that ignore it lose nothing.

### Record (after a run)

1. Build `RunRecord` from the result: prompt, result summary, `ChangedPaths`
   (from `workspace.ChangedPaths`), foreman/report verdict, success.
2. `mem.Record(ctx, rec)` — best-effort.

### MCP tool surface

A thin MCP server (in `internal/memory/mcp`, served alongside TagIt's existing
ACP/gateway surface) exposes scoped tools so MCP-capable agents recall/record
on demand:

| Tool | Args | Effect |
|---|---|---|
| `memory_recall` | `{repo, query, limit}` | returns relevant episodes + facts |
| `memory_note`   | `{repo, fact, tags}`   | stores a durable fact (`Note`) |

Auto-inject covers non-MCP agents (e.g. `codex exec`); the MCP tools cover
MCP-capable agents (e.g. Claude Code). Both back onto the same engine — "both",
as agreed.

### Skill packaging (optional, docs-only)

A `SKILL.md` documenting the `memory_recall`/`memory_note` tools so the capability
can be dropped into a coding agent as a skill. No code beyond the MCP server.

## Auditability

Memory writes are themselves auditable (core to the product promise):

- `Record` emits a `memory.recorded` event; `Note` emits `memory.noted`.
- When recall context is injected into a prompt, TagIt logs a `memory.recalled`
  event noting scope, query, and how many episodes/facts were injected.

These go through the existing event store so the full "what was the agent fed"
trail is replayable.

## Error handling / degradation

- All `Recall`/`Record`/`Note` calls are best-effort: log on error, never fail the run.
- `Recall` is bounded by a timeout (e.g. 3s) so it cannot stall a run.
- If the engine fails to initialize (bad path, corrupt db), fall back to
  `nopMemory` and warn once.
- Memory can be disabled via config (`memory.enabled=false`) → `nopMemory`.

## Testing

- `run`/scheduler unit tests use a **fake `Memory`** (in-memory map) — no CortexDB,
  no network, deterministic. Keeps existing fakes pattern intact.
- `agentgoMemory` integration test uses a real CortexDB store in `t.TempDir()` in
  **lexical mode** (no embedder, no API key → CI-safe). Cases:
  - `Record` then `Recall` returns the record (round-trip).
  - **Cross-agent**: record with `Agent="codex"`, recall (for a would-be `claude`
    run) returns it — proving agent is not a key.
  - `Note` then `Recall` surfaces the durable fact.
  - `Recall` on an empty/unknown repo returns an empty, non-error result.
- MCP server tested by invoking the tool handlers directly with a fake `Memory`.

## Scope of this sub-project (build order)

1. `internal/memory`: interface, `nopMemory`, `agentgoMemory` (lexical), tests.
2. Wire `Recall` (auto-inject) + `Record` into **rage** mode only; audit events.
3. MCP server (`memory_recall`, `memory_note`) + tests.
4. (Fast-follow) wire collab/senate; SKILL.md.

Each later sub-project (B chat surface, C governance, D ambient) gets its own
spec → plan → implementation cycle.
