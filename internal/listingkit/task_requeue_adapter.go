package listingkit

import submissiondomain "task-processor/internal/listing/submission"

type taskRequeueSubmitterFunc func(taskID string) error

func (f taskRequeueSubmitterFunc) Submit(taskID string) error { return f(taskID) }

func adaptSubmissionDomainRequeueResult(result *submissiondomain.RequeueResult) *RequeuePendingTasksResult {
	if result == nil {
		return nil
	}
	adapted := &RequeuePendingTasksResult{
		RequeuedTaskIDs: append([]string(nil), result.RequeuedTaskIDs...),
		Skipped:         make([]TaskRequeueSkip, 0, len(result.Skipped)),
		Failed:          make([]TaskRequeueFailure, 0, len(result.Failed)),
	}
	for _, skip := range result.Skipped {
		adapted.Skipped = append(adapted.Skipped, TaskRequeueSkip{
			TaskID: skip.TaskID,
			Status: TaskStatus(skip.Status),
			Reason: skip.Reason,
		})
	}
	for _, failure := range result.Failed {
		adapted.Failed = append(adapted.Failed, TaskRequeueFailure{
			TaskID: failure.TaskID,
			Status: TaskStatus(failure.Status),
			Error:  failure.Error,
		})
	}
	return adapted
}
