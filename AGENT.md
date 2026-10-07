# AGENT.md — voxt Project Guide

This document provides comprehensive information for AI agents working on the voxt codebase.

---

## Project Overview

**voxt** is a local voice-to-text clipboard tool for Linux (Ubuntu 25.10, Wayland/GNOME 49).

### Core Workflow
1. User presses F9 → recording starts (beep, tray icon turns red)
2. User speaks in English, Gujarati, Hindi, or Sindhi
3. User presses F9 again → recording stops (beep, tray icon turns green)
4. Audio saved locally as WAV file
5. Audio sent to Groq API for transcription (Whisper model)
6. Notification shows transcribed text for 3 seconds
7. Text auto-copied to clipboard (romanized if non-English)

### Why This Exists
- User thinks in Gujarati/Hindi/Sindhi, types in English
- Speaking is faster than typing
- Zero-friction voice-to-clipboard workflow
- No local GPU usage (API-based transcription)
- All output in English alphabets (romanized)

### Language Support
| Language | Input | Output Example |
|----------|-------|----------------|
| English | Speech | "hello how are you" |
| Gujarati | Speech | "kem cho shu haal chhe" |
| Hindi | Speech | "kya haal hai bhai" |
| Sindhi | Speech | "twanjo nalo cha aye" |

All non-English languages are automatically romanized to English alphabets.

---

## Technology Stack

| Component | Technology | Notes |
|-----------|------------|-------|
| Language | Go 1.21+ | Single binary, embedded assets |
| Audio Recording | PortAudio | `github.com/gordonklaus/portaudio` |
| System Tray | systray | `github.com/getlantern/systray` (AppIndicator) |
| Hotkey | evdev | Direct `/dev/input/` access (Wayland-compatible) |
| Notifications | DBus | `github.com/esiqveland/notify` + `github.com/godbus/dbus/v5` |
| Clipboard | clipboard | `golang.design/x/clipboard` (Wayland-compatible) |
| Transcription | Groq API | Whisper-large-v3, standard `net/http` |
| Database | SQLite | `github.com/mattn/go-sqlite3` |
| Keyring | go-keyring | `github.com/zalando/go-keyring` |

---

## File Structure

```
voxt/
├── main.go                 # Application entry point, orchestration
├── go.mod                  # Go module definition with dependencies
├── go.sum                  # Dependency checksums (generated)
├── AGENT.md                # This file
│
├── config/
│   └── config.go           # Configuration management (~/.config/voxt/config.json)
│
├── audio/
│   └── recorder.go         # Audio recording and WAV file creation
│
├── notify/
│   └── notify.go           # Desktop notification system (DBus)
│
├── clipboard/
│   └── clipboard.go        # Clipboard operations
│
├── transcribe/
│   └── groq.go             # Groq Whisper API client with romanization prompt
│
├── hotkey/
│   └── hotkey.go           # Global hotkey listener (evdev)
│
├── tray/
│   └── tray.go             # System tray icon and menu
│
├── db/
│   └── sqlite.go           # SQLite database for transcription history
│
├── keyring/
│   └── keyring.go          # Secure API key storage
│
├── ui/
│   └── panel.go            # Minimal GTK4 settings panel (optional)
│
├── assets/
│   ├── icon_idle.png       # Green circle, 22x22px (idle state)
│   ├── icon_recording.png  # Red circle, 22x22px (recording state)
│   ├── beep_start.wav      # 880Hz sine wave, 0.1s (recording start)
│   └── beep_stop.wav       # 440Hz sine wave, 0.1s (recording stop)
│
└── cmd/
    └── genassets/
        └── main.go         # Tool to regenerate asset files
```

---

## Component Details

### 1. main.go — Application Core

**Responsibilities:**
- Initialize all components in correct order
- Wire components together
- Handle application lifecycle
- Manage recording state machine

**Key Structure:**
```go
type App struct {
    tray           *tray.Tray
    notifier       *notify.Notifier
    config         *config.Config
    recorder       *audio.Recorder
    transcriber    *transcribe.Transcriber
    database       *db.DB
    hotkeyListener *hotkey.HotkeyListener
    state          AppState
    mu             sync.Mutex
}
```

**State Machine:**
- `StateIdle` + F9 pressed → `startRecording()`
- `StateRecording` + F9 pressed → `stopRecording()`
- `StateTranscribing` + F9 pressed → ignored (already processing)

**Embedded Assets:**
```go
//go:embed assets/icon_idle.png assets/icon_recording.png
var iconFS embed.FS

//go:embed assets/beep_start.wav assets/beep_stop.wav
var soundFS embed.FS
```

---

### 2. tray/tray.go — System Tray

**Technology:** `github.com/getlantern/systray` (uses AppIndicator on Linux)

**Key Structure:**
```go
type Tray struct {
    idleIcon      []byte
    recordingIcon []byte
    settingsItem  *systray.MenuItem
    quitItem      *systray.MenuItem
    onSettings    func()
}
```

**Key Functions:**
| Function | Description |
|----------|-------------|
| `NewTray(idleIcon, recordingIcon []byte) *Tray` | Create tray with icons |
| `(t *Tray) Run(onReady func())` | Start tray event loop (blocking) |
| `(t *Tray) SetIdle()` | Switch to green icon, "Ready" tooltip |
| `(t *Tray) SetRecording()` | Switch to red icon, "Recording" tooltip |

**Menu Structure:**
1. "Settings" — Opens config file with xdg-open
2. ─────────── (separator)
3. "Quit" — Exits application

**Icon States:**
| State | Icon | Tooltip |
|-------|------|---------|
| Idle | Green circle | "Voxt - Ready" |
| Recording | Red circle | "Voxt - Recording..." |

---

### 3. notify/notify.go — Desktop Notifications

**Technology:** DBus notifications (freedesktop.org standard)

**Key Functions:**
| Function | Description |
|----------|-------------|
| `NewNotifier() (*Notifier, error)` | Connect to DBus session bus |
| `(n *Notifier) ShowTranscription(text string) error` | Show notification with transcribed text (3s) |
| `(n *Notifier) ShowError(message string) error` | Show error notification (5s) |
| `(n *Notifier) Close() error` | Close DBus connection |

**Notification App Name:** "Voxt"

**Usage:**
- Transcription complete: Shows text for 3 seconds
- Errors: Shows error message for 5 seconds

---

### 4. transcribe/groq.go — Groq API Client

**API Endpoint:** `https://api.groq.com/openai/v1/audio/transcriptions`

**Model:** `whisper-large-v3`

**Romanization Prompt:**
```
Transcribe using English alphabet only. Write Gujarati, Hindi, and Sindhi words in romanized form.
Examples:
- Gujarati: "kem cho, shu haal chhe" not "કેમ છો"
- Hindi: "kya haal hai bhai" not "क्या हाल है"
- Sindhi: "twanjo nalo cha aye" not "توهان جو نالو ڇا آهي"
Always use Latin script. Never use Devanagari, Gujarati, or Arabic script.
```

**Key Functions:**
| Function | Description |
|----------|-------------|
| `NewTranscriber(apiKey string) *Transcriber` | Create new transcriber |
| `(t *Transcriber) Transcribe(audioPath string) (string, error)` | Send audio file, return romanized text |

---

### 5. config/config.go — Configuration Management

**Config File Location:** `~/.config/voxt/config.json`

**Config Structure:**
```go
type Config struct {
    GroqAPIKey    string `json:"groq_api_key"`
    Hotkey        string `json:"hotkey"`
    RecordingsDir string `json:"recordings_dir"`
    LanguageMode  string `json:"language_mode"`
    AutoStart     bool   `json:"auto_start"`
    OverlayX      int    `json:"overlay_x"`
    OverlayY      int    `json:"overlay_y"`
}
```

**Default Values:**
```json
{
    "groq_api_key": "",
    "hotkey": "F9",
    "recordings_dir": "~/recordings",
    "language_mode": "romanized",
    "auto_start": false
}
```

---

### 6. hotkey/hotkey.go — Global Hotkey Listener

**Technology:** Linux evdev (direct `/dev/input/event*` access)

**Why evdev:**
- Works on Wayland (X11 hotkey libraries don't work on Wayland)
- No root required (needs `input` group membership)
- Monitors all keyboard devices simultaneously

**Supported Keys:** F1-F12

**Permission Requirements:**
```bash
sudo usermod -a -G input $USER
# Then log out and back in
```

---

### 7. audio/recorder.go — Audio Recording

**Audio Format (Groq API compatible):**
| Parameter | Value |
|-----------|-------|
| Sample Rate | 16000 Hz (16 kHz) |
| Channels | 1 (Mono) |
| Bit Depth | 16-bit |
| Format | WAV (RIFF/PCM) |

**File Naming Convention:**
```
~/recordings/YYYY-MM-DD/audio_HHMMSS.wav
```

---

### 8. db/sqlite.go — History Database

**Database Location:** `~/.local/share/voxt/history.db`

**Schema:**
```sql
CREATE TABLE transcriptions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
    text TEXT NOT NULL,
    audio_path TEXT,
    duration_ms INTEGER
);
```

---

## Build Instructions

### Development Build
```bash
cd voxt
go mod tidy
go build -o voxt
./voxt
```

### Release Build (smaller binary)
```bash
go build -ldflags="-s -w" -o voxt
```

### Install
```bash
sudo cp voxt /usr/local/bin/
```

---

## Configuration

### Getting a Groq API Key
1. Go to https://console.groq.com/
2. Sign up / Log in
3. Go to API Keys
4. Create new key
5. Add to `~/.config/voxt/config.json`

### Setting Hotkey
Edit `~/.config/voxt/config.json`:
```json
{
    "hotkey": "F9"
}
```
Supported: F1-F12 (requires app restart)

---

## Runtime Behavior

### Normal Operation
1. Tray icon appears (green)
2. Press F9 → beep, icon turns red, recording starts
3. Speak in English, Gujarati, Hindi, or Sindhi
4. Press F9 → beep, icon turns green, recording stops
5. Audio saved to `~/recordings/YYYY-MM-DD/audio_HHMMSS.wav`
6. Audio sent to Groq API with romanization prompt
7. Notification shows transcribed text (3 seconds)
8. Text auto-copied to clipboard

### Error States
| Error | Notification | Recovery |
|-------|--------------|----------|
| No API key | "No API key configured" | Edit config |
| No microphone | "Microphone not available" | Check audio settings |
| Hotkey permission | Log warning | Add user to input group |
| Network error | "Transcription failed" | Audio already saved locally |
| API error | "Transcription failed" | Audio already saved locally |

---

## Dependencies

### System Dependencies (apt)
```bash
sudo apt install libportaudio2 libportaudio-dev libayatana-appindicator3-dev
```

### User Permissions
```bash
sudo usermod -a -G input $USER  # For hotkey access
# Log out and back in
```

---

## Common Agent Tasks

### Adding a New Language
1. Edit `transcribe/groq.go`
2. Add example to `romanizationPrompt` constant
3. Follow the pattern: `- Language: "romanized example" not "native script"`

### Changing Hotkey Options
1. Edit `hotkey/hotkey.go`
2. Add key code to `keyNameToCode` map
3. Key codes are in `/usr/include/linux/input-event-codes.h`

### Adding New Tray Menu Items
1. Edit `tray/tray.go`
2. Add new `*systray.MenuItem` field
3. Create item in `onReady()` with `systray.AddMenuItem()`
4. Handle clicks in the goroutine

---

## Testing Checklist

- [ ] App starts without errors
- [ ] Tray icon appears (green)
- [ ] F9 starts recording (beep, red icon)
- [ ] F9 stops recording (beep, green icon)
- [ ] WAV file saved to correct location
- [ ] Notification shows transcribed text
- [ ] Text copied to clipboard
- [ ] Gujarati romanized correctly ("kem cho")
- [ ] Hindi romanized correctly ("kya haal hai")
- [ ] Sindhi romanized correctly ("twanjo nalo")
- [ ] Settings menu opens config file
- [ ] Quit menu exits app cleanly
