package slack

import (
	"testing"

	"github.com/liliang-cn/tagit/internal/chatbot"
)

// compile-time check: slackContext satisfies chatbot.ContextProvider.
var _ chatbot.ContextProvider = (*slackContext)(nil)

func TestNewSlackContextConstructs(t *testing.T) {
	if newSlackContext("xoxb-token") == nil {
		t.Fatal("newSlackContext returned nil")
	}
}
