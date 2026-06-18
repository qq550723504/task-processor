package pipeline

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
	sheinmanagedclient "task-processor/internal/shein/managedclient"
)

type sheinManagedRuntimeFactory struct {
	processor *SheinProcessor
}

func (f sheinManagedRuntimeFactory) NewAPIClient(ctx context.Context, storeID int64) sheinpub.RuntimeAPIClient {
	if f.processor == nil || f.processor.GetManagementClient() == nil || storeID <= 0 {
		return nil
	}
	_ = ctx
	return sheinmanagedclient.NewAPIClient(storeID, f.processor.GetManagementClient())
}
