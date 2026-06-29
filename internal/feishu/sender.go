package feishu

import (
	"context"
	"encoding/json"
	"fmt"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/liliang-cn/tagit/internal/chatbot"
)

type larkSender struct{ cli *lark.Client }

// NewSender builds a Feishu-API-backed chatbot.Sender.
func NewSender(appID, appSecret string) chatbot.Sender {
	return &larkSender{cli: lark.NewClient(appID, appSecret)}
}

// Reply ignores chatID: Feishu replies target the message_id (rootMessageID),
// which already implies the chat.
func (s *larkSender) Reply(ctx context.Context, _ string, rootMessageID, text string) error {
	req := larkim.NewReplyMessageReqBuilder().
		MessageId(rootMessageID).
		Body(larkim.NewReplyMessageReqBodyBuilder().
			MsgType("text").
			Content(textContentJSON(text)).
			Build()).
		Build()
	resp, err := s.cli.Im.Message.Reply(ctx, req)
	if err != nil {
		return fmt.Errorf("feishu reply: %w", err)
	}
	if !resp.Success() {
		return fmt.Errorf("feishu reply failed: code=%d msg=%s", resp.Code, resp.Msg)
	}
	return nil
}

// textContentJSON encodes plain text into Feishu's text message content JSON.
func textContentJSON(text string) string {
	b, _ := json.Marshal(struct {
		Text string `json:"text"`
	}{Text: text})
	return string(b)
}
