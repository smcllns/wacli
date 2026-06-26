package app

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
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
	caps, ok := data["capabilities"].([]any)
	if !ok {
		t.Fatalf("capabilities = %T, want JSON array", data["capabilities"])
	}
	if !containsCapability(caps, "mark_read") || !containsCapability(caps, "quoted_send_text") {
		t.Fatalf("capabilities = %v, want mark_read and quoted_send_text", caps)
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

func containsCapability(caps []any, target string) bool {
	for _, cap := range caps {
		if cap == target {
			return true
		}
	}
	return false
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

func TestRunDaemonHandlesMarkReadInProcess(t *testing.T) {
	a := newTestAppWithFakeWA(t)
	fake := a.wa.(*fakeWA)
	socketPath := shortSocketPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = a.RunDaemon(ctx, DaemonOptions{SocketPath: socketPath, QueueSize: 4}) }()
	waitForUnixSocket(t, socketPath)

	resp := sendDaemonTestCommand(t, socketPath, `{"type":"mark_read","chatJid":"120363427307015739@g.us","msgIds":[" m1 "],"senderJid":"15551234567@s.whatsapp.net","timestamp":"2026-06-26T15:00:00Z"}`)
	if !resp.Success {
		t.Fatalf("resp = %+v", resp)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if got := fake.lastReadChat.String(); got != "120363427307015739@g.us" {
		t.Fatalf("read chat = %s", got)
	}
	if got := fake.lastReadSender.String(); got != "15551234567@s.whatsapp.net" {
		t.Fatalf("read sender = %s", got)
	}
	if len(fake.lastReadIDs) != 1 || fake.lastReadIDs[0] != "m1" {
		t.Fatalf("read ids = %+v", fake.lastReadIDs)
	}
	if got := fake.lastReadTimestamp.Format(time.RFC3339); got != "2026-06-26T15:00:00Z" {
		t.Fatalf("read timestamp = %s", got)
	}
}

func TestRunDaemonHandlesQuotedSendTextInProcess(t *testing.T) {
	a := newTestAppWithFakeWA(t)
	fake := a.wa.(*fakeWA)
	socketPath := shortSocketPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = a.RunDaemon(ctx, DaemonOptions{SocketPath: socketPath, QueueSize: 4}) }()
	waitForUnixSocket(t, socketPath)

	resp := sendDaemonTestCommand(t, socketPath, `{"type":"send_text","chatJid":"120363427307015739@g.us","message":"reply","replyToMsgId":"orig","replyToSenderJid":"15551234567@s.whatsapp.net","replyToText":"question"}`)
	if !resp.Success {
		t.Fatalf("resp = %+v", resp)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if got := fake.lastProtoTo.String(); got != "120363427307015739@g.us" {
		t.Fatalf("proto to = %s", got)
	}
	ext := fake.lastProtoMessage.GetExtendedTextMessage()
	if ext == nil || ext.GetText() != "reply" {
		t.Fatalf("extended text = %+v", ext)
	}
	ctxInfo := ext.GetContextInfo()
	if ctxInfo == nil || ctxInfo.GetStanzaID() != "orig" || ctxInfo.GetParticipant() != "15551234567@s.whatsapp.net" {
		t.Fatalf("context info = %+v", ctxInfo)
	}
	if ctxInfo.GetQuotedMessage().GetConversation() != "question" {
		t.Fatalf("quoted message = %+v", ctxInfo.GetQuotedMessage())
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

func TestRunDaemonRefusesNonSocketPath(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-socket")
	if err := os.WriteFile(path, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := newTestAppWithFakeWA(t)
	err := a.RunDaemon(context.Background(), DaemonOptions{SocketPath: path, QueueSize: 4})
	if err == nil || err.Error() != "daemon socket path exists and is not a socket" {
		t.Fatalf("err = %v, want non-socket error", err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "keep" {
		t.Fatalf("non-socket path was modified: data=%q err=%v", data, err)
	}
}

func TestRunDaemonRemovesSubscriberAfterDisconnect(t *testing.T) {
	a := newTestAppWithFakeWA(t)
	socketPath := shortSocketPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = a.RunDaemon(ctx, DaemonOptions{SocketPath: socketPath, QueueSize: 4}) }()
	waitForUnixSocket(t, socketPath)

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Write([]byte(`{"type":"subscribe"}` + "\n")); err != nil {
		t.Fatal(err)
	}
	var ack DaemonResponse
	if err := json.NewDecoder(conn).Decode(&ack); err != nil {
		t.Fatal(err)
	}
	if !ack.Success {
		t.Fatalf("ack = %+v", ack)
	}
	_ = conn.Close()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		resp := sendDaemonTestCommand(t, socketPath, `{"type":"health"}`)
		data := resp.Data.(map[string]any)
		if data["subscriberCount"].(float64) == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("subscriber was not removed after disconnect")
}

func TestRunDaemonSecuresSocketPermissions(t *testing.T) {
	a := newTestAppWithFakeWA(t)
	socketPath := shortSocketPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- a.RunDaemon(ctx, DaemonOptions{SocketPath: socketPath, QueueSize: 4}) }()
	waitForUnixSocketOrError(t, socketPath, errCh)

	st, err := os.Stat(socketPath)
	if err != nil {
		t.Fatal(err)
	}
	if got := st.Mode().Perm(); got != 0o600 {
		t.Fatalf("socket permissions = %o, want 600", got)
	}
}

func TestRunDaemonStopsOnLiveMessageStoreError(t *testing.T) {
	a := newTestAppWithFakeWA(t)
	fake := a.wa.(*fakeWA)
	socketPath := shortSocketPath(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	errCh := make(chan error, 1)
	go func() { errCh <- a.RunDaemon(ctx, DaemonOptions{SocketPath: socketPath, QueueSize: 4}) }()
	waitForUnixSocketOrError(t, socketPath, errCh)

	if err := a.db.Close(); err != nil {
		t.Fatal(err)
	}
	chat := types.JID{User: "123", Server: types.DefaultUserServer}
	fake.emit(&events.Message{
		Info: types.MessageInfo{
			MessageSource: types.MessageSource{Chat: chat, Sender: chat},
			ID:            "m-store-error",
			Timestamp:     time.Now().UTC(),
			PushName:      "Alice",
		},
		Message: &waProto.Message{Conversation: proto.String("boom")},
	})

	select {
	case err := <-errCh:
		if err == nil || !strings.Contains(err.Error(), "store daemon live message") {
			t.Fatalf("RunDaemon err = %v, want store daemon live message error", err)
		}
	case <-time.After(time.Second):
		t.Fatalf("RunDaemon did not stop after live message store error")
	}
}
