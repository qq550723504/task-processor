package api

// StoreRespDTO 店铺信息响应DTO
type StoreRespDTO struct {
	ID                      int64  `json:"id"`
	TenantID                int64  `json:"tenantId"`
	StoreID                 string `json:"storeId"`
	Name                    string `json:"name"`
	Username                string `json:"username"`
	Password                string `json:"password"`
	LoginUrl                string `json:"loginUrl"`
	ShopType                string `json:"shopType"`
	Region                  string `json:"region"`
	Platform                string `json:"platform"`
	DailyLimit              *int   `json:"dailyLimit,omitempty"`
	DailyLimitType          string `json:"dailyLimitType,omitempty"`
	FixedStockCount         *int   `json:"fixedStockCount,omitempty"`
	SkuGenerateStrategy     string `json:"skuGenerateStrategy"`
	Prefix                  string `json:"prefix"`
	Suffix                  string `json:"suffix"`
	Proxy                   string `json:"proxy"`
	EnableAutoListing       *bool  `json:"enableAutoListing,omitempty"`
	EnableAutoLogin         *bool  `json:"enableAutoLogin,omitempty"`
	EnableDraft             *bool  `json:"enableDraft,omitempty"`
	EnableAutoPrice         *bool  `json:"enableAutoPrice,omitempty"`
	EnableRebargain         *bool  `json:"enableRebargain,omitempty"`
	TemuPriceRejectStrategy string `json:"temuPriceRejectStrategy,omitempty"`
	PriceType               string `json:"priceType,omitempty"`
	Remark                  string `json:"remark"`
	Status                  int16  `json:"status"`
}

// StorePageReqDTO 分页查询店铺请求
type StorePageReqDTO struct {
	Platform        string `json:"platform,omitempty"`
	TenantID        int64  `json:"tenantId,omitempty"`
	PageNo          int    `json:"pageNo"`
	PageSize        int    `json:"pageSize"`
	EnableAutoPrice *bool  `json:"enableAutoPrice,omitempty"`
}

// StoreStatusUpdateReqDTO 店铺状态更新请求DTO
type StoreStatusUpdateReqDTO struct {
	ID     int64 `json:"id"`
	Status int16 `json:"status"`
}

// StoreIdUpdateReqDTO 修改店铺StoreID请求DTO
type StoreIdUpdateReqDTO struct {
	ID      int64  `json:"id"`
	StoreID string `json:"storeId"`
}

// StorePauseStatusRespDTO 店铺暂停状态详情
type StorePauseStatusRespDTO struct {
	Paused     bool   `json:"paused"`
	PauseType  string `json:"pauseType"`
	Reason     string `json:"reason"`
	PausedAt   int64  `json:"pausedAt"`
	PauseUntil int64  `json:"pauseUntil"`
	TTLSeconds int64  `json:"ttlSeconds"`
}

// StoreAPI 店铺管理API接口定义
type StoreAPI interface {
	GetStore(id int64) (*StoreRespDTO, error)
	PageStores(req *StorePageReqDTO) (*PageResult[*StoreRespDTO], error)
	GetStoreCookie(id int64) (string, error)
	UpdateStoreId(req *StoreIdUpdateReqDTO) (bool, error)
	UpdateStoreStatus(req *StoreStatusUpdateReqDTO) (bool, error)
	DeleteStoreCookie(id int64) (bool, error)
	SetStorePauseStatus(id int64, pause bool, pauseType string) (bool, error)
	GetStorePauseStatus(id int64) (bool, error)
	GetStorePauseStatusDetail(id int64) (*StorePauseStatusRespDTO, error)
}
