package listingkit

import "context"

func (s *service) GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *RevisionHistoryDetailQuery) (*ListingKitRevisionHistoryDetail, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}
	detail, err := buildRevisionHistoryDetail(task.Result, revisionID, query)
	if err != nil {
		return nil, err
	}
	if detail != nil && detail.Record != nil {
		detail.Record.StoreResolution = sheinStoreResolutionSummaryFromSnapshot(sheinStoreResolutionSnapshotFromTask(task))
	}
	return detail, nil
}
