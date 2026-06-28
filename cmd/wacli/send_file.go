package main

import (
	"context"

	"github.com/steipete/wacli/internal/app"
	"go.mau.fi/whatsmeow/types"
)

func sendFile(ctx context.Context, a interface {
	SendFile(context.Context, types.JID, string, string, string, string) (app.SendFileResult, error)
}, to types.JID, filePath, filename, caption, mimeOverride string) (string, map[string]string, error) {
	result, err := a.SendFile(ctx, to, filePath, filename, caption, mimeOverride)
	if err != nil {
		return "", nil, err
	}
	return result.MessageID, result.Meta, nil
}

func chatKindFromJID(j types.JID) string {
	if j.Server == types.GroupServer {
		return "group"
	}
	if j.IsBroadcastList() {
		return "broadcast"
	}
	if j.Server == types.DefaultUserServer {
		return "dm"
	}
	return "unknown"
}
