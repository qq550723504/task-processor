//go:build !linux && !windows

package ownershipmigration

import "context"

// Other ports retain their existing root identity checks; production subtree
// evidence is provided by the Linux and Windows implementations.
func validateProfileSubtrees(ctx context.Context, _ []AccountEvidence) error {
	return ctx.Err()
}
