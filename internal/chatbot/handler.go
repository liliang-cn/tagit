package chatbot

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"
)

// SubmitArgs are the platform-agnostic inputs to a run submission.
type SubmitArgs struct {
	Repo   string
	Prompt string
	Agent  string
	Mode   string
}

// Enqueuer submits a run and returns a job id to track its progress.
type Enqueuer interface {
	Submit(ctx context.Context, args SubmitArgs) (jobID string, err error)
}

// ProgressFunc streams progress for a job back into the chat thread rooted at
// rootMessageID in chatID. It is run in its own goroutine and must not block the
// caller.
type ProgressFunc func(jobID, chatID, rootMessageID string)

// ContextProvider returns a short transcript of recent messages in a chat for
// extra context, or "" if unavailable. Best-effort: errors -> "".
type ContextProvider interface {
	RecentContext(ctx context.Context, chatID, messageID string) string
}

// Handler turns an @mention in a bound group chat into a TagIt run and acks it.
type Handler struct {
	store    BindingStore
	enq      Enqueuer
	snd      Sender
	progress ProgressFunc
	ctxProv  ContextProvider

	mu   sync.Mutex
	seen map[string]struct{}
}

// NewHandler wires the handler with its dependencies. progress may be nil.
func NewHandler(store BindingStore, enq Enqueuer, snd Sender, progress ProgressFunc) *Handler {
	return &Handler{
		store:    store,
		enq:      enq,
		snd:      snd,
		progress: progress,
		seen:     make(map[string]struct{}),
	}
}

// SetContextProvider sets an optional provider used to prepend recent chat
// context to task prompts. nil-safe.
func (h *Handler) SetContextProvider(p ContextProvider) {
	h.ctxProv = p
}

// Handle processes one incoming message: best-effort, never returns an error.
func (h *Handler) Handle(ctx context.Context, msg IncomingMessage) {
	if !msg.IsGroup || !msg.Mentioned || msg.Text == "" {
		return
	}
	if !h.markSeen(msg.MessageID) {
		return
	}

	if strings.HasPrefix(strings.TrimSpace(msg.Text), "/") {
		reply := h.Command(ctx, msg.ChatID, msg.Text)
		h.reply(ctx, msg.ChatID, msg.MessageID, reply)
		return
	}

	binding, ok := h.store.For(msg.ChatID)
	if !ok {
		h.reply(ctx, msg.ChatID, msg.MessageID, "This chat isn't linked to a repo yet. Use /bind <repo-path> to link it.")
		return
	}

	h.reply(ctx, msg.ChatID, msg.MessageID, "收到，开始干 🛠️")

	mode := binding.Mode
	if mode == "" {
		mode = "rage"
	}

	prompt := msg.Text
	if h.ctxProv != nil {
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		cxt := h.ctxProv.RecentContext(cctx, msg.ChatID, msg.MessageID)
		cancel()
		prompt = composePrompt(cxt, msg.Text)
	}

	jobID, err := h.enq.Submit(ctx, SubmitArgs{
		Repo:   binding.Repo,
		Prompt: prompt,
		Agent:  binding.Agent,
		Mode:   mode,
	})
	if err != nil {
		h.reply(ctx, msg.ChatID, msg.MessageID, "Failed to start: "+err.Error())
		return
	}

	if h.progress != nil {
		go h.progress(jobID, msg.ChatID, msg.MessageID)
	}
}

// markSeen records the message id and reports whether it is new (not a duplicate).
func (h *Handler) markSeen(messageID string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if _, ok := h.seen[messageID]; ok {
		return false
	}
	h.seen[messageID] = struct{}{}
	return true
}

// composePrompt prepends recent conversation context to the task prompt.
func composePrompt(contextText, task string) string {
	if strings.TrimSpace(contextText) == "" {
		return task
	}
	return "Recent conversation in this chat (for context, latest last):\n" +
		contextText +
		"\n\n---\nThe latest request to act on:\n" + task
}

func (h *Handler) reply(ctx context.Context, chatID, rootMessageID, text string) {
	if err := h.snd.Reply(ctx, chatID, rootMessageID, text); err != nil {
		log.Printf("chatbot: reply failed: %v", err)
	}
}
