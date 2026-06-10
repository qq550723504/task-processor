package listingkit

import (
	"context"
)

func (s *service) SubmitTask(ctx context.Context, taskID string, req *SubmitTaskRequest) (*ListingKitPreview, error) {
	return s.taskSubmissionOrDefault().SubmitTask(ctx, taskID, req)
}
