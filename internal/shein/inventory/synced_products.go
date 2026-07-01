package inventory

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/pkg/types"
	"task-processor/internal/shein/productsync"
)

const inventoryProductSourceSheinSyncedProduct = "listingkit_shein_synced_products"
const syncedInventoryProductPageSize = 100

type SyncedInventoryProductSource interface {
	ListSyncedInventoryProducts(ctx context.Context, query SyncedInventoryProductQuery) ([]SyncedInventoryProductRecord, int64, error)
	UpdateSyncedInventoryProductAttributes(ctx context.Context, tenantID, storeID int64, skcName string, attributes string) (int, error)
}

type SyncedInventoryProductQuery struct {
	TenantID int64
	StoreID  int64
	IsActive *bool
	Page     int
	PageSize int
}

type SyncedInventoryProductRecord struct {
	ID                      int64
	TenantID                int64
	StoreID                 int64
	SPUName                 string
	SPUCode                 string
	SKCName                 string
	SKCCode                 string
	CategoryID              int64
	BrandName               string
	ProductNameMulti        string
	MainImageURL            string
	ShelfStatus             string
	PriceSnapshot           string
	InventorySnapshot       string
	SiteSnapshot            string
	InventorySyncAttributes string
	PublishTime             *time.Time
	FirstShelfTime          *time.Time
	IsActive                bool
}

func (s *inventorySyncServiceImpl) fetchSyncedProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]*InventoryProductSnapshot, error) {
	if s.syncedProductSource == nil {
		return nil, nil
	}
	active := true
	page := 1
	items := make([]*InventoryProductSnapshot, 0)
	for {
		rows, total, err := s.syncedProductSource.ListSyncedInventoryProducts(ctx, SyncedInventoryProductQuery{
			TenantID: tenantID,
			StoreID:  storeID,
			IsActive: &active,
			Page:     page,
			PageSize: syncedInventoryProductPageSize,
		})
		if err != nil {
			return nil, err
		}
		for i := range rows {
			items = append(items, syncedInventoryProductSnapshotFromRecord(&rows[i]))
		}
		if len(rows) == 0 || int64(page*syncedInventoryProductPageSize) >= total {
			break
		}
		page++
	}
	return items, nil
}

func syncedInventoryProductSnapshotFromRecord(record *SyncedInventoryProductRecord) *InventoryProductSnapshot {
	if record == nil {
		return nil
	}
	productID, parentProductID, region := syncedInventoryMappingIdentity(record.InventorySyncAttributes)
	stock := syncedInventoryStock(record.InventorySnapshot)
	salePrice, currency := syncedInventoryPrice(record.PriceSnapshot)
	title := strings.TrimSpace(record.ProductNameMulti)
	if title == "" {
		title = strings.TrimSpace(record.SPUName)
	}
	shelfStatus := sheinInventoryShelfStatusOnShelf
	if !record.IsActive {
		shelfStatus = sheinInventoryShelfStatusOffShelf
	}
	return &InventoryProductSnapshot{
		ID:                record.ID,
		Source:            inventoryProductSourceSheinSyncedProduct,
		StoreID:           record.StoreID,
		Platform:          "SHEIN",
		CategoryID:        record.CategoryID,
		Region:            region,
		ParentProductID:   parentProductID,
		ProductID:         productID,
		Title:             title,
		SpecialPrice:      types.FlexibleString(salePrice),
		PriceCurrency:     currency,
		Stock:             types.FlexibleString(stock),
		Brand:             record.BrandName,
		MainImageURL:      record.MainImageURL,
		Attributes:        record.InventorySyncAttributes,
		PlatformProductID: record.SKCName,
		PlatformStatus:    record.ShelfStatus,
		ShelfStatus:       shelfStatus,
		PublishTime:       types.ToFlexibleTime(record.PublishTime),
		ShelfTime:         types.ToFlexibleTime(record.FirstShelfTime),
		PlatformData:      record.SiteSnapshot,
		TenantID:          record.TenantID,
	}
}

func syncedInventoryMappingIdentity(attributes string) (string, string, string) {
	var skcList []productsync.EnrichedSkcInfo
	if strings.TrimSpace(attributes) == "" || json.Unmarshal([]byte(attributes), &skcList) != nil {
		return "", "", ""
	}
	for _, skc := range skcList {
		for _, sku := range skc.SkuInfo {
			if sku.MappingInfo == nil || sku.MappingInfo.ProductID == "" {
				continue
			}
			parentProductID := ""
			if sku.MappingInfo.ParentProductID != nil {
				parentProductID = *sku.MappingInfo.ParentProductID
			}
			return sku.MappingInfo.ProductID, parentProductID, sku.MappingInfo.Region
		}
	}
	return "", "", ""
}

func syncedInventoryStock(snapshot string) string {
	var payload struct {
		Available int `json:"available"`
		Total     int `json:"total"`
	}
	if strings.TrimSpace(snapshot) == "" || json.Unmarshal([]byte(snapshot), &payload) != nil {
		return ""
	}
	if payload.Available > 0 {
		return strconv.Itoa(payload.Available)
	}
	if payload.Total > 0 {
		return strconv.Itoa(payload.Total)
	}
	return ""
}

func syncedInventoryPrice(snapshot string) (string, string) {
	var payload struct {
		SalePrice float64 `json:"sale_price"`
		Currency  string  `json:"currency"`
	}
	if strings.TrimSpace(snapshot) == "" || json.Unmarshal([]byte(snapshot), &payload) != nil || payload.SalePrice <= 0 {
		return "", strings.TrimSpace(payload.Currency)
	}
	return strconv.FormatFloat(payload.SalePrice, 'f', -1, 64), strings.TrimSpace(payload.Currency)
}
