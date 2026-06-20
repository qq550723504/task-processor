// Package inventory 提供 SHEIN 平台库存同步功能
package inventory

import (
	"task-processor/internal/listingruntime"
	"task-processor/internal/pkg/types"
	"task-processor/internal/shein"
)

type InventoryProductSnapshot struct {
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

const (
	sheinInventoryShelfStatusOnShelf  = 2
	sheinInventoryShelfStatusOffShelf = 3
)

// MonitorResult 监控结果
type MonitorResult struct {
	TotalProducts     int // 总产品数
	ProcessedProducts int // 已处理产品数
	SkippedProducts   int // 跳过的产品数
	PriceChanges      int // 价格变化数
	StockChanges      int // 库存变化数
	AmazonFetched     int // 成功获取Amazon数据数
	AmazonFailed      int // 获取Amazon数据失败数
}

// SKUMappingData SKU映射数据（包含映射信息和库存）
type SKUMappingData struct {
	MappingInfo *listingruntime.ProductImportMapping
	Stock       int
}

// AmazonMonitorData 类型已统一到 shein.AmazonMonitorData，此处保留别名以兼容现有代码
// Deprecated: 请直接使用 shein.AmazonMonitorData
type AmazonMonitorData = shein.AmazonMonitorData
