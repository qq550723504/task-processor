// Package task 提供任务消息载荷类型定义
package task

// CrawlerPayload 爬虫消息载荷
type CrawlerPayload struct {
	ID             int64  `json:"id"`
	TenantID       int64  `json:"tenantId"`
	StoreID        int64  `json:"storeId"`
	SourcePlatform string `json:"sourcePlatform"` // 爬虫平台（如 amazon、1688）
	Region         string `json:"region"`
	ProductID      string `json:"productId"`
	Priority       int    `json:"priority"`
	ReplyTo        string `json:"reply_to"`
	CreateTime     int64  `json:"createTime"`
	UpdateTime     int64  `json:"updateTime"`
	RetryCount     int    `json:"retryCount"`
	MaxRetries     int    `json:"maxRetryCount"`
}

// TaskPayload 任务消息载荷
type TaskPayload struct {
	TaskID         int64  `json:"taskId"`
	TenantID       int64  `json:"tenantId"`
	StoreID        int64  `json:"storeId"`
	SourcePlatform string `json:"sourcePlatform"` // 数据来源平台（爬虫平台，如 amazon、1688）
	TargetPlatform string `json:"targetPlatform"` // 目标上架平台（如 shein、temu）
	Region         string `json:"region"`
	CategoryID     int64  `json:"categoryId"`
	ProductID      string `json:"productId"`
	Priority       int    `json:"priority"`
	Remark         string `json:"remark"`
	CreateTime     int64  `json:"createTime"`
	UpdateTime     int64  `json:"updateTime"`
	RetryCount     int    `json:"retryCount"`
	MaxRetryCount  int    `json:"maxRetryCount"`
}

// ResultPayload 结果消息载荷
type ResultPayload struct {
	TaskID       int64          `json:"taskId"`
	Status       string         `json:"status"`
	Message      string         `json:"message"`
	Data         map[string]any `json:"data,omitempty"`
	ProcessTime  int64          `json:"processTime"`
	ErrorCode    string         `json:"errorCode,omitempty"`
	ErrorMessage string         `json:"errorMessage,omitempty"`
	RetryCount   int            `json:"retryCount"`
	NodeID       string         `json:"nodeId"`
	Timestamp    int64          `json:"timestamp"`
}

// SuccessData 成功结果数据
type SuccessData struct {
	Platform       string `json:"platform"`
	TargetPlatform string `json:"target_platform,omitempty"`
	SourcePlatform string `json:"source_platform,omitempty"`
	ProductID      string `json:"product_id"`
	StoreID        int64  `json:"store_id"`
}

// ToMap 转换为 map
func (s *SuccessData) ToMap() map[string]any {
	return map[string]any{
		"platform":        s.Platform,
		"target_platform": s.TargetPlatform,
		"source_platform": s.SourcePlatform,
		"product_id":      s.ProductID,
		"store_id":        s.StoreID,
	}
}

// NewSuccessData 创建成功数据
func NewSuccessData(targetPlatform, sourcePlatform, productID string, storeID int64) *SuccessData {
	return &SuccessData{
		Platform:       targetPlatform,
		TargetPlatform: targetPlatform,
		SourcePlatform: sourcePlatform,
		ProductID:      productID,
		StoreID:        storeID,
	}
}
