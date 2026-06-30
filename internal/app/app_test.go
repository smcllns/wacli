package app

import (
	"context"
	"testing"

	"go.mau.fi/whatsmeow/types/events"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()
	a, err := New(Options{StoreDir: dir})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}

func TestConnectReturnsPermanentDisconnectEvent(t *testing.T) {
	a := newTestApp(t)
	f := newFakeWA()
	a.wa = f
	f.connectEvents = []interface{}{&events.LoggedOut{OnConnect: true, Reason: events.ConnectFailureLoggedOut}}

	err := a.Connect(context.Background(), false, nil)
	if err == nil || err.Error() != "permanent connect disconnect: 401: logged out from another device" {
		t.Fatalf("err = %v", err)
	}
}
