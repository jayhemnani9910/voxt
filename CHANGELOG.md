# Changelog

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

## [Unreleased]

### Added

- Demo GIF, MIT license file, CI, tests, contributing guide, issue and PR templates, this changelog.

### Changed

- The module path is now `github.com/jayhemnani9910/voxt`, so `go install` works.
- README rewritten.

### Removed

- `AGENTS.md`.

## 2026-10-07

### Fixed

- First run no longer deadlocks while saving the default config.
- After a device error or the 10 minute cap, the recorder stops, saves and transcribes what it captured. Later recordings used to fail with "already recording".
- Quit no longer hangs on the hotkey listener.
- The stop beep is no longer in the recording.
- A failed `xdg-open` and a failed X11 clipboard write are now reported.
- An empty Llama reply keeps the Whisper text, and empty transcripts skip Llama.

### Changed

- An API key added in Settings is picked up without a restart.
- Recordings and history are readable only by the owner.
- Logs hold the length of a transcription, not its text.
- The autostart `Exec` path is quoted.

### Removed

- Unused `GetSupportedKeys`, `CheckPermissions`, `Tray.SetOnQuit`, `Tray.Quit` and `Notifier.ShowInfo`.

## 2026-02-04

### Added

- First version: F9 hotkey over evdev, PortAudio recording to WAV, Groq Whisper transcription, Llama formatting, clipboard copy, desktop notifications, tray icon, SQLite history.
