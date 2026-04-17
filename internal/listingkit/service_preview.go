package listingkit

import "context"

func (s *service) GetTaskPreview(ctx context.Context, taskID string, platform string) (*ListingKitPreview, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildListingKitPreview(task, platform)
}
