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
	// SKC编码
	SKC string `json:"skc"`
	// 商品名称
	GoodsName string `json:"goods_name"`
	// 商品图片URL
	Image string `json:"image"`
	// 供应商编号
	SupplierNo string `json:"supplier_no"`
	// 活动库存
	ActStock int `json:"act_stock"`
	// 降价幅度（百分比）
	DropRate int `json:"drop_rate"`
	// 预留活动库存
	ReservedActStock int `json:"reserved_act_stock"`
	// 站点价格信息列表
	SitePriceInfoList []ActivityRegistrationSitePriceDTO `json:"site_price_info_list"`
	// 报名状态（1:已报名，2:报名失败）
	RegistrationStatus int `json:"registration_status"`
	// 报名失败原因
	FailureReason string `json:"failure_reason,omitempty"`

	// 扩展字段
	Platform     string `json:"platform"`
	TenantID     int64  `json:"tenant_id"`
	StoreID      int64  `json:"store_id"`
	Region       string `json:"region"`
	ActivityID   string `json:"activity_id"`
	ActivityName string `json:"activity_name"`

	// 成本和利润相关字段
	CostPrice  float64 `json:"cost_price,omitempty"`
	ProfitRate float64 `json:"profit_rate,omitempty"`
}

// ActivityRegistrationSitePriceDTO 活动报名站点价格信息
type ActivityRegistrationSitePriceDTO struct {
	// 站点代码
	SiteCode string `json:"site_code"`
	// 销售价格
	SalePrice float64 `json:"sale_price"`
	// 币种
	Currency string `json:"currency"`
	// 是否可用
	IsAvailable bool `json:"is_available"`
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
