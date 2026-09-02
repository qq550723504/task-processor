package listingkit

import (
	"context"
	"time"

	"task-processor/internal/listingkit/core"
	"task-processor/internal/product/catalog/canonical"
	sheinpub "task-processor/internal/publishing/shein"
	sdsadapter "task-processor/internal/sds/adapter"
	sdsusecase "task-processor/internal/sds/usecase"
	sdsworkflow "task-processor/internal/sds/workflow"
)

type stubApplyRevisionRepo struct {
	task           *Task
	saveCalls      int
	failOnSaveCall int
}

func (r *stubApplyRevisionRepo) CreateTask(_ context.Context, task *Task) error {
	r.task = task
	return nil
}
func (r *stubApplyRevisionRepo) GetTask(context.Context, string) (*Task, error) { return r.task, nil }
func (r *stubApplyRevisionRepo) ListTasks(context.Context, *TaskListQuery) ([]Task, int64, error) {
	if r.task == nil {
		return []Task{}, 0, nil
	}
	return []Task{*r.task}, 1, nil
}
func (*stubApplyRevisionRepo) MarkProcessing(context.Context, string) error { return nil }
func (*stubApplyRevisionRepo) MarkCompleted(context.Context, string, *ListingKitResult) error {
	return nil
}
func (*stubApplyRevisionRepo) MarkNeedsReview(context.Context, string, *ListingKitResult, string) error {
	return nil
}
func (*stubApplyRevisionRepo) MarkFailed(context.Context, string, string) error { return nil }
func (r *stubApplyRevisionRepo) MarkBlockedRetryable(_ context.Context, taskID string, block *RetryableBlock, errorMsg string) error {
	if r.task == nil || r.task.ID != taskID {
		return core.ErrTaskNotFound
	}
	r.task.Status, r.task.RetryableBlock, r.task.Error, r.task.UpdatedAt = core.TaskStatusBlockedRetryable, block, errorMsg, time.Now()
	return nil
}
func (*stubApplyRevisionRepo) ListRecoverableTasks(context.Context, *RecoverableTaskQuery) ([]Task, error) {
	return []Task{}, nil
}
func (r *stubApplyRevisionRepo) RecoverBlockedTaskNow(_ context.Context, taskID string, recoveredAt time.Time) error {
	if r.task == nil || r.task.ID != taskID {
		return core.ErrTaskNotFound
	}
	r.task.Status, r.task.RetryableBlock, r.task.Error, r.task.UpdatedAt = core.TaskStatusPending, nil, "", recoveredAt
	return nil
}
func (*stubApplyRevisionRepo) BulkRecoverBlockedTasks(context.Context, *RecoverBlockedTasksQuery) (int64, error) {
	return 0, nil
}
func (*stubApplyRevisionRepo) PrepareRetry(context.Context, string) error        { return nil }
func (*stubApplyRevisionRepo) IncrementRetryCount(context.Context, string) error { return nil }
func (r *stubApplyRevisionRepo) SaveTaskResult(_ context.Context, _ string, result *ListingKitResult) error {
	r.saveCalls++
	if r.failOnSaveCall > 0 && r.saveCalls == r.failOnSaveCall {
		return context.DeadlineExceeded
	}
	r.task.Result = result
	return nil
}

type stubRevisionSheinAttributeResolver struct{}

func (stubRevisionSheinAttributeResolver) Resolve(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package) *sheinpub.AttributeResolution {
	return &sheinpub.AttributeResolution{Status: "resolved"}
}

type stubRevisionSheinSaleResolver struct{}

func (stubRevisionSheinSaleResolver) Resolve(*sheinpub.BuildRequest, *canonical.Product, *sheinpub.Package) *sheinpub.SaleAttributeResolution {
	return &sheinpub.SaleAttributeResolution{Status: "resolved"}
}

type stubStandardProductWorkflowClient struct {
	calls []StandardProductWorkflowStartInput
}

func (s *stubStandardProductWorkflowClient) StartStandardProduct(_ context.Context, input StandardProductWorkflowStartInput) error {
	s.calls = append(s.calls, input)
	return nil
}

type stubPlatformAdaptWorkflowClient struct {
	calls []PlatformAdaptWorkflowStartInput
}

func (s *stubPlatformAdaptWorkflowClient) StartPlatformAdaptation(_ context.Context, input PlatformAdaptWorkflowStartInput) error {
	s.calls = append(s.calls, input)
	return nil
}

func boolPtr(value bool) *bool { return &value }

type stubWorkflowSDSSyncService struct{}

func (*stubWorkflowSDSSyncService) SyncFromRemoteImage(context.Context, sdsusecase.RemoteImageInput) (*sdsworkflow.SyncResult, error) {
	return nil, nil
}
func (*stubWorkflowSDSSyncService) SyncFromLocalFile(context.Context, sdsusecase.LocalFileInput) (*sdsworkflow.SyncResult, error) {
	return nil, nil
}
func (*stubWorkflowSDSSyncService) SyncFromImageResult(context.Context, sdsusecase.ImageResultInput) (*sdsadapter.SyncResult, error) {
	return nil, nil
}
func (*stubWorkflowSDSSyncService) SyncFromImageRequest(context.Context, sdsusecase.ImageRequestInput) (*sdsadapter.SyncResult, error) {
	return nil, nil
}
