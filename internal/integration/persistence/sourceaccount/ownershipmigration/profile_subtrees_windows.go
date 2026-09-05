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
	for _, account := range accounts {
		// Walk directory-entry metadata only. WalkDir never follows symlinks;
		// reject descendant junction/mount reparse aliases before traversing them,
		// extending the existing no-symlink profile contract below the account root.
		err := filepath.WalkDir(account.ProfileDirectory, func(_ string, entry fs.DirEntry, walkErr error) error {
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
			return nil
		})
		if err != nil {
			return fmt.Errorf("source account %d: %w", account.Previous.ID, err)
		}
	}
	return nil
}
