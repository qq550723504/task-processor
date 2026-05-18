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
	page, err := buildRevisionHistoryPage(task.Result, query)
	if err != nil {
		return nil, err
	}
	storeResolution := sheinStoreResolutionSummaryFromSnapshot(sheinStoreResolutionSnapshotFromTask(task))
	if page != nil && storeResolution != nil {
		for idx := range page.Items {
			page.Items[idx].StoreResolution = storeResolution
		}
	}
	return page, nil
}
