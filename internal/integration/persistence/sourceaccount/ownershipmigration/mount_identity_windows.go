//go:build windows

package ownershipmigration

// Windows directory junctions and mount-point reparse aliases are rejected by
// verifyDirectoryInfo through Lstat/EvalSymlinks before this hook is reached.
// Direct same-object aliases are additionally caught by indexed file identities.
func receiptParentAliasesProfileSubtree(_, _ string) (bool, error) {
	return false, nil
}
