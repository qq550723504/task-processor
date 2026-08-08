package listingkit

import (
	"context"
	"strings"
	"task-processor/internal/listingkit/core"
)

func (s *service) RetryTaskChildTask(ctx context.Context, taskID string, req *RetryChildTaskRequest) (*TaskResult, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" || req == nil || strings.TrimSpace(req.Kind) == "" {
		return nil, core.ErrChildTaskRetryInvalidRequest
	}
	if s == nil || s.repo == nil {
		return nil, core.ErrTaskNotFound
	}

	task, err := s.repo.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, core.ErrTaskNotFound
	}
	if task.Status == core.TaskStatusPending || task.Status == core.TaskStatusProcessing {
		return nil, core.ErrChildTaskRetryConflict
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
		return nil, core.ErrChildTaskNotRetryable
	}
	state, ok := childTaskStateByKind(result, kind)
	if !ok {
		return nil, core.ErrChildTaskNotFound
	}
	if state.Status == string(core.TaskStatusProcessing) || state.Status == string(core.TaskStatusPending) {
		return nil, core.ErrChildTaskRetryConflict
	}
	if state.Status != string(core.TaskStatusFailed) && state.Status != string(core.TaskStatusCompleted) {
		return nil, core.ErrChildTaskNotRetryable
	}

	pruneChildTaskRetryArtifacts(result, kind)
	recorder := newWorkflowRecorder(result)
	switch kind {
	case "sds_catalog_product":
		err = s.retrySDSCatalogProduct(ctx, task, result, recorder)
	case "sds_design_sync":
		err = s.retrySDSDesignSync(ctx, task, result, recorder)
	default:
		err = core.ErrChildTaskNotRetryable
	}
	if err != nil {
		markChildTask(result, kind, state.TaskID, string(core.TaskStatusFailed), err.Error())
		return s.persistRetriedChildTaskResult(ctx, task, result, kind, err)
	}
	return s.persistRetriedChildTaskResult(ctx, task, result, kind, nil)
}
