//go:build linux

package sourceaccountownershiprehearsal

import "os"

func rehearsalStorageCandidates() []string {
	return []string{"/dev/shm", os.TempDir()}
}
