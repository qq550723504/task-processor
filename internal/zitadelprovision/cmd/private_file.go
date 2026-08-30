package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func writePrivateFile(path string, content []byte) (returnErr error) {
	return writePrivateFileWithReplace(path, content, replacePrivateFile)
}

func writePrivateFileWithReplace(path string, content []byte, replace func(string, string) error) (returnErr error) {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create private directory: %w", err)
	}
	if err := rejectSymlinkPath(directory); err != nil {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".runtime-env-*.tmp")
	if err != nil {
		return fmt.Errorf("create private temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	// Protect the empty temporary file before any secret bytes are written.
	if err := protectPrivateFile(temporaryPath); err != nil {
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		return fmt.Errorf("write private temporary file: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		return fmt.Errorf("sync private temporary file: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close private temporary file: %w", err)
	}
	if err := rejectSymlinkPath(directory); err != nil {
		return err
	}
	if _, err := os.Lstat(path); err == nil {
		if err := rejectSymlinkPath(path); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect private runtime file: %w", err)
	}
	if err := replace(temporaryPath, path); err != nil {
		return fmt.Errorf("install private runtime file: %w", err)
	}
	return nil
}
