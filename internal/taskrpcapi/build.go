package taskrpcapi

import (
	managementclient "task-processor/internal/infra/clients/management"
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
	return NewHandler(provider.GetTaskRPCClient(), localStatusProvider)
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
