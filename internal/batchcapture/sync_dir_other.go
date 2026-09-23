//go:build !windows

package batchcapture

import "os"

// syncDir flushes the directory entry that the atomic replace in Save created.
//
// On POSIX filesystems fsyncing the file is not enough: the rename itself lives in
// the directory, so a crash can leave a flushed file that the directory does not
// name. The error is returned rather than discarded, because a caller may only
// hand off after the record is confirmed durable.
func syncDir(dir string) error {
	f, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return f.Sync()
}
