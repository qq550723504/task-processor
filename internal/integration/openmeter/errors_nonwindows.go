//go:build !windows

package openmeter

import (
	"errors"
	"syscall"
)

func isConnectionRefusedOrReset(err error) bool {
	return errors.Is(err, syscall.ECONNREFUSED) || errors.Is(err, syscall.ECONNRESET)
}
