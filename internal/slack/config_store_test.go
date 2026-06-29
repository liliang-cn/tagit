package slack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/liliang-cn/tagit/internal/chatbot"
)

func TestConfigStoreRoundTripPreservesCreds(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "slack.json")

	seed := Config{BotToken: "xoxb-123", AppToken: "xapp-456"}
	data, _ := json.MarshalIndent(seed, "", "  ")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	store := NewConfigStore(path)

	if _, ok := store.For("c1"); ok {
		t.Fatal("expected no binding initially")
	}

	if err := store.Set(chatbot.Binding{ChatID: "c1", Repo: "/r", Agent: "codex", Mode: "senate"}); err != nil {
		t.Fatal(err)
	}

	got, ok := NewConfigStore(path).For("c1")
	if !ok || got.Repo != "/r" || got.Agent != "codex" || got.Mode != "senate" {
		t.Fatalf("For(c1) = %+v ok=%v", got, ok)
	}

	cfg, enabled, err := Load(path)
	if err != nil || !enabled {
		t.Fatalf("Load after Set: enabled=%v err=%v", enabled, err)
	}
	if cfg.BotToken != "xoxb-123" || cfg.AppToken != "xapp-456" {
		t.Fatalf("creds dropped: %+v", cfg)
	}

	if err := store.Delete("c1"); err != nil {
		t.Fatal(err)
	}
	if _, ok := store.For("c1"); ok {
		t.Fatal("binding should be deleted")
	}
	cfg, _, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.BotToken != "xoxb-123" || cfg.AppToken != "xapp-456" {
		t.Fatalf("creds dropped after delete: %+v", cfg)
	}
}

func TestConfigStoreMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "absent.json")
	store := NewConfigStore(path)
	if _, ok := store.For("c1"); ok {
		t.Fatal("missing file should yield no bindings")
	}
	if err := store.Set(chatbot.Binding{ChatID: "c1", Repo: "/r"}); err != nil {
		t.Fatal(err)
	}
	if got, ok := store.For("c1"); !ok || got.Repo != "/r" {
		t.Fatalf("For after Set on missing file = %+v ok=%v", got, ok)
	}
}
