package currentapplication

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestContextWithShutdownFileCancelsOnlyAfterOwnedFileAppears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stop")
	ctx, cancel, err := ContextWithShutdownFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	select {
	case <-ctx.Done():
		t.Fatal("context canceled before shutdown file existed")
	case <-time.After(50 * time.Millisecond):
	}
	if err := os.WriteFile(path, []byte("stop"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ctx.Done():
	case <-time.After(time.Second):
		t.Fatal("context was not canceled after shutdown file appeared")
	}
}

func TestContextWithShutdownFileRejectsRelativePath(t *testing.T) {
	ctx, cancel, err := ContextWithShutdownFile(context.Background(), "stop")
	if err == nil || ctx != nil || cancel != nil {
		t.Fatalf("ContextWithShutdownFile() = %#v, %#v, %v", ctx, cancel, err)
	}
}
