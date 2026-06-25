package taskrpcapi

import (
	managementclient "task-processor/internal/infra/clients/management"
	managementapi "task-processor/internal/infra/clients/management/api"
	kernelmodule "task-processor/internal/kernel/module"
)

type ClientProvider interface {
	GetTaskRPCClient() *managementclient.TaskRPCAPIClient
}

type BuildResult struct {
	Handler Handler
	Module  kernelmodule.Module
}

func BuildHandler(provider ClientProvider, localStatusProvider LocalStatusProvider) (Handler, error) {
	if provider == nil {
		return nil, nil
	}
	return NewHandler(newManagementTaskRPCClient(provider.GetTaskRPCClient()), localStatusProvider)
}

func BuildModule(provider ClientProvider, localStatusProvider LocalStatusProvider) (*BuildResult, error) {
	handler, err := BuildHandler(provider, localStatusProvider)
	if err != nil {
		return nil, err
	}
	if handler == nil {
		return nil, nil
	}
	return &BuildResult{
		Handler: handler,
		Module:  NewHTTPModule(handler),
	}, nil
}

type managementTaskRPCClient struct {
	client *managementclient.TaskRPCAPIClient
}

func newManagementTaskRPCClient(client *managementclient.TaskRPCAPIClient) Client {
	if client == nil {
		return nil
	}
	return managementTaskRPCClient{client: client}
}

func (c managementTaskRPCClient) GetTaskStatus(taskID int64) (*TaskStatusRespDTO, error) {
	status, err := c.client.GetTaskStatus(taskID)
	if err != nil || status == nil {
		return nil, err
	}
	return taskStatusFromManagement(status), nil
}

func (c managementTaskRPCClient) RetryTask(taskID int64) (*TaskActionRespDTO, error) {
	action, err := c.client.RetryTask(taskID)
	if err != nil || action == nil {
		return nil, err
	}
	return taskActionFromManagement(action), nil
}

func (c managementTaskRPCClient) CancelTask(taskID int64) (*TaskActionRespDTO, error) {
	action, err := c.client.CancelTask(taskID)
	if err != nil || action == nil {
		return nil, err
	}
	return taskActionFromManagement(action), nil
}

func (c managementTaskRPCClient) GetQueueStats() (string, error) {
	return c.client.GetQueueStats()
}

func taskStatusFromManagement(status *managementapi.TaskStatusRespDTO) *TaskStatusRespDTO {
	return &TaskStatusRespDTO{
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
		CreatedAt:        status.CreatedAt,
		StartedAt:        status.StartedAt,
		CompletedAt:      status.CompletedAt,
		ProcessingTimeMs: status.ProcessingTimeMs,
		QueueName:        status.QueueName,
		ProcessingNode:   status.ProcessingNode,
		ProgressPercent:  status.ProgressPercent,
		Result:           status.Result,
		ErrorMessage:     status.ErrorMessage,
		ErrorStack:       status.ErrorStack,
		ExecutionLogs:    status.ExecutionLogs,
		NextRetryAt:      status.NextRetryAt,
		TaskDetails:      status.TaskDetails,
	}
}

func taskActionFromManagement(action *managementapi.TaskActionRespDTO) *TaskActionRespDTO {
	return &TaskActionRespDTO{
		TaskID:          action.TaskID,
		Action:          action.Action,
		Success:         action.Success,
		StatusKey:       action.StatusKey,
		StatusName:      action.StatusName,
		CanonicalStatus: action.CanonicalStatus,
		ErrorMessage:    action.ErrorMessage,
		ActionTime:      action.ActionTime,
	}
}
