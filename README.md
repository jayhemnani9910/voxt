# Voxt

A voice-to-text clipboard tool for Linux. Press a hotkey, speak, and get your transcription copied to clipboard.

## Features

- **Global Hotkey**: Press F9 to start/stop recording (configurable)
- **Fast Transcription**: Uses Groq's Whisper API for quick, accurate results
- **Clipboard Integration**: Transcribed text automatically copied to clipboard
- **Desktop Notifications**: Visual feedback when transcription is ready
- **System Tray**: Minimal UI with tray icon showing recording state
- **History**: SQLite database stores all transcriptions
- **Wayland & X11**: Works on both display servers

## Requirements

### System Dependencies

**Ubuntu/Debian:**
```bash
sudo apt install portaudio19-dev libayatana-appindicator3-dev
```

**Fedora:**
```bash
sudo dnf install portaudio-devel libayatana-appindicator-gtk3-devel
```

### Input Group Access

To capture global hotkeys, your user must be in the `input` group:
```bash
sudo usermod -a -G input $USER
```
Log out and back in for this to take effect.

### Groq API Key

Get a free API key from [console.groq.com](https://console.groq.com)

## Installation

### From Source

```bash
# Clone the repository
git clone https://github.com/jayhemnani9910/voxt.git
cd voxt

# Check dependencies
make check

# Build
make build

# Install to /usr/local/bin (optional)
make install
```

### Configuration

On first run, voxt creates a config file at `~/.config/voxt/config.json`:

```json
{
  "groq_api_key": "your-api-key-here",
  "hotkey": "F9",
  "recordings_dir": "~/recordings",
  "language_mode": "romanized",
  "auto_start": false
}
```

Edit this file to add your Groq API key.

## Usage

1. Run `voxt` (or it starts automatically if autostart is enabled)
2. Look for the green tray icon
3. Press **F9** to start recording (icon turns red, beep plays)
4. Speak your message
5. Press **F9** again to stop (beep plays, transcription starts)
6. Text is copied to clipboard and notification appears

### Tray Menu

- **Settings**: Opens config file for editing
- **Quit**: Exit voxt

## Supported Hotkeys

F1, F2, F3, F4, F5, F6, F7, F8, F9 (default), F10, F11, F12

## File Locations

| File | Location |
|------|----------|
| Config | `~/.config/voxt/config.json` |
| Database | `~/.local/share/voxt/history.db` |
| Recordings | `~/recordings/YYYY-MM-DD/` |

## Build Targets

```bash
make build      # Build binary
make release    # Build optimized release
make install    # Install to /usr/local/bin
make check      # Verify system dependencies
make clean      # Remove build artifacts
make help       # Show all targets
```

## Tech Stack

- **Language**: Go
- **Audio**: PortAudio
- **Transcription**: Groq Whisper API
- **System Tray**: getlantern/systray
- **Notifications**: DBus (freedesktop)
- **Clipboard**: Wayland (wl-copy) / X11
- **Database**: SQLite

## License

MIT
