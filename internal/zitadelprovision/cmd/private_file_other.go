//go:build !windows

package main

import (
	"errors"
	"os"
)

func protectPrivateFile(path string) error {
	if err := os.Chmod(path, 0o600); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if info.Mode().Perm() != 0o600 {
		return errors.New("private runtime file mode is not 0600")
	}
	return nil
}

func replacePrivateFile(source string, target string) error {
	return os.Rename(source, target)
}

func isUnsafePathComponent(_ string, info os.FileInfo) (bool, error) {
	return info.Mode()&os.ModeSymlink != 0, nil
}
