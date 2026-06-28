package slack

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileDisabled(t *testing.T) {
	cfg, enabled, err := Load(filepath.Join(t.TempDir(), "nope.json"))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if enabled || cfg != nil {
		t.Fatalf("missing file should be disabled; got enabled=%v cfg=%v", enabled, cfg)
	}
}

func TestLoadAndBindingLookup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "slack.json")
	if err := os.WriteFile(path, []byte(`{
	  "bot_token":"xoxb-1","app_token":"xapp-1",
	  "bindings":[{"chat_id":"C1","repo":"/r","agent":"codex","mode":"rage"}]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, enabled, err := Load(path)
	if err != nil || !enabled {
		t.Fatalf("Load() enabled=%v err=%v", enabled, err)
	}
	if cfg.BotToken != "xoxb-1" || cfg.AppToken != "xapp-1" {
		t.Fatalf("creds not parsed: %+v", cfg)
	}
	b, ok := cfg.BindingFor("C1")
	if !ok || b.Repo != "/r" || b.Agent != "codex" || b.Mode != "rage" {
		t.Fatalf("BindingFor() = %+v ok=%v", b, ok)
	}
	if _, ok := cfg.BindingFor("C_unknown"); ok {
		t.Fatal("unknown chat must not resolve")
	}
}

func TestLoadMissingTokensErrors(t *testing.T) {
	cases := map[string]string{
		"no bot token": `{"app_token":"xapp-1"}`,
		"no app token": `{"bot_token":"xoxb-1"}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "slack.json")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, _, err := Load(path); err == nil {
				t.Fatal("expected error for missing token")
			}
		})
	}
}

func TestLoadMalformedErrors(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := Load(path); err == nil {
		t.Fatal("malformed config should error")
	}
}
