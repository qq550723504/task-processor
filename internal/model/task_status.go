package model

import "fmt"

// TaskStatus represents the lifecycle state of an import task in management.
type TaskStatus int16

const (
	TaskStatusPending      TaskStatus = 0
	TaskStatusProcessing   TaskStatus = 1
	TaskStatusCrawled      TaskStatus = 2
	TaskStatusCrawlFailed  TaskStatus = 3
	TaskStatusPendingRetry TaskStatus = 4
	TaskStatusQueued       TaskStatus = 5
	TaskStatusPublished    TaskStatus = 6
	TaskStatusRepublishing TaskStatus = 7
	TaskStatusDraft        TaskStatus = 8
	TaskStatusCancelled    TaskStatus = 9
	TaskStatusPaused       TaskStatus = 10
	TaskStatusResumed      TaskStatus = 11
	TaskStatusResuming     TaskStatus = 12
	TaskStatusTerminated   TaskStatus = 13
)

var taskStatusNames = map[TaskStatus]string{
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

var taskStatusTransitions = map[TaskStatus]map[TaskStatus]struct{}{
	TaskStatusPending: {
		TaskStatusProcessing: {},
		TaskStatusCancelled:  {},
	},
	TaskStatusProcessing: {
		TaskStatusCrawled:      {},
		TaskStatusCrawlFailed:  {},
		TaskStatusPendingRetry: {},
		TaskStatusPublished:    {},
		TaskStatusDraft:        {},
		TaskStatusPaused:       {},
		TaskStatusTerminated:   {},
		TaskStatusCancelled:    {},
	},
	TaskStatusCrawled: {
		TaskStatusQueued:    {},
		TaskStatusDraft:     {},
		TaskStatusCancelled: {},
	},
	TaskStatusCrawlFailed: {
		TaskStatusPendingRetry: {},
		TaskStatusTerminated:   {},
		TaskStatusCancelled:    {},
	},
	TaskStatusPendingRetry: {
		TaskStatusProcessing: {},
		TaskStatusCancelled:  {},
	},
	TaskStatusQueued: {
		TaskStatusPublished:  {},
		TaskStatusDraft:      {},
		TaskStatusCancelled:  {},
		TaskStatusTerminated: {},
	},
	TaskStatusPublished: {
		TaskStatusRepublishing: {},
		TaskStatusPaused:       {},
	},
	TaskStatusPaused: {
		TaskStatusResumed:    {},
		TaskStatusCancelled:  {},
		TaskStatusTerminated: {},
	},
	TaskStatusResumed: {
		TaskStatusResuming: {},
	},
	TaskStatusResuming: {
		TaskStatusPublished:  {},
		TaskStatusPaused:     {},
		TaskStatusTerminated: {},
	},
}

func ParseTaskStatus(code int16) (TaskStatus, error) {
	status := TaskStatus(code)
	if !status.IsValid() {
		return 0, fmt.Errorf("unknown task status code: %d", code)
	}
	return status, nil
}

func (s TaskStatus) IsValid() bool {
	_, ok := taskStatusNames[s]
	return ok
}

func (s TaskStatus) String() string {
	if name, ok := taskStatusNames[s]; ok {
		return name
	}
	return fmt.Sprintf("未知状态(%d)", s)
}

func (s TaskStatus) Int16() int16 {
	return int16(s)
}

func (s TaskStatus) IsTerminal() bool {
	switch s {
	case TaskStatusPublished, TaskStatusCancelled, TaskStatusTerminated, TaskStatusDraft:
		return true
	default:
		return false
	}
}

func (s TaskStatus) IsProcessing() bool {
	switch s {
	case TaskStatusProcessing, TaskStatusRepublishing, TaskStatusResuming:
		return true
	default:
		return false
	}
}

func (s TaskStatus) CanTransitionTo(target TaskStatus) bool {
	if !s.IsValid() || !target.IsValid() {
		return false
	}
	if s == target {
		return true
	}

	allowed, ok := taskStatusTransitions[s]
	if !ok {
		return false
	}

	_, ok = allowed[target]
	return ok
}

func ValidateTaskStatusTransition(from, to TaskStatus) error {
	if !from.IsValid() {
		return fmt.Errorf("invalid source task status: %d", from)
	}
	if !to.IsValid() {
		return fmt.Errorf("invalid target task status: %d", to)
	}
	if !from.CanTransitionTo(to) {
		return fmt.Errorf("invalid task status transition: %s -> %s", from.String(), to.String())
	}
	return nil
}

func ValidateTaskStatusTransitionCode(fromCode int16, to TaskStatus) error {
	from, err := ParseTaskStatus(fromCode)
	if err != nil {
		return err
	}
	return ValidateTaskStatusTransition(from, to)
}
