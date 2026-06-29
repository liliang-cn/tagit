package chatbot

import (
	"context"
	"os"
	"strings"
)

const helpText = "Commands:\n" +
	"/help — show this help\n" +
	"/status — show this chat's repo binding\n" +
	"/bind <repo-path> — link this chat to a repo\n" +
	"/agent <id> — set the agent for this chat\n" +
	"/mode <rage|collab|senate> — set the run mode\n" +
	"/unbind — unlink this chat"

// Command parses and executes a config command for chatID and returns the reply
// text. `text` may be the full "/bind /repo" form (from @mention) or the
// slash-remainder "bind /repo" form (from a native Slack slash command). It
// never enqueues a run.
func (h *Handler) Command(ctx context.Context, chatID, text string) string {
	text = strings.TrimSpace(text)
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return "Unknown command. Try /help."
	}
	word := fields[0]
	arg := strings.TrimSpace(strings.TrimPrefix(text, word))
	cmd := strings.ToLower(strings.TrimPrefix(word, "/"))

	switch cmd {
	case "help":
		return helpText
	case "status":
		return h.cmdStatus(chatID)
	case "bind":
		return h.cmdBind(chatID, arg)
	case "agent":
		return h.cmdAgent(chatID, arg)
	case "mode":
		return h.cmdMode(chatID, arg)
	case "unbind":
		return h.cmdUnbind(chatID)
	default:
		return "Unknown command. Try /help."
	}
}

func (h *Handler) cmdStatus(chatID string) string {
	b, ok := h.store.For(chatID)
	if !ok {
		return "This chat isn't linked yet. Use /bind <repo-path> to link it."
	}
	agent := b.Agent
	if agent == "" {
		agent = "(default)"
	}
	mode := b.Mode
	if mode == "" {
		mode = "rage"
	}
	return "📍 repo: " + b.Repo + "\nagent: " + agent + "\nmode: " + mode
}

func (h *Handler) cmdBind(chatID, path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return "Usage: /bind <repo-path>"
	}
	info, err := os.Stat(path)
	if err != nil {
		return "Can't use that path: " + err.Error()
	}
	if !info.IsDir() {
		return "Not a directory: " + path
	}
	b, _ := h.store.For(chatID)
	b.ChatID = chatID
	b.Repo = path
	if err := h.store.Set(b); err != nil {
		return "Failed to save: " + err.Error()
	}
	return "✅ Linked this chat to " + path + "."
}

func (h *Handler) cmdAgent(chatID, id string) string {
	id = strings.TrimSpace(id)
	if id == "" {
		return "Usage: /agent <id>"
	}
	b, ok := h.store.For(chatID)
	if !ok {
		return "Bind a repo first: /bind <repo-path>."
	}
	b.Agent = id
	if err := h.store.Set(b); err != nil {
		return "Failed to save: " + err.Error()
	}
	return "✅ Agent set to " + id + "."
}

func (h *Handler) cmdMode(chatID, m string) string {
	m = strings.ToLower(strings.TrimSpace(m))
	switch m {
	case "rage", "collab", "senate":
	default:
		return "Usage: /mode <rage|collab|senate>"
	}
	b, ok := h.store.For(chatID)
	if !ok {
		return "Bind a repo first: /bind <repo-path>."
	}
	b.Mode = m
	if err := h.store.Set(b); err != nil {
		return "Failed to save: " + err.Error()
	}
	return "✅ Mode set to " + m + "."
}

func (h *Handler) cmdUnbind(chatID string) string {
	if _, ok := h.store.For(chatID); !ok {
		return "This chat wasn't linked."
	}
	if err := h.store.Delete(chatID); err != nil {
		return "Failed to unlink: " + err.Error()
	}
	return "✅ Unlinked this chat."
}
