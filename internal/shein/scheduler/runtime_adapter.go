package scheduler

import "task-processor/internal/infra/clients/management"

type ManagementRuntime = managementRuntime

func NewManagementRuntime(client *management.ClientManager) ManagementRuntime {
	if client == nil {
		return nil
	}
	return client
}
