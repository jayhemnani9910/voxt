package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCreatesDefaultOnFirstRun(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Hotkey != "F9" {
		t.Errorf("Hotkey = %q, want F9", cfg.Hotkey)
	}

	// The file holds the API key, so only the owner may read it.
	info, err := os.Stat(filepath.Join(home, ".config", "voxt", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Errorf("config mode = %o, want 600", perm)
	}
}

func TestLoadReadsExistingFile(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	path := filepath.Join(home, ".config", "voxt", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	data := `{"groq_api_key": "test", "recordings_dir": "~/clips"}`
	if err := os.WriteFile(path, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GroqAPIKey != "test" {
		t.Errorf("GroqAPIKey = %q, want test", cfg.GroqAPIKey)
	}
	if want := filepath.Join(home, "clips"); cfg.RecordingsDir != want {
		t.Errorf("RecordingsDir = %q, want %q", cfg.RecordingsDir, want)
	}
	if cfg.Hotkey != "F9" {
		t.Errorf("missing hotkey should default to F9, got %q", cfg.Hotkey)
	}
}
