package taskrpcapi

import managementclient "task-processor/internal/infra/clients/management"

type ClientProvider interface {
	GetTaskRPCClient() *managementclient.TaskRPCAPIClient
}

func BuildHandler(provider ClientProvider, localStatusProvider LocalStatusProvider) (Handler, error) {
	if provider == nil {
		return nil, nil
	}
	return NewHandler(provider.GetTaskRPCClient(), localStatusProvider)
}
