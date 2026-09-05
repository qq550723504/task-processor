//go:build !linux && !windows

package ownershipmigration

import (
	"context"
	"fmt"
)

// Linux and Windows are the only hosts with complete descendant-alias checks.
// Keep other ports buildable but never approve unchecked preflight evidence.
func validateProfileSubtrees(ctx context.Context, _ []AccountEvidence) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("profile subtree validation is unsupported on this platform")
}
