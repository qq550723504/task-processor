package taskstatus

import (
	"fmt"

	apptaskstatus "task-processor/internal/app/taskstatus"
	"task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/listingruntime"
)

type ImportTaskStatusClient = apptaskstatus.ImportTaskStatusClient
type UpdateInput = apptaskstatus.UpdateInput
type Service = apptaskstatus.Service
type TaskStatusSnapshot = managementapi.TaskStatusRespDTO

type RuntimeTaskStatusUpdater interface {
	UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error
}

type RuntimeWithTaskRPC interface {
	RuntimeTaskStatusUpdater
	GetTaskStatus(taskID int64) (*TaskStatusSnapshot, error)
}

func NewService(component string, clientProvider func() ImportTaskStatusClient) *Service {
	return apptaskstatus.NewService(component, clientProvider)
}

type runtimeTaskStatusAdapter struct {
	client RuntimeTaskStatusUpdater
}

type managementTaskStatusRuntime struct {
	client *management.ClientManager
}

func NewRuntimeTaskStatusAdapter(client RuntimeTaskStatusUpdater) ImportTaskStatusClient {
	if client == nil {
		return nil
	}
	return runtimeTaskStatusAdapter{client: client}
}

func NewManagementTaskStatusRuntime(client *management.ClientManager) RuntimeWithTaskRPC {
	if client == nil {
		return nil
	}
	return managementTaskStatusRuntime{client: client}
}

func (a runtimeTaskStatusAdapter) UpdateTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if a.client == nil {
		return fmt.Errorf("task status runtime is not initialized")
	}
	return a.client.UpdateRuntimeTaskStatus(req)
}

func (r managementTaskStatusRuntime) UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if r.client == nil {
		return fmt.Errorf("management client is not initialized")
	}
	return r.client.UpdateRuntimeTaskStatus(req)
}

func (r managementTaskStatusRuntime) GetTaskStatus(taskID int64) (*TaskStatusSnapshot, error) {
	if r.client == nil {
		return nil, fmt.Errorf("management client is not initialized")
	}
	taskRPCClient := r.client.GetTaskRPCClient()
	if taskRPCClient == nil {
		return nil, fmt.Errorf("task rpc client is not initialized")
	}
	return taskRPCClient.GetTaskStatus(taskID)
}

func (r managementTaskStatusRuntime) GetRuntimeImportTask(taskID int64) (*listingruntime.ImportTask, error) {
	if r.client == nil {
		return nil, fmt.Errorf("management client is not initialized")
	}
	return r.client.GetRuntimeImportTask(taskID)
}
