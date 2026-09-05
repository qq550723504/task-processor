//go:build !windows && !linux

package ownershipmigration

import (
	"fmt"
	"os"
	"reflect"
)

// directoryFilesystemIdentity keeps non-production Unix-like ports buildable
// while failing closed when FileInfo.Sys does not expose device/inode identity.
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
