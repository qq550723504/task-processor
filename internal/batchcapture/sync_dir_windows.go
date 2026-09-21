//go:build windows

package batchcapture

// syncDir reports whether the directory entry update that made an atomic replace
// durable has been flushed.
//
// On Windows there is nothing to flush: NTFS journals metadata, so the rename in
// Save is already durable once it returns, and FlushFileBuffers is not supported
// on a directory handle (it fails with "Access is denied"). This file states that
// platform fact explicitly instead of leaving a swallowed error in the shared code
// path, which is what made the durability contract impossible to review.
//
// The file contents themselves are still flushed before the replace (Save calls
// Sync on the temporary file), which is the part Windows does support.
func syncDir(string) error { return nil }
