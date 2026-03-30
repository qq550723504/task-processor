package sherr

// TaskHandledError 表示任务状态已在业务分支内完成落地，不应再走通用失败流转。
type TaskHandledError struct {
	message string
}

func (e *TaskHandledError) Error() string {
	return e.message
}

// NewTaskHandledError 创建已处理完成错误。
func NewTaskHandledError(message string) error {
	return &TaskHandledError{message: message}
}

// IsTaskHandledError 检查错误是否表示任务已经完成了状态处理。
func IsTaskHandledError(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*TaskHandledError)
	return ok
}
