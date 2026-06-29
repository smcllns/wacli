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

func newSendEditCmd(flags *rootFlags) *cobra.Command {
	var to string
	var targetID string
	var message string

	cmd := &cobra.Command{
		Use:   "edit",
		Short: "Edit a previously sent text message",
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" || targetID == "" || message == "" {
				return fmt.Errorf("--to, --message-id, and --message are required")
			}

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

			toJID, err := wa.ParseUserOrJID(to)
			if err != nil {
				return err
			}

			resp, err := a.WA().SendEdit(ctx, toJID, types.MessageID(targetID), message)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"sent":      true,
					"to":        toJID.String(),
					"id":        resp.ID,
					"target_id": targetID,
				})
			}
			fmt.Fprintf(os.Stdout, "Edited message %s in %s (event id %s)\n", targetID, toJID.String(), resp.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "recipient phone number or JID")
	cmd.Flags().StringVar(&targetID, "message-id", "", "ID of the previously sent message to edit")
	cmd.Flags().StringVar(&message, "message", "", "new message text")
	return cmd
}
