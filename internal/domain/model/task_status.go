// Package model 提供数据结构定义
package model

// TaskStatus 任务状态类型
type TaskStatus int16

// 任务状态常量定义
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

// String 返回任务状态的字符串表示
func (s TaskStatus) String() string {
	if name, ok := TaskStatusNames[s]; ok {
		return name
	}
	return "未知状态"
}

// Int16 返回任务状态的int16值
func (s TaskStatus) Int16() int16 {
	return int16(s)
}

// IsTerminal 检查是否为终态
func (s TaskStatus) IsTerminal() bool {
	return s == TaskStatusPublished ||
		s == TaskStatusCancelled ||
		s == TaskStatusTerminated ||
		s == TaskStatusDraft
}

// IsProcessing 检查是否为处理中状态
func (s TaskStatus) IsProcessing() bool {
	return s == TaskStatusProcessing ||
		s == TaskStatusRepublishing ||
		s == TaskStatusResuming
}

// CanTransitionTo 检查是否可以转换到目标状态
func (s TaskStatus) CanTransitionTo(target TaskStatus) bool {
	// 简化的状态转换规则，实际应用中可能更复杂
	switch s {
	case TaskStatusPending:
		return target == TaskStatusProcessing || target == TaskStatusCancelled
	case TaskStatusProcessing:
		return target == TaskStatusCrawled || target == TaskStatusCrawlFailed || target == TaskStatusCancelled
	case TaskStatusCrawled:
		return target == TaskStatusQueued || target == TaskStatusDraft || target == TaskStatusCancelled
	case TaskStatusCrawlFailed:
		return target == TaskStatusPendingRetry || target == TaskStatusCancelled
	case TaskStatusPendingRetry:
		return target == TaskStatusProcessing || target == TaskStatusCancelled
	case TaskStatusQueued:
		return target == TaskStatusPublished || target == TaskStatusDraft || target == TaskStatusCancelled
	case TaskStatusPublished:
		return target == TaskStatusRepublishing || target == TaskStatusPaused
	case TaskStatusPaused:
		return target == TaskStatusResumed || target == TaskStatusCancelled
	case TaskStatusResumed:
		return target == TaskStatusResuming
	case TaskStatusResuming:
		return target == TaskStatusPublished || target == TaskStatusPaused
	default:
		return false
	}
}
