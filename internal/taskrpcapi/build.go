package taskrpcapi

import (
	"encoding/json"
	"fmt"
	"time"

	kernelmodule "task-processor/internal/kernel/module"
)

type BuildResult struct {
	Handler Handler
	Module  kernelmodule.Module
}

func BuildHandler(localStatusProvider LocalStatusProvider) (Handler, error) {
	if localStatusProvider == nil {
		return nil, nil
	}
	return NewHandler(newLocalTaskRPCClient(localStatusProvider), localStatusProvider)
}

func BuildModule(localStatusProvider LocalStatusProvider) (*BuildResult, error) {
	handler, err := BuildHandler(localStatusProvider)
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

type localTaskRPCClient struct {
	localStatusProvider LocalStatusProvider
}

func newLocalTaskRPCClient(localStatusProvider LocalStatusProvider) Client {
	if localStatusProvider == nil {
		return nil
	}
	return localTaskRPCClient{localStatusProvider: localStatusProvider}
}

func (c localTaskRPCClient) GetTaskStatus(taskID int64) (*TaskStatusRespDTO, error) {
	return nil, fmt.Errorf("task status lookup is unavailable after management task RPC retirement: task_id=%d", taskID)
}

func (c localTaskRPCClient) RetryTask(taskID int64) (*TaskActionRespDTO, error) {
	return retiredTaskAction(taskID, "retry"), nil
}

func (c localTaskRPCClient) CancelTask(taskID int64) (*TaskActionRespDTO, error) {
	return retiredTaskAction(taskID, "cancel"), nil
}

func (c localTaskRPCClient) GetQueueStats() (string, error) {
	if c.localStatusProvider == nil {
		return "{}", nil
	}
	payload, err := json.Marshal(c.localStatusProvider())
	if err != nil {
		return "", err
	}
	return string(payload), nil
}

func retiredTaskAction(taskID int64, action string) *TaskActionRespDTO {
	return &TaskActionRespDTO{
		TaskID:          taskID,
		Action:          action,
		Success:         false,
		StatusKey:       "UNAVAILABLE",
		StatusName:      "不可用",
		CanonicalStatus: "failed",
		ErrorMessage:    "management task RPC has been retired",
		ActionTime:      time.Now().Format(time.RFC3339),
	}
}
