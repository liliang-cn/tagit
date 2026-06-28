package feishu

import (
	"encoding/json"
	"regexp"
	"strings"
)

// IncomingMessage is the normalized form of a received Feishu message.
type IncomingMessage struct {
	MessageID string
	ChatID    string
	ChatType  string // "group" | "p2p" | "topic_group"
	Text      string // mention tokens stripped
	Mentioned bool   // the bot was @mentioned
}

var mentionToken = regexp.MustCompile(`@_user_\d+\s*`)

// parseTextContent extracts plain text from a Feishu text-message Content JSON
// ({"text":"..."}) and strips @mention placeholder tokens. Non-text → "".
func parseTextContent(contentJSON string) string {
	var c struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(contentJSON), &c); err != nil {
		return ""
	}
	return strings.TrimSpace(mentionToken.ReplaceAllString(c.Text, ""))
}
