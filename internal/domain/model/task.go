package model

// Task 任务结构体
type Task struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenantId"`
	StoreID        int64  `json:"storeId"`
	Platform       string `json:"platform"`       // 目标上架平台
	SourcePlatform string `json:"sourcePlatform"` // 数据来源平台（爬虫平台）
	Region         string `json:"region"`
	CategoryID     int64  `json:"categoryId"`
	ProductID      string `json:"productId"` // ASIN或产品ID
	Status         int16  `json:"status"`
	ErrorMessage   string `json:"errorMessage"`
	RetryCount     int    `json:"retryCount"`
	MaxRetryCount  int    `json:"maxRetryCount"`
	Remark         string `json:"remark"`
	Priority       int    `json:"priority"`
	CreateTime     int64  `json:"createTime"` // Unix时间戳（毫秒）
	UpdateTime     int64  `json:"updateTime"` // Unix时间戳（毫秒）
	Creator        string `json:"creator"`
	Updater        string `json:"updater"`
}
