package slack

import (
	"context"
	"log"
	"regexp"
	"strings"

	"github.com/liliang-cn/tagit/internal/api"
	"github.com/liliang-cn/tagit/internal/chatbot"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

// Bot runs the Slack Socket Mode event loop and routes @mentions to the shared handler.
type Bot struct {
	cfg       *Config
	handler   *chatbot.Handler
	client    *socketmode.Client
	web       *slack.Client
	botUserID string
}

// NewBot wires the Slack sender + shared handler over TagIt's api.Client.
// path is the slack.json file backing the persistent binding store.
func NewBot(cfg *Config, path string, apiClient *api.Client) *Bot {
	snd := NewSender(cfg.BotToken)
	enq := chatbot.NewAPIEnqueuer(apiClient)
	progress := chatbot.NewProgressFunc(apiClient, snd)
	store := NewConfigStore(path)
	handler := chatbot.NewHandler(store, enq, snd, progress)
	handler.SetContextProvider(newSlackContext(cfg.BotToken))
	web := slack.New(cfg.BotToken, slack.OptionAppLevelToken(cfg.AppToken))
	client := socketmode.New(web)
	return &Bot{cfg: cfg, handler: handler, client: client, web: web}
}

// Start runs the Socket Mode loop until ctx is cancelled.
func (b *Bot) Start(ctx context.Context) error {
	// Resolve our own bot user id so we can ignore our own messages (avoid
	// loops) and recognise mentions in plain message events.
	if auth, err := b.web.AuthTestContext(ctx); err == nil {
		b.botUserID = auth.UserID
	}
	go func() {
		for evt := range b.client.Events {
			switch evt.Type {
			case socketmode.EventTypeEventsAPI:
				eventsAPI, ok := evt.Data.(slackevents.EventsAPIEvent)
				if !ok {
					continue
				}
				if evt.Request != nil {
					b.client.Ack(*evt.Request)
				}
				if eventsAPI.Type == slackevents.CallbackEvent {
					switch inner := eventsAPI.InnerEvent.Data.(type) {
					case *slackevents.AppMentionEvent:
						msg := toIncomingMessage(inner)
						log.Printf("slack: received mention chat=%s user=%s thread=%s text=%q", msg.ChatID, inner.User, msg.ThreadID, msg.Text)
						b.handler.Handle(ctx, msg)
					case *slackevents.MessageEvent:
						if msg, ok := b.threadReply(inner); ok {
							log.Printf("slack: received thread reply chat=%s user=%s thread=%s text=%q", msg.ChatID, inner.User, msg.ThreadID, msg.Text)
							b.handler.Handle(ctx, msg)
						}
					}
				}
			case socketmode.EventTypeConnected:
				log.Printf("slack: socket-mode connected")
			case socketmode.EventTypeSlashCommand:
				cmd, ok := evt.Data.(slack.SlashCommand)
				if !ok {
					continue
				}
				log.Printf("slack: received slash %q chat=%s text=%q", cmd.Command, cmd.ChannelID, cmd.Text)
				reply := b.handler.Command(ctx, cmd.ChannelID, cmd.Text)
				if evt.Request != nil {
					b.client.Ack(*evt.Request, map[string]interface{}{
						"response_type": "ephemeral",
						"text":          reply,
					})
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
		MessageID: e.TimeStamp,
		ChatID:    e.Channel,
		ThreadID:  e.ThreadTimeStamp, // "" for a top-level mention
		Text:      stripSlackMention(e.Text),
		Mentioned: true,
		IsGroup:   true,
	}
}

// threadReply maps a plain Slack message event to an IncomingMessage when it is
// a follow-up reply in a thread (not a mention, not from a bot). It returns
// ok=false for anything that should not be treated as a thread continuation:
// non-thread messages, bot/self messages, and messages that @mention us (those
// arrive via app_mention instead, so handling them here would double-fire).
func (b *Bot) threadReply(e *slackevents.MessageEvent) (chatbot.IncomingMessage, bool) {
	if e.ThreadTimeStamp == "" {
		return chatbot.IncomingMessage{}, false
	}
	if e.BotID != "" || e.SubType == "bot_message" || (b.botUserID != "" && e.User == b.botUserID) {
		return chatbot.IncomingMessage{}, false
	}
	if b.botUserID != "" && strings.Contains(e.Text, "<@"+b.botUserID+">") {
		return chatbot.IncomingMessage{}, false
	}
	text := stripSlackMention(e.Text)
	if text == "" {
		return chatbot.IncomingMessage{}, false
	}
	return chatbot.IncomingMessage{
		MessageID: e.TimeStamp,
		ChatID:    e.Channel,
		ThreadID:  e.ThreadTimeStamp,
		Text:      text,
		Mentioned: false,
		IsGroup:   true,
	}, true
}

var slackMentionRe = regexp.MustCompile(`^\s*<@[A-Z0-9]+>\s*`)

// stripSlackMention removes a leading Slack mention token (<@U…>) and trims the
// surrounding whitespace.
func stripSlackMention(text string) string {
	return strings.TrimSpace(slackMentionRe.ReplaceAllString(text, ""))
}
