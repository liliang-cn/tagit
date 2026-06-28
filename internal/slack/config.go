package slack

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/liliang-cn/roma/internal/chatbot"
)

// Config is the on-disk Slack bot configuration (~/.roma/slack.json): a bot
// token (xoxb-) used for Web API calls, an app-level token (xapp-) for the
// Socket Mode connection, plus chat-to-repo bindings.
type Config struct {
	BotToken string           `json:"bot_token"`
	AppToken string           `json:"app_token"`
	Bindings chatbot.Bindings `json:"bindings"`
}

// Load reads the config. A missing file means the feature is disabled:
// it returns (nil, false, nil). A present-but-broken file returns an error.
func Load(path string) (*Config, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read slack config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, false, fmt.Errorf("parse slack config: %w", err)
	}
	if cfg.BotToken == "" || cfg.AppToken == "" {
		return nil, false, fmt.Errorf("slack config missing bot_token/app_token")
	}
	return &cfg, true, nil
}

// BindingFor returns the binding for a chat id.
func (c *Config) BindingFor(chatID string) (chatbot.Binding, bool) {
	return c.Bindings.For(chatID)
}
