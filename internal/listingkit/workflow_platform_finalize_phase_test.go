package listingkit

import (
	"context"
	"testing"
	"time"
)

func TestWithSheinReviewContentOptimizationTimeoutBoundsContext(t *testing.T) {
	ctx, cancel := withSheinReviewContentOptimizationTimeout(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("optimization context has no deadline")
	}
	remaining := time.Until(deadline)
	if remaining <= 0 || remaining > sheinReviewContentOptimizationTimeout {
		t.Fatalf("optimization deadline remaining = %v, want within %v", remaining, sheinReviewContentOptimizationTimeout)
	}
}
