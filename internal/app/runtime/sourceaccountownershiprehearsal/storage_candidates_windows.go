//go:build windows

package sourceaccountownershiprehearsal

import "os"

func rehearsalStorageCandidates() []string {
	return []string{os.TempDir()}
}
