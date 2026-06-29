package feishu

import (
	"context"
	"strings"

	lark "github.com/larksuite/oapi-sdk-go/v3"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"github.com/liliang-cn/tagit/internal/chatbot"
)

const feishuContextMaxChars = 3000

type feishuContext struct{ cli *lark.Client }

// newFeishuContext builds a chatbot.ContextProvider backed by the Feishu API.
func newFeishuContext(appID, appSecret string) chatbot.ContextProvider {
	return &feishuContext{cli: lark.NewClient(appID, appSecret)}
}

// RecentContext lists up to ~20 of the most recent messages in the chat, keeps
// only text messages, and returns them oldest→newest as plain lines. Best-effort:
// any API error returns "".
func (c *feishuContext) RecentContext(ctx context.Context, chatID, _ string) string {
	req := larkim.NewListMessageReqBuilder().
		ContainerIdType("chat").
		ContainerId(chatID).
		SortType("ByCreateTimeDesc").
		PageSize(20).
		Build()
	resp, err := c.cli.Im.Message.List(ctx, req)
	if err != nil || !resp.Success() || resp.Data == nil {
		return ""
	}

	// Items come newest→oldest; collect text and reverse to oldest→newest.
	var lines []string
	for _, m := range resp.Data.Items {
		if m == nil || deref(m.MsgType) != "text" || m.Body == nil {
			continue
		}
		text := parseTextContent(deref(m.Body.Content))
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

	// Cap total to ~feishuContextMaxChars, dropping oldest lines if over.
	for {
		out := strings.Join(lines, "\n")
		if len(out) <= feishuContextMaxChars || len(lines) == 1 {
			return out
		}
		lines = lines[1:]
	}
}
