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

// progressLine derives a short phase line from a real TagIt event, or "" if the
// event should be skipped.
func progressLine(rec events.Record) string {
	switch rec.Type {
	case events.TypeRelayNodeStarted:
		return "… running a step"
	case events.TypeRuntimeStarted:
		return "… agent working"
	case events.TypeSessionStateChanged:
		return "… " + payloadString(rec.Payload, "state", "status")
	case events.TypeExecutionCompletedDetected:
		return "… wrapping up"
	default:
		return ""
	}
}

// finalSummary builds the closing message from the terminal job status and
// optional error message.
func finalSummary(status, errMsg string) string {
	if strings.Contains(status, "fail") || errMsg != "" {
		msg := "❌ " + status
		if errMsg != "" {
			msg += ": " + errMsg
		}
		return msg
	}
	if strings.Contains(status, "succ") {
		return "✅ Done — " + status
	}
	return "Run " + status
}

// payloadString returns the first non-empty string value among keys.
func payloadString(p map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := p[k]
		if !ok {
			continue
		}
		if s, ok := v.(string); ok && s != "" {
			return s
		}
	}
	return ""
}
