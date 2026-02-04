package tray

import (
	"github.com/getlantern/systray"
)

// Tray manages the system tray icon and menu
type Tray struct {
	idleIcon      []byte
	recordingIcon []byte

	settingsItem *systray.MenuItem
	quitItem     *systray.MenuItem

	onSettings func()
	onQuit     func()

	done chan struct{}
}

// NewTray creates a new system tray manager
func NewTray(idleIcon, recordingIcon []byte) *Tray {
	return &Tray{
		idleIcon:      idleIcon,
		recordingIcon: recordingIcon,
		done:          make(chan struct{}),
	}
}

// SetOnSettings sets the callback for when Settings is clicked
func (t *Tray) SetOnSettings(callback func()) {
	if callback == nil {
		return
	}
	t.onSettings = callback
}

// SetOnQuit sets the callback for when Quit is clicked
func (t *Tray) SetOnQuit(callback func()) {
	if callback == nil {
		return
	}
	t.onQuit = callback
}

// Run starts the system tray event loop (blocking)
func (t *Tray) Run(onReady func()) {
	systray.Run(func() {
		t.onReady()
		if onReady != nil {
			onReady()
		}
	}, t.onExit)
}

// onReady is called when the tray is ready
func (t *Tray) onReady() {
	systray.SetIcon(t.idleIcon)
	systray.SetTitle("")
	systray.SetTooltip("Voxt - Ready")

	t.settingsItem = systray.AddMenuItem("Settings", "Open settings")
	systray.AddSeparator()
	t.quitItem = systray.AddMenuItem("Quit", "Exit Voxt")

	// Handle menu clicks
	go func() {
		for {
			select {
			case <-t.settingsItem.ClickedCh:
				if t.onSettings != nil {
					t.onSettings()
				}
			case <-t.quitItem.ClickedCh:
				if t.onQuit != nil {
					t.onQuit()
				}
				systray.Quit()
				return
			case <-t.done:
				return
			}
		}
	}()
}

// onExit is called when the tray is exiting
func (t *Tray) onExit() {
	close(t.done)
}

// SetIdle sets the tray to idle state (green icon)
func (t *Tray) SetIdle() {
	systray.SetIcon(t.idleIcon)
	systray.SetTooltip("Voxt - Ready")
}

// SetRecording sets the tray to recording state (red icon)
func (t *Tray) SetRecording() {
	systray.SetIcon(t.recordingIcon)
	systray.SetTooltip("Voxt - Recording...")
}

// Quit exits the system tray
func (t *Tray) Quit() {
	systray.Quit()
}
