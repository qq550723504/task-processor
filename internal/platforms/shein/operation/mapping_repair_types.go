// Package operation 提供SHEIN平台SKU映射关系修复相关类型定义
package operation

import (
	"time"

	managementapi "task-processor/internal/infra/clients/management/api"
	"task-processor/internal/platforms/shein/api/product"
)

// MappingRepairRequest SKU映射关系修复请求
type MappingRepairRequest struct {
	TenantID  int64      `json:"tenantId"`            // 租户ID
	StoreID   int64      `json:"storeId"`             // 店铺ID
	SkuCode   string     `json:"skuCode"`             // SKU编码
	SpuCode   string     `json:"spuCode"`             // SPU编码
	SpuName   string     `json:"spuName"`             // SPU名称
	Reason    string     `json:"reason"`              // 修复原因
	Priority  int        `json:"priority"`            // 优先级 1-高 2-中 3-低
	RetryTime *time.Time `json:"retryTime,omitempty"` // 重试时间
}

// MappingRepairResult SKU映射关系修复结果
type MappingRepairResult struct {
	SkuCode     string                                     `json:"skuCode"`     // SKU编码
	Success     bool                                       `json:"success"`     // 是否成功
	MappingInfo *managementapi.ProductImportMappingRespDTO `json:"mappingInfo"` // 映射信息
	Error       string                                     `json:"error"`       // 错误信息
	RepairTime  time.Time                                  `json:"repairTime"`  // 修复时间
}

// MappingRepairConfig SKU映射关系修复配置
type MappingRepairConfig struct {
	MaxRetryCount    int           `json:"maxRetryCount"`    // 最大重试次数
	RetryInterval    time.Duration `json:"retryInterval"`    // 重试间隔
	BatchSize        int           `json:"batchSize"`        // 批处理大小
	EnableAutoRepair bool          `json:"enableAutoRepair"` // 是否启用自动修复
	RepairTimeout    time.Duration `json:"repairTimeout"`    // 修复超时时间
}

// DefaultMappingRepairConfig 默认修复配置
func DefaultMappingRepairConfig() *MappingRepairConfig {
	return &MappingRepairConfig{
		MaxRetryCount:    3,
		RetryInterval:    5 * time.Minute,
		BatchSize:        50,
		EnableAutoRepair: true,
		RepairTimeout:    30 * time.Second,
	}
}

// MappingRepairContext SKU映射关系修复上下文
type MappingRepairContext struct {
	Request     *MappingRepairRequest       `json:"request"`     // 修复请求
	SkuInfo     *product.SkuInfo            `json:"skuInfo"`     // SKU信息
	ProductInfo *product.ProductListItem    `json:"productInfo"` // 产品信息
	StoreInfo   *managementapi.StoreRespDTO `json:"storeInfo"`   // 店铺信息
	RetryCount  int                         `json:"retryCount"`  // 重试次数
	StartTime   time.Time                   `json:"startTime"`   // 开始时间
}

// MappingRepairStrategy SKU映射关系修复策略
type MappingRepairStrategy interface {
	// CanRepair 判断是否可以修复
	CanRepair(ctx *MappingRepairContext) bool

	// Repair 执行修复
	Repair(ctx *MappingRepairContext) (*MappingRepairResult, error)

	// GetStrategyName 获取策略名称
	GetStrategyName() string
}

// MappingRepairStats SKU映射关系修复统计
type MappingRepairStats struct {
	TotalRequests  int64     `json:"totalRequests"`  // 总请求数
	SuccessCount   int64     `json:"successCount"`   // 成功数量
	FailedCount    int64     `json:"failedCount"`    // 失败数量
	RetryCount     int64     `json:"retryCount"`     // 重试数量
	LastRepairTime time.Time `json:"lastRepairTime"` // 最后修复时间
	AverageTime    float64   `json:"averageTime"`    // 平均修复时间(秒)
}
