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

func newSendReactCmd(flags *rootFlags) *cobra.Command {
	var to string
	var id string
	var reaction string
	var sender string

	cmd := &cobra.Command{
		Use:   "react",
		Short: "Send a reaction to a message",
		RunE: func(cmd *cobra.Command, args []string) error {
			if to == "" || id == "" {
				return fmt.Errorf("--to and --id are required")
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

			var senderJID types.JID
			if sender != "" {
				senderJID, err = wa.ParseUserOrJID(sender)
				if err != nil {
					return fmt.Errorf("invalid --sender: %w", err)
				}
			}

			msgID, err := a.WA().SendReaction(ctx, toJID, senderJID, types.MessageID(id), reaction)
			if err != nil {
				return err
			}

			if flags.asJSON {
				return out.WriteJSON(os.Stdout, map[string]any{
					"sent":     true,
					"to":       toJID.String(),
					"id":       msgID,
					"target":   id,
					"reaction": reaction,
				})
			}
			if reaction == "" {
				fmt.Fprintf(os.Stdout, "Removed reaction from %s in %s (id %s)\n", id, toJID.String(), msgID)
			} else {
				fmt.Fprintf(os.Stdout, "Reacted %s to %s in %s (id %s)\n", reaction, id, toJID.String(), msgID)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&to, "to", "", "recipient phone number or JID")
	cmd.Flags().StringVar(&id, "id", "", "target message ID to react to")
	cmd.Flags().StringVar(&reaction, "reaction", "👍", "reaction emoji (empty string to remove)")
	cmd.Flags().StringVar(&sender, "sender", "", "sender JID (for group messages)")
	return cmd
}
