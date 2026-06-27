// package sync 提供 TEMU 产品同步内部快照模型
package sync

import (
	"task-processor/internal/listingadmin"
	"task-processor/internal/pkg/types"
)

type TemuProductSnapshot struct {
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

func (prod *TemuProductSnapshot) toProductDataItemDTO() listingadmin.ProductDataItemDTO {
	return listingadmin.ProductDataItemDTO{
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

func (prod *TemuProductSnapshot) toBatchSaveReq(items []listingadmin.ProductDataItemDTO) *listingadmin.ProductDataBatchSaveReqDTO {
	return &listingadmin.ProductDataBatchSaveReqDTO{
		Platform: prod.Platform,
		TenantID: prod.TenantID,
		Region:   prod.Region,
		StoreID:  prod.StoreID,
		Products: items,
	}
}
