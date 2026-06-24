package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	appPkg "github.com/steipete/wacli/internal/app"
)

func newDaemonCmd(flags *rootFlags) *cobra.Command {
	var socketPath string
	var queueSize int

	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Run the realtime WhatsApp daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.TrimSpace(socketPath) == "" {
				return fmt.Errorf("--socket is required")
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			a, lk, err := newApp(ctx, flags, true, false)
			if err != nil {
				return err
			}
			defer closeApp(a, lk)

			return a.RunDaemon(ctx, appPkg.DaemonOptions{SocketPath: socketPath, QueueSize: queueSize})
		},
	}

	cmd.Flags().StringVar(&socketPath, "socket", "", "Unix socket path")
	cmd.Flags().IntVar(&queueSize, "queue-size", 256, "maximum queued daemon write commands")
	return cmd
}
