package api

// StoreRespDTO 店铺信息响应DTO
type StoreRespDTO struct {
	ID                      int64  `json:"id"`                                // 主键ID
	TenantID                int64  `json:"tenantId"`                          //租户ID
	StoreID                 string `json:"storeId"`                           // 店铺ID
	Name                    string `json:"name"`                              // 店铺名称
	Username                string `json:"username"`                          // 登录用户名
	Password                string `json:"password"`                          // 登录密码
	LoginUrl                string `json:"loginUrl"`                          // 登录地址
	ShopType                string `json:"shopType"`                          // 店铺类型
	Region                  string `json:"region"`                            // 店铺地区
	Platform                string `json:"platform"`                          // 平台类型
	DailyLimit              *int   `json:"dailyLimit,omitempty"`              // 每日上架限制
	DailyLimitType          string `json:"dailyLimitType,omitempty"`          // 每日限制类型: SKC或SKU
	FixedStockCount         *int   `json:"fixedStockCount,omitempty"`         // 固定库存数量
	SkuGenerateStrategy     string `json:"skuGenerateStrategy"`               // SKU生成策略
	Prefix                  string `json:"prefix"`                            // SKU前缀
	Suffix                  string `json:"suffix"`                            // SKU后缀
	Proxy                   string `json:"proxy"`                             // 代理地址
	EnableAutoListing       *bool  `json:"enableAutoListing,omitempty"`       // 是否启用自动上架
	EnableAutoLogin         *bool  `json:"enableAutoLogin,omitempty"`         // 是否启用自动登录
	EnableDraft             *bool  `json:"enableDraft,omitempty"`             // 是否启用草稿模式
	EnableAutoPrice         *bool  `json:"enableAutoPrice,omitempty"`         // 是否启用自动定价
	EnableRebargain         *bool  `json:"enableRebargain,omitempty"`         // 是否启用重新议价
	TemuPriceRejectStrategy string `json:"temuPriceRejectStrategy,omitempty"` // TEMU核价不通过时的处理策略
	Remark                  string `json:"remark"`                            // 备注信息
	Status                  int16  `json:"status"`                            // 是否启用: 0-禁用 1-启用（管理员用）
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

	// DeleteStoreCookie 通过店铺ID删除用户Cookie
	DeleteStoreCookie(id int64) (bool, error)

	// SetStorePauseStatus 设置店铺任务暂停状态
	// pauseType: auth_expired(认证过期) 或 quota_limit(配额限制)，空字符串时使用默认值 quota_limit
	SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error)
}

// ManagementAPI 管理系统API接口定义
type ManagementAPI interface {
	// GetStore 通过店铺ID获取店铺信息
	GetStore(id int64) (*StoreRespDTO, error)

	// GetStoreCookie 通过店铺ID获取用户Cookie
	GetStoreCookie(id int64) (string, error)

	// UpdateStoreId 修改店铺的StoreID
	UpdateStoreId(req *StoreIdUpdateReqDTO) (bool, error)

	// UpdateStoreStatus 更新店铺状态
	UpdateStoreStatus(req *StoreStatusUpdateReqDTO) (bool, error)

	// DeleteStoreCookie 通过店铺ID删除用户Cookie
	DeleteStoreCookie(id int64) (bool, error)

	// SetStorePauseStatus 设置店铺任务暂停状态
	// pauseType: auth_expired(认证过期) 或 quota_limit(配额限制)，空字符串时使用默认值 quota_limit
	SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error)
}
