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

type RuntimeTaskStatusUpdater interface {
	UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error
}

type RuntimeWithTaskRPC interface {
	RuntimeTaskStatusUpdater
	GetTaskStatus(taskID int64) (*managementapi.TaskStatusRespDTO, error)
}

func NewService(component string, clientProvider func() ImportTaskStatusClient) *Service {
	return apptaskstatus.NewService(component, clientProvider)
}

type managementClientAdapter struct {
	client RuntimeTaskStatusUpdater
}

type managementRuntime struct {
	client *management.ClientManager
}

func NewManagementClientAdapter(client RuntimeTaskStatusUpdater) ImportTaskStatusClient {
	if client == nil {
		return nil
	}
	return managementClientAdapter{client: client}
}

func NewManagementRuntime(client *management.ClientManager) RuntimeWithTaskRPC {
	if client == nil {
		return nil
	}
	return managementRuntime{client: client}
}

func (a managementClientAdapter) UpdateTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if a.client == nil {
		return fmt.Errorf("management client is not initialized")
	}
	return a.client.UpdateRuntimeTaskStatus(req)
}

func (r managementRuntime) UpdateRuntimeTaskStatus(req *listingruntime.TaskStatusUpdate) error {
	if r.client == nil {
		return fmt.Errorf("management client is not initialized")
	}
	return r.client.UpdateRuntimeTaskStatus(req)
}

func (r managementRuntime) GetTaskStatus(taskID int64) (*managementapi.TaskStatusRespDTO, error) {
	if r.client == nil {
		return nil, fmt.Errorf("management client is not initialized")
	}
	taskRPCClient := r.client.GetTaskRPCClient()
	if taskRPCClient == nil {
		return nil, fmt.Errorf("task rpc client is not initialized")
	}
	return taskRPCClient.GetTaskStatus(taskID)
}
