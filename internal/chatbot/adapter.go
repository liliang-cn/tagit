package chatbot

import (
	"context"
	"time"

	"github.com/liliang-cn/roma/internal/api"
	"github.com/liliang-cn/roma/internal/events"
	"github.com/liliang-cn/roma/internal/run"
)

// NewAPIEnqueuer adapts a romad api.Client into an Enqueuer.
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

// NewProgressFunc returns a ProgressFunc that streams a job's events from the
// daemon into the chat thread via snd, throttled to one line every 5s.
func NewProgressFunc(c *api.Client, snd Sender) ProgressFunc {
	return func(jobID, rootMessageID string) {
		ctx := context.Background()
		ch := make(chan events.Record, 32)
		go func() {
			defer close(ch)
			_ = c.StreamJobEvents(ctx, jobID, ch)
		}()
		streamProgress(ctx, snd, rootMessageID, ch, 5*time.Second, time.Now)
	}
}
