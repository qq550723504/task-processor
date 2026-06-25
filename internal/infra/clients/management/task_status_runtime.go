package management

import (
	"fmt"

	managementapi "task-processor/internal/listingadmin"
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
	return managementapi.TaskStatusSnapshotFromDTO(status), nil
}

func (r taskStatusRuntime) GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error) {
	if r.client == nil {
		return nil, fmt.Errorf("task status runtime is not initialized")
	}
	return r.client.GetRuntimeImportTask(taskID)
}
