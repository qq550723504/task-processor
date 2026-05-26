package listingkit

import (
	"context"
)

func (s *service) CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error) {
	return s.taskLifecycleOrDefault().CreateGenerateTask(ctx, req)
}

func (s *service) enqueueOrRunStudioTask(ctx context.Context, task *Task) (*Task, error) {
	return s.taskLifecycleOrDefault().enqueueOrRunStudioTask(ctx, task)
}

func (s *service) runTaskInline(ctx context.Context, task *Task) (*Task, error) {
	return s.taskLifecycleOrDefault().runTaskInline(ctx, task)
}

func (s *service) enqueueTask(ctx context.Context, task *Task) error {
	return s.taskLifecycleOrDefault().enqueueTask(ctx, task)
}

func (s *service) GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error) {
	return s.taskLifecycleOrDefault().GetTaskResult(ctx, taskID)
}

func (s *service) ListTasks(ctx context.Context, query *TaskListQuery) (*TaskListPage, error) {
	return s.taskLifecycleOrDefault().ListTasks(ctx, query)
}

func (s *service) GetSDSBaselineReadiness(ctx context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {
	return s.taskLifecycleOrDefault().GetSDSBaselineReadiness(ctx, query)
}

func (s *service) WarmSDSBaseline(ctx context.Context, req *WarmSDSBaselineRequest) (*SDSBaselineReadiness, error) {
	return s.warmSDSBaseline(ctx, req)
}

func (s *service) taskLifecycleOrDefault() *taskLifecycleService {
	if s.taskLifecycle != nil {
		return s.taskLifecycle
	}
	s.taskLifecycle = newTaskLifecycleService(taskLifecycleServiceConfig{
		repo: s.repo,
		requestDefaults: func() generateRequestDefaults {
			return s.requestDefaults
		},
		taskSubmitter: func() TaskSubmitter {
			return s.taskSubmitter
		},
		standardWorkflow: func() (StandardProductWorkflowClient, bool) {
			return s.standardProductWorkflowClient, s.standardProductWorkflowEnabled
		},
		processListingKit: s.ProcessListingKit,
		resolveStoreSelection: func(ctx context.Context, task *Task) (*sheinStoreSelection, error) {
			return s.resolveSheinStoreSelection(ctx, task)
		},
		buildResultPayload: func(ctx context.Context, task *Task) (*ListingKitResult, error) {
			return s.buildTaskResultPayload(ctx, task)
		},
	})
	return s.taskLifecycle
}
