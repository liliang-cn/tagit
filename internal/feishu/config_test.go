package feishu

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
	path := filepath.Join(t.TempDir(), "feishu.json")
	if err := os.WriteFile(path, []byte(`{
	  "app_id":"cli_x","app_secret":"sec",
	  "bindings":[{"chat_id":"oc_1","repo":"/r","agent":"codex","mode":"rage"}]
	}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, enabled, err := Load(path)
	if err != nil || !enabled {
		t.Fatalf("Load() enabled=%v err=%v", enabled, err)
	}
	if cfg.AppID != "cli_x" || cfg.AppSecret != "sec" {
		t.Fatalf("creds not parsed: %+v", cfg)
	}
	b, ok := cfg.BindingFor("oc_1")
	if !ok || b.Repo != "/r" || b.Agent != "codex" || b.Mode != "rage" {
		t.Fatalf("BindingFor() = %+v ok=%v", b, ok)
	}
	if _, ok := cfg.BindingFor("oc_unknown"); ok {
		t.Fatal("unknown chat must not resolve")
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
