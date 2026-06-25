package management

import (
	"fmt"

	"task-processor/internal/listingruntime"
	"task-processor/internal/taskstatus"
)

type taskStatusRuntime struct {
	client *ClientManager
}

func NewTaskStatusRuntime(client *ClientManager) taskstatus.RuntimeWithTaskRPC {
	if client == nil {
		return nil
	}
	return taskStatusRuntime{client: client}
}

func (r taskStatusRuntime) UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if r.client == nil {
		return fmt.Errorf("task status runtime is not initialized")
	}
	return r.client.UpdateRuntimeTaskStatus(req)
}

func (r taskStatusRuntime) GetTaskStatus(taskID int64) (*taskstatus.TaskStatusSnapshot, error) {
	if r.client == nil {
		return nil, fmt.Errorf("task status runtime is not initialized")
	}
	taskRPCClient := r.client.GetTaskRPCClient()
	if taskRPCClient == nil {
		return nil, fmt.Errorf("task rpc client is not initialized")
	}
	status, err := taskRPCClient.GetTaskStatus(taskID)
	if err != nil || status == nil {
		return nil, err
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
	}, nil
}

func (r taskStatusRuntime) GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error) {
	if r.client == nil {
		return nil, fmt.Errorf("task status runtime is not initialized")
	}
	return r.client.GetRuntimeImportTask(taskID)
}
