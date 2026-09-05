//go:build !linux && !windows

package ownershipmigration

// The production preflight hosts are Windows and Linux. Other ports retain the
// existing symlink-resolution and indexed same-object checks; no Linux-style bind
// mount inference is claimed on those platforms.
func receiptParentAliasesProfileSubtree(_, _ string) (bool, error) {
	return false, nil
}
