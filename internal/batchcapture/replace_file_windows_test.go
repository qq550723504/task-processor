//go:build windows

package batchcapture

import (
	"testing"

	"golang.org/x/sys/windows"
)

// Design section 4 D2.2 requires the Windows replacement to be
// MoveFileExW(MOVEFILE_REPLACE_EXISTING|MOVEFILE_WRITE_THROUGH), and Go's os.Rename
// only passes MOVEFILE_REPLACE_EXISTING.
//
// This is pinned by asserting the flag combination instead of by testing durability,
// because write-through only makes a difference across a power loss and cannot be
// observed from a running process: if the flag were dropped, every functional test
// would still pass while the queue silently lost its crash guarantee. The bit that
// must not disappear is the one this test names.
func TestReplaceIsWriteThroughOnWindows(t *testing.T) {
	if replaceFileFlags&windows.MOVEFILE_WRITE_THROUGH == 0 {
		t.Fatal("the Windows replacement dropped MOVEFILE_WRITE_THROUGH, so a power loss can restore the previous queue file")
	}
	if replaceFileFlags&windows.MOVEFILE_REPLACE_EXISTING == 0 {
		t.Fatal("the Windows replacement dropped MOVEFILE_REPLACE_EXISTING, so replacing an existing queue file would fail")
	}
}
