package api

// DailyListingCountGetReqDTO 获取每日上架数量请求DTO
type DailyListingCountGetReqDTO struct {
	TenantID int64  `json:"tenantId"`
	StoreID  int64  `json:"storeId"`
	UserID   int64  `json:"userId"`
	Date     string `json:"date"`
}

// DailyListingCountSetReqDTO 设置每日上架数量请求DTO
type DailyListingCountSetReqDTO struct {
	TenantID int64  `json:"tenantId"`
	StoreID  int64  `json:"storeId"`
	UserID   int64  `json:"userId"`
	Date     string `json:"date"`
	Count    int64  `json:"count"`
}

// DailyListingCountRespDTO 每日上架数量响应DTO
type DailyListingCountRespDTO struct {
	TenantID int64  `json:"tenantId"`
	StoreID  int64  `json:"storeId"`
	UserID   int64  `json:"userId"`
	Date     string `json:"date"`
	Count    int64  `json:"count"`
}

// DailyListingCountAPI 每日上架数量API接口定义
type DailyListingCountAPI interface {
	GetDailyListingCount(tenantID, storeID, userID int64, date string) (*DailyListingCountRespDTO, error)
	SetDailyListingCount(req *DailyListingCountSetReqDTO) error
	SetRemainingListingQuota(tenantID, storeID int64, quota int) (bool, error)
}
