//go:build windows

package batchcapture

import (
	"golang.org/x/sys/windows"
)

// replaceFileFlags is the flag combination design section 4 D2.2 mandates for the
// atomic replace on Windows.
//
// MOVEFILE_REPLACE_EXISTING swaps in the new file, and MOVEFILE_WRITE_THROUGH makes
// MoveFileExW wait until the replacement itself is durable. Write-through is the
// Windows equivalent of flushing the directory entry on POSIX: without it the call
// only reports that the name now points somewhere else in the cache, so a power
// loss after Save returned success could expose the previous queue file — the old,
// still re-capturable state that must never come back.
//
// os.Rename is deliberately not used on this path: Go implements it with
// MoveFileEx(MOVEFILE_REPLACE_EXISTING) alone, which is not a confirmed durable
// replacement.
const replaceFileFlags = windows.MOVEFILE_REPLACE_EXISTING | windows.MOVEFILE_WRITE_THROUGH

// replaceFile atomically replaces target with source and does not return until the
// replacement is durable.
func replaceFile(source, target string) error {
	sourcePointer, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	targetPointer, err := windows.UTF16PtrFromString(target)
	if err != nil {
		return err
	}
	return windows.MoveFileEx(sourcePointer, targetPointer, replaceFileFlags)
}
