package taskrpcapi

import (
	"encoding/json"
	"fmt"

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
