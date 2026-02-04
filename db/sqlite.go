package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Transcription represents a stored transcription record
type Transcription struct {
	ID         int64
	Timestamp  time.Time
	Text       string
	AudioPath  string
	DurationMS int64
}

// DB handles SQLite database operations for transcription history
type DB struct {
	conn *sql.DB
	path string
}

// Init creates and initializes the database
func Init() (*DB, error) {
	// Create data directory
	dataDir, err := getDataDir()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}

	dbPath := filepath.Join(dataDir, "history.db")

	conn, err := sql.Open("sqlite3", dbPath+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	conn.SetMaxOpenConns(25)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	db := &DB{conn: conn, path: dbPath}

	if err := db.migrate(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return db, nil
}

// getDataDir returns ~/.local/share/voxt
func getDataDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".local", "share", "voxt"), nil
}

// migrate creates the database schema
func (db *DB) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS transcriptions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		text TEXT NOT NULL,
		audio_path TEXT,
		duration_ms INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_timestamp ON transcriptions(timestamp DESC);
	CREATE INDEX IF NOT EXISTS idx_text ON transcriptions(text);
	`

	_, err := db.conn.Exec(schema)
	return err
}

// Save stores a new transcription record
func (db *DB) Save(text, audioPath string, durationMS int64) (*Transcription, error) {
	tx, err := db.conn.Begin()
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.Exec(
		"INSERT INTO transcriptions (text, audio_path, duration_ms) VALUES (?, ?, ?)",
		text, audioPath, durationMS,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to save transcription: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get insert ID: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &Transcription{
		ID:         id,
		Timestamp:  time.Now(),
		Text:       text,
		AudioPath:  audioPath,
		DurationMS: durationMS,
	}, nil
}

// Close closes the database connection
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}
