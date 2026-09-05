//go:build windows

package ownershipmigration

import (
	"testing"

	"golang.org/x/sys/windows"
)

// Native API result fixtures; this does not exercise a real network share.
func TestWindowsStorageIsolation(t *testing.T) {
	for _, kind := range []uint32{windows.DRIVE_REMOTE, windows.DRIVE_UNKNOWN, windows.DRIVE_NO_ROOT_DIR, 100} {
		if err := validateWindowsDriveType(kind); err == nil {
			t.Fatalf("unprovable drive type %d accepted", kind)
		}
	}
	for _, kind := range []uint32{windows.DRIVE_FIXED, windows.DRIVE_REMOVABLE, windows.DRIVE_RAMDISK, windows.DRIVE_CDROM} {
		if err := validateWindowsDriveType(kind); err != nil {
			t.Fatalf("local drive type %d rejected: %v", kind, err)
		}
	}
	dir := t.TempDir()
	if _, err := directoryFilesystemIdentity(dir, nil); err != nil {
		t.Fatalf("native local storage identity rejected: %v", err)
	}
	if _, err := directoryFilesystemIdentity(dir+`\missing`, nil); err == nil {
		t.Fatal("unresolvable directory accepted")
	}
}
