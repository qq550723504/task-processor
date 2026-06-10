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

func (s *service) GetTaskExport(ctx context.Context, taskID string, platform string) (*ListingKitExport, error) {
	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	export, err := buildListingKitExport(task, platform)
	if err != nil {
		return nil, err
	}
	tasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	projection := buildAssetGenerationProjection(task.Result, tasks)
	export.AssetGenerationSummary = projection.Summary
	export.AssetGenerationTasks = projection.Tasks
	if len(export.AssetRenderPreviews) == 0 && task.Result != nil {
		export.AssetRenderPreviews = buildAssetRenderPreviews(task.Result.AssetBundle)
	}
	if len(export.PlatformAssetRenderPreviews) == 0 && task.Result != nil {
		export.PlatformAssetRenderPreviews = buildPlatformAssetRenderPreviews(task.Result)
	}
	export.AssetGenerationQueue = projection.Queue
	export.AssetGenerationOverview = projection.Overview
	return export, nil
}

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

func (s *service) ListTasks(ctx context.Context, query *TaskListQuery) (*TaskListPage, error) {
	return s.taskLifecycleOrDefault().ListTasks(ctx, query)
}

func (s *service) GetSDSBaselineReadiness(ctx context.Context, query *SDSBaselineReadinessQuery) (*SDSBaselineReadiness, error) {
	return s.taskLifecycleOrDefault().GetSDSBaselineReadiness(ctx, query)
}

func (s *service) WarmSDSBaseline(ctx context.Context, req *WarmSDSBaselineRequest) (*SDSBaselineReadiness, error) {
	return s.warmSDSBaseline(ctx, req)
}
