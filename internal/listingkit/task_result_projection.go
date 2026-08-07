package listingkit

type listingKitTaskResultProjection struct {
	Lifecycle       TaskResultLifecycleFields
	SourceReference *SourceReference
	ReviewReasons   []string
	RetryableBlock  *RetryableBlock
}

func buildTaskResultProjection(task *Task, resultPayload *ListingKitResult) *listingKitTaskResultProjection {
	if task == nil {
		return nil
	}

	reviewReasons, effectiveError := buildTaskResultReviewState(task, resultPayload)
	projection := &listingKitTaskResultProjection{
		Lifecycle: TaskResultLifecycleFields{
			Status:    task.Status,
			Error:     effectiveError,
			CreatedAt: task.CreatedAt,
		},
		ReviewReasons:  reviewReasons,
		RetryableBlock: cloneRetryableBlock(task.RetryableBlock),
	}
	if task.Request != nil {
		projection.SourceReference = cloneSourceReference(task.Request.Source)
	}
	if taskStatusIsTerminal(task.Status) {
		projection.Lifecycle.CompletedAt = &task.UpdatedAt
	}
	return projection
}
