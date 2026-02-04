package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var configMutex sync.RWMutex

// Config represents the application configuration
type Config struct {
	GroqAPIKey    string `json:"groq_api_key"`
	Hotkey        string `json:"hotkey"`
	RecordingsDir string `json:"recordings_dir"`
	LanguageMode  string `json:"language_mode"`
	AutoStart     bool   `json:"auto_start"`
	OverlayX      int    `json:"overlay_x"`
	OverlayY      int    `json:"overlay_y"`
}

// DefaultConfig returns a Config with default values
func DefaultConfig() *Config {
	return &Config{
		GroqAPIKey:    "",
		Hotkey:        "F9",
		RecordingsDir: "~/recordings",
		LanguageMode:  "romanized",
		AutoStart:     false,
		OverlayX:      -1,
		OverlayY:      -1,
	}
}

// Load reads the configuration from ~/.config/voxt/config.json
// If the file doesn't exist, it creates a default config file
func Load() (*Config, error) {
	configMutex.RLock()
	defer configMutex.RUnlock()

	configPath, err := getConfigPath()
	if err != nil {
		return nil, err
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		cfg := DefaultConfig()
		if err := cfg.Save(); err != nil {
			return nil, err
		}
		return cfg, nil
	}

	// Read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	// Apply defaults for any missing fields
	if cfg.Hotkey == "" {
		cfg.Hotkey = "F9"
	}
	if cfg.LanguageMode == "" {
		cfg.LanguageMode = "romanized"
	}
	if cfg.OverlayX == 0 {
		cfg.OverlayX = -1
	}
	if cfg.OverlayY == 0 {
		cfg.OverlayY = -1
	}

	// Expand ~ in RecordingsDir
	cfg.RecordingsDir = ExpandPath(cfg.RecordingsDir)

	return cfg, nil
}

// Save writes the configuration to ~/.config/voxt/config.json
// Uses 0600 permissions since config contains API key
func (c *Config) Save() error {
	configMutex.Lock()
	defer configMutex.Unlock()

	configPath, err := getConfigPath()
	if err != nil {
		return err
	}

	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}

	// Marshal config to JSON with indentation
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}

	// Write to file with restricted permissions (contains API key)
	return os.WriteFile(configPath, data, 0600)
}

// getConfigPath returns the full path to the config file
func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "voxt", "config.json"), nil
}

// GetConfigPath returns the config file path (public version)
func GetConfigPath() (string, error) {
	return getConfigPath()
}

// ExpandPath expands ~ to the user's home directory
func ExpandPath(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return filepath.Clean(path)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Clean(path)
	}

	if len(path) == 1 {
		return filepath.Clean(home)
	}

	if path[1] == '/' || path[1] == filepath.Separator {
		return filepath.Clean(filepath.Join(home, path[2:]))
	}

	return filepath.Clean(path)
}

// SetAutoStart manages the autostart desktop file
func (c *Config) SetAutoStart(enabled bool) error {
	autostartFile, err := getAutostartPath()
	if err != nil {
		return err
	}

	if enabled {
		// Create autostart directory if it doesn't exist
		if err := os.MkdirAll(filepath.Dir(autostartFile), 0755); err != nil {
			return err
		}

		// Get executable path
		execPath, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		execPath, err = filepath.EvalSymlinks(execPath)
		if err != nil {
			return fmt.Errorf("failed to resolve executable path: %w", err)
		}

		// Create desktop file
		desktopContent := fmt.Sprintf(`[Desktop Entry]
Type=Application
Name=Voxt
Exec=%s
Hidden=false
X-GNOME-Autostart-enabled=true
`, execPath)
		if err := os.WriteFile(autostartFile, []byte(desktopContent), 0644); err != nil {
			return err
		}
	} else {
		// Remove autostart file if it exists
		os.Remove(autostartFile)
	}

	c.AutoStart = enabled
	return c.Save()
}

// getAutostartPath returns the path to the autostart desktop file
func getAutostartPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "autostart", "voxt.desktop"), nil
}

// GetAutoStart checks if autostart file exists
func (c *Config) GetAutoStart() bool {
	autostartFile, err := getAutostartPath()
	if err != nil {
		return false
	}
	_, err = os.Stat(autostartFile)
	return err == nil
}
