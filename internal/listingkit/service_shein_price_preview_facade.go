package listingkit

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
)

func (s *service) PreviewSheinPrice(ctx context.Context, taskID string, req *SheinPricePreviewRequest) (*sheinpub.PricingReview, error) {
	return s.sheinAdminOrDefault().PreviewSheinPrice(ctx, taskID, req)
}
