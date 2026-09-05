package ownershipmigration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidateReceiptTarget verifies the destination without creating anything.
// The receipt must live outside the profile tree and its parent path must not
// traverse a symlink/junction alias. Filesystem identities of the profile root,
// tenant directories and verified account directories are indexed so a direct
// bind-mount alias fails closed without quadratic scans.
func ValidateReceiptTarget(profileRoot, path string, r Receipt) error {
	if !filepath.IsAbs(profileRoot) || !filepath.IsAbs(path) {
		return fmt.Errorf("absolute profile root and receipt path required")
	}
	profileRoot = filepath.Clean(profileRoot)
	path = filepath.Clean(path)
	if _, err := os.Lstat(path); err == nil {
		return fmt.Errorf("receipt destination already exists")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot inspect receipt destination")
	}
	parent := filepath.Dir(path)
	if _, err := verifyDirectoryInfo(parent); err != nil {
		return fmt.Errorf("receipt parent missing, non-directory or aliased")
	}
	if insidePath(profileRoot, parent) {
		return fmt.Errorf("receipt path must be outside the browser profile root")
	}

	protectedPaths := map[string]struct{}{profileRoot: {}}
	for _, account := range r.Accounts {
		if account.ProfileDirectory == "" || !filepath.IsAbs(account.ProfileDirectory) {
			return fmt.Errorf("invalid profile directory in receipt")
		}
		for p := filepath.Clean(account.ProfileDirectory); ; p = filepath.Dir(p) {
			protectedPaths[p] = struct{}{}
			if sameCleanPath(p, profileRoot) || filepath.Dir(p) == p {
				break
			}
		}
	}
	protectedIdentities := make(map[string]struct{}, len(protectedPaths))
	for protectedPath := range protectedPaths {
		info, err := verifyDirectoryInfo(protectedPath)
		if err != nil {
			return fmt.Errorf("cannot verify protected profile directory")
		}
		identity, err := directoryFilesystemIdentity(protectedPath, info)
		if err != nil {
			return fmt.Errorf("cannot identify protected profile directory")
		}
		protectedIdentities[identity] = struct{}{}
	}
	for p := parent; ; p = filepath.Dir(p) {
		info, err := os.Stat(p)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("cannot verify receipt parent ancestry")
		}
		identity, err := directoryFilesystemIdentity(p, info)
		if err != nil {
			return fmt.Errorf("cannot identify receipt parent ancestry")
		}
		if _, exists := protectedIdentities[identity]; exists {
			return fmt.Errorf("receipt parent aliases the browser profile tree")
		}
		if filepath.Dir(p) == p {
			break
		}
	}
	return nil
}

func insidePath(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)))
}

func sameCleanPath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}

// WriteReceipt publishes a complete, synced file with no replacement. Hard-link
// publication fails closed on filesystems without support. A process interruption
// before publication can leave only a .pending file, never a partial final receipt.
// Returning nil is the final success decision for this publication attempt; later
// context cancellation does not retroactively invalidate an already committed receipt.
func WriteReceipt(ctx context.Context, path string, r Receipt) error {
	return writeReceipt(ctx, path, r, nil)
}

func writeReceipt(ctx context.Context, path string, r Receipt, beforePublish func()) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.Version != 1 || r.Stage != "preflight_only" || r.SnapshotConsistency != "separate_non_atomic_snapshots" || r.Digest != receiptDigest(r) {
		return fmt.Errorf("invalid preflight receipt")
	}
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ownership-receipt-*.pending")
	if err != nil {
		return fmt.Errorf("cannot create receipt staging file")
	}
	defer func() { _ = tmp.Close(); _ = os.Remove(tmp.Name()) }()
	if _, err = tmp.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("cannot write receipt")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return fmt.Errorf("cannot sync receipt")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("cannot close receipt")
	}
	if beforePublish != nil {
		beforePublish()
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = os.Link(tmp.Name(), path); err != nil {
		return fmt.Errorf("cannot publish receipt: destination exists or filesystem does not support exclusive hard-link publication")
	}
	if err = ctx.Err(); err != nil {
		if removeErr := os.Remove(path); removeErr != nil {
			return fmt.Errorf("receipt publication cancelled but published receipt cleanup failed: %w", removeErr)
		}
		return err
	}
	return nil
}
