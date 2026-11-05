# Contributing

## Setup

You need Go 1.21 or newer and the system libraries. On Ubuntu or Debian:

```bash
sudo apt install -y golang-go gcc pkg-config portaudio19-dev libayatana-appindicator3-dev libx11-dev
git clone https://github.com/jayhemnani9910/voxt.git
cd voxt
make check
make build
```

## Tests and style

```bash
make test   # go test -v ./...
make lint   # go fmt and go vet
```

CI runs gofmt, `go vet`, the build and the tests on every push and pull request.

Tests must not call Groq or need a key. `transcribe/groq_test.go` shows how to fake it. Tests that touch config or history should point `HOME` at a temp dir, so they can't read your real files.

Keep functions small and code `gofmt`-clean. New code goes in the existing package it belongs to.

## Branches and pull requests

Branch from `master` and open a pull request against it. Keep each PR to one change. Say what changed and how you tested it. For tray or hotkey changes, say which desktop and display server you tried it on.

## Bug reports

Use the [bug report template](https://github.com/jayhemnani9910/voxt/issues/new?template=bug_report.md). Don't paste your API key, and be careful with logs, recordings and transcripts, since they hold what you said.

I try to reply to issues within a few days.
