# TagIt: Runtime Orchestrator for Multi-Agents

> "It just fires up all the coding agents at once and points them at the same problem." — someone

> "It not only does nothing, it also burns a lot of tokens." — someone else

## What is TagIt

TagIt runs **multiple coding agents simultaneously** — Claude, Codex, Gemini, Copilot, OpenCode, or any CLI-driven agent — and orchestrates them into a single coherent result.

Instead of asking one agent for the answer, TagIt can:

- **Parallelize**: run the same problem across multiple agents at the same time
- **Coordinate**: let a starter agent scope work, dispatch subtasks to delegate agents, and synthesize the results
- **Vote**: agents propose solutions independently, TagIt collects their outputs and runs anonymous peer review — the best proposal wins
- **Merge safely**: each agent works in an isolated `git worktree`; TagIt merges the winning implementation back to your main branch automatically via `git apply --3way`

The result is not just "what one agent said" — it's the outcome of a structured deliberation process backed by structured artifacts, event records, and policy gates.

---

**`tagitd`** is the kernel. It owns the queue, sessions, task states, policy checks, workspaces, artifacts, and recovery.
**`tagit`** is the client. You use it to run work, inspect progress, approve plans, and debug sessions.

TagIt supports these `tagit run` modes:

| Mode | Description |
|------|-------------|
| **Rage** | Single-agent execution with worker/foreman rounds until the job is truly done. This is the default when you run with one agent. |
| **Collab** | Starter scopes work, delegates implement in parallel workspaces, and the starter reviews and synthesizes the result. |
| **Senate** | Multi-stage voting flow: agents propose plans, vote on the plan, implement against the accepted plan, then vote again on the implementations before merging the winner. |

Other execution surfaces:

- **Curia**: approval-oriented decision flow that produces a `DecisionPack` plus `ExecutionPlan`
- **Graph**: DAG execution via `tagit debug graph run`, separate from `tagit run`

---

## Install

### Homebrew (macOS/Linux)

```sh
brew install --HEAD liliang-cn/tap/tagit
```

(Until releases are tagged, `--HEAD` builds from `main`. Requires a one-time `brew tap liliang-cn/tap`; the formula is in `HomebrewFormula/tagit.rb`.)

### One-liner (Linux & macOS)

```sh
curl -fsSL https://raw.githubusercontent.com/liliang-cn/tagit/main/install.sh | sh
```

Custom install directory:

```sh
curl -fsSL https://raw.githubusercontent.com/liliang-cn/tagit/main/install.sh | INSTALL_DIR=/usr/local/bin sh
```

The installer:
- Detects your OS and architecture (linux/darwin × amd64/arm64)
- Uses `go install` if Go ≥ 1.22 is available; otherwise, it downloads a prebuilt binary from GitHub Releases
- Creates `~/.tagit/` (TagIt home directory)
- Verifies the binaries actually run after installation
- Warns if the install directory is not in `PATH`

If `~/.local/bin` is not in your PATH, add it:

```sh
# zsh
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.zshrc && source ~/.zshrc

# bash
echo 'export PATH="$HOME/.local/bin:$PATH"' >> ~/.bashrc && source ~/.bashrc
```

### Build from source

Requires Go ≥ 1.22 and git.

```sh
git clone https://github.com/liliang-cn/tagit.git
cd tagit
make install        # installs to ~/.local/bin
```

---

## Quick Start

### 1. Register agents

TagIt has no built-in agents. Register whichever CLI coding tools you have installed.
For `claude`, `codex`, `gemini`, and `copilot`, command arguments are filled in
automatically — add only the ones you actually have on `PATH`:

```sh
tagit agent add claude "Claude" $(which claude)
tagit agent add codex  "Codex"  $(which codex)

# confirm
tagit agent list
```

### 2. Start the daemon

```sh
tagit start
# tagitd started (pid=12345, log=~/.tagit/tagitd.log)
```

### 3. Run a task

```sh
# single agent — defaults to rage
tagit run --prompt "add input validation to the user registration handler" --agent claude

# single agent from a prompt file
tagit run --prompt-file ./prompt.txt --agent claude

# multi-agent — defaults to senate
tagit run --prompt "refactor the payment module and add unit tests" --agent claude --with codex

# explicit collab mode
tagit run --mode collab --prompt "answer a repo question" --agent codex --with claude

# two-stage plan vote + implementation vote
tagit run --mode senate --prompt "build a feature and pick the best implementation" --agent codex --with claude

# explicit rage mode
tagit run --mode rage --prompt "keep going until the feature is actually complete" --agent codex
```

### 4. Inspect progress

```sh
tagit status                        # daemon state + queue summary
tagit queue list                    # all jobs
tagit queue attach <job_id>         # stream live output
tagit result show <session_id>      # final result summary
```

### 5. Stop the daemon

```sh
tagit stop
```

---

## Usage Reference

### Daemon management

```sh
tagit start [--acp-port <port>]   # start tagitd in background
tagit stop                         # stop tagitd (SIGTERM, fallback SIGKILL after 10s)
tagit status                       # daemon state, queue counts, sqlite stats
```

Logs are written to `~/.tagit/tagitd.log`. PID is stored in `~/.tagit/tagitd.pid`.

### Running tasks

```sh
tagit run (--prompt "<prompt>" | --prompt-file <path>) [--mode <collab|senate|rage>] [--agent <id>] [--with <id,...>] [--cwd <dir>] [--continuous] [--max-rounds <n>] [-d] [-f] [--verbose] [--policy-override] [--override-actor <id>]
```

Notes:
- Default mode selection is automatic: one agent uses `rage`; multiple agents use `senate`.
- `--prompt` and `--prompt-file` are mutually exclusive; one of them is required.
- Default behavior is `-f`: submit, follow progress, and print the final result when the run completes.
- Use `-d` to submit in the background and return immediately.
- `--verbose` prints per-node execution details instead of only the main progress lines.
- `rage` is single-agent only.
- `collab` uses the starter agent plus delegates named by `--with`.

### Queue management

```sh
tagit queue list [--status <status>]   # list jobs
tagit queue show <job_id>              # job details as JSON
tagit queue attach <job_id>            # stream output in real time
tagit approve <job_id>                 # approve a pending job
tagit reject  <job_id>                 # reject a pending job
tagit cancel  <job_id>                 # cancel a running job
```

### Agent management

```sh
tagit agent list
tagit agent add <id> <name> <path> [--arg <arg>] [--alias <a>] [--pty] [--mcp] [--json]
tagit agent remove <id>
tagit agent inspect <id>
```

### Results and history

```sh
tagit result show <session_id>
tagit debug session list
tagit debug session show <session_id>
tagit debug task list --session <session_id>
tagit debug event list --session <session_id>
tagit debug artifact list --session <session_id>
tagit debug artifact show <artifact_id>
```

### Graph (DAG) execution

```sh
tagit debug graph run --file examples/curia-test.json
```

---

## How Merge-Back Works

Agents run in isolated git worktrees under `~/.tagit/workspaces/`. When an agent finishes and emits:

```
TAGIT_MERGE_BACK: direct_merge | <reason>
TAGIT_MERGE_FILE: path/to/changed/file
```

TagIt automatically applies the patch back to the main repository using `git apply --3way`. If there are conflicts or policy blocks, the merge is held for manual review.

---

## Development

```sh
make build    # build to bin/
make test     # run full test suite with -race
make install  # install to ~/.local/bin
```

---

## More

- Architecture and design notes: [`DESIGN.md`](./DESIGN.md)
- Agent configuration reference: [`AGENTS.md`](./AGENTS.md)
- Platform runtime notes: [`docs/running-tagitd.md`](./docs/running-tagitd.md)
