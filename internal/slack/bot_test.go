package slack

import (
	"testing"

	"github.com/liliang-cn/tagit/internal/chatbot"
	"github.com/slack-go/slack/slackevents"
)

func TestToIncomingMessageStripsMention(t *testing.T) {
	got := toIncomingMessage(&slackevents.AppMentionEvent{
		TimeStamp: "123.45",
		Channel:   "C1",
		Text:      "<@U123> do X",
	})
	want := chatbot.IncomingMessage{
		MessageID: "123.45",
		ChatID:    "C1",
		Text:      "do X",
		Mentioned: true,
		IsGroup:   true,
	}
	if got != want {
		t.Fatalf("toIncomingMessage = %+v, want %+v", got, want)
	}
}

func TestStripSlackMentionPlain(t *testing.T) {
	if got := stripSlackMention("  hello world  "); got != "hello world" {
		t.Fatalf("stripSlackMention = %q, want %q", got, "hello world")
	}
}
