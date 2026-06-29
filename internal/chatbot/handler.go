package chatbot

import (
	"context"
	"log"
	"sync"
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

// Handler turns an @mention in a bound group chat into a TagIt run and acks it.
type Handler struct {
	bindings Bindings
	enq      Enqueuer
	snd      Sender
	progress ProgressFunc

	mu   sync.Mutex
	seen map[string]struct{}
}

// NewHandler wires the handler with its dependencies. progress may be nil.
func NewHandler(bindings Bindings, enq Enqueuer, snd Sender, progress ProgressFunc) *Handler {
	return &Handler{
		bindings: bindings,
		enq:      enq,
		snd:      snd,
		progress: progress,
		seen:     make(map[string]struct{}),
	}
}

// Handle processes one incoming message: best-effort, never returns an error.
func (h *Handler) Handle(ctx context.Context, msg IncomingMessage) {
	if !msg.IsGroup || !msg.Mentioned || msg.Text == "" {
		return
	}
	if !h.markSeen(msg.MessageID) {
		return
	}

	binding, ok := h.bindings.For(msg.ChatID)
	if !ok {
		h.reply(ctx, msg.ChatID, msg.MessageID, "This chat isn't linked to a repo yet. Link it before mentioning me.")
		return
	}

	h.reply(ctx, msg.ChatID, msg.MessageID, "收到，开始干 🛠️")

	mode := binding.Mode
	if mode == "" {
		mode = "rage"
	}
	jobID, err := h.enq.Submit(ctx, SubmitArgs{
		Repo:   binding.Repo,
		Prompt: msg.Text,
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

func (h *Handler) reply(ctx context.Context, chatID, rootMessageID, text string) {
	if err := h.snd.Reply(ctx, chatID, rootMessageID, text); err != nil {
		log.Printf("chatbot: reply failed: %v", err)
	}
}
