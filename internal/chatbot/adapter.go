package chatbot

import (
	"context"
	"time"

	"github.com/liliang-cn/tagit/internal/api"
	"github.com/liliang-cn/tagit/internal/events"
	"github.com/liliang-cn/tagit/internal/run"
)

// NewAPIEnqueuer adapts a tagitd api.Client into an Enqueuer.
func NewAPIEnqueuer(c *api.Client) Enqueuer {
	return apiEnqueuer{c: c}
}

type apiEnqueuer struct{ c *api.Client }

func (e apiEnqueuer) Submit(ctx context.Context, args SubmitArgs) (string, error) {
	resp, err := e.c.Submit(ctx, api.SubmitRequest{
		Prompt:       args.Prompt,
		Mode:         run.NormalizeMode(args.Mode),
		StarterAgent: args.Agent,
		WorkingDir:   args.Repo,
	})
	if err != nil {
		return "", err
	}
	return resp.JobID, nil
}

// NewProgressFunc returns a ProgressFunc that polls the daemon for a job's
// status and events, posting new progress lines into the chat thread and a
// final summary when the job reaches a terminal state. Polling (rather than the
// SSE stream's close) is what reliably detects completion, so the thread always
// gets a "done" notification.
func NewProgressFunc(c *api.Client, snd Sender) ProgressFunc {
	return func(jobID, chatID, rootMessageID string) {
		ctx := context.Background()
		seen := map[string]bool{}
		lastLine := ""
		errs := 0
		conversational := false
		for {
			time.Sleep(3 * time.Second)
			req, err := c.QueueGet(ctx, jobID)
			if err != nil {
				if errs++; errs > 40 {
					return
				}
				continue
			}
			errs = 0
			if req.SessionID != "" {
				if evList, err := c.EventList(ctx, req.SessionID, req.TaskID, events.Type("")); err == nil {
					for _, e := range evList {
						if seen[e.ID] {
							continue
						}
						seen[e.ID] = true
						if e.Type == events.TypeConversationReplied {
							conversational = true
						}
						line := progressLine(e)
						next, post := dedupLine(lastLine, line)
						lastLine = next
						if post {
							_ = snd.Reply(ctx, chatID, rootMessageID, line)
						}
					}
				}
			}
			if isTerminalStatus(string(req.Status)) {
				// A conversational reply already delivered the agent's answer;
				// don't follow it with a generic "check the repo" summary.
				if !conversational {
					_ = snd.Reply(ctx, chatID, rootMessageID, finalSummary(string(req.Status), req.Error))
				}
				return
			}
		}
	}
}

// dedupLine collapses consecutive identical progress lines. Given the last
// posted line and a candidate line, it returns the new "last" value to carry
// forward and whether the candidate should be posted. Empty candidates are
// dropped (and don't reset the last value); a candidate equal to last is
// skipped.
func dedupLine(last, line string) (newLast string, post bool) {
	if line == "" || line == last {
		return last, false
	}
	return line, true
}
