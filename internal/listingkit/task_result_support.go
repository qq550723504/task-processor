package listingkit

import "context"

func (s *service) buildTaskResultPayload(ctx context.Context, task *Task) (*ListingKitResult, error) {
	if task == nil || task.Result == nil {
		return nil, nil
	}
	copied := *task.Result
	tasks, err := s.listAssetGenerationTasks(ctx, task.ID)
	if err != nil {
		return nil, err
	}
	decorateListingKitResultGeneration(&copied, tasks)
	if copied.Shein != nil {
		if selection, selectionErr := s.resolveSheinStoreSelection(ctx, task); selectionErr == nil {
			copied.SheinStoreResolution = buildSheinStoreResolutionSummary(selection, task, nil)
		}
	}
	return &copied, nil
}

func buildTaskResult(task *Task, resultPayload *ListingKitResult) *TaskResult {
	if task == nil {
		return nil
	}
	result := &TaskResult{
		TaskID:                  task.ID,
		TenantID:                task.TenantID,
		Status:                  task.Status,
		Result:                  resultPayload,
		Error:                   task.Error,
		ReviewReasons:           reviewReasonsFromTask(task),
		CreatedAt:               task.CreatedAt,
	}
	if resultPayload != nil && resultPayload.Shein != nil {
		result.SheinWorkflowStatus = deriveSheinWorkflowStatus(resultPayload.Shein)
		if latest := latestSheinSubmissionOutcomeEvent(resultPayload.Shein); latest != nil {
			result.SheinLatestSubmissionStatus = latest.Status
			result.SheinLatestSubmissionError = latest.ErrorMessage
		} else if resultPayload.Shein.Submission != nil {
			result.SheinLatestSubmissionStatus = resultPayload.Shein.Submission.LastStatus
			result.SheinLatestSubmissionError = resultPayload.Shein.Submission.LastError
		}
		if resultPayload.Shein.Submission != nil {
			result.SheinSubmissionRemoteStatus = resultPayload.Shein.Submission.RemoteStatus
		}
	}
	if taskStatusIsTerminal(task.Status) {
		result.CompletedAt = &task.UpdatedAt
	}
	return result
}

func taskStatusIsTerminal(status TaskStatus) bool {
	return status == TaskStatusCompleted || status == TaskStatusNeedsReview || status == TaskStatusFailed
}
