package api

// ProductImportMappingCreateReqDTO 产品导入映射关系创建请求DTO
type ProductImportMappingCreateReqDTO struct {
	ID                      *int64   `json:"id,omitempty"` // 映射关系ID（更新时使用）
	TenantID                int64    `json:"tenantId"`
	ImportTaskId            int64    `json:"importTaskId"`                      // 导入任务ID
	StoreId                 int64    `json:"storeId"`                           // 店铺ID
	Platform                string   `json:"platform"`                          // 平台类型
	Region                  string   `json:"region"`                            // 区域
	ProductId               string   `json:"productId"`                         // 用户导入的产品ID或ASIN
	Sku                     *string  `json:"sku,omitempty"`                     // 生成的SKU编号
	CostPrice               *float64 `json:"costPrice"`                         // 成本价
	PlatformProductId       *string  `json:"platformProductId,omitempty"`       // 平台返回的产品ID
	ProfitRuleId            *int64   `json:"profitRuleId,omitempty"`            // 利润规则ID
	SalePriceMultiplier     *string  `json:"salePriceMultiplier,omitempty"`     // 应用的售价倍数
	DiscountPriceMultiplier *string  `json:"discountPriceMultiplier,omitempty"` // 应用的折扣价倍数
	Status                  *int16   `json:"status,omitempty"`                  // 映射状态
	Remark                  *string  `json:"remark,omitempty"`                  // 备注
	ParentProductId         *string  `json:"parentProductId,omitempty"`         // 父产品ID
	PlatformParentProductId *string  `json:"platformParentProductId,omitempty"` // 平台父产品ID
	FilterRuleId            *int64   `json:"filterRuleId,omitempty"`            // 筛选规则ID
	FilterRuleRange         *string  `json:"filterRuleRange,omitempty"`         // 筛选规则范围
}

// ProductImportMappingGetReqDTO 通过店铺ID、平台、区域和平台产品ID获取产品导入映射关系请求DTO
type ProductImportMappingGetReqDTO struct {
	PlatformProductId string `json:"platformProductId"` // 平台产品ID
	StoreId           int64  `json:"storeId"`           // 店铺ID
}

// ProductImportMappingCheckReqDTO 检查产品是否已上架请求DTO
type ProductImportMappingCheckReqDTO struct {
	StoreId   int64  `json:"storeId"`   // 店铺ID
	Platform  string `json:"platform"`  // 平台类型
	Region    string `json:"region"`    // 区域
	ProductId string `json:"productId"` // 产品ID或ASIN
}

// ProductImportMappingRespDTO 产品导入映射关系响应DTO
type ProductImportMappingRespDTO struct {
	ID                      int64    `json:"id"`                      // 主键ID
	ImportTaskId            int64    `json:"importTaskId"`            // 导入任务ID
	StoreId                 int64    `json:"storeId"`                 // 店铺ID
	Platform                string   `json:"platform"`                // 平台类型
	Region                  string   `json:"region"`                  // 区域
	ProductId               string   `json:"productId"`               // 用户导入的产品ID或ASIN
	ParentProductId         *string  `json:"parentProductId"`         // 用户导入的父级产品ID
	PlatformProductId       *string  `json:"platformProductId"`       // 平台返回的产品ID
	PlatformParentProductId *string  `json:"platformParentProductId"` // 平台返回的父级产品ID
	Sku                     *string  `json:"sku"`                     // 生成的SKU编号
	CostPrice               *float64 `json:"costPrice"`               // 成本价
	FilterRuleId            *int64   `json:"filterRuleId"`            // 过滤规则ID
	FilterRuleRange         *string  `json:"filterRuleRange"`         // 过滤规则范围
	ProfitRuleId            *int64   `json:"profitRuleId"`            // 利润规则ID
	SalePriceMultiplier     *float64 `json:"salePriceMultiplier"`     // 应用的售价倍数
	DiscountPriceMultiplier *float64 `json:"discountPriceMultiplier"` // 应用的折扣价倍数
	Status                  int16    `json:"status"`                  // 映射状态 0-草稿箱 2-已上架 3-已下架
	Remark                  *string  `json:"remark"`                  // 备注
	TenantId                int64    `json:"tenantId"`                // 租户ID
}

// ProductImportMappingGetBySkuReqDTO 通过SKU获取产品导入映射关系请求DTO
type ProductImportMappingGetBySkuReqDTO struct {
	Sku     string `json:"sku"`     // SKU编号
	StoreId int64  `json:"storeId"` // 店铺ID
}

// ProductImportMappingGetByTaskAndSkuReqDTO 根据任务ID和SKU查询映射关系请求DTO
type ProductImportMappingGetByTaskAndSkuReqDTO struct {
	ImportTaskId int64  `json:"importTaskId"` // 导入任务ID
	Sku          string `json:"sku"`          // SKU编号
}

// ProductImportMappingGetByPlatformProductIdAndStoreReqDTO 通过平台产品ID和店铺ID获取产品导入映射关系请求DTO
type ProductImportMappingGetByPlatformProductIdAndStoreReqDTO struct {
	PlatformProductId string `json:"platformProductId"` // 平台产品ID（如SHEIN的SkuCode）
	StoreId           int64  `json:"storeId"`           // 店铺ID
}

// ProductImportMappingAPI 产品导入映射API接口定义
type ProductImportMappingAPI interface {
	// CreateProductImportMapping 创建产品导入映射关系
	CreateProductImportMapping(createReqDTO *ProductImportMappingCreateReqDTO) (int64, error)

	// GetProductImportMappingByPlatformProductId 通过店铺ID、平台、区域和平台产品ID获取产品导入映射关系
	GetProductImportMappingByPlatformProductId(req *ProductImportMappingGetReqDTO) (*ProductImportMappingRespDTO, error)

	// CheckProductExists 检查产品是否已上架
	CheckProductExists(req *ProductImportMappingCheckReqDTO) (bool, error)

	// GetProductImportMappingBySku 通过SKU获取产品导入映射关系
	GetProductImportMappingBySku(req *ProductImportMappingGetBySkuReqDTO) (*ProductImportMappingRespDTO, error)

	// GetProductImportMappingByTaskAndSku 根据任务ID和SKU查询映射关系
	GetProductImportMappingByTaskAndSku(importTaskId int64, sku string) (*ProductImportMappingRespDTO, error)

	// GetProductImportMappingByPlatformProductIdAndStore 通过平台产品ID和店铺ID获取产品导入映射关系
	GetProductImportMappingByPlatformProductIdAndStore(req *ProductImportMappingGetByPlatformProductIdAndStoreReqDTO) (*ProductImportMappingRespDTO, error)

	// UpdateProductImportMapping 更新产品导入映射关系
	UpdateProductImportMapping(updateReqDTO *ProductImportMappingCreateReqDTO) error
}
