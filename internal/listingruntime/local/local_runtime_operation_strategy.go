package local

import (
	"context"
	"errors"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingruntime"
)

func (r *LocalRuntime) GetRuntimeOperationStrategy(storeID int64) (*listingruntime.OperationStrategy, error) {
	if r == nil || (r.resources == nil && r.provider == nil) || storeID == 0 {
		return nil, nil
	}
	repo := r.operationStrategyRepository()
	if repo == nil {
		return nil, nil
	}
	strategy, err := repo.GetLatestByStoreID(context.Background(), storeID)
	if err != nil {
		if errors.Is(err, listingadmin.ErrOperationStrategyNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return runtimeOperationStrategyFromListing(strategy), nil
}
