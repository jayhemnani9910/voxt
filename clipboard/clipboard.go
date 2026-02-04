package clipboard

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"golang.design/x/clipboard"
)

var useWayland bool

// Init initializes the clipboard subsystem.
// This must be called before any clipboard operations.
// Returns an error if initialization fails.
func Init() error {
	// Check if running on Wayland
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if _, err := exec.LookPath("wl-copy"); err == nil {
			useWayland = true
			return nil
		}
	}

	// Fall back to X11 clipboard
	err := clipboard.Init()
	if err != nil {
		return fmt.Errorf("failed to initialize clipboard: %w", err)
	}
	return nil
}

// Copy writes the given text to the system clipboard.
// Returns an error if the text is empty or whitespace only.
func Copy(text string) error {
	if strings.TrimSpace(text) == "" {
		return fmt.Errorf("cannot copy empty text to clipboard")
	}

	if useWayland {
		// Use stdin instead of command line args to handle long text
		cmd := exec.Command("wl-copy")
		cmd.Stdin = strings.NewReader(text)
		return cmd.Run()
	}

	// X11 clipboard Write returns a channel that can be used to detect when
	// clipboard ownership is lost, not an error. The write itself is synchronous.
	clipboard.Write(clipboard.FmtText, []byte(text))
	return nil
}
