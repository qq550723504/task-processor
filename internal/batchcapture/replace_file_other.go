//go:build !windows

package batchcapture

import "os"

// replaceFile atomically replaces target with source.
//
// On POSIX, rename(2) is the atomic replacement, but it is only durable once the
// containing directory has been flushed; Save calls syncDir immediately afterwards
// and refuses to report success if that flush fails (design section 4 D2.2).
func replaceFile(source, target string) error {
	return os.Rename(source, target)
}
