//go:build !windows

package ownershipmigration

import (
	"fmt"
	"os"
	"reflect"
)

// directoryFilesystemIdentity returns the same underlying directory identity that
// os.SameFile compares on POSIX-like systems, but in an indexable representation.
// Reflection keeps this file buildable across non-Windows ports while failing
// closed on a platform whose FileInfo.Sys value does not expose device/inode data.
func directoryFilesystemIdentity(_ string, info os.FileInfo) (string, error) {
	if info == nil || info.Sys() == nil {
		return "", fmt.Errorf("filesystem identity unavailable")
	}
	value := reflect.ValueOf(info.Sys())
	if value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return "", fmt.Errorf("filesystem identity unavailable")
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return "", fmt.Errorf("filesystem identity unavailable")
	}
	device := value.FieldByName("Dev")
	inode := value.FieldByName("Ino")
	if !device.IsValid() || !inode.IsValid() || !device.CanInterface() || !inode.CanInterface() {
		return "", fmt.Errorf("filesystem identity unavailable")
	}
	return fmt.Sprintf("%T:%v:%T:%v", device.Interface(), device.Interface(), inode.Interface(), inode.Interface()), nil
}
