//go:build windows

package batchcapture

import "testing"

// TestSyncDirReportsNothingToFlushOnWindows records the platform fact the
// durability path depends on: Windows has no directory fsync (FlushFileBuffers is
// rejected for a directory handle with "Access is denied"), and NTFS journals the
// metadata update that Save's atomic replace performs. Returning nil here is a
// statement about the platform, not a swallowed error.
func TestSyncDirReportsNothingToFlushOnWindows(t *testing.T) {
	if err := syncDir(t.TempDir()); err != nil {
		t.Fatalf("syncDir on windows: %v", err)
	}
}
