//go:build windows

package batchcapture

// syncDir reports whether the directory entry update that made an atomic replace
// durable has been flushed.
//
// On Windows there is nothing left to flush here: replaceFile already performs the
// replacement with MOVEFILE_WRITE_THROUGH (replace_file_windows.go), so MoveFileExW
// does not return until the replacement is durable. FlushFileBuffers is not
// supported on a directory handle in any case (it fails with "Access is denied"),
// and reporting that platform limitation as a durability failure would break every
// save on Windows.
//
// The file contents themselves are flushed before the replace (Save calls Sync on
// the temporary file), which is the part Windows does support. This file states the
// platform fact explicitly instead of leaving a swallowed error in the shared code
// path, which is what made the durability contract impossible to review.
func syncDir(string) error { return nil }
