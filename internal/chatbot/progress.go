package chatbot

import (
	"context"
	"strings"
	"time"

	"github.com/liliang-cn/tagit/internal/events"
)

// streamProgress posts throttled progress lines into the thread rooted at
// rootMessageID in chatID, draining ch until it closes or ctx is cancelled. It
// handles progress only: the caller fetches and posts the final summary once the
// stream closes. now supplies the clock so throttling is testable.
func streamProgress(ctx context.Context, snd Sender, chatID, rootMessageID string, ch <-chan events.Record, throttle time.Duration, now func() time.Time) {
	var last time.Time
	var haveLast bool

	for {
		select {
		case <-ctx.Done():
			return
		case rec, ok := <-ch:
			if !ok {
				return
			}
			line := progressLine(rec)
			if line == "" {
				continue
			}
			t := now()
			if haveLast && t.Sub(last) < throttle {
				continue
			}
			last = t
			haveLast = true
			_ = snd.Reply(ctx, chatID, rootMessageID, line)
		}
	}
}

// progressLine derives a short, human progress line from a real TagIt event, or
// "" if the event should be skipped. Curated to a handful of meaningful
// milestones so the thread shows real activity without flooding.
func progressLine(rec events.Record) string {
	switch rec.Type {
	case events.TypeConversationReplied:
		// A conversational run: the agent answered directly instead of doing
		// repo work. Post that answer verbatim.
		if text, ok := rec.Payload["text"].(string); ok {
			return strings.TrimSpace(text)
		}
		return ""
	case events.TypeMemoryRecalled:
		return "🧠 recalled past context from this repo"
	case events.TypeRelayNodeStarted:
		return "▶️ running a step"
	case events.TypeRuntimeStarted:
		return "🔧 agent is working…"
	case events.TypeSemanticReportProduced:
		return "🔎 reviewed the output"
	case events.TypeArtifactStored:
		return "📦 produced a result"
	case events.TypeMergeBackRequested:
		return "🔀 merging changes back to the repo"
	case events.TypeExecutionCompletedDetected:
		return "✓ finishing up"
	default:
		return ""
	}
}

// isTerminalStatus reports whether a queue status means the job is done.
func isTerminalStatus(status string) bool {
	switch status {
	case "succeeded", "failed", "cancelled", "rejected":
		return true
	default:
		return false
	}
}

// finalSummary builds the closing message from the terminal job status and
// optional error message.
func finalSummary(status, errMsg string) string {
	switch {
	case status == "succeeded":
		return "✅ Done — the task succeeded. Check the repo for the changes."
	case status == "failed" || errMsg != "":
		msg := "❌ Failed"
		if errMsg != "" {
			msg += ": " + errMsg
		}
		return msg
	case status == "cancelled":
		return "🛑 Cancelled."
	default:
		return "Run " + status
	}
}
