# TagIt — an open-source, self-hosted Claude Tag

> @mention an AI teammate in your team chat and it does the work — self-hosted, model-agnostic, auditable.

Like Anthropic's **Claude Tag**, TagIt lets you **@mention a coding agent in a group chat**, hand it a task, and get the result back in the thread. Unlike Claude Tag, TagIt is:

- **Open-source & self-hosted** — your code and chat never leave your machine
- **Multi-model** — Claude Code, Codex, or any CLI agent (not one vendor)
- **Multi-platform** — Feishu (飞书) and Slack, no public URL needed (long-connection / Socket Mode)
- **Multiplayer + memory** — one shared agent per channel that remembers past runs in the repo
- **Auditable** — every action is an event in a local store; agents work in isolated `git worktrees`; policy gates

Under the hood: a daemon (`tagitd`) + CLI (`tagit`) that orchestrates one or many coding agents (parallel / vote / worker-foreman) and merges the winning result back via `git apply --3way`.

---

## Install

### Let Claude Code / Codex install it for you

Paste this to your agent:

> Install TagIt from https://github.com/liliang-cn/tagit — clone it, `cd` in, run `make install` (needs Go ≥ 1.25; builds `tagit` + `tagitd` into `~/.local/bin`). Then register my agents: `tagit agent add claude "Claude" $(which claude)` and `tagit agent add codex "Codex" $(which codex)`. Finally run `tagit --help` and show me the output.

### Homebrew (macOS/Linux)

```sh
brew install --HEAD liliang-cn/tap/tagit
```

### From source

```sh
git clone https://github.com/liliang-cn/tagit.git && cd tagit
make install      # → ~/.local/bin/{tagit,tagitd}   (Go ≥ 1.25)
```

---

## Use

### 1. Register the agents you have

```sh
tagit agent add claude "Claude" $(which claude)
tagit agent add codex  "Codex"  $(which codex)
tagit agent list
```

### 2. Run a task from the CLI

```sh
tagit start                                                    # start the daemon
tagit run --agent claude --prompt "add input validation to the signup handler"
tagit run --mode senate --agent codex --with claude --prompt "build X, pick the best implementation"
tagit status                                                   # daemon + queue
tagit stop
```

Modes: **rage** (one agent, worker/foreman rounds — default) · **collab** (delegates in parallel) · **senate** (propose → vote → implement → vote → merge).

### 3. @mention it in chat — the "Tag" experience

Drop bot credentials into `~/.tagit/feishu.json` (or `slack.json`):

```json
{ "app_id": "cli_xxx", "app_secret": "xxx", "bindings": [] }
```

`tagit start`, add the bot to a group, then configure and run **entirely from chat**:

```
@TagIt /bind /path/to/repo      link this channel to a repo
@TagIt /agent codex             set the agent   (also: /mode, /status, /unbind, /help)
@TagIt add input validation to the signup handler
```

It acks (**收到，开始干 🛠️**), works in an isolated `git worktree`, streams progress, and posts **✅ Done** in the thread.

- **Feishu**: a self-built app subscribing `im.message.receive_v1` over **long connection** — no public URL. Full walkthrough: **[docs/feishu-setup.md](docs/feishu-setup.md)**.
- **Slack**: an app in **Socket Mode** (`xapp-` + `xoxb-` tokens) subscribing `app_mention`.

---

State lives in `~/.tagit/` (SQLite + git worktrees). Target repos are separate — pick per run with `--cwd`, or run from inside the repo.
