package currentapplication

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// ContextWithShutdownFile provides the task-owned Windows supervisor with a
// graceful, run-private stop boundary. It never removes or creates the file.
func ContextWithShutdownFile(parent context.Context, path string) (context.Context, context.CancelFunc, error) {
	if parent == nil || !filepath.IsAbs(path) {
		return nil, nil, errors.New("shutdown file path must be absolute")
	}
	if _, err := os.Lstat(path); err == nil {
		ctx, cancel := context.WithCancel(parent)
		cancel()
		return ctx, cancel, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if _, err := os.Lstat(path); err == nil || !errors.Is(err, os.ErrNotExist) {
					cancel()
					return
				}
			}
		}
	}()
	return ctx, cancel, nil
}
