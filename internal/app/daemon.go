package app

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/steipete/wacli/internal/wa"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

var ErrDaemonQueueFull = errors.New("daemon write queue full")
var ErrDaemonQueueClosed = errors.New("daemon write queue closed")

type DaemonOptions struct {
	SocketPath string
	QueueSize  int
}

type DaemonResponse struct {
	Type    string `json:"type"`
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
}

type DaemonEvent struct {
	Type        string `json:"type"`
	RowID       int64  `json:"rowid"`
	ChatJID     string `json:"chatJid"`
	MsgID       string `json:"msgId"`
	SenderJID   string `json:"senderJid,omitempty"`
	Timestamp   string `json:"timestamp"`
	FromMe      bool   `json:"fromMe"`
	Text        string `json:"text,omitempty"`
	DisplayText string `json:"displayText,omitempty"`
	MediaType   string `json:"mediaType,omitempty"`
}

type DaemonCommand struct {
	Type      string `json:"type"`
	ChatJID   string `json:"chatJid,omitempty"`
	Message   string `json:"message,omitempty"`
	SenderJID string `json:"senderJid,omitempty"`
	MsgID     string `json:"msgId,omitempty"`
	Reaction  string `json:"reaction,omitempty"`
	FilePath  string `json:"filePath,omitempty"`
	Name      string `json:"name,omitempty"`
}

type DaemonResult struct {
	Data any
	Err  error
	Done chan struct{}
}

type daemonWriteQueue struct {
	mu     sync.Mutex
	closed bool
	jobs   chan daemonWriteJob
	slots  chan struct{}
	done   chan struct{}
}

type daemonWriteJob struct {
	ctx context.Context
	cmd DaemonCommand
	res *DaemonResult
}

func (a *App) RunDaemon(ctx context.Context, opts DaemonOptions) error {
	if strings.TrimSpace(opts.SocketPath) == "" {
		return errors.New("daemon socket path is required")
	}
	if err := os.MkdirAll(filepath.Dir(opts.SocketPath), 0o700); err != nil {
		return fmt.Errorf("create daemon socket dir: %w", err)
	}
	if err := removeStaleDaemonSocket(opts.SocketPath); err != nil {
		return err
	}

	if err := a.EnsureAuthed(); err != nil {
		return err
	}
	if err := a.Connect(ctx, false, nil); err != nil {
		return err
	}

	listener, err := net.Listen("unix", opts.SocketPath)
	if err != nil {
		return fmt.Errorf("listen daemon socket: %w", err)
	}
	defer listener.Close()
	defer os.Remove(opts.SocketPath)

	queue := newDaemonWriteQueue(opts.QueueSize, a.handleDaemonWriteCommand)
	defer queue.Close()

	subscribers := daemonSubscribers{subscribers: map[chan DaemonEvent]struct{}{}}
	handlerID := a.wa.AddEventHandler(func(evt interface{}) {
		msg, ok := evt.(*events.Message)
		if !ok {
			return
		}
		pm := wa.ParseLiveMessage(msg)
		if pm.ID == "" || pm.Chat.IsEmpty() {
			return
		}
		if err := a.storeParsedMessage(ctx, pm); err != nil {
			return
		}
		rowid, err := a.db.MessageRowID(pm.Chat.String(), pm.ID)
		if err != nil {
			return
		}
		subscribers.broadcast(DaemonEvent{
			Type:        "message",
			RowID:       rowid,
			ChatJID:     pm.Chat.String(),
			MsgID:       pm.ID,
			SenderJID:   pm.SenderJID,
			Timestamp:   pm.Timestamp.UTC().Format(time.RFC3339Nano),
			FromMe:      pm.FromMe,
			Text:        pm.Text,
			DisplayText: a.buildDisplayText(context.Background(), pm),
			MediaType:   daemonMediaType(pm.Media),
		})
	})
	defer a.wa.RemoveEventHandler(handlerID)

	errCh := make(chan error, 1)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-ctx.Done():
					errCh <- nil
				default:
					errCh <- err
				}
				return
			}
			go a.handleDaemonConn(ctx, conn, queue, &subscribers, opts.SocketPath)
		}
	}()

	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		return err
	}
}

type daemonSubscribers struct {
	mu          sync.Mutex
	subscribers map[chan DaemonEvent]struct{}
}

func (s *daemonSubscribers) add(ch chan DaemonEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.subscribers[ch] = struct{}{}
}

func (s *daemonSubscribers) remove(ch chan DaemonEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.subscribers[ch]; !ok {
		return
	}
	delete(s.subscribers, ch)
	close(ch)
}

func (s *daemonSubscribers) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.subscribers)
}

func (s *daemonSubscribers) broadcast(event DaemonEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for ch := range s.subscribers {
		select {
		case ch <- event:
		default:
			delete(s.subscribers, ch)
			close(ch)
		}
	}
}

func removeStaleDaemonSocket(socketPath string) error {
	st, err := os.Lstat(socketPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat daemon socket path: %w", err)
	}
	if st.Mode()&os.ModeSocket == 0 {
		return errors.New("daemon socket path exists and is not a socket")
	}

	conn, err := net.Dial("unix", socketPath)
	if err == nil {
		_ = conn.Close()
		return errors.New("daemon socket already has a live listener")
	}
	if err := os.Remove(socketPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove stale daemon socket: %w", err)
	}
	return nil
}

func (a *App) handleDaemonConn(ctx context.Context, conn net.Conn, queue *daemonWriteQueue, subscribers *daemonSubscribers, socketPath string) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	encoder := json.NewEncoder(conn)
	for scanner.Scan() {
		cmd, err := parseDaemonCommand(scanner.Bytes())
		if err == nil {
			err = validateDaemonCommand(cmd)
		}
		if err != nil {
			_ = encoder.Encode(DaemonResponse{Type: "response", Success: false, Error: err.Error()})
			continue
		}
		if cmd.Type == "health" {
			_ = encoder.Encode(DaemonResponse{Type: "response", Success: true, Data: map[string]any{
				"socketPath":      socketPath,
				"storeDir":        a.StoreDir(),
				"connected":       a.wa != nil && a.wa.IsConnected(),
				"queueDepth":      len(queue.slots),
				"queueMaxDepth":   cap(queue.slots),
				"subscriberCount": subscribers.count(),
				"ts":              time.Now().UTC().Format(time.RFC3339Nano),
			}})
			continue
		}
		if cmd.Type == "subscribe" {
			events := make(chan DaemonEvent, 64)
			disconnected := make(chan struct{})
			subscribers.add(events)
			defer subscribers.remove(events)
			go func() {
				_, _ = io.Copy(io.Discard, conn)
				close(disconnected)
			}()
			_ = encoder.Encode(DaemonResponse{Type: "response", Success: true, Data: map[string]any{"subscribed": true}})
			for {
				select {
				case <-ctx.Done():
					return
				case <-disconnected:
					return
				case event, ok := <-events:
					if !ok {
						return
					}
					if err := encoder.Encode(event); err != nil {
						return
					}
				}
			}
		}
		res := queue.Enqueue(ctx, cmd)
		<-res.Done
		if res.Err != nil {
			_ = encoder.Encode(DaemonResponse{Type: "response", Success: false, Error: res.Err.Error()})
			continue
		}
		_ = encoder.Encode(DaemonResponse{Type: "response", Success: true, Data: res.Data})
	}
}

func (a *App) handleDaemonWriteCommand(ctx context.Context, cmd DaemonCommand) (any, error) {
	switch cmd.Type {
	case "send_text":
		jid, err := types.ParseJID(cmd.ChatJID)
		if err != nil {
			return nil, err
		}
		id, err := a.wa.SendText(ctx, jid, cmd.Message)
		if err != nil {
			return nil, err
		}
		return map[string]any{"message_id": id}, nil
	case "send_react":
		chat, err := types.ParseJID(cmd.ChatJID)
		if err != nil {
			return nil, err
		}
		sender, err := types.ParseJID(cmd.SenderJID)
		if err != nil {
			return nil, err
		}
		id, err := a.wa.SendReaction(ctx, chat, sender, types.MessageID(cmd.MsgID), cmd.Reaction)
		if err != nil {
			return nil, err
		}
		return map[string]any{"message_id": id}, nil
	case "group_rename":
		jid, err := types.ParseJID(cmd.ChatJID)
		if err != nil {
			return nil, err
		}
		return nil, a.wa.SetGroupName(ctx, jid, cmd.Name)
	case "group_photo":
		jid, err := types.ParseJID(cmd.ChatJID)
		if err != nil {
			return nil, err
		}
		avatar, err := os.ReadFile(cmd.FilePath)
		if err != nil {
			return nil, err
		}
		pictureID, err := a.wa.SetGroupPhoto(ctx, jid, avatar)
		if err != nil {
			return nil, err
		}
		return map[string]any{"picture_id": pictureID}, nil
	default:
		return nil, fmt.Errorf("daemon command %q is not implemented", cmd.Type)
	}
}

func daemonMediaType(media *wa.Media) string {
	if media == nil {
		return ""
	}
	return media.Type
}

func parseDaemonCommand(line []byte) (DaemonCommand, error) {
	var cmd DaemonCommand
	if err := json.Unmarshal(line, &cmd); err != nil {
		return DaemonCommand{}, fmt.Errorf("parse daemon command: %w", err)
	}
	cmd.Type = strings.TrimSpace(cmd.Type)
	if cmd.Type == "" {
		return DaemonCommand{}, errors.New("daemon command type is required")
	}
	switch cmd.Type {
	case "health", "subscribe", "send_text", "send_react", "send_file", "group_rename", "group_photo", "refresh_groups", "shutdown":
		return cmd, nil
	default:
		return DaemonCommand{}, fmt.Errorf("unknown daemon command type %q", cmd.Type)
	}
}

func validateDaemonCommand(cmd DaemonCommand) error {
	switch cmd.Type {
	case "send_text":
		if strings.TrimSpace(cmd.ChatJID) == "" {
			return errors.New("send_text requires chatJid")
		}
		if strings.TrimSpace(cmd.Message) == "" {
			return errors.New("send_text requires message")
		}
	case "send_react":
		if strings.TrimSpace(cmd.ChatJID) == "" {
			return errors.New("send_react requires chatJid")
		}
		if strings.TrimSpace(cmd.MsgID) == "" {
			return errors.New("send_react requires msgId")
		}
		if strings.TrimSpace(cmd.SenderJID) == "" {
			return errors.New("send_react requires senderJid")
		}
	case "send_file":
		if strings.TrimSpace(cmd.ChatJID) == "" {
			return errors.New("send_file requires chatJid")
		}
		if strings.TrimSpace(cmd.FilePath) == "" {
			return errors.New("send_file requires filePath")
		}
	case "group_rename":
		if strings.TrimSpace(cmd.ChatJID) == "" {
			return errors.New("group_rename requires chatJid")
		}
		if strings.TrimSpace(cmd.Name) == "" {
			return errors.New("group_rename requires name")
		}
	case "group_photo":
		if strings.TrimSpace(cmd.ChatJID) == "" {
			return errors.New("group_photo requires chatJid")
		}
		if strings.TrimSpace(cmd.FilePath) == "" {
			return errors.New("group_photo requires filePath")
		}
	}
	return nil
}

func newDaemonWriteQueue(limit int, handler func(context.Context, DaemonCommand) (any, error)) *daemonWriteQueue {
	if limit <= 0 {
		limit = 1
	}
	q := &daemonWriteQueue{
		jobs:  make(chan daemonWriteJob, limit),
		slots: make(chan struct{}, limit),
		done:  make(chan struct{}),
	}
	go func() {
		defer close(q.done)
		for job := range q.jobs {
			job.res.Data, job.res.Err = handler(job.ctx, job.cmd)
			<-q.slots
			close(job.res.Done)
		}
	}()
	return q
}

func (q *daemonWriteQueue) Enqueue(ctx context.Context, cmd DaemonCommand) *DaemonResult {
	res := &DaemonResult{Done: make(chan struct{})}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		res.Err = ErrDaemonQueueClosed
		close(res.Done)
		return res
	}
	select {
	case q.slots <- struct{}{}:
	default:
		res.Err = ErrDaemonQueueFull
		close(res.Done)
		return res
	}

	q.jobs <- daemonWriteJob{ctx: ctx, cmd: cmd, res: res}
	return res
}

func (q *daemonWriteQueue) Close() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	close(q.jobs)
	q.mu.Unlock()
	<-q.done
}
