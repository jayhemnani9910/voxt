# Repository Guidelines

Voxt is a Go (1.21) Linux/Wayland voice-to-text clipboard tool. This guide captures the repo conventions used for day-to-day development.

## Project Structure & Module Organization

- `main.go`: application entry point and state machine wiring packages together.
- Core packages: `audio/`, `hotkey/`, `tray/`, `notify/`, `clipboard/`, `transcribe/`, `db/`, `config/`, `keyring/`.
- `assets/`: embedded tray icons + beep WAVs (generated as-needed).
- `cmd/genassets/`: helper to regenerate assets.
- `ui/`: optional GTK4 settings panel.

## Build, Test, and Development Commands

- `make check`: verify system deps (Go, PortAudio, AppIndicator, input group).
- `make build` / `make release`: build local/optimized `./voxt`.
- `make run`: build + run locally.
- `make fmt` / `make vet` / `make lint`: format + static checks.
- `make test`: run `go test -v ./...` (tests may be absent today).
- `make assets-regen` or `go run ./cmd/genassets`: regenerate `assets/` if needed.

## Coding Style & Naming Conventions

- Keep code `gofmt`-clean; use `make fmt` before pushing.
- Prefer small, focused packages; add new functionality under an existing package when possible (e.g., transcription logic in `transcribe/`).
- Go naming: exported `PascalCase`, unexported `camelCase`; file names match existing patterns (e.g., `recorder.go`, `groq.go`).

## Testing Guidelines

- Use standard Go tests in `*_test.go` files, colocated with the package under test.
- Avoid live Groq API calls in tests; use `net/http/httptest` and injectable clients/mocks.

## Commit & Pull Request Guidelines

- This workspace snapshot may not include `.git`; default to Conventional Commits (`feat:`, `fix:`, `chore:`) with imperative subjects.
- PRs should include: what changed, how to test (commands + expected behavior), and for tray/UI changes a screenshot or the desktop environment details (e.g., GNOME/Wayland).

## Security & Configuration Tips

- Never commit API keys. Config is at `~/.config/voxt/config.json`; the Groq key may also live in the system keyring (`voxt` / `groq-api-key`).
- Audio is written to `~/recordings` by default—treat recordings and logs as sensitive artifacts.

## Additional Context

- See `AGENT.md` for a deeper architecture/workflow overview.
