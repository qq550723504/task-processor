package api

// DailyListingCountGetReqDTO 获取每日上架数量请求DTO
type DailyListingCountGetReqDTO struct {
	TenantID int64  `json:"tenantId"` // 租户编号
	StoreID  int64  `json:"storeId"`  // 店铺ID
	UserID   int64  `json:"userId"`   // 用户ID
	Date     string `json:"date"`     // 日期 (格式: 2006-01-02)
}

// DailyListingCountSetReqDTO 设置每日上架数量请求DTO
type DailyListingCountSetReqDTO struct {
	TenantID int64  `json:"tenantId"` // 租户编号
	StoreID  int64  `json:"storeId"`  // 店铺ID
	UserID   int64  `json:"userId"`   // 用户ID
	Date     string `json:"date"`     // 日期 (格式: 2006-01-02)
	Count    int64  `json:"count"`    // 数量
}

// DailyListingCountRespDTO 每日上架数量响应DTO
type DailyListingCountRespDTO struct {
	TenantID int64  `json:"tenantId"` // 租户编号
	StoreID  int64  `json:"storeId"`  // 店铺ID
	UserID   int64  `json:"userId"`   // 用户ID
	Date     string `json:"date"`     // 日期
	Count    int64  `json:"count"`    // 数量
}

// DailyListingCountAPI 每日上架数量API接口定义
type DailyListingCountAPI interface {
	// GetDailyListingCount 获取每日上架数量
	GetDailyListingCount(tenantID, storeID, userID int64, date string) (*DailyListingCountRespDTO, error)

	// SetDailyListingCount 设置每日上架数量
	SetDailyListingCount(req *DailyListingCountSetReqDTO) error

	// SetRemainingListingQuota 设置剩余发品额度
	SetRemainingListingQuota(tenantID, storeID int64, quota int) (bool, error)
}
