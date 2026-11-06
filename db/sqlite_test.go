package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveKeepsTranscription(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	database, err := Init()
	if err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	saved, err := database.Save("hello", "/tmp/audio.wav", 0)
	if err != nil {
		t.Fatal(err)
	}

	var text string
	row := database.conn.QueryRow("SELECT text FROM transcriptions WHERE id = ?", saved.ID)
	if err := row.Scan(&text); err != nil {
		t.Fatal(err)
	}
	if text != "hello" {
		t.Errorf("stored %q, want hello", text)
	}

	info, err := os.Stat(filepath.Join(home, ".local", "share", "voxt"))
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0700 {
		t.Errorf("data dir mode = %o, want 700", perm)
	}
}
