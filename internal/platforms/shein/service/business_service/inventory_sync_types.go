// Package scheduler 提供SHEIN平台调度器相关服务的类型定义
package scheduler

import managementapi "task-processor/internal/pkg/management/api"

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
	MappingInfo *managementapi.ProductImportMappingRespDTO
	Stock       int
}

// AmazonMonitorData Amazon监控数据
type AmazonMonitorData struct {
	ASIN          string  `json:"asin"`
	Price         float64 `json:"price"`
	Stock         int     `json:"stock"`
	LastCheckTime int64   `json:"last_check_time"` // Unix时间戳（秒）
}
