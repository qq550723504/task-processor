//go:build !linux

package ownershipmigration

import "testing"

func supportedTestTempDir(t *testing.T) string {
	t.Helper()
	return t.TempDir()
}
