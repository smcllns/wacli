package app

import (
	"context"
	"database/sql"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/steipete/wacli/internal/pathutil"
	"github.com/steipete/wacli/internal/store"
)

type mediaJob struct {
	chatJID string
	msgID   string
}

type MediaDownloadResult struct {
	ChatJID      string
	MsgID        string
	Path         string
	Bytes        int64
	MediaType    string
	MimeType     string
	DownloadedAt time.Time
}

func (a *App) ResolveMediaOutputPath(info store.MediaDownloadInfo, requested string) (string, error) {
	filename := mediaFilename(info)

	if strings.TrimSpace(requested) != "" {
		out := requested
		if !filepath.IsAbs(out) {
			if abs, err := filepath.Abs(out); err == nil {
				out = abs
			}
		}
		if st, err := os.Stat(out); err == nil && st.IsDir() {
			return filepath.Join(out, filename), nil
		}
		if strings.HasSuffix(out, string(os.PathSeparator)) {
			return filepath.Join(out, filename), nil
		}
		return out, nil
	}

	baseDir := filepath.Join(a.opts.StoreDir, "media", pathutil.SanitizeSegment(info.ChatJID), pathutil.SanitizeSegment(info.MsgID))
	if info.MediaType != "" {
		baseDir = filepath.Join(baseDir, pathutil.SanitizeSegment(info.MediaType))
	}
	if abs, err := filepath.Abs(baseDir); err == nil {
		baseDir = abs
	}
	return filepath.Join(baseDir, filename), nil
}

func mediaFilename(info store.MediaDownloadInfo) string {
	name := strings.TrimSpace(info.Filename)
	ext := ""
	if strings.TrimSpace(info.MimeType) != "" {
		if exts, err := mime.ExtensionsByType(info.MimeType); err == nil && len(exts) > 0 {
			ext = exts[0]
		}
	}

	if name == "" {
		base := "message-" + pathutil.SanitizeSegment(info.MsgID)
		if ext == "" {
			ext = ".bin"
		}
		return pathutil.SanitizeFilename(base + ext)
	}

	name = pathutil.SanitizeFilename(name)
	if ext != "" && filepath.Ext(name) == "" {
		name += ext
	}
	return name
}

func (a *App) runMediaWorkers(ctx context.Context, jobs <-chan mediaJob, workers int) (func(), error) {
	if workers <= 0 {
		workers = 2
	}
	if jobs == nil {
		return func() {}, nil
	}

	ctx, cancel := context.WithCancel(ctx)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-jobs:
					if strings.TrimSpace(job.chatJID) == "" || strings.TrimSpace(job.msgID) == "" {
						continue
					}
					if err := a.downloadMediaJob(ctx, job); err != nil {
						fmt.Fprintf(os.Stderr, "media download failed for %s/%s: %v\n", job.chatJID, job.msgID, err)
					}
				}
			}
		}()
	}

	stop := func() {
		cancel()
		wg.Wait()
	}
	return stop, nil
}

func (a *App) DownloadMedia(ctx context.Context, chatJID, msgID, outputPath string) (MediaDownloadResult, error) {
	info, err := a.db.GetMediaDownloadInfo(chatJID, msgID)
	if err != nil {
		return MediaDownloadResult{}, err
	}
	if strings.TrimSpace(info.MediaType) == "" || strings.TrimSpace(info.DirectPath) == "" || len(info.MediaKey) == 0 {
		return MediaDownloadResult{}, fmt.Errorf("message has no downloadable media metadata (run `wacli sync` first)")
	}

	targetPath, err := a.ResolveMediaOutputPath(info, outputPath)
	if err != nil {
		return MediaDownloadResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(targetPath), 0700); err != nil {
		return MediaDownloadResult{}, err
	}

	bytes, err := a.wa.DownloadMediaToFile(ctx, info.DirectPath, info.FileEncSHA256, info.FileSHA256, info.MediaKey, info.FileLength, info.MediaType, "", targetPath)
	if err != nil {
		return MediaDownloadResult{}, err
	}

	now := time.Now().UTC()
	if err := a.db.MarkMediaDownloaded(info.ChatJID, info.MsgID, targetPath, now); err != nil {
		return MediaDownloadResult{}, err
	}
	return MediaDownloadResult{
		ChatJID:      info.ChatJID,
		MsgID:        info.MsgID,
		Path:         targetPath,
		Bytes:        bytes,
		MediaType:    info.MediaType,
		MimeType:     info.MimeType,
		DownloadedAt: now,
	}, nil
}

func (a *App) downloadMediaJob(ctx context.Context, job mediaJob) error {
	info, err := a.db.GetMediaDownloadInfo(job.chatJID, job.msgID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil
		}
		return err
	}
	if strings.TrimSpace(info.LocalPath) != "" {
		return nil
	}
	if strings.TrimSpace(info.MediaType) == "" || strings.TrimSpace(info.DirectPath) == "" || len(info.MediaKey) == 0 {
		return nil
	}
	_, err = a.DownloadMedia(ctx, job.chatJID, job.msgID, "")
	return err
}
