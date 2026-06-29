package feishu

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/liliang-cn/tagit/internal/chatbot"
)

// Config is the on-disk Feishu bot configuration (~/.tagit/feishu.json).
type Config struct {
	AppID     string           `json:"app_id"`
	AppSecret string           `json:"app_secret"`
	Bindings  chatbot.Bindings `json:"bindings"`
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
func (c *Config) BindingFor(chatID string) (chatbot.Binding, bool) {
	return c.Bindings.For(chatID)
}

type configStore struct {
	path string
	mu   sync.Mutex
}

// NewConfigStore returns a BindingStore backed by the feishu.json at path. It
// does read-modify-write of the whole config file, preserving AppID/AppSecret.
func NewConfigStore(path string) chatbot.BindingStore { return &configStore{path: path} }

// load reads the config file, treating a missing file as an empty config.
func (s *configStore) load() (Config, error) {
	var cfg Config
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read feishu config: %w", err)
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse feishu config: %w", err)
	}
	return cfg, nil
}

func (s *configStore) save(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("encode feishu config: %w", err)
	}
	if err := os.WriteFile(s.path, data, 0o600); err != nil {
		return fmt.Errorf("write feishu config: %w", err)
	}
	return nil
}

// For re-reads the file so external edits and restarts are picked up.
func (s *configStore) For(chatID string) (chatbot.Binding, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.load()
	if err != nil {
		return chatbot.Binding{}, false
	}
	return cfg.Bindings.For(chatID)
}

func (s *configStore) Set(b chatbot.Binding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.load()
	if err != nil {
		return err
	}
	found := false
	for i := range cfg.Bindings {
		if cfg.Bindings[i].ChatID == b.ChatID {
			cfg.Bindings[i] = b
			found = true
			break
		}
	}
	if !found {
		cfg.Bindings = append(cfg.Bindings, b)
	}
	return s.save(cfg)
}

func (s *configStore) Delete(chatID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cfg, err := s.load()
	if err != nil {
		return err
	}
	out := cfg.Bindings[:0]
	for _, b := range cfg.Bindings {
		if b.ChatID != chatID {
			out = append(out, b)
		}
	}
	cfg.Bindings = out
	return s.save(cfg)
}
