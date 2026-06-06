package listingkit

type RequeuePendingTasksRequest struct {
	TaskIDs []string `json:"task_ids"`
}

type RequeuePendingTasksResult struct {
	RequeuedTaskIDs []string             `json:"requeued_task_ids,omitempty"`
	Skipped         []TaskRequeueSkip    `json:"skipped,omitempty"`
	Failed          []TaskRequeueFailure `json:"failed,omitempty"`
}

type TaskRequeueSkip struct {
	TaskID string     `json:"task_id"`
	Status TaskStatus `json:"status,omitempty"`
	Reason string     `json:"reason,omitempty"`
}

type TaskRequeueFailure struct {
	TaskID string     `json:"task_id"`
	Status TaskStatus `json:"status,omitempty"`
	Error  string     `json:"error,omitempty"`
}
