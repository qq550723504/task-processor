//go:build linux

package sourceaccountownershiprehearsal

import (
	"os"
	"path/filepath"
	"strconv"
)

func sameExecutableParent() bool {
	self, err := os.Executable()
	if err != nil {
		return false
	}
	parent, err := os.Readlink(filepath.Join("/proc", strconv.Itoa(os.Getppid()), "exe"))
	if err != nil {
		return false
	}
	selfInfo, err := os.Stat(self)
	if err != nil {
		return false
	}
	parentInfo, err := os.Stat(parent)
	return err == nil && os.SameFile(selfInfo, parentInfo)
}
