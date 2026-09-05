//go:build !linux && !windows

package ownershipmigration

import (
	"context"
	"errors"
	"testing"
)

func TestUnsupportedPortProfileSubtreeValidationFailsClosed(t *testing.T) {
	if err := validateProfileSubtrees(context.Background(), nil); err == nil {
		t.Fatal("unsupported platform approved unchecked profile evidence")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := validateProfileSubtrees(ctx, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation lost: %v", err)
	}
}
