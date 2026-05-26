package listingkit

import "context"

func (s *service) GetTaskRevisionHistory(ctx context.Context, taskID string, query *RevisionHistoryQuery) (*ListingKitRevisionHistoryPage, error) {
	return s.taskRevisionOrDefault().GetTaskRevisionHistory(ctx, taskID, query)
}
