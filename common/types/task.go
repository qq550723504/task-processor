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

// TaskStatus 任务状态枚举
type TaskStatus int16

const (
	TaskStatusPending    TaskStatus = 0 // 待处理
	TaskStatusProcessing TaskStatus = 1 // 处理中
	TaskStatusCompleted  TaskStatus = 2 // 已完成
	TaskStatusFailed     TaskStatus = 3 // 失败
	TaskStatusRetry      TaskStatus = 4 // 重试
)

// Int16 返回状态的int16值
func (s TaskStatus) Int16() int16 {
	return int16(s)
}

// String 返回状态的字符串表示
func (s TaskStatus) String() string {
	switch s {
	case TaskStatusPending:
		return "待处理"
	case TaskStatusProcessing:
		return "处理中"
	case TaskStatusCompleted:
		return "已完成"
	case TaskStatusFailed:
		return "失败"
	case TaskStatusRetry:
		return "重试"
	default:
		return "未知"
	}
}
