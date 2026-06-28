package feishu

import (
	"encoding/json"
	"regexp"
	"strings"
)

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
