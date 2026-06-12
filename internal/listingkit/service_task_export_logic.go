package listingkit

import "context"

func (s *service) GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error) {
	return s.taskExportOrDefault().GetTaskExport(ctx, taskID, platform)
}
