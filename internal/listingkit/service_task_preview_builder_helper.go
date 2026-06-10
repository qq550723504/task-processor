package listingkit

import (
	"context"
)

func (s *service) buildTaskPreview(ctx context.Context, task *Task, platform string) (*ListingKitPreview, error) {
	preview, err := buildListingKitPreview(task, platform)
	if err != nil {
		return nil, err
	}
	s.decorateSheinCookieAvailabilityPreview(ctx, task, preview)
	return preview, nil
}
