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

// RecentContext returns a plain-text transcript (oldest→newest) for extra
// context. When threadID is set it returns that thread's replies; otherwise the
// channel's recent messages. Best-effort: any API error returns "".
func (c *slackContext) RecentContext(ctx context.Context, chatID, threadID, _ string) string {
	var lines []string
	if threadID != "" {
		// Thread replies come oldest→newest already.
		msgs, _, _, err := c.api.GetConversationRepliesContext(ctx, &slack.GetConversationRepliesParameters{
			ChannelID: chatID,
			Timestamp: threadID,
			Limit:     50,
		})
		if err != nil {
			return ""
		}
		for _, m := range msgs {
			if m.BotID != "" { // skip bot/app messages (our own progress spam)
				continue
			}
			if text := stripSlackMention(m.Text); text != "" {
				lines = append(lines, text)
			}
		}
	} else {
		resp, err := c.api.GetConversationHistoryContext(ctx, &slack.GetConversationHistoryParameters{
			ChannelID: chatID,
			Limit:     20,
		})
		if err != nil || resp == nil {
			return ""
		}
		// Channel history is newest→oldest; collect then reverse below.
		for _, m := range resp.Messages {
			if m.BotID != "" { // skip bot/app messages
				continue
			}
			if text := stripSlackMention(m.Text); text != "" {
				lines = append(lines, text)
			}
		}
		for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
			lines[i], lines[j] = lines[j], lines[i]
		}
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
