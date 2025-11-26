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
	TaskStatusPending      TaskStatus = 0  // 待处理
	TaskStatusProcessing   TaskStatus = 1  // 处理中
	TaskStatusCrawled      TaskStatus = 2  // 已抓取
	TaskStatusCrawlFailed  TaskStatus = 3  // 抓取失败
	TaskStatusPendingRetry TaskStatus = 4  // 待重试
	TaskStatusQueued       TaskStatus = 5  // 队列中
	TaskStatusPublished    TaskStatus = 6  // 已上架
	TaskStatusRepublishing TaskStatus = 7  // 重新上架中
	TaskStatusDraft        TaskStatus = 8  // 草稿箱
	TaskStatusCancelled    TaskStatus = 9  // 已取消
	TaskStatusPaused       TaskStatus = 10 // 已暂停
	TaskStatusResumed      TaskStatus = 11 // 已恢复
	TaskStatusResuming     TaskStatus = 12 // 恢复中
	TaskStatusTerminated   TaskStatus = 13 // 已终止
)

// TaskStatusNames 任务状态名称映射
var TaskStatusNames = map[TaskStatus]string{
	TaskStatusPending:      "待处理",
	TaskStatusProcessing:   "处理中",
	TaskStatusCrawled:      "已抓取",
	TaskStatusCrawlFailed:  "抓取失败",
	TaskStatusPendingRetry: "待重试",
	TaskStatusQueued:       "队列中",
	TaskStatusPublished:    "已上架",
	TaskStatusRepublishing: "重新上架中",
	TaskStatusDraft:        "草稿箱",
	TaskStatusCancelled:    "已取消",
	TaskStatusPaused:       "已暂停",
	TaskStatusResumed:      "已恢复",
	TaskStatusResuming:     "恢复中",
	TaskStatusTerminated:   "已终止",
}

// Int16 返回状态的int16值
func (s TaskStatus) Int16() int16 {
	return int16(s)
}

// String 返回状态的字符串表示
func (s TaskStatus) String() string {
	if name, ok := TaskStatusNames[s]; ok {
		return name
	}
	return "未知状态"
}
