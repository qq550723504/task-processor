package resources

import (
	"context"

	"task-processor/internal/listingkit"
	"task-processor/internal/shein/inventory"
)

type sheinSyncedInventoryProductFeed struct {
	repo listingkit.SheinSyncRepository
}

func newSheinSyncedInventoryProductFeed(repo listingkit.SheinSyncRepository) inventory.SyncedInventoryProductFeed {
	if repo == nil {
		return nil
	}
	return sheinSyncedInventoryProductFeed{repo: repo}
}

func (s sheinSyncedInventoryProductFeed) ListSyncedInventoryProducts(ctx context.Context, query inventory.SyncedInventoryProductQuery) ([]inventory.SyncedInventoryProductRecord, int64, error) {
	rows, total, err := s.repo.ListSyncedProducts(ctx, &listingkit.SheinSyncedProductQuery{
		TenantID: query.TenantID,
		StoreID:  query.StoreID,
		IsActive: query.IsActive,
		Page:     query.Page,
		PageSize: query.PageSize,
	})
	if err != nil {
		return nil, 0, err
	}
	out := make([]inventory.SyncedInventoryProductRecord, 0, len(rows))
	for _, row := range rows {
		out = append(out, inventory.SyncedInventoryProductRecord{
			ID:                      row.ID,
			TenantID:                row.TenantID,
			StoreID:                 row.StoreID,
			SPUName:                 row.SPUName,
			SPUCode:                 row.SPUCode,
			SKCName:                 row.SKCName,
			SKCCode:                 row.SKCCode,
			CategoryID:              row.CategoryID,
			BrandName:               row.BrandName,
			ProductNameMulti:        row.ProductNameMulti,
			MainImageURL:            row.MainImageURL,
			ShelfStatus:             row.ShelfStatus,
			PriceSnapshot:           row.PriceSnapshot,
			InventorySnapshot:       row.InventorySnapshot,
			SiteSnapshot:            row.SiteSnapshot,
			InventorySyncAttributes: row.InventorySyncAttributes,
			PublishTime:             row.PublishTime,
			FirstShelfTime:          row.FirstShelfTime,
			IsActive:                row.IsActive,
		})
	}
	return out, total, nil
}

func (s sheinSyncedInventoryProductFeed) UpdateSyncedInventoryProductAttributes(ctx context.Context, tenantID, storeID int64, skcName string, attributes string) (int, error) {
	return s.repo.UpdateSyncedProductInventoryAttributes(ctx, tenantID, storeID, skcName, attributes)
}
