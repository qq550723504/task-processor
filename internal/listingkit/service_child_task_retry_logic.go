package listingkit

import (
	"context"
	"strings"
)

func (s *service) RetryTaskChildTask(ctx context.Context, taskID string, req *RetryChildTaskRequest) (*TaskResult, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || req == nil || strings.TrimSpace(req.Kind) == "" {
		return nil, ErrChildTaskRetryInvalidRequest
	}
	if s == nil || s.repo == nil {
		return nil, ErrTaskNotFound
	}

	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, ErrTaskNotFound
	}
	if task.Status == TaskStatusPending || task.Status == TaskStatusProcessing {
		return nil, ErrChildTaskRetryConflict
	}
	if task.Result == nil {
		return nil, ErrTaskResultUnavailable
	}

	result, err := cloneListingKitResult(task.Result)
	if err != nil {
		return nil, err
	}
	result = normalizeListingKitResultSemanticFields(result)
	kind := strings.TrimSpace(req.Kind)
	if !childTaskRetrySupportedKind(kind) {
		return nil, ErrChildTaskNotRetryable
	}
	state, ok := childTaskStateByKind(result, kind)
	if !ok {
		return nil, ErrChildTaskNotFound
	}
	if state.Status == string(TaskStatusProcessing) || state.Status == string(TaskStatusPending) {
		return nil, ErrChildTaskRetryConflict
	}
	if state.Status != string(TaskStatusFailed) && state.Status != string(TaskStatusCompleted) {
		return nil, ErrChildTaskNotRetryable
	}

	pruneChildTaskRetryArtifacts(result, kind)
	recorder := newWorkflowRecorder(result)
	switch kind {
	case "sds_catalog_product":
		err = s.retrySDSCatalogProduct(ctx, task, result, recorder)
	case "sds_design_sync":
		err = s.retrySDSDesignSync(ctx, task, result, recorder)
	default:
		err = ErrChildTaskNotRetryable
	}
	if err != nil {
		markChildTask(result, kind, state.TaskID, string(TaskStatusFailed), err.Error())
		return s.persistRetriedChildTaskResult(ctx, task, result, kind, err)
	}
	return s.persistRetriedChildTaskResult(ctx, task, result, kind, nil)
}
