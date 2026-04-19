package listingkit

import "context"

func (s *service) GetTaskRevisionHistory(ctx context.Context, taskID string, query *RevisionHistoryQuery) (*ListingKitRevisionHistoryPage, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	return buildRevisionHistoryPage(task.Result, query)
}
