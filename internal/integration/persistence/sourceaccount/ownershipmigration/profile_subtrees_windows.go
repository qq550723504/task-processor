//go:build windows

package ownershipmigration

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"

	"golang.org/x/sys/windows"
)

func validateProfileSubtrees(ctx context.Context, accounts []AccountEvidence) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	hardLinkOwners := make(map[string]int64)
	for _, account := range accounts {
		var walk func(string) error
		walk = func(directory string) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			handle, err := os.Open(directory)
			if err != nil {
				return fmt.Errorf("cannot inspect profile directory entries")
			}
			defer handle.Close()
			for {
				if err := ctx.Err(); err != nil {
					return err
				}
				entries, readErr := handle.ReadDir(profileDirectoryReadBatchSize)
				if readErr != nil && readErr != io.EOF {
					return fmt.Errorf("cannot inspect profile directory entries")
				}
				for _, entry := range entries {
					if err := ctx.Err(); err != nil {
						return err
					}
					path := filepath.Join(directory, entry.Name())
					if entry.Type()&os.ModeSymlink != 0 {
						return fmt.Errorf("browser profile contains a descendant reparse alias")
					}
					info, err := entry.Info()
					if err != nil {
						return fmt.Errorf("cannot inspect profile directory attributes")
					}
					attributes, ok := info.Sys().(*syscall.Win32FileAttributeData)
					if !ok || attributes == nil {
						return fmt.Errorf("browser profile entry attributes are unavailable")
					}
					// Directory enumeration may report a junction as ModeIrregular.
					if attributes.FileAttributes&windows.FILE_ATTRIBUTE_DIRECTORY != 0 && attributes.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
						return fmt.Errorf("browser profile directory identity is unavailable or reparsed")
					}
					if info.Mode().IsRegular() {
						information, err := windowsFilesystemObjectInformation(path)
						if err != nil {
							return fmt.Errorf("cannot identify browser profile entry")
						}
						if information.NumberOfLinks > 1 {
							identity := windowsFilesystemObjectIdentity(information)
							if owner, exists := hardLinkOwners[identity]; exists && owner != account.Previous.ID {
								return fmt.Errorf("source accounts %d and %d share hard-linked browser profile state", owner, account.Previous.ID)
							}
							hardLinkOwners[identity] = account.Previous.ID
						}
					}
					if info.IsDir() {
						if err := walk(path); err != nil {
							return err
						}
					}
				}
				if readErr == io.EOF {
					return nil
				}
			}
		}
		// Fixed-size batches bound high-fanout memory and cancellation latency.
		err := walk(account.ProfileDirectory)
		if err != nil {
			return fmt.Errorf("source account %d: %w", account.Previous.ID, err)
		}
	}
	return nil
}

const profileDirectoryReadBatchSize = 128
