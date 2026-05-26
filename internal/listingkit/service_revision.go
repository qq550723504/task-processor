package listingkit

import "context"

func (s *service) ApplyTaskRevision(ctx context.Context, taskID string, req *ApplyRevisionRequest) (*ListingKitPreview, error) {
	return s.taskRevisionOrDefault().ApplyTaskRevision(ctx, taskID, req)
}

func (s *service) taskRevisionOrDefault() *taskRevisionService {
	if s.taskRevision != nil {
		return s.taskRevision
	}
	s.taskRevision = newTaskRevisionService(taskRevisionServiceConfig{
		repo: s.repo,
		resolveManualSheinSaleAttributeValueIDs: func(ctx context.Context, task *Task, req *ApplyRevisionRequest) error {
			return s.resolveManualSheinSaleAttributeValueIDs(ctx, task, req)
		},
		mutateTaskResult: s.mutateTaskResult,
		refreshSheinDerivedState: func(task *Task, req *ApplyRevisionRequest) {
			s.refreshSheinDerivedState(task, req)
		},
		buildTaskPreview: s.buildTaskPreview,
	})
	return s.taskRevision
}
