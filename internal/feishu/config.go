package feishu

import (
	"encoding/json"
	"fmt"
	"os"
)

// Binding maps one Feishu group chat to a repo and run defaults.
type Binding struct {
	ChatID string `json:"chat_id"`
	Repo   string `json:"repo"`
	Agent  string `json:"agent,omitempty"`
	Mode   string `json:"mode,omitempty"`
}

// Config is the on-disk Feishu bot configuration (~/.roma/feishu.json).
type Config struct {
	AppID     string    `json:"app_id"`
	AppSecret string    `json:"app_secret"`
	Bindings  []Binding `json:"bindings"`
}

// Load reads the config. A missing file means the feature is disabled:
// it returns (nil, false, nil). A present-but-broken file returns an error.
func Load(path string) (*Config, bool, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("read feishu config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, false, fmt.Errorf("parse feishu config: %w", err)
	}
	if cfg.AppID == "" || cfg.AppSecret == "" {
		return nil, false, fmt.Errorf("feishu config missing app_id/app_secret")
	}
	return &cfg, true, nil
}

// BindingFor returns the binding for a chat id.
func (c *Config) BindingFor(chatID string) (Binding, bool) {
	for _, b := range c.Bindings {
		if b.ChatID == chatID {
			return b, true
		}
	}
	return Binding{}, false
}
