package chatbot

// Binding maps one chat (Feishu chat_id / Slack channel id) to a repo + run defaults.
type Binding struct {
	ChatID string `json:"chat_id"`
	Repo   string `json:"repo"`
	Agent  string `json:"agent,omitempty"`
	Mode   string `json:"mode,omitempty"`
}

// Bindings is a lookup set.
type Bindings []Binding

// For returns the binding for a chat id.
func (bs Bindings) For(chatID string) (Binding, bool) {
	for _, b := range bs {
		if b.ChatID == chatID {
			return b, true
		}
	}
	return Binding{}, false
}
