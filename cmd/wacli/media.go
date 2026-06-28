package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/steipete/wacli/internal/out"
)

func newMediaCmd(flags *rootFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "media",
		Short: "Media download",
	}
	cmd.AddCommand(newMediaDownloadCmd(flags))
	return cmd
}

func newMediaDownloadCmd(flags *rootFlags) *cobra.Command {
	var chat string
	var id string
	var outputPath string

	cmd := &cobra.Command{
		Use:   "download",
		Short: "Download media for a message",
		RunE: func(cmd *cobra.Command, args []string) error {
			if chat == "" || id == "" {
				return fmt.Errorf("--chat and --id are required")
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

			result, err := a.DownloadMedia(ctx, chat, id, outputPath)
			if err != nil {
				return err
			}

			resp := map[string]any{
				"chat":          result.ChatJID,
				"id":            result.MsgID,
				"path":          result.Path,
				"bytes":         result.Bytes,
				"media_type":    result.MediaType,
				"mime_type":     result.MimeType,
				"downloaded":    true,
				"downloaded_at": result.DownloadedAt.Format(time.RFC3339Nano),
			}
			if flags.asJSON {
				return out.WriteJSON(os.Stdout, resp)
			}
			fmt.Fprintf(os.Stdout, "%s (%d bytes)\n", result.Path, result.Bytes)
			return nil
		},
	}

	cmd.Flags().StringVar(&chat, "chat", "", "chat JID")
	cmd.Flags().StringVar(&id, "id", "", "message ID")
	cmd.Flags().StringVar(&outputPath, "output", "", "output file or directory (default: store media dir)")
	_ = cmd.MarkFlagRequired("chat")
	_ = cmd.MarkFlagRequired("id")
	return cmd
}
