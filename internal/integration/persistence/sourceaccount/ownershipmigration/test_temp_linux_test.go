//go:build linux

package ownershipmigration

import (
	"os"
	"testing"
)

func supportedTestTempDir(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile("/proc/self/mountinfo")
	if err != nil {
		t.Fatal(err)
	}
	entries, err := parseLinuxMountInfo(data)
	if err != nil {
		t.Fatal(err)
	}
	mounts := indexLinuxMounts(entries)
	for _, base := range []string{"/dev/shm", os.TempDir()} {
		if info, statErr := os.Stat(base); statErr != nil || !info.IsDir() {
			continue
		}
		dir, makeErr := os.MkdirTemp(base, "ownershipmigration-test-")
		if makeErr != nil {
			continue
		}
		if _, identityErr := linuxFilesystemLocation(dir, mounts); identityErr == nil {
			t.Cleanup(func() { _ = os.RemoveAll(dir) })
			return dir
		}
		_ = os.RemoveAll(dir)
	}
	t.Fatal("no temporary directory uses a filesystem supported by ownership preflight")
	return ""
}
