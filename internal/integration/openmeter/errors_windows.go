//go:build windows

package openmeter

import (
	"errors"
	"syscall"

	"golang.org/x/sys/windows"
)

func isConnectionRefusedOrReset(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET) || errors.Is(err, windows.WSAECONNREFUSED) || errors.Is(err, windows.WSAECONNRESET)
}
