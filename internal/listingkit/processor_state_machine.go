package listingkit

import "fmt"

type ProcessorStateMachine struct {
	maxRetries int
}

func NewProcessorStateMachine(maxRetries int) *ProcessorStateMachine {
	if maxRetries <= 0 {
		maxRetries = 2
	}
	return &ProcessorStateMachine{maxRetries: maxRetries}
}

func (m *ProcessorStateMachine) CanProcess(task *Task) error {
	if task == nil {
		return fmt.Errorf("task cannot be nil")
	}
	if task.Status != TaskStatusPending {
		return fmt.Errorf("task status %q is not processable", task.Status)
	}
	return nil
}

func (m *ProcessorStateMachine) ShouldRetry(task *Task) bool {
	if task == nil {
		return false
	}
	if task.Status != TaskStatusPending {
		return false
	}
	return task.RetryCount < m.maxRetries
}
