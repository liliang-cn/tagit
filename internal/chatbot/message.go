package chatbot

// IncomingMessage is the normalized, platform-agnostic form of a received chat message.
type IncomingMessage struct {
	MessageID string // platform message id / ts of this specific message
	ChatID    string // group/channel id
	ThreadID  string // canonical thread-root id; replies post here. "" if the message neither sits in nor opens a thread.
	Text      string // plain text, mention tokens stripped (adapter's job)
	Mentioned bool   // the bot was @mentioned
	IsGroup   bool   // came from a group/channel (not a 1:1 DM)
	FromBot   bool   // authored by a bot / our own app — never react (avoids loops)
}

// ReplyRoot is the thread id replies should attach to: the thread root when the
// message is in/opening a thread, otherwise the message itself.
func (m IncomingMessage) ReplyRoot() string {
	if m.ThreadID != "" {
		return m.ThreadID
	}
	return m.MessageID
}
