package pipeline

import (
	"context"

	sheinclient "task-processor/internal/shein/client"
	sheinmanagedclient "task-processor/internal/shein/managedclient"
)

type sheinManagedRuntimeFactory struct {
	processor *SheinProcessor
}

func (f sheinManagedRuntimeFactory) NewAPIClient(ctx context.Context, storeID int64) *sheinclient.APIClient {
	if f.processor == nil || f.processor.GetManagementClient() == nil || storeID <= 0 {
		return nil
	}
	_ = ctx
	return sheinmanagedclient.NewAPIClient(storeID, f.processor.GetManagementClient())
}
