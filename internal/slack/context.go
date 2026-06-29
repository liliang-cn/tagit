package slack

import (
	"context"
	"strings"

	"github.com/liliang-cn/tagit/internal/chatbot"
	"github.com/slack-go/slack"
)

const slackContextMaxChars = 3000

type slackContext struct{ api *slack.Client }

// newSlackContext builds a chatbot.ContextProvider backed by the Slack Web API.
func newSlackContext(botToken string) chatbot.ContextProvider {
	return &slackContext{api: slack.New(botToken)}
}

// RecentContext fetches up to ~20 recent messages of the channel and returns
// them oldest→newest as plain text lines. Best-effort: any API error returns "".
func (c *slackContext) RecentContext(ctx context.Context, chatID, _ string) string {
	resp, err := c.api.GetConversationHistoryContext(ctx, &slack.GetConversationHistoryParameters{
		ChannelID: chatID,
		Limit:     20,
	})
	if err != nil || resp == nil {
		return ""
	}

	// Slack returns newest→oldest; collect text and reverse to oldest→newest.
	var lines []string
	for _, m := range resp.Messages {
		text := stripSlackMention(m.Text)
		if text == "" {
			continue
		}
		lines = append(lines, text)
	}
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}
	if len(lines) == 0 {
		return ""
	}

	// Cap total to ~slackContextMaxChars, dropping oldest lines if over.
	for {
		out := strings.Join(lines, "\n")
		if len(out) <= slackContextMaxChars || len(lines) == 1 {
			return out
		}
		lines = lines[1:]
	}
}
