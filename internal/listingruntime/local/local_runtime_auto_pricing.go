package local

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"task-processor/internal/listingadmin"
	"task-processor/internal/platformtask"
)

func (r *LocalRuntime) ListRuntimeAutoPricingStoreIDs(ctx context.Context, platform string) ([]int64, error) {
	if r == nil || (r.resources == nil && r.provider == nil) {
		return nil, fmt.Errorf("local listing runtime is not initialized")
	}
	repo := r.GetLocalStoreRepository()
	if repo == nil {
		return nil, fmt.Errorf("store repository is not configured")
	}

	enableAutoPrice := true
	pageNo := 1
	storeIDs := make([]int64, 0, runtimeStoreDiscoveryPageSize)
	for {
		page, err := repo.ListStores(ctx, listingadmin.StoreQuery{
			Page:            pageNo,
			PageSize:        runtimeStoreDiscoveryPageSize,
			Platform:        platform,
			EnableAutoPrice: &enableAutoPrice,
		})
		if err != nil {
			return nil, err
		}
		if page == nil || len(page.Items) == 0 {
			break
		}

		for _, store := range page.Items {
			if store.ID == 0 || !strings.EqualFold(store.Platform, platform) {
				continue
			}
			storeIDs = append(storeIDs, store.ID)
		}

		if page.Total > 0 && int64(page.Page*page.PageSize) >= page.Total {
			break
		}
		if len(page.Items) < runtimeStoreDiscoveryPageSize {
			break
		}
		pageNo++
	}
	return dedupeInt64s(storeIDs), nil
}

func (r *LocalRuntime) GetAutoPricingStoreConfig(ctx context.Context, storeID int64) (*platformtask.AutoPricingStoreConfig, error) {
	if r == nil || (r.resources == nil && r.provider == nil) {
		return nil, fmt.Errorf("local listing runtime is not initialized")
	}
	if storeID <= 0 {
		return nil, errors.New("store id is required")
	}
	repo := r.GetLocalStoreRepository()
	if repo == nil {
		return nil, fmt.Errorf("store repository is not configured")
	}
	store, err := repo.FindStoreByID(ctx, storeID)
	if err != nil {
		return nil, err
	}
	if store == nil {
		return nil, errors.New("store not found")
	}
	return &platformtask.AutoPricingStoreConfig{
		Name:            store.Name,
		EnableAutoPrice: store.EnableAutoPrice,
		EnableRebargain: store.EnableRebargain,
	}, nil
}
