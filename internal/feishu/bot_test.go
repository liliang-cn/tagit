package feishu

import (
	"testing"

	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func sp(s string) *string { return &s }

func TestToIncomingMessage(t *testing.T) {
	ev := &larkim.P2MessageReceiveV1{Event: &larkim.P2MessageReceiveV1Data{
		Message: &larkim.EventMessage{
			MessageId:   sp("om_1"),
			ChatId:      sp("oc_1"),
			ChatType:    sp("group"),
			MessageType: sp("text"),
			Content:     sp(`{"text":"@_user_1 do X"}`),
			Mentions:    []*larkim.MentionEvent{{Key: sp("@_user_1")}},
		},
	}}
	msg := toIncomingMessage(ev)
	if msg.MessageID != "om_1" || msg.ChatID != "oc_1" || !msg.IsGroup || !msg.Mentioned || msg.Text != "do X" {
		t.Fatalf("bad mapping: %+v", msg)
	}
}

func TestToIncomingMessageNilSafe(t *testing.T) {
	if got := toIncomingMessage(&larkim.P2MessageReceiveV1{}); got.MessageID != "" {
		t.Fatalf("nil event should map to empty: %+v", got)
	}
}
