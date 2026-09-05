//go:build windows

package ownershipmigration

import (
	"context"
	"fmt"
	"io/fs"
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
		// Walk directory-entry metadata only. WalkDir never follows symlinks;
		// reject descendant junction/mount reparse aliases before traversing them,
		// extending the existing no-symlink profile contract below the account root.
		err := filepath.WalkDir(account.ProfileDirectory, func(path string, entry fs.DirEntry, walkErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if walkErr != nil {
				return fmt.Errorf("cannot inspect profile directory entries")
			}
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
			// Directory enumeration may report a junction as ModeIrregular and
			// IsDir=false. Use the native directory/reparse attributes as well.
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
			return nil
		})
		if err != nil {
			return fmt.Errorf("source account %d: %w", account.Previous.ID, err)
		}
	}
	return nil
}
