//go:build windows

package ownershipmigration

import (
	"fmt"
	"os"

	"golang.org/x/sys/windows"
)

// directoryFilesystemIdentity uses the volume serial plus Windows file index,
// matching the stable object identity used for same-file decisions on one host.
func directoryFilesystemIdentity(path string, _ os.FileInfo) (string, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}
	handle, err := windows.CreateFile(
		name,
		windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)
	var information windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(handle, &information); err != nil {
		return "", err
	}
	return fmt.Sprintf("%08x:%08x:%08x", information.VolumeSerialNumber, information.FileIndexHigh, information.FileIndexLow), nil
}
