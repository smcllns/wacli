package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestParseDaemonCommandRequiresType(t *testing.T) {
	_, err := parseDaemonCommand([]byte(`{"chatJid":"120@g.us"}`))
	if err == nil || err.Error() != "daemon command type is required" {
		t.Fatalf("err = %v, want required type", err)
	}
}

func TestParseDaemonCommandRejectsUnknownType(t *testing.T) {
	_, err := parseDaemonCommand([]byte(`{"type":"bogus"}`))
	if err == nil || err.Error() != `unknown daemon command type "bogus"` {
		t.Fatalf("err = %v, want unknown type", err)
	}
}

func TestValidateDaemonCommandRequiresChatJIDForSendText(t *testing.T) {
	cmd, err := parseDaemonCommand([]byte(`{"type":"send_text","message":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDaemonCommand(cmd); err == nil || err.Error() != "send_text requires chatJid" {
		t.Fatalf("err = %v, want chatJid requirement", err)
	}
}

func TestValidateDaemonCommandRejectsBlankMarkReadMessageIDs(t *testing.T) {
	cmd, err := parseDaemonCommand([]byte(`{"type":"mark_read","chatJid":"120@g.us","msgIds":["m1","  "],"timestamp":"2026-06-26T15:00:00Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDaemonCommand(cmd); err == nil || err.Error() != "mark_read msgIds cannot contain blanks" {
		t.Fatalf("err = %v, want blank msgIds rejection", err)
	}
}

func TestValidateDaemonCommandRequiresSenderForGroupMarkRead(t *testing.T) {
	cmd, err := parseDaemonCommand([]byte(`{"type":"mark_read","chatJid":"120363427307015739@g.us","msgIds":["m1"],"timestamp":"2026-06-26T15:00:00Z"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDaemonCommand(cmd); err == nil || err.Error() != "mark_read requires senderJid for group chats" {
		t.Fatalf("err = %v, want group sender requirement", err)
	}
}

func TestValidateDaemonCommandRequiresReplyMessageIDForQuotedText(t *testing.T) {
	cmd, err := parseDaemonCommand([]byte(`{"type":"send_text","chatJid":"15551234567@s.whatsapp.net","message":"reply","replyToText":"question"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDaemonCommand(cmd); err == nil || err.Error() != "send_text quoted replies require replyToMsgId" {
		t.Fatalf("err = %v, want reply message id requirement", err)
	}
}

func TestValidateDaemonCommandRequiresReplySenderForGroupQuotedText(t *testing.T) {
	cmd, err := parseDaemonCommand([]byte(`{"type":"send_text","chatJid":"120363427307015739@g.us","message":"reply","replyToMsgId":"orig"}`))
	if err != nil {
		t.Fatal(err)
	}
	if err := validateDaemonCommand(cmd); err == nil || err.Error() != "send_text quoted replies require replyToSenderJid for group chats" {
		t.Fatalf("err = %v, want group reply sender requirement", err)
	}
}

func TestDaemonWriteQueueRejectsWhenFull(t *testing.T) {
	q := newDaemonWriteQueue(1, func(ctx context.Context, cmd DaemonCommand) (any, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	defer q.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	first := q.Enqueue(ctx, DaemonCommand{Type: "send_text", ChatJID: "120@g.us", Message: "one"})
	select {
	case <-first.Done:
		t.Fatalf("first command completed before queue filled")
	case <-time.After(20 * time.Millisecond):
	}

	second := q.Enqueue(ctx, DaemonCommand{Type: "send_text", ChatJID: "120@g.us", Message: "two"})
	<-second.Done
	if !errors.Is(second.Err, ErrDaemonQueueFull) {
		t.Fatalf("second err = %v, want ErrDaemonQueueFull", second.Err)
	}
}

func TestDaemonWriteQueueSerializesCommands(t *testing.T) {
	started := make(chan string, 2)
	release := make(chan struct{})
	q := newDaemonWriteQueue(2, func(ctx context.Context, cmd DaemonCommand) (any, error) {
		started <- cmd.Message
		<-release
		return cmd.Message, nil
	})
	defer q.Close()

	ctx := context.Background()
	first := q.Enqueue(ctx, DaemonCommand{Type: "send_text", ChatJID: "120@g.us", Message: "one"})
	second := q.Enqueue(ctx, DaemonCommand{Type: "send_text", ChatJID: "120@g.us", Message: "two"})

	if got := <-started; got != "one" {
		t.Fatalf("first started = %q", got)
	}
	select {
	case got := <-started:
		t.Fatalf("second started before first released: %q", got)
	case <-time.After(20 * time.Millisecond):
	}

	release <- struct{}{}
	<-first.Done
	if first.Err != nil || first.Data != "one" {
		t.Fatalf("first = (%v, %v)", first.Data, first.Err)
	}
	if got := <-started; got != "two" {
		t.Fatalf("second started = %q", got)
	}
	release <- struct{}{}
	<-second.Done
	if second.Err != nil || second.Data != "two" {
		t.Fatalf("second = (%v, %v)", second.Data, second.Err)
	}
}

func TestDaemonWriteQueueReturnsErrorAfterClose(t *testing.T) {
	q := newDaemonWriteQueue(1, func(ctx context.Context, cmd DaemonCommand) (any, error) {
		return nil, nil
	})
	q.Close()
	res := q.Enqueue(context.Background(), DaemonCommand{Type: "send_text", ChatJID: "120@g.us", Message: "hi"})
	<-res.Done
	if !errors.Is(res.Err, ErrDaemonQueueClosed) {
		t.Fatalf("err = %v, want ErrDaemonQueueClosed", res.Err)
	}
}

func TestDaemonSubscribersRemoveLaggingSubscriberOnOverflow(t *testing.T) {
	s := daemonSubscribers{subscribers: map[chan DaemonEvent]struct{}{}}
	ch := make(chan DaemonEvent, 1)
	s.add(ch)
	s.broadcast(DaemonEvent{Type: "message", MsgID: "one"})
	s.broadcast(DaemonEvent{Type: "message", MsgID: "two"})
	if got := s.count(); got != 0 {
		t.Fatalf("subscriber count = %d, want 0 after overflow", got)
	}
	if _, ok := <-ch; !ok {
		t.Fatalf("first buffered event missing before close")
	}
	if _, ok := <-ch; ok {
		t.Fatalf("subscriber channel still open after overflow")
	}
}
