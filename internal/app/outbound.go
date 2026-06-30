package app

import (
	"context"
	"errors"

	"github.com/steipete/wacli/internal/wa"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
)

func (a *App) StoreConfirmedOutboundText(ctx context.Context, chat types.JID, resp whatsmeow.SendResponse, text string) error {
	if resp.ID == "" {
		return errors.New("send response missing message id")
	}
	if resp.Timestamp.IsZero() {
		return errors.New("send response missing timestamp")
	}
	_, err := a.storeParsedMessage(ctx, wa.ParsedMessage{
		Chat:      chat,
		ID:        string(resp.ID),
		Timestamp: resp.Timestamp.UTC(),
		FromMe:    true,
		Text:      text,
	})
	return err
}

func (a *App) StoreConfirmedOutboundMessage(ctx context.Context, chat types.JID, resp whatsmeow.SendResponse, msg *waProto.Message) error {
	if resp.ID == "" {
		return errors.New("send response missing message id")
	}
	if resp.Timestamp.IsZero() {
		return errors.New("send response missing timestamp")
	}
	_, err := a.storeParsedMessage(ctx, wa.ParseOutgoingMessage(chat, resp.ID, resp.Timestamp, msg))
	return err
}

func (a *App) StoreConfirmedOutboundReaction(ctx context.Context, chat types.JID, resp whatsmeow.SendResponse, targetID types.MessageID, reaction string) error {
	if resp.ID == "" {
		return errors.New("send response missing message id")
	}
	if resp.Timestamp.IsZero() {
		return errors.New("send response missing timestamp")
	}
	_, err := a.storeParsedMessage(ctx, wa.ParsedMessage{
		Chat:          chat,
		ID:            string(resp.ID),
		Timestamp:     resp.Timestamp.UTC(),
		FromMe:        true,
		ReactionToID:  string(targetID),
		ReactionEmoji: reaction,
	})
	return err
}

func (a *App) StoreConfirmedOutboundEdit(ctx context.Context, chat types.JID, resp whatsmeow.SendResponse, targetID types.MessageID, text string) error {
	if resp.ID == "" {
		return errors.New("send response missing message id")
	}
	if resp.Timestamp.IsZero() {
		return errors.New("send response missing timestamp")
	}
	updated, err := a.db.UpdateMessageTextDisplay(chat.String(), string(targetID), text, text)
	if err != nil {
		return err
	}
	if !updated {
		return errors.New("edit target message not found in local store")
	}
	return nil
}
