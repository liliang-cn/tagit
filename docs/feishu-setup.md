# Feishu (飞书) @Tag Bot — Setup Guide

Turn TagIt into a Claude-Tag-style teammate inside a Feishu group: `@mention` the
bot, it runs the task in a bound repo and replies in the thread. Runs over a
**long connection (WebSocket)** — **no public URL / tunnel required**.

Console labels below are shown in Chinese to match the Feishu Open Platform UI.

---

## Prerequisites

- `tagit` + `tagitd` installed (`make install` or `brew install --HEAD liliang-cn/tap/tagit`).
- At least one coding agent on `PATH` (e.g. `claude`, `codex`).

---

## 1. Create a self-built app

1. Go to the Feishu Open Platform → 开发者后台 → **创建企业自建应用** (create a self-built app). Give it a name (e.g. `tagit`).
2. Open **凭证与基础信息** and copy the **App ID** (`cli_...`) and **App Secret**. You'll put these in the config file later.

## 2. Add the bot capability

- **添加应用能力 → 机器人** → add it. This is what lets the app send/receive chat messages.

## 3. Grant permissions

**权限管理 → 开通权限**, add both:

| Permission | What it's for |
|---|---|
| `im:message` | the bot can **send** messages (replies, progress, results) |
| `im:message.group_at_msg:readonly` | the bot can **receive** group messages that @mention it |

## 4. Subscribe to the message event (the key step)

**事件与回调 → 事件配置**:

1. **订阅方式** → edit (✏️) → choose **「使用长连接接收事件」** (receive events over long connection). Do **not** use a callback URL.
2. **添加事件** → add **接收消息 `im.message.receive_v1`**.

> If you skip this, the bot connects but **never receives** your @mentions — the
> most common "no reaction" cause.

## 5. Publish the app

**版本管理与发布 → 创建版本 → 发布**.

> ⚠️ Permission and event changes only take effect **after you publish a new
> version**. If you added permissions/events but didn't republish, the bot still
> won't receive messages.

## 6. Configure TagIt

Create `~/.tagit/feishu.json`:

```json
{
  "app_id": "cli_xxxxxxxxxxxxxxxx",
  "app_secret": "your-app-secret",
  "bindings": []
}
```

`bindings` can stay empty — you'll link channels to repos from chat (Step 8).
Or pre-fill them:

```json
"bindings": [
  { "chat_id": "oc_xxx", "repo": "/path/to/repo", "agent": "codex", "mode": "rage" }
]
```

Register the agents you want to use:

```sh
tagit agent add codex  "Codex"  $(which codex)
tagit agent add claude "Claude" $(which claude)
```

## 7. Start the daemon and add the bot to a group

```sh
tagit start
tail -f ~/.tagit/tagitd.log   # expect: "feishu: starting long-connection bot" + "connected to wss://..."
```

In Feishu, create or open a group and **add the bot** to it.

## 8. Link the channel and run a task — all from chat

`@mention` the bot with a slash command to configure it (no file editing, no restart):

| Command | Effect |
|---|---|
| `@tagit /bind /path/to/repo` | link **this group** to a repo / working directory |
| `@tagit /agent codex` | set the agent for this group |
| `@tagit /mode rage` | set the run mode (`rage` · `collab` · `senate`) |
| `@tagit /status` | show the current binding (repo / agent / mode) |
| `@tagit /unbind` | unlink this group |
| `@tagit /help` | list commands |

Then just delegate work:

```
@tagit add input validation to the signup handler
```

The bot acks (**收到，开始干 🛠️**), runs the agent in an isolated `git worktree`,
streams progress (🧠 recalled context · 🔧 working · 🔎 reviewed · 🔀 merging),
and posts **✅ Done** in the thread.

---

## Troubleshooting

| Symptom | Cause / fix |
|---|---|
| No reaction at all | (a) `tagitd` not running, or (b) event subscription not set to **长连接** + **`im.message.receive_v1`**, or (c) you didn't **republish** after changing config. The log shows `connected to wss://...` even when events aren't subscribed — connection ≠ event delivery. |
| Bot replies "isn't linked to a repo yet" | The pipe works! Run `@tagit /bind /path/to/repo`. |
| Run fails instantly: `unknown agent "codex"` | Register it: `tagit agent add codex "Codex" $(which codex)`. The daemon reloads agent config before each job — no restart needed. |
| `/bind` says the path isn't a directory | Use an absolute path to an existing repo on the **machine running tagitd**. |

To see exactly what the bot receives, the daemon logs each message:
`feishu: received message chat=oc_... group=true mentioned=true text="..."` — handy
for grabbing a `chat_id`.
