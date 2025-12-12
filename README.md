# wacli

WhatsApp CLI built on top of `whatsmeow`, focused on:

- Best-effort local sync of message history + continuous capture
- Fast offline search
- Sending messages
- Contact + group management

This is a third-party tool that uses the WhatsApp Web protocol via `whatsmeow` and is not affiliated with WhatsApp.

## Status

Core implementation is in place. See `docs/spec.md` for the full design notes.

## Build

```bash
go build ./cmd/wacli
```

Optional (recommended): enable SQLite FTS5 for fast message search:

```bash
go build -tags sqlite_fts5 ./cmd/wacli
```

## Install (Homebrew)

```bash
brew tap steipete/tap
brew install steipete/tap/wacli
```

## Quick start

Default store directory is `~/.wacli` (override with `--store DIR`).

```bash
# 1) Authenticate (shows QR), then bootstrap sync
./wacli auth

# 2) Keep syncing (never shows QR; requires prior auth)
./wacli sync

# Diagnostics
./wacli doctor

# Search messages
./wacli messages search "meeting"

# Download media for a message (after syncing)
./wacli media download --chat 1234567890@s.whatsapp.net --id <message-id>

# Send a message
./wacli send text --to 1234567890 --message "hello"

# Send a file
./wacli send file --to 1234567890 --file ./pic.jpg --caption "hi"

# List groups and manage participants
./wacli groups list
./wacli groups rename --jid 123456789@g.us --name "New name"
```

## Prior Art / Credit

This project is heavily inspired by (and learns from) the excellent `whatsapp-cli` by Vicente Reig:

- `https://github.com/vicentereig/whatsapp-cli`

## High-level UX

- `wacli auth`: interactive login (shows QR code), then immediately performs initial data sync.
- `wacli sync`: non-interactive sync loop (never shows QR; errors if not authenticated).
- Output is human-readable by default; pass `--json` for machine-readable output.

## Storage

Defaults to `~/.wacli` (override with `--store DIR`).

## License

See `LICENSE`.
