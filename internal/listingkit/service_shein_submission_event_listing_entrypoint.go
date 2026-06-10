package listingkit

import "context"

func (s *service) GetSubmissionEvents(ctx context.Context, taskID string) (*SheinSubmissionEventPage, error) {
	return s.sheinAdminOrDefault().GetSubmissionEvents(ctx, taskID)
}
