package model

import "strings"

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

// IsValid 验证任务是否有效
func (t *Task) IsValid() bool {
	return t.ID != 0 && t.ProductID != ""
}

// IsCrawlerTask 判断是否是爬虫任务
func (t *Task) IsCrawlerTask() bool {
	return strings.Contains(strings.ToLower(t.Platform), "crawler")
}

// GetBasePlatform 获取基础平台名称（移除 .crawler 后缀）
func (t *Task) GetBasePlatform() string {
	return strings.TrimSuffix(t.Platform, ".crawler")
}

// CanRetry 判断是否可以重试
func (t *Task) CanRetry() bool {
	return t.RetryCount < t.MaxRetryCount
}

// IsHighPriority 判断是否是高优先级任务
func (t *Task) IsHighPriority() bool {
	return t.Priority >= 1 && t.Priority <= 3
}

// IsNormalPriority 判断是否是普通优先级任务
func (t *Task) IsNormalPriority() bool {
	return t.Priority >= 4 && t.Priority <= 7
}

// IsLowPriority 判断是否是低优先级任务
func (t *Task) IsLowPriority() bool {
	return t.Priority >= 8 && t.Priority <= 10
}

// GetPriorityLevel 获取优先级级别描述
func (t *Task) GetPriorityLevel() string {
	switch {
	case t.IsHighPriority():
		return "high"
	case t.IsNormalPriority():
		return "normal"
	default:
		return "low"
	}
}

// IsVariantTask 判断是否是变体任务
func (t *Task) IsVariantTask() bool {
	return t.Remark == "variant"
}

// PlatformMatches 判断平台是否匹配（忽略大小写，忽略 .crawler 后缀）
func (t *Task) PlatformMatches(targetPlatform string) bool {
	taskBasePlatform := t.GetBasePlatform()
	targetBasePlatform := strings.TrimSuffix(targetPlatform, ".crawler")
	return strings.EqualFold(taskBasePlatform, targetBasePlatform)
}
