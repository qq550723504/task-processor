package listingkit

import (
	"strings"
	"time"

	"task-processor/internal/listingkit/submission"
)

func normalizeRequeueTaskIDs(req *RequeuePendingTasksRequest) []string {
	if req == nil {
		return nil
	}
	seen := make(map[string]struct{}, len(req.TaskIDs))
	taskIDs := make([]string, 0, len(req.TaskIDs))
	for _, taskID := range req.TaskIDs {
		taskID = strings.TrimSpace(taskID)
		if taskID == "" {
			continue
		}
		if _, ok := seen[taskID]; ok {
			continue
		}
		seen[taskID] = struct{}{}
		taskIDs = append(taskIDs, taskID)
	}
	return taskIDs
}

func submitTaskWithRetry(submitter TaskSubmitter, taskID string, maxWait time.Duration) error {
	if submitter == nil {
		return ErrTaskRequeueUnavailable
	}
	return submission.RetryEnqueueSubmit(taskID, maxWait, submitter.Submit)
}
