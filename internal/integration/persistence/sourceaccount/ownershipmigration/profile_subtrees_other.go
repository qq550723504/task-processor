//go:build !linux

package ownershipmigration

import "context"

// Windows reparse aliases remain guarded by directory verification and indexed
// file identities. This hook adds Linux mount-table evidence only.
func validateProfileSubtrees(ctx context.Context, _ []AccountEvidence) error {
	return ctx.Err()
}
