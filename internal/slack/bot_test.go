package slack

import (
	"context"
	"strings"
	"testing"

	"github.com/liliang-cn/tagit/internal/chatbot"
	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
)

// fakeStore is an in-memory chatbot.BindingStore for slash-command tests.
type fakeStore struct{ m map[string]chatbot.Binding }

func newFakeStore(seed ...chatbot.Binding) *fakeStore {
	s := &fakeStore{m: make(map[string]chatbot.Binding)}
	for _, b := range seed {
		s.m[b.ChatID] = b
	}
	return s
}

func (s *fakeStore) For(chatID string) (chatbot.Binding, bool) {
	b, ok := s.m[chatID]
	return b, ok
}
func (s *fakeStore) Set(b chatbot.Binding) error { s.m[b.ChatID] = b; return nil }
func (s *fakeStore) Delete(chatID string) error  { delete(s.m, chatID); return nil }

type fakeSender struct{}

func (fakeSender) Reply(context.Context, string, string, string) error { return nil }

type fakeEnqueuer struct{}

func (fakeEnqueuer) Submit(context.Context, chatbot.SubmitArgs) (string, error) {
	return "job", nil
}

// TestSlashCommandReply verifies the handler.Command call the bot makes when it
// receives a slack.SlashCommand event — the same mapping done in Start's
// EventTypeSlashCommand case.
func TestSlashCommandReply(t *testing.T) {
	store := newFakeStore(chatbot.Binding{ChatID: "C1", Repo: "/r", Agent: "codex", Mode: "senate"})
	h := chatbot.NewHandler(store, fakeEnqueuer{}, fakeSender{}, nil)

	cmd := slack.SlashCommand{ChannelID: "C1", Text: "status"}
	reply := h.Command(context.Background(), cmd.ChannelID, cmd.Text)
	for _, want := range []string{"/r", "codex", "senate"} {
		if !strings.Contains(reply, want) {
			t.Fatalf("slash status reply missing %q: %q", want, reply)
		}
	}
}

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
