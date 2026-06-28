package slack

import (
	"context"
	"log"
	"regexp"
	"strings"

	"github.com/liliang-cn/roma/internal/api"
	"github.com/liliang-cn/roma/internal/chatbot"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Bot runs the Slack Socket Mode event loop and routes @mentions to the shared handler.
type Bot struct {
	cfg     *Config
	handler *chatbot.Handler
	client  *socketmode.Client
}

// NewBot wires the Slack sender + shared handler over ROMA's api.Client.
func NewBot(cfg *Config, apiClient *api.Client) *Bot {
	snd := NewSender(cfg.BotToken)
	enq := chatbot.NewAPIEnqueuer(apiClient)
	progress := chatbot.NewProgressFunc(apiClient, snd)
	handler := chatbot.NewHandler(cfg.Bindings, enq, snd, progress)
	api := slack.New(cfg.BotToken, slack.OptionAppLevelToken(cfg.AppToken))
	client := socketmode.New(api)
	return &Bot{cfg: cfg, handler: handler, client: client}
}

// Start runs the Socket Mode loop until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) error {
	go func() {
		for evt := range b.client.Events {
			if evt.Type != socketmode.EventTypeEventsAPI {
				continue
			}
			eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
			if !ok {
				continue
			}
			if evt.Request != nil {
				b.client.Ack(*evt.Request)
			}
			if eventsAPI.Type == slackevents.CallbackEvent {
				if mention, ok := eventsAPI.InnerEvent.Data.(*slackevents.AppMentionEvent); ok {
					b.handler.Handle(ctx, toIncomingMessage(mention))
				}
			}
		}
	}()
	log.Printf("slack: starting socket-mode bot (bindings=%d)", len(b.cfg.Bindings))
	return b.client.RunContext(ctx)
}

// toIncomingMessage maps a Slack app_mention to the shared IncomingMessage.
// Slack app_mention only fires when the bot is @mentioned, in a channel.
func toIncomingMessage(e *slackevents.AppMentionEvent) chatbot.IncomingMessage {
	return chatbot.IncomingMessage{
		MessageID: e.TimeStamp, // used as thread root for replies
		ChatID:    e.Channel,
		Text:      stripSlackMention(e.Text),
		Mentioned: true,
		IsGroup:   true,
	}
}

var slackMentionRe = regexp.MustCompile(`^\s*<@[A-Z0-9]+>\s*`)

// stripSlackMention removes a leading Slack mention token (<@U…>) and trims the
// surrounding whitespace.
func stripSlackMention(text string) string {
	return strings.TrimSpace(slackMentionRe.ReplaceAllString(text, ""))
}
