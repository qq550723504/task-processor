package listingkit

import "context"

func (s *service) GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	return buildListingKitExport(task, platform)
}
