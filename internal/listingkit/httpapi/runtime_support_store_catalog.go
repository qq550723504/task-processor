package httpapi

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/listingadmin"
	"task-processor/internal/listingkit"
)

type sheinListingStoreCatalog struct {
	repo listingadmin.StoreRepository
}

type listingAdminStoreAccessValidator struct {
	repo listingadmin.StoreRepository
}

func listingAdminRequestContext(ctx context.Context) context.Context {
	return listingadmin.WithRequestRoles(ctx, listingkit.RequestRolesFromContext(ctx))
}

func activeStoreStatusFilter() *int16 {
	status := int16(0)
	return &status
}

func (v listingAdminStoreAccessValidator) ValidateStoreAccess(ctx context.Context, tenantID, storeID int64, expectedPlatform string) (listingkit.StoreAccess, error) {
	if v.repo == nil || tenantID <= 0 || storeID <= 0 {
		return listingkit.StoreAccess{}, listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable")
	}
	store, err := v.repo.GetStore(listingAdminRequestContext(ctx), tenantID, storeID)
	if err != nil || store == nil || store.ID != storeID || store.TenantID != tenantID || !strings.EqualFold(strings.TrimSpace(store.Platform), strings.TrimSpace(expectedPlatform)) {
		return listingkit.StoreAccess{}, listingkit.NewStoreAccessError(listingkit.StoreAccessUnavailable, "store is unavailable")
	}
	if store.Status != 0 {
		return listingkit.StoreAccess{}, listingkit.NewStoreAccessError(listingkit.StoreAccessDisabled, "store is disabled")
	}
	return listingkit.StoreAccess{
		ID:       store.ID,
		TenantID: store.TenantID,
		Platform: strings.TrimSpace(store.Platform),
		Enabled:  true,
	}, nil
}

func (c sheinListingStoreCatalog) GetStoreInfo(ctx context.Context, tenantID, storeID int64) (*listingkit.SheinStoreInfo, error) {
	if c.repo == nil {
		return nil, fmt.Errorf("listing admin store repository is not configured")
	}
	store, err := c.repo.GetStore(listingAdminRequestContext(ctx), tenantID, storeID)
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

func (c sheinListingStoreCatalog) ListStoreOptions(ctx context.Context, tenantID int64) ([]listingkit.SheinStoreOption, error) {
	if c.repo == nil {
		return nil, fmt.Errorf("listing admin store repository is not configured")
	}
	options := make([]listingkit.SheinStoreOption, 0)
	const pageSize = 200
	for pageNumber := 1; ; pageNumber++ {
		page, err := c.repo.ListStores(listingAdminRequestContext(ctx), listingadmin.StoreQuery{
			TenantID:   tenantID,
			Platform:   "SHEIN",
			ReadAccess: true,
			Status:     activeStoreStatusFilter(),
			Page:       pageNumber,
			PageSize:   pageSize,
		})
		if err != nil {
			return nil, err
		}
		if page == nil || len(page.Items) == 0 {
			break
		}
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
				Status:   item.Status,
			})
		}
		responsePageSize := page.PageSize
		if responsePageSize <= 0 {
			responsePageSize = pageSize
		}
		if int64(pageNumber*responsePageSize) >= page.Total || len(page.Items) < responsePageSize {
			break
		}
	}
	return options, nil
}
