package feishu

import (
	"context"
	"log"

	"github.com/liliang-cn/roma/internal/api"
	"github.com/liliang-cn/roma/internal/chatbot"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
	larkws "github.com/larksuite/oapi-sdk-go/v3/ws"
)

// Bot runs the Feishu long-connection event loop and routes @mentions to the shared handler.
type Bot struct {
	cfg     *Config
	handler *chatbot.Handler
}

// NewBot wires the Feishu sender + shared handler over ROMA's api.Client.
func NewBot(cfg *Config, apiClient *api.Client) *Bot {
	snd := NewSender(cfg.AppID, cfg.AppSecret)
	enq := chatbot.NewAPIEnqueuer(apiClient)
	progress := chatbot.NewProgressFunc(apiClient, snd)
	handler := chatbot.NewHandler(cfg.Bindings, enq, snd, progress)
	return &Bot{cfg: cfg, handler: handler}
}

// Start blocks, running the long connection until ctx is cancelled. Reconnects automatically.
func (b *Bot) Start(ctx context.Context) error {
	d := dispatcher.NewEventDispatcher("", "").
		OnP2MessageReceiveV1(func(ctx context.Context, e *larkim.P2MessageReceiveV1) error {
			b.handler.Handle(ctx, toIncomingMessage(e))
			return nil
		})
	cli := larkws.NewClient(b.cfg.AppID, b.cfg.AppSecret,
		larkws.WithEventHandler(d),
		larkws.WithAutoReconnect(true))
	log.Printf("feishu: starting long-connection bot (app=%s, bindings=%d)", b.cfg.AppID, len(b.cfg.Bindings))
	return cli.Start(ctx)
}

func toIncomingMessage(e *larkim.P2MessageReceiveV1) chatbot.IncomingMessage {
	var m chatbot.IncomingMessage
	if e == nil || e.Event == nil || e.Event.Message == nil {
		return m
	}
	msg := e.Event.Message
	m.MessageID = deref(msg.MessageId)
	m.ChatID = deref(msg.ChatId)
	m.IsGroup = deref(msg.ChatType) == "group"
	m.Mentioned = len(msg.Mentions) > 0
	if deref(msg.MessageType) == "text" {
		m.Text = parseTextContent(deref(msg.Content))
	}
	return m
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
