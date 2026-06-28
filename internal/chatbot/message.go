package chatbot

// IncomingMessage is the normalized, platform-agnostic form of a received chat message.
type IncomingMessage struct {
	MessageID string // platform message id / ts used as the thread root for replies
	ChatID    string // group/channel id
	Text      string // plain text, mention tokens stripped (adapter's job)
	Mentioned bool   // the bot was @mentioned
	IsGroup   bool   // came from a group/channel (not a 1:1 DM)
}
