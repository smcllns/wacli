package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/out"
	"github.com/steipete/wacli/internal/wa"
	"go.mau.fi/whatsmeow/types"
)

func newMessagesReadCmd(flags *rootFlags) *cobra.Command {
	var chat string

	cmd := &cobra.Command{
		Use:   "read [message-ids...]",
		Short: "Mark messages as read (send read receipts)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx, cancel := withTimeout(context.Background(), flags)
			defer cancel()

			a, lk, err := newApp(ctx, flags, true, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			if err := a.EnsureAuthed(); err != nil {
				return err
			}
			if err := a.Connect(ctx, false, nil); err != nil {
				return err
			}

			if chat == "" {
				return fmt.Errorf("--chat is required")
			}
			chatJID, err := wa.ParseUserOrJID(chat)
			if err != nil {
				return fmt.Errorf("invalid --chat: %w", err)
			}

			// Look up the latest message to get timestamp and sender
			lastID := args[len(args)-1]
			msg, err := a.DB().GetMessage(chatJID.String(), lastID)
			if err != nil {
				return fmt.Errorf("message %s not found in DB for chat %s", lastID, chatJID)
			}

			var senderJID types.JID
			if msg.SenderJID != "" {
				senderJID, _ = wa.ParseUserOrJID(msg.SenderJID)
			}

			ids := make([]types.MessageID, len(args))
			for i, id := range args {
				ids[i] = types.MessageID(id)
			}

			if err := a.WA().MarkRead(ctx, ids, msg.Timestamp, chatJID, senderJID); err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"read":  true,
					"chat":  chatJID.String(),
					"count": len(ids),
					"ids":   args,
				})
			}
			fmt.Fprintf(os.Stdout, "Marked %d message(s) as read in %s\n", len(ids), chatJID.String())
			return nil
		},
	}

	cmd.Flags().StringVar(&chat, "chat", "", "chat JID (auto-detected from message if omitted)")
	return cmd
}
