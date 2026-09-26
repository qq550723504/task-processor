//go:build !windows

package batchcapture

import (
	"path/filepath"
	"testing"
)

// TestSyncDirPropagatesFailures is the regression test for the first defect found
// in review: the directory flush after the atomic replace was discarded, so Save
// could report success even though the rename was not durable. A caller that has
// been told the write succeeded may then hand off, which is the one thing the
// crash-safe queue must prevent.
func TestSyncDirPropagatesFailures(t *testing.T) {
	if err := syncDir(t.TempDir()); err != nil {
		t.Fatalf("syncDir on an existing directory: %v", err)
	}
	if err := syncDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("syncDir accepted a directory that does not exist")
	}
}
