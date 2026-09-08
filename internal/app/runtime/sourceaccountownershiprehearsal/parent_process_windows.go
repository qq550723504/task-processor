//go:build windows

package sourceaccountownershiprehearsal

import (
	"os"

	"golang.org/x/sys/windows"
)

func sameExecutableParent() bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(os.Getppid()))
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)
	buffer := make([]uint16, 32768)
	size := uint32(len(buffer))
	if err = windows.QueryFullProcessImageName(handle, 0, &buffer[0], &size); err != nil {
		return false
	}
	parent := windows.UTF16ToString(buffer[:size])
	selfInfo, err := os.Stat(self)
	if err != nil {
		return false
	}
	parentInfo, err := os.Stat(parent)
	return err == nil && os.SameFile(selfInfo, parentInfo)
}
