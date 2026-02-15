# Add `send react` Command to wacli

## Context
wacli (steipete/wacli) uses whatsmeow which supports reactions, but the CLI doesn't expose them.
Fork → add `send react` → build from source → use in whatsapp-poll.sh to ack messages cleanly.

## Tasks

- [x] Fork steipete/wacli → smcllns/wacli, clone, create feat/wacli-reactions branch
- [x] Add `SendReaction()` to `internal/wa/client.go` (delegates to `cli.BuildReaction()`)
- [x] Add to `WAClient` interface in `internal/app/app.go`
- [x] Add fake impl in `internal/app/fake_wa_test.go`
- [x] Create `cmd/wacli/send_react_cmd.go` (cobra command)
- [x] Register in `cmd/wacli/send.go`
- [x] Build from source, install to /opt/homebrew/bin, verify --help
- [x] All tests pass
- [x] Add `react` command to whatsapp.sh skill (agent-murphy)
- [x] Include [MSGID:] tags in cmd_check output
- [x] Update whatsapp-poll.sh to react instead of text ack
- [x] E2E test: react ✓, change reaction ✓, remove reaction ✓
- [ ] Upstream: evaluate PR to steipete/wacli (pending code review)

## Key Decision
Initially manually constructed `ReactionMessage` protobuf — message sent without error but was silently ignored by recipient. Fixed by using whatsmeow's `BuildReaction()` which correctly sets `SenderTimestampMS` and builds the `MessageKey` via `BuildMessageKey`.
