package lock

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLockBlocksOtherProcess(t *testing.T) {
	if os.Getenv("WACLI_LOCK_HELPER") == "1" {
		dir := os.Getenv("WACLI_LOCK_DIR")
		lk, err := Acquire(dir)
		if err != nil {
			t.Fatalf("helper acquire: %v", err)
		}
		defer lk.Release()
		_, _ = os.Stdout.WriteString("READY\n")
		// Hold until parent kills us.
		select {}
	}

	dir := t.TempDir()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=TestLockBlocksOtherProcess")
	cmd.Env = append(os.Environ(),
		"WACLI_LOCK_HELPER=1",
		"WACLI_LOCK_DIR="+dir,
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_, _ = cmd.Process.Wait()
	}()

	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("read helper output: %v", err)
	}
	if strings.TrimSpace(line) != "READY" {
		t.Fatalf("unexpected helper output: %q", line)
	}

	lk, err := Acquire(dir)
	if err == nil {
		_ = lk.Release()
		t.Fatalf("expected lock acquire to fail")
	}
	if !strings.Contains(err.Error(), "store is locked") {
		t.Fatalf("expected 'store is locked' error, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "LOCK")); statErr != nil {
		t.Fatalf("expected LOCK file to exist: %v", statErr)
	}
}

