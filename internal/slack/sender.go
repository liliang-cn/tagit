package slack

import (
	"context"

	"github.com/liliang-cn/tagit/internal/chatbot"
	"github.com/slack-go/slack"
)

type slackSender struct{ api *slack.Client }

// NewSender builds a Slack Web-API-backed chatbot.Sender using the bot token.
// Reply posts into the thread: chatID is the channel, rootMessageID is the
// thread ts.
func NewSender(botToken string) chatbot.Sender {
	return &slackSender{api: slack.New(botToken)}
}

func (s *slackSender) Reply(ctx context.Context, chatID, rootMessageID, text string) error {
	_, _, err := s.api.PostMessageContext(ctx, chatID,
		slack.MsgOptionText(text, false),
		slack.MsgOptionTS(rootMessageID))
	return err
}
