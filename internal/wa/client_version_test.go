package wa

import (
	"testing"

	"go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

func TestLoginPayloadUsesUpdatedWebVersion(t *testing.T) {
	device := &store.Device{ID: &types.JID{User: "1234567890", Device: 1, Server: types.DefaultUserServer}}
	version := device.GetClientPayload().GetUserAgent().GetAppVersion()
	got := store.WAVersionContainer{version.GetPrimary(), version.GetSecondary(), version.GetTertiary()}
	// Upstream's October 17 baseline; the June version was rejected with 405 on October 26.
	baseline := store.WAVersionContainer{2, 3000, 1050728964}
	if got.LessThan(baseline) {
		t.Fatalf("login advertises outdated WhatsApp web version %s; want at least %s", got, baseline)
	}
}
