package feishu

import (
	"testing"

	"github.com/liliang-cn/tagit/internal/chatbot"
)

// compile-time check: feishuContext satisfies chatbot.ContextProvider.
var _ chatbot.ContextProvider = (*feishuContext)(nil)

func TestNewFeishuContextConstructs(t *testing.T) {
	if newFeishuContext("app", "secret") == nil {
		t.Fatal("newFeishuContext returned nil")
	}
}
