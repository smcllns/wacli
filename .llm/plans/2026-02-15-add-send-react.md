# Add `send react` Command to wacli

## Context
wacli (steipete/wacli) uses whatsmeow which supports reactions, but the CLI doesn't expose them.
Fork → add `send react` → build from source → use in whatsapp-poll.sh to ack messages cleanly.

## Tasks

- [x] Fork steipete/wacli → smcllns/wacli, clone, create feat/wacli-reactions branch
- [x] Add `SendReaction()` to `internal/wa/client.go`
- [x] Add to `WAClient` interface in `internal/app/app.go`
- [x] Add fake impl in `internal/app/fake_wa_test.go`
- [x] Create `cmd/wacli/send_react_cmd.go` (cobra command)
- [x] Register in `cmd/wacli/send.go`
- [x] Build from source, install to /opt/homebrew/bin, verify --help
- [x] All tests pass
- [x] Add `react` command to whatsapp.sh skill (agent-murphy)
- [x] Include [MSGID:] tags in cmd_check output
- [x] Update whatsapp-poll.sh to react instead of text ack
- [ ] E2E test: send a reaction to a real message
- [ ] Upstream: evaluate PR to steipete/wacli (deferred)
