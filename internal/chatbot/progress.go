package chatbot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/liliang-cn/roma/internal/events"
)

// terminalTypes are the event types that end a run and trigger a final summary.
var terminalTypes = map[string]struct{}{
	"session.completed": {},
	"session.failed":    {},
	"job.completed":     {},
	"job.failed":        {},
}

// streamProgress posts throttled progress lines into the thread rooted at
// rootMessageID, draining ch until a terminal event arrives, ctx is cancelled,
// or the channel closes. On a terminal event it posts a final summary and
// returns. now supplies the clock so throttling is testable.
func streamProgress(ctx context.Context, snd Sender, rootMessageID string, ch <-chan events.Record, throttle time.Duration, now func() time.Time) {
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
			if _, terminal := terminalTypes[string(rec.Type)]; terminal {
				snd.Reply(ctx, rootMessageID, finalSummary(rec))
				return
			}
			phase := payloadString(rec.Payload, "phase", "state")
			if phase == "" {
				continue
			}
			t := now()
			if haveLast && t.Sub(last) < throttle {
				continue
			}
			last = t
			haveLast = true
			snd.Reply(ctx, rootMessageID, "… "+phase)
		}
	}
}

// finalSummary builds the closing message for a terminal event.
func finalSummary(rec events.Record) string {
	failed := strings.HasSuffix(string(rec.Type), ".failed")
	icon := "✅"
	if failed {
		icon = "❌"
	}
	status := payloadString(rec.Payload, "status", "state")
	msg := icon
	if status != "" {
		msg += " " + status
	} else if failed {
		msg += " failed"
	} else {
		msg += " done"
	}
	if files := payloadString(rec.Payload, "changed_files", "files"); files != "" {
		msg += "\nchanged files: " + files
	}
	return msg
}

// payloadString returns the first non-empty string value among keys.
func payloadString(p map[string]any, keys ...string) string {
	for _, k := range keys {
		v, ok := p[k]
		if !ok {
			continue
		}
		switch s := v.(type) {
		case string:
			if s != "" {
				return s
			}
		case []string:
			if len(s) > 0 {
				return strings.Join(s, ", ")
			}
		case []any:
			parts := make([]string, 0, len(s))
			for _, e := range s {
				if es, ok := e.(string); ok && es != "" {
					parts = append(parts, es)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, ", ")
			}
		case fmt.Stringer:
			if str := s.String(); str != "" {
				return str
			}
		}
	}
	return ""
}
