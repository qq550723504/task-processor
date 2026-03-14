// Package api 活动报名记录API接口定义
package api

import "time"

// ActivityRegistrationAPI 活动报名记录API接口定义
type ActivityRegistrationAPI interface {
	// BatchSaveActivityRegistrations 批量保存活动报名记录
	BatchSaveActivityRegistrations(registrations []*ActivityRegistrationDTO) error
}

// ActivityRegistrationDTO 活动报名记录数据传输对象
type ActivityRegistrationDTO struct {
	SKC                string                             `json:"skc"`
	GoodsName          string                             `json:"goods_name"`
	Image              string                             `json:"image"`
	SupplierNo         string                             `json:"supplier_no"`
	ActStock           int                                `json:"act_stock"`
	DropRate           int                                `json:"drop_rate"`
	ReservedActStock   int                                `json:"reserved_act_stock"`
	SitePriceInfoList  []ActivityRegistrationSitePriceDTO `json:"site_price_info_list"`
	RegistrationStatus int                                `json:"registration_status"`
	FailureReason      string                             `json:"failure_reason,omitempty"`
	Platform           string                             `json:"platform"`
	TenantID           int64                              `json:"tenant_id"`
	StoreID            int64                              `json:"store_id"`
	Region             string                             `json:"region"`
	ActivityID         string                             `json:"activity_id"`
	ActivityName       string                             `json:"activity_name"`
	CostPrice          float64                            `json:"cost_price,omitempty"`
	ProfitRate         float64                            `json:"profit_rate,omitempty"`
}

// ActivityRegistrationSitePriceDTO 活动报名站点价格信息
type ActivityRegistrationSitePriceDTO struct {
	SiteCode    string  `json:"site_code"`
	SalePrice   float64 `json:"sale_price"`
	Currency    string  `json:"currency"`
	IsAvailable bool    `json:"is_available"`
}

// ActivityRegistrationSyncRequest 活动报名记录同步请求
type ActivityRegistrationSyncRequest struct {
	Platform            string                    `json:"platform"`
	TenantID            int64                     `json:"tenant_id"`
	Region              string                    `json:"region"`
	StoreID             int64                     `json:"store_id"`
	ActivityID          string                    `json:"activity_id"`
	ActivityName        string                    `json:"activity_name"`
	RegistrationRecords []ActivityRegistrationDTO `json:"registration_records"`
	RegistrationTime    int64                     `json:"registration_time"`
}

// ActivityRegistrationSyncResult 活动报名记录同步结果
type ActivityRegistrationSyncResult struct {
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	TotalCount   int       `json:"total_count"`
	SuccessCount int       `json:"success_count"`
	FailedCount  int       `json:"failed_count"`
	ErrorMsg     string    `json:"error_msg,omitempty"`
}
