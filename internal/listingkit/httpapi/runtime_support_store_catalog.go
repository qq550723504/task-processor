package httpapi

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
)

type sheinManagementStoreCatalog struct {
	repo listingadmin.StoreRepository
}

func (c sheinManagementStoreCatalog) GetStoreInfo(ctx context.Context, tenantID, storeID int64) (*listingkit.SheinStoreInfo, error) {
	if c.repo == nil {
		return nil, fmt.Errorf("listing admin store repository is not configured")
	}
	store, err := c.repo.GetStore(ctx, tenantID, storeID)
	if err != nil {
		return nil, err
	}
	if store == nil || store.ID <= 0 {
		return nil, fmt.Errorf("store info is unavailable")
	}
	return &listingkit.SheinStoreInfo{
		ID:       store.ID,
		TenantID: store.TenantID,
		StoreID:  strings.TrimSpace(store.StoreID),
		Name:     strings.TrimSpace(store.Name),
		Platform: strings.TrimSpace(store.Platform),
		Region:   strings.TrimSpace(store.Region),
		LoginURL: strings.TrimSpace(store.LoginURL),
		Proxy:    strings.TrimSpace(store.Proxy),
	}, nil
}

func (c sheinManagementStoreCatalog) ListStoreOptions(ctx context.Context, tenantID int64) ([]listingkit.SheinStoreOption, error) {
	if c.repo == nil {
		return nil, fmt.Errorf("listing admin store repository is not configured")
	}
	page, err := c.repo.ListStores(ctx, listingadmin.StoreQuery{
		TenantID: tenantID,
		Platform: "shein",
		Page:     1,
		PageSize: 200,
	})
	if err != nil || page == nil || len(page.Items) == 0 {
		return nil, err
	}
	options := make([]listingkit.SheinStoreOption, 0, len(page.Items))
	for _, item := range page.Items {
		if item.ID <= 0 {
			continue
		}
		options = append(options, listingkit.SheinStoreOption{
			ID:       item.ID,
			StoreID:  strings.TrimSpace(item.StoreID),
			Name:     strings.TrimSpace(item.Name),
			Platform: strings.TrimSpace(item.Platform),
			Region:   strings.TrimSpace(item.Region),
		})
	}
	return options, nil
}
