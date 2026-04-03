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

// TryConsumeDailyQuotaReqDTO 原子预占每日上架额度请求DTO
type TryConsumeDailyQuotaReqDTO struct {
	TenantID  int64  `json:"tenantId"`
	StoreID   int64  `json:"storeId"`
	UserID    int64  `json:"userId"`
	Date      string `json:"date"`
	Increment int64  `json:"increment"`
	Limit     int64  `json:"limit"`
}

// TryConsumeDailyQuotaRespDTO 原子预占每日上架额度响应DTO
type TryConsumeDailyQuotaRespDTO struct {
	Allowed      bool  `json:"allowed"`
	NewCount     int64 `json:"newCount"`
	Remaining    int64 `json:"remaining"`
	ReachedLimit bool  `json:"reachedLimit"`
}

// RollbackDailyQuotaReqDTO 回滚每日上架额度请求DTO
type RollbackDailyQuotaReqDTO struct {
	TenantID  int64  `json:"tenantId"`
	StoreID   int64  `json:"storeId"`
	UserID    int64  `json:"userId"`
	Date      string `json:"date"`
	Decrement int64  `json:"decrement"`
}

// DailyListingCountAPI 每日上架数量API接口定义
type DailyListingCountAPI interface {
	GetDailyListingCount(tenantID, storeID, userID int64, date string) (*DailyListingCountRespDTO, error)
	SetDailyListingCount(req *DailyListingCountSetReqDTO) error
	TryConsumeDailyQuota(req *TryConsumeDailyQuotaReqDTO) (*TryConsumeDailyQuotaRespDTO, error)
	RollbackDailyQuota(req *RollbackDailyQuotaReqDTO) (int64, error)
	SetRemainingListingQuota(tenantID, storeID int64, quota int) (bool, error)
}
