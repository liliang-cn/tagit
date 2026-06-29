# 飞书 @Tag Bot — Design

Date: 2026-06-28
Status: Approved (brainstorming)
Sub-project: B (of the "open-source, self-hosted, multi-model, auditable @agent team assistant" effort)

## Purpose

Bring the Claude-Tag-style experience to TagIt on **Feishu (飞书)**: a teammate you
`@mention` in a group chat to delegate a coding task. The bot picks up the task,
runs it through TagIt (reusing the queue + run modes + cross-agent memory from
sub-project A), streams progress back into the same thread, and posts the final
result. Self-hosted and model-agnostic.

## Decisions (from brainstorming)

- **Platform:** Feishu self-built app "tagit" (`app_id = cli_aacbb99842f89cbd`),
  with the bot capability. (App Secret supplied at runtime, never committed.)
- **Event transport: long connection (WebSocket).** TagIt dials out to Feishu via
  the official Go SDK (`github.com/larksuite/oapi-sdk-go/v3`) long-connection
  event dispatcher. **No public callback URL / tunnel required** — ideal for a
  self-hosted daemon. (Webhook/callback mode is explicitly not used.)
- **Channel → repo binding:** each Feishu group maps to one repo/working dir
  (+ optional agent + mode), configured in `~/.tagit/feishu.json`. A group is a
  long-lived context; memory is keyed by `Scope.Channel = chat_id` so the bot
  gets smarter per group over time.
- **Feedback: real-time progress.** Immediately ack in-thread, stream throttled
  worker/foreman progress into the thread, then post the final result.
- **Default run:** rage mode + the group's configured agent (falls back to the
  registry default agent).

## Non-goals (YAGNI)

- Single (1:1) chat with the bot — group chats only.
- Interactive message-card buttons / approvals over Feishu.
- Multi-IM support now (Slack/Discord) — but the adapter boundary is kept clean
  so another platform can be added later.
- Webhook/callback event transport (long connection only).

## Architecture

A new module `internal/feishu` (platform adapter; isolated so another IM could
follow). It depends on `domain`, the `queue`/`run`/`api` service layers, and the
Feishu SDK. It must not be imported by lower layers.

```
Feishu group: "@tagit 给登录接口加参数校验"
        │  (long connection / WebSocket — TagIt dials out)
   ┌────▼─────────────────────────────────────────────┐
   │ internal/feishu                                   │
   │  • Client: SDK long-connection event loop         │
   │  • Dispatcher: im.message.receive_v1 handler      │
   │      - confirm the bot was @mentioned             │
   │      - dedup by message_id                         │
   │      - extract plain-text prompt                   │
   │      - resolve chat_id -> Binding (repo/agent/mode)│
   │  • Sender: Feishu message API (ack/progress/final) │
   │  • Config: ~/.tagit/feishu.json (creds + bindings)  │
   └────┬─────────────────────────────────────────────┘
        │ enqueue (reuse existing queue + run.Service)
        ▼
   queue.Request{ Prompt, StarterAgent, Mode=rage, WorkingDir=repo,
                  Scope.Channel=chat_id }  ──►  run (rage/…, memory-aware)
        │ subscribe StreamJobEvents(jobID)
        ▼  throttled progress  ──►  Sender  ──►  same Feishu thread
        final result ──► Sender ──► thread
```

### Components (each independently testable)

- **`Config`** (`config.go`): loads `~/.tagit/feishu.json`:
  `{ "app_id", "app_secret", "bindings": [ { "chat_id", "repo", "agent", "mode" } ] }`.
  Missing file → feature disabled (return a disabled marker, no error).
- **`Bot`** (`bot.go`): owns the SDK long-connection client; wires the event
  dispatcher; lifecycle Start/Stop; auto-reconnect (SDK-managed).
- **`handler`** (`handler.go`): pure-ish logic for one received message →
  decide-to-act → build a `queue.Request`. Takes interfaces (an `Enqueuer`, a
  `Sender`, a `Deduper`, a `Clock`) so it is unit-testable without the SDK.
- **`Sender`** (`sender.go`): interface `Reply(ctx, threadOrChatRef, text) error`
  + a Feishu-API implementation. Tests use a fake.
- **`progress`** (`progress.go`): subscribes to `StreamJobEvents(jobID)`, throttles
  (coalesce to at most one update per interval and/or per phase change), and calls
  `Sender`. Final-result formatting (success/fail + changed files) lives here.

### Reuse (not rebuilt)

- `queue` + `run.Service` (rage/collab/senate; memory wired in A).
- `memory` via `Scope.Channel = chat_id`.
- `api.Client.StreamJobEvents` for live progress.
- `policy` + event store give audit for free.

## Data flow (one @mention)

1. Long connection delivers `im.message.receive_v1`.
2. Handler confirms the bot is in `mentions`; if not, ignore.
3. Dedup by `message_id` (in-memory LRU/set bounded; Feishu may redeliver).
4. Extract plain text (strip the @mention token) as the prompt.
5. Resolve `chat_id` → `Binding`. No binding → reply "this group isn't linked to a
   repo yet" and stop.
6. `Sender.Reply(chat, "收到，开始干 🛠️")` (ack), capturing the root message to thread on.
7. Enqueue `queue.Request{ Prompt, StarterAgent: binding.agent, Mode: binding.mode||rage,
   WorkingDir: binding.repo, Scope:{Channel: chat_id} }`.
8. `progress` subscribes to the job's events, posts throttled updates into the thread.
9. On completion: post final result (status + short summary + changed files).

## Error handling

- **Reconnect:** SDK long connection auto-reconnects; `Bot` logs drops and resumes.
- **Missing/invalid config:** feature disabled, single log line, TagIt unaffected.
- **Send/enqueue/run failure:** reply a concise error into the thread; never crash
  the bot loop. All handler paths are best-effort around the run.
- **Duplicate delivery:** message_id dedup prevents double-running.
- **Throttling:** progress coalesced (e.g. ≤1 msg / 5s, plus phase-change and
  terminal) to avoid flooding the thread.

## Security

- App Secret only in `~/.tagit/feishu.json` (outside the repo) or env; never logged,
  never committed. `feishu.json` documented as secret; recommend rotating the
  secret after sharing.
- Optional per-binding `allowed_open_ids` allowlist (default: anyone in the group)
  — schema reserved, enforcement is a fast-follow if needed.

## Testing

- `handler` tested with recorded `im.message.receive_v1` JSON fixtures + fakes for
  Enqueuer/Sender/Deduper/Clock: @-detection, dedup, prompt extraction, binding
  resolution (hit/miss), ack + enqueue shape. No live Feishu, no creds → CI-safe.
- `progress` tested with a synthetic event stream + fake Sender: throttling,
  phase-change emission, final formatting.
- `config` tested with temp JSON (present/missing/malformed).
- `Sender` Feishu-API impl: thin; covered by a construction/smoke test, exercised
  for real only in manual integration.
- Manual integration (real Feishu group) is the acceptance gate, run once creds +
  a linked group are configured.

## Configuration & ops

- `~/.tagit/feishu.json` (gitignored location, under TagIt home). Example:
  ```json
  {
    "app_id": "cli_aacbb99842f89cbd",
    "app_secret": "<secret>",
    "bindings": [
      { "chat_id": "oc_xxx", "repo": "/path/to/repo", "agent": "codex", "mode": "rage" }
    ]
  }
  ```
- The daemon starts the `Bot` at boot when config is present (best-effort, like the
  memory backend); absence leaves TagIt unchanged.
- A `tagit feishu` CLI surface (e.g. `bind <chat_id> <repo>`, `status`) is a small
  fast-follow; not required for the core loop.

## Build constraints

- New dependency `github.com/larksuite/oapi-sdk-go/v3` (Feishu official Go SDK,
  long-connection support). Pure Go; verify it doesn't force a toolchain bump
  beyond the current go 1.25.

## Scope of this sub-project (build order)

1. `internal/feishu`: Config + Bindings (+ tests).
2. Sender interface + Feishu-API impl.
3. handler (event → decide → enqueue + ack) with fixtures/fakes.
4. progress streamer (StreamJobEvents → throttled thread updates + final).
5. Bot (SDK long-connection wiring) + daemon startup hook (best-effort, off when unconfigured).
6. Manual integration verification against the real "tagit" app + a linked group.

Later sub-projects: C (governance/token metering), D (ambient). Single-chat,
cards, and multi-IM are deferred.
