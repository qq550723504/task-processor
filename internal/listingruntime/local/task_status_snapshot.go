package local

import (
	api "task-processor/internal/taskrpcapi"
	"task-processor/internal/taskstatus"
)

func taskStatusSnapshotFromDTO(status *api.TaskStatusRespDTO) *taskstatus.TaskStatusSnapshot {
	if status == nil {
		return nil
	}
	return &taskstatus.TaskStatusSnapshot{
		TaskID:           status.TaskID,
		Status:           status.Status,
		StatusKey:        status.StatusKey,
		StatusName:       status.StatusName,
		CanonicalStatus:  status.CanonicalStatus,
		Platform:         status.Platform,
		Region:           status.Region,
		TaskType:         status.TaskType,
		Priority:         status.Priority,
		RetryCount:       status.RetryCount,
		MaxRetries:       status.MaxRetries,
		ProcessingTimeMs: status.ProcessingTimeMs,
		QueueName:        status.QueueName,
		ProcessingNode:   status.ProcessingNode,
		ProgressPercent:  status.ProgressPercent,
		Result:           status.Result,
		ErrorMessage:     status.ErrorMessage,
		ErrorStack:       status.ErrorStack,
		ExecutionLogs:    status.ExecutionLogs,
		TaskDetails:      status.TaskDetails,
	}
}
