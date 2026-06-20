package pipeline

import (
	"context"

	sheinpub "task-processor/internal/publishing/shein"
)

type sheinManagedRuntimeFactory struct {
	processor *SheinProcessor
}

func (f sheinManagedRuntimeFactory) NewAPIClient(ctx context.Context, storeID int64) sheinpub.RuntimeAPIClient {
	if f.processor == nil || storeID <= 0 {
		return nil
	}
	_ = ctx
	return f.processor.NewManagedAPIClientWithStoreInfo(storeID, nil)
}
