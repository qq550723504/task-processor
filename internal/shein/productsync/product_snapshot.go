// Package productsync 提供 SHEIN 产品同步内部快照模型
package productsync

import "task-processor/internal/pkg/types"

type ProductSnapshot struct {
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
