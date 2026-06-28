package chatbot

import "context"

// Sender posts text into a chat thread, replying to the triggering message
// (Feishu: reply to message_id; Slack: post into chatID with thread_ts = the
// message ts).
type Sender interface {
	Reply(ctx context.Context, chatID, rootMessageID, text string) error
}
