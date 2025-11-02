package api

// StoreRespDTO 店铺信息响应DTO
type StoreRespDTO struct {
	ID                  int64  `json:"id"`                          // 主键ID
	StoreID             string `json:"storeId"`                     // 店铺ID
	Name                string `json:"name"`                        // 店铺名称
	Username            string `json:"username"`                    // 登录用户名
	Password            string `json:"password"`                    // 登录密码
	LoginUrl            string `json:"loginUrl"`                    // 登录地址
	ShopType            string `json:"shopType"`                    // 店铺类型
	Region              string `json:"region"`                      // 店铺地区
	Platform            string `json:"platform"`                    // 平台类型
	DailyLimit          *int   `json:"dailyLimit,omitempty"`        // 每日上架限制
	FixedStockCount     *int   `json:"fixedStockCount,omitempty"`   // 固定库存数量
	SkuGenerateStrategy string `json:"skuGenerateStrategy"`         // SKU生成策略
	Prefix              string `json:"prefix"`                      // SKU前缀
	Suffix              string `json:"suffix"`                      // SKU后缀
	Proxy               string `json:"proxy"`                       // 代理地址
	EnableAutoListing   *bool  `json:"enableAutoListing,omitempty"` // 是否启用自动上架
	EnableAutoLogin     *bool  `json:"enableAutoLogin,omitempty"`   // 是否启用自动登录
	EnableDraft         *bool  `json:"enableDraft,omitempty"`       // 是否启用草稿模式
	EnableAutoPrice     *bool  `json:"enableAutoPrice,omitempty"`   // 是否启用自动定价
	EnableRebargain     *bool  `json:"enableRebargain,omitempty"`   // 是否启用重新议价
	Remark              string `json:"remark"`                      // 备注信息
	Status              int16  `json:"status"`                      // 是否启用: 0-禁用 1-启用（管理员用）
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

// StoreStatusUpdateReqDTO 店铺状态更新请求DTO
type StoreStatusUpdateReqDTO struct {
	ID     int64 `json:"id"`     // 主键ID
	Status int16 `json:"status"` // 状态: 1-禁用 0-启用
}

// StoreIdUpdateReqDTO 修改店铺StoreID请求DTO
type StoreIdUpdateReqDTO struct {
	ID      int64  `json:"id"`      // 主键ID
	StoreID string `json:"storeId"` // 店铺ID
}

// ProductImportMappingCreateReqDTO 产品导入映射关系创建请求DTO
type ProductImportMappingCreateReqDTO struct {
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
}

// StoreAPI 店铺管理API接口定义
type StoreAPI interface {
	// GetStore 通过店铺ID获取店铺信息
	GetStore(id int64) (*StoreRespDTO, error)

	// GetStoreCookie 通过店铺ID获取用户Cookie
	GetStoreCookie(id int64) (string, error)

	// UpdateStoreId 修改店铺的StoreID
	UpdateStoreId(req *StoreIdUpdateReqDTO) (bool, error)

	// UpdateStoreStatus 更新店铺状态
	UpdateStoreStatus(req *StoreStatusUpdateReqDTO) (bool, error)

	// CreateProductImportMapping 创建产品导入映射关系
	CreateProductImportMapping(createReqDTO *ProductImportMappingCreateReqDTO) (int64, error)

	// GetProductImportMappingBydPlatformProductId 通过店铺ID、平台、区域和平台产品ID获取产品导入映射关系
	GetProductImportMappingByPlatformProductId(req *ProductImportMappingGetReqDTO) (*ProductImportMappingRespDTO, error)

	// DeleteStoreCookie 通过店铺ID删除用户Cookie
	DeleteStoreCookie(id int64) (bool, error)

	// SetStorePauseStatus 设置店铺任务暂停状态
	SetStorePauseStatus(id int64, pause bool) (bool, error)
}
