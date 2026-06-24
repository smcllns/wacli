package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

func TestRunDaemonRespondsToHealth(t *testing.T) {
	a := newTestAppWithFakeWA(t)
	socketPath := shortSocketPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() { errCh <- a.RunDaemon(ctx, DaemonOptions{SocketPath: socketPath, QueueSize: 4}) }()
	waitForUnixSocket(t, socketPath)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"type":"health"}` + "\n")); err != nil {
		t.Fatal(err)
	}

	var resp DaemonResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	if !resp.Success || resp.Type != "response" {
		t.Fatalf("resp = %+v, want successful response", resp)
	}
	data := resp.Data.(map[string]any)
	if data["socketPath"] != socketPath {
		t.Fatalf("socketPath = %v, want %s", data["socketPath"], socketPath)
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("RunDaemon err = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("RunDaemon did not stop")
	}
}

func TestRunDaemonRejectsInvalidCommand(t *testing.T) {
	a := newTestAppWithFakeWA(t)
	socketPath := shortSocketPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- a.RunDaemon(ctx, DaemonOptions{SocketPath: socketPath, QueueSize: 4}) }()
	waitForUnixSocketOrError(t, socketPath, errCh)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(`{"type":"send_text","message":"hi"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(conn).ReadBytes('\n')
	if err != nil {
		t.Fatal(err)
	}
	var resp DaemonResponse
	if err := json.Unmarshal(line, &resp); err != nil {
		t.Fatal(err)
	}
	if resp.Success || resp.Error != "send_text requires chatJid" {
		t.Fatalf("resp = %+v, want validation error", resp)
	}
}

func shortSocketPath(t *testing.T) string {
	t.Helper()
	path := filepath.Join(os.TempDir(), fmt.Sprintf("waldo-%d.sock", time.Now().UnixNano()))
	t.Cleanup(func() { _ = os.Remove(path) })
	return path
}

func newTestAppWithFakeWA(t *testing.T) *App {
	t.Helper()
	a, err := New(Options{StoreDir: t.TempDir(), Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.Close)
	a.wa = newFakeWA()
	return a
}

func waitForUnixSocket(t *testing.T, socketPath string) {
	t.Helper()
	waitForUnixSocketOrError(t, socketPath, nil)
}

func waitForUnixSocketOrError(t *testing.T, socketPath string, errCh <-chan error) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if errCh != nil {
			select {
			case err := <-errCh:
				t.Fatalf("RunDaemon exited before socket was ready: %v", err)
			default:
			}
		}
		conn, err := net.Dial("unix", socketPath)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for socket %s", socketPath)
}
func TestRunDaemonHandlesSendTextInProcess(t *testing.T) {
	a := newTestAppWithFakeWA(t)
	fake := a.wa.(*fakeWA)
	socketPath := shortSocketPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = a.RunDaemon(ctx, DaemonOptions{SocketPath: socketPath, QueueSize: 4}) }()
	waitForUnixSocket(t, socketPath)

	resp := sendDaemonTestCommand(t, socketPath, `{"type":"send_text","chatJid":"120363427307015739@g.us","message":"hi"}`)
	if !resp.Success {
		t.Fatalf("resp = %+v", resp)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.lastTextTo.String() != "120363427307015739@g.us" || fake.lastTextMessage != "hi" {
		t.Fatalf("sent text = (%s, %q)", fake.lastTextTo, fake.lastTextMessage)
	}
}

func sendDaemonTestCommand(t *testing.T, socketPath string, command string) DaemonResponse {
	t.Helper()
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte(command + "\n")); err != nil {
		t.Fatal(err)
	}
	var resp DaemonResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestRunDaemonSubscribeEmitsStoredLiveMessage(t *testing.T) {
	a := newTestAppWithFakeWA(t)
	fake := a.wa.(*fakeWA)
	socketPath := shortSocketPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = a.RunDaemon(ctx, DaemonOptions{SocketPath: socketPath, QueueSize: 4}) }()
	waitForUnixSocket(t, socketPath)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	decoder := json.NewDecoder(conn)
	if _, err := conn.Write([]byte(`{"type":"subscribe"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	var ack DaemonResponse
	if err := decoder.Decode(&ack); err != nil {
		t.Fatal(err)
	}
	if !ack.Success {
		t.Fatalf("ack = %+v", ack)
	}

	chat := types.JID{User: "123", Server: types.DefaultUserServer}
	fake.emit(&events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: chat},
			ID:            "m-live-daemon",
			Timestamp:     time.Now().UTC(),
			PushName:      "Alice",
		},
		Message: &waProto.Message{Conversation: proto.String("hello daemon")},
	})

	var event DaemonEvent
	if err := decoder.Decode(&event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "message" || event.ChatJID != chat.String() || event.MsgID != "m-live-daemon" || event.Text != "hello daemon" || event.RowID <= 0 {
		t.Fatalf("event = %+v", event)
	}
}

func TestRunDaemonDoesNotUnlinkLiveSocket(t *testing.T) {
	socketPath := shortSocketPath(t)
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	a := newTestAppWithFakeWA(t)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	err = a.RunDaemon(ctx, DaemonOptions{SocketPath: socketPath, QueueSize: 4})
	if err == nil || err.Error() != "daemon socket already has a live listener" {
		t.Fatalf("err = %v, want live listener error", err)
	}

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatalf("existing socket was removed or broken: %v", err)
	}
	_ = conn.Close()
}
