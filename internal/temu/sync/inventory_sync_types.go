// package sync 提供TEMU库存同步相关类型定义
package sync

import (
	managementapi "task-processor/internal/listingadmin"
	"task-processor/internal/pkg/types"
)

type TemuInventoryProductSnapshot struct {
	ID                int64
	Source            string
	ImportTaskID      int64
	StoreID           int64
	Platform          string
	CategoryID        int64
	Region            string
	ParentProductID   string
	ProductID         string
	Title             string
	Description       string
	OriginalPrice     types.FlexibleString
	SpecialPrice      types.FlexibleString
	PriceCurrency     string
	Stock             types.FlexibleString
	Brand             string
	Category          string
	MainImageURL      string
	ImageURLs         string
	Attributes        string
	SourceURL         string
	Status            int16
	RawJSONDataID     int64
	PlatformProductID string
	PlatformStatus    string
	ShelfStatus       int
	PublishTime       *types.FlexibleTime
	ShelfTime         *types.FlexibleTime
	LastSyncTime      *types.FlexibleTime
	PlatformData      string
	TenantID          int64
	CreateTime        *types.FlexibleTime
	UpdateTime        *types.FlexibleTime
	Creator           string
	Updater           string
	Deleted           bool
}

func temuInventoryProductSnapshotFromDTO(prod *managementapi.ProductDataDTO) *TemuInventoryProductSnapshot {
	if prod == nil {
		return nil
	}

	return &TemuInventoryProductSnapshot{
		ID:                prod.ID,
		Source:            prod.Source,
		ImportTaskID:      prod.ImportTaskID,
		StoreID:           prod.StoreID,
		Platform:          prod.Platform,
		CategoryID:        prod.CategoryID,
		Region:            prod.Region,
		ParentProductID:   prod.ParentProductID,
		ProductID:         prod.ProductID,
		Title:             prod.Title,
		Description:       prod.Description,
		OriginalPrice:     prod.OriginalPrice,
		SpecialPrice:      prod.SpecialPrice,
		PriceCurrency:     prod.PriceCurrency,
		Stock:             prod.Stock,
		Brand:             prod.Brand,
		Category:          prod.Category,
		MainImageURL:      prod.MainImageURL,
		ImageURLs:         prod.ImageURLs,
		Attributes:        prod.Attributes,
		SourceURL:         prod.SourceURL,
		Status:            prod.Status,
		RawJSONDataID:     prod.RawJSONDataID,
		PlatformProductID: prod.PlatformProductID,
		PlatformStatus:    prod.PlatformStatus,
		ShelfStatus:       prod.ShelfStatus,
		PublishTime:       prod.PublishTime,
		ShelfTime:         prod.ShelfTime,
		LastSyncTime:      prod.LastSyncTime,
		PlatformData:      prod.PlatformData,
		TenantID:          prod.TenantID,
		CreateTime:        prod.CreateTime,
		UpdateTime:        prod.UpdateTime,
		Creator:           prod.Creator,
		Updater:           prod.Updater,
		Deleted:           prod.Deleted,
	}
}

func (prod *TemuInventoryProductSnapshot) toProductDataItemDTO() managementapi.ProductDataItemDTO {
	return managementapi.ProductDataItemDTO{
		PlatformProductID:  prod.PlatformProductID,
		ProductName:        prod.Title,
		ProductSku:         prod.ProductID,
		ProductPrice:       prod.OriginalPrice,
		ProductStock:       prod.Stock,
		ProductCategory:    prod.Category,
		ProductImage:       prod.MainImageURL,
		ProductDescription: prod.Description,
		ShelfStatus:        &prod.ShelfStatus,
		PublishTime:        prod.PublishTime,
		ShelfTime:          prod.ShelfTime,
		Brand:              prod.Brand,
		CategoryID:         &prod.CategoryID,
		SpecialPrice:       prod.SpecialPrice,
		PriceCurrency:      prod.PriceCurrency,
		ImageUrls:          prod.ImageURLs,
		Attributes:         prod.Attributes,
		PlatformStatus:     prod.PlatformStatus,
		PlatformData:       prod.PlatformData,
		ParentProductID:    prod.ParentProductID,
		CreateTime:         prod.CreateTime,
		UpdateTime:         prod.UpdateTime,
	}
}

func (prod *TemuInventoryProductSnapshot) toBatchSaveReq() *managementapi.ProductDataBatchSaveReqDTO {
	return &managementapi.ProductDataBatchSaveReqDTO{
		Platform: prod.Platform,
		TenantID: prod.TenantID,
		Region:   prod.Region,
		StoreID:  prod.StoreID,
		Products: []managementapi.ProductDataItemDTO{prod.toProductDataItemDTO()},
	}
}
