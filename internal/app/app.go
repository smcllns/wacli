package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/steipete/wacli/internal/store"
	"github.com/steipete/wacli/internal/wa"
)

type Options struct {
	StoreDir      string
	Version       string
	JSON          bool
	AllowUnauthed bool
}

type App struct {
	opts Options
	wa   *wa.Client
	db   *store.DB
}

func New(opts Options) (*App, error) {
	if opts.StoreDir == "" {
		return nil, fmt.Errorf("store dir is required")
	}
	if err := os.MkdirAll(opts.StoreDir, 0700); err != nil {
		return nil, fmt.Errorf("create store dir: %w", err)
	}

	indexPath := filepath.Join(opts.StoreDir, "wacli.db")

	db, err := store.Open(indexPath)
	if err != nil {
		return nil, err
	}

	return &App{opts: opts, db: db}, nil
}

func (a *App) OpenWA() error {
	if a.wa != nil {
		return nil
	}
	sessionPath := filepath.Join(a.opts.StoreDir, "session.db")
	cli, err := wa.New(wa.Options{
		StorePath: sessionPath,
	})
	if err != nil {
		return err
	}

	a.wa = cli
	return nil
}

func (a *App) Close() {
	if a.wa != nil {
		a.wa.Close()
	}
	if a.db != nil {
		_ = a.db.Close()
	}
}

func (a *App) EnsureAuthed() error {
	if err := a.OpenWA(); err != nil {
		return err
	}
	if a.wa.IsAuthed() {
		return nil
	}
	return fmt.Errorf("not authenticated; run `wacli auth`")
}

func (a *App) WA() *wa.Client     { return a.wa }
func (a *App) DB() *store.DB      { return a.db }
func (a *App) StoreDir() string   { return a.opts.StoreDir }
func (a *App) Version() string    { return a.opts.Version }
func (a *App) AllowUnauthed() bool { return a.opts.AllowUnauthed }

func (a *App) Connect(ctx context.Context, allowQR bool, qrWriter func(string)) error {
	if err := a.OpenWA(); err != nil {
		return err
	}
	return a.wa.Connect(ctx, wa.ConnectOptions{
		AllowQR:  allowQR,
		OnQRCode: qrWriter,
	})
}
