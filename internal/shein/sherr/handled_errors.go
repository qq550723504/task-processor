package sherr

import "task-processor/internal/model"

// TaskHandledError 表示任务已得出最终状态，等待由上层统一落地。
type TaskHandledError struct {
	message      string
	targetStatus model.TaskStatus
	errorMessage string
}

func (e *TaskHandledError) Error() string {
	return e.message
}

func (e *TaskHandledError) TargetStatus() model.TaskStatus {
	return e.targetStatus
}

func (e *TaskHandledError) ErrorMessage() string {
	return e.errorMessage
}

// NewTaskHandledError 创建一个需要由上层统一回写状态的已处理错误。
func NewTaskHandledError(targetStatus model.TaskStatus, message, errorMessage string) error {
	return &TaskHandledError{
		message:      message,
		targetStatus: targetStatus,
		errorMessage: errorMessage,
	}
}

// AsTaskHandledError 返回 TaskHandledError 及其断言结果。
func AsTaskHandledError(err error) (*TaskHandledError, bool) {
	if err == nil {
		return nil, false
	}
	handledErr, ok := err.(*TaskHandledError)
	return handledErr, ok
}

// IsTaskHandledError 检查错误是否表示任务已经完成了状态处理。
func IsTaskHandledError(err error) bool {
	_, ok := AsTaskHandledError(err)
	return ok
}
