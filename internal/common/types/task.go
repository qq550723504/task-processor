package types

// Task 任务结构体
type Task struct {
	ID         string `json:"id"`
	TenantID   int64  `json:"tenantId"`
	ProductID  string `json:"productId"`
	Platform   string `json:"platform"`
	Region     string `json:"region"`
	StoreID    int64  `json:"storeId"`
	CategoryID int64  `json:"categoryId"`
	CreateTime int64  `json:"createTime"`
	RetryCount int    `json:"retryCount"`
	Priority   int    `json:"priority"`
	Creator    string `json:"creator"`
}
