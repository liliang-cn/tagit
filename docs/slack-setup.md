# Slack @Tag Bot — Setup Guide

Turn TagIt into a Claude-Tag-style teammate inside a Slack channel: `@mention`
the bot or use the native `/tagit` slash command, and it runs the task in a
bound repo and replies in the thread. Runs over **Socket Mode** — **no public
URL / Request URL required**.

---

## Prerequisites

- `tagit` + `tagitd` installed (`make install` or `brew install --HEAD liliang-cn/tap/tagit`).
- At least one coding agent on `PATH` (e.g. `claude`, `codex`).
- A Slack workspace where you can create an app.

## 1. Create the app (Socket Mode)

In [api.slack.com/apps](https://api.slack.com/apps) → **Create New App** → from
scratch.

- **Socket Mode** → enable it. Generate an **App-Level Token** with scope
  `connections:write` — this is the `xapp-…` token.
- **OAuth & Permissions** → add bot scopes `app_mentions:read`, `chat:write`,
  `commands`. Install to the workspace to get the **Bot Token** (`xoxb-…`).
- **Event Subscriptions** → enable, and subscribe the bot event `app_mention`.
  With Socket Mode on there is no Request URL to fill in.

## 2. Register the `/tagit` slash command (autocomplete)

**Features → Slash Commands → Create New Command**:

- **Command**: `/tagit`
- **Short Description**: `Configure and run TagIt in this channel`
- **Usage Hint**: `bind <repo> | agent <id> | mode <m> | status | unbind | help`

With **Socket Mode** enabled, **no Request URL is needed** — invocations arrive
over the socket. Once registered, `/tagit` **autocompletes** in the message box,
and the remainder you type (e.g. `bind /repo`) is parsed as a subcommand.

## 3. Run it

Drop the tokens into `~/.tagit/slack.json`:

```json
{ "bot_token": "xoxb-…", "app_token": "xapp-…", "bindings": [] }
```

`tagit start`, invite the bot to a channel, then configure and run **from chat**:

```
/tagit bind /path/to/repo      link this channel to a repo
/tagit agent codex             set the agent (also: mode, status, unbind, help)
/tagit add input validation to the signup handler   ← run a task
```

The `@TagIt /bind …` mention form still works too as a universal fallback
(`@TagIt /status`, `@TagIt do the thing`). Both the slash command and the
@mention share one command code path, so behavior is identical.
