package pipeline

import (
	"fmt"

	"task-processor/internal/productenrich"
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

func (m *TaskStateMachine) CanProcess(task *productenrich.Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}

	switch task.Status {
	case productenrich.TaskStatusPending:
		return nil
	case productenrich.TaskStatusCompleted:
		return productenrich.NewNoRetryError(fmt.Errorf("task already completed"))
	case productenrich.TaskStatusProcessing:
		return productenrich.NewNoRetryError(fmt.Errorf("task is already processing"))
	case productenrich.TaskStatusFailed:
		return productenrich.NewNoRetryError(fmt.Errorf("task is failed and must be reset before reprocessing"))
	default:
		return productenrich.NewNoRetryError(fmt.Errorf("task status %q is not processable", task.Status))
	}
}

func (m *TaskStateMachine) ClassifyFailure(err error) productenrich.FailureDisposition {
	return productenrich.ClassifyProcessFailure(err)
}

func (m *TaskStateMachine) ShouldRetry(task *productenrich.Task) bool {
	if task == nil {
		return false
	}
	return task.RetryCount < m.maxRetries
}
