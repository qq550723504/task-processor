package pipeline

import (
	"fmt"

	productimage "task-processor/internal/productimage"
)

type TaskStateMachine struct {
	maxRetries int
}

func NewTaskStateMachine(maxRetries int) *TaskStateMachine {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &TaskStateMachine{maxRetries: maxRetries}
}

func (m *TaskStateMachine) CanProcess(task *productimage.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	switch task.Status {
	case productimage.TaskStatusPending:
		return nil
	case productimage.TaskStatusCompleted:
		return productimage.NewNoRetryError(fmt.Errorf("task already completed"))
	case productimage.TaskStatusNeedsReview:
		return productimage.NewNoRetryError(fmt.Errorf("task requires manual review before reprocessing"))
	case productimage.TaskStatusRejected:
		return productimage.NewNoRetryError(fmt.Errorf("task is rejected and must be retried manually"))
	case productimage.TaskStatusProcessing:
		return productimage.NewNoRetryError(fmt.Errorf("task is already processing"))
	case productimage.TaskStatusFailed:
		return productimage.NewNoRetryError(fmt.Errorf("task is failed and must be reset before reprocessing"))
	default:
		return productimage.NewNoRetryError(fmt.Errorf("task status %q is not processable", task.Status))
	}
}

func (m *TaskStateMachine) ClassifyFailure(err error) productimage.FailureDisposition {
	return productimage.ClassifyProcessFailure(err)
}

func (m *TaskStateMachine) ShouldRetry(task *productimage.Task) bool {
	if task == nil {
		return false
	}
	return task.RetryCount < m.maxRetries
}
