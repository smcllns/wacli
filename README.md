# 🗃️ wacli — WhatsApp CLI: sync, search, send.

WhatsApp CLI built on top of `whatsmeow`, focused on:

- Best-effort local sync of message history + continuous capture
- Fast offline search
- Sending messages
- Contact + group management

This is a third-party tool that uses the WhatsApp Web protocol via `whatsmeow` and is not affiliated with WhatsApp.

This project is a fork of [steipete/wacli](https://github.com/steipete/wacli).

## Updates (smcllns/wacli fork)

### 0.3.0

- Send: `wacli send react` to send emoji reactions to messages.
- Updated docs to clarify how to set linked device name

## Install / Build

- `go build -tags sqlite_fts5 -o ./dist/wacli ./cmd/wacli`

## Quick start

Default store directory is `~/.wacli` (override with `--store DIR`).

```bash
# 1) Authenticate (shows QR), then bootstrap sync
wacli auth
# Note: optionally set the linked device name during auth:
# WACLI_DEVICE_LABEL="wacli(mac mini)" WACLI_DEVICE_PLATFORM=DESKTOP wacli auth

# 2) Keep syncing (never shows QR; requires prior auth)
wacli sync --follow

# Diagnostics
wacli doctor

# Search messages
wacli messages search "meeting"

# Backfill older messages for a chat (best-effort; requires your primary device online)
wacli history backfill --chat 1234567890@s.whatsapp.net --requests 10 --count 50

# Download media for a message (after syncing)
wacli media download --chat 1234567890@s.whatsapp.net --id <message-id>

# Send a message
wacli send text --to 1234567890 --message "hello"

# Send a file
wacli send file --to 1234567890 --file ./pic.jpg --caption "hi"
# Or override display name
wacli send file --to 1234567890 --file /tmp/abc123 --filename report.pdf

# Send a reaction
wacli send react --to 1234567890 --id <message-id> --emoji 👍

# List groups and manage participants
wacli groups list
wacli groups rename --jid 123456789@g.us --name "New name"
```

## Prior Art / Credit

This project is a fork of [steipete/wacli](https://github.com/steipete/wacli), which is heavily inspired by (and learns from) the excellent `whatsapp-cli` by Vicente Reig:

- [`whatsapp-cli`](https://github.com/vicentereig/whatsapp-cli)

## High-level UX

- `wacli auth`: interactive login (shows QR code), then immediately performs initial data sync.
- `wacli sync`: non-interactive sync loop (never shows QR; errors if not authenticated).
- Output is human-readable by default; pass `--json` for machine-readable output.

## Storage

Defaults to `~/.wacli` (override with `--store DIR`).

## Environment overrides

Set these when running `wacli auth` to control how the device appears in WhatsApp → Settings → Linked Devices:

```bash
WACLI_DEVICE_LABEL="wacli(air)" WACLI_DEVICE_PLATFORM=DESKTOP wacli auth
```

- `WACLI_DEVICE_LABEL`: label shown for the linked device (e.g. `wacli(air)`). Defaults to empty ("Other device").
- `WACLI_DEVICE_PLATFORM`: icon for the linked device. Valid values from the [`DeviceProps_PlatformType`](https://github.com/go-whatsapp/whatsmeow/blob/main/proto/waCompanionReg/WAWebProtobufsCompanionReg.pb.go) enum, e.g. `DESKTOP` (computer icon), `CATALINA` (Apple icon). Defaults to `CHROME` ("Other device") if unset.

> These are registered at pair time — to change them, delete `~/.wacli/session.db` and re-run `wacli auth`.

## Backfilling older history

`wacli sync` stores whatever WhatsApp Web sends opportunistically. To try to fetch *older* messages, use on-demand history sync requests to your **primary device** (your phone).

Important notes:

- This is **best-effort**: WhatsApp may not return full history.
- Your **primary device must be online**.
- Requests are **per chat** (DM or group). `wacli` uses the *oldest locally stored message* in that chat as the anchor.
- Recommended `--count` is `50` per request.

### Backfill one chat

```bash
wacli history backfill --chat 1234567890@s.whatsapp.net --requests 10 --count 50
```

### Backfill all chats (script)

This loops through chats already known in your local DB:

```bash
wacli --json chats list --limit 100000 \
  | jq -r '.[].JID' \
  | while read -r jid; do
      wacli history backfill --chat "$jid" --requests 3 --count 50
    done
```

## License

See `LICENSE`.
