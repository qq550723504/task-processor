package listingkit

import "context"

func (s *service) GetTaskRevisionHistory(ctx context.Context, taskID string, query *RevisionHistoryQuery) (*ListingKitRevisionHistoryPage, error) {
	return s.taskRevisionOrDefault().GetTaskRevisionHistory(ctx, taskID, query)
}

func (s *service) GetTaskRevisionHistoryDetail(ctx context.Context, taskID string, revisionID string, query *RevisionHistoryDetailQuery) (*ListingKitRevisionHistoryDetail, error) {
	return s.taskRevisionOrDefault().GetTaskRevisionHistoryDetail(ctx, taskID, revisionID, query)
}

func (s *service) ApplyTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*ListingKitPreview, error) {
	return s.taskRevisionOrDefault().ApplyTaskRevision(ctx, taskID, req)
}

func (s *service) ValidateTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*RevisionValidationResult, error) {
	return s.taskRevisionOrDefault().ValidateTaskRevision(ctx, taskID, req)
}
