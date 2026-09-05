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
	// Remote/mapped shares cannot use local volume serials as isolation proof.
	// Query only this path's volume, never enumerate unrelated network drives.
	volume := make([]uint16, 32768)
	if err := windows.GetVolumePathName(name, &volume[0], uint32(len(volume))); err != nil {
		return "", fmt.Errorf("cannot resolve directory storage: %w", err)
	}
	if err := validateWindowsDriveType(windows.GetDriveType(&volume[0])); err != nil {
		return "", err
	}
	information, err := windowsFilesystemObjectInformation(path)
	if err != nil {
		return "", err
	}
	return windowsFilesystemObjectIdentity(information), nil
}

func windowsFilesystemObjectInformation(path string) (windows.ByHandleFileInformation, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return windows.ByHandleFileInformation{}, err
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
		return windows.ByHandleFileInformation{}, err
	}
	defer windows.CloseHandle(handle)
	var information windows.ByHandleFileInformation
	if err = windows.GetFileInformationByHandle(handle, &information); err != nil {
		return windows.ByHandleFileInformation{}, err
	}
	return information, nil
}

func windowsFilesystemObjectIdentity(information windows.ByHandleFileInformation) string {
	return fmt.Sprintf("%08x:%08x:%08x", information.VolumeSerialNumber, information.FileIndexHigh, information.FileIndexLow)
}

func validateWindowsDriveType(driveType uint32) error {
	switch driveType {
	case windows.DRIVE_FIXED, windows.DRIVE_REMOVABLE, windows.DRIVE_RAMDISK, windows.DRIVE_CDROM:
		return nil
	default:
		return fmt.Errorf("cannot prove profile/receipt isolation on remote or unknown storage")
	}
}
