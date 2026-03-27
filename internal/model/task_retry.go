package model

const (
	defaultTaskMaxRetries = 3
	retryPriorityPenalty  = 10
)

type RetryDecision struct {
	OriginalPriority int
	CurrentPriority  int
	RetryCount       int
	MaxRetries       int
	Exhausted        bool
}

func ResolveTaskMaxRetries(configured int) int {
	if configured <= 0 {
		return defaultTaskMaxRetries
	}
	return configured
}

func ApplyRetryFailure(task *Task, configuredMaxRetries int) RetryDecision {
	decision := RetryDecision{
		MaxRetries: ResolveTaskMaxRetries(configuredMaxRetries),
	}
	if task == nil {
		decision.Exhausted = true
		return decision
	}

	task.RetryCount++
	decision.RetryCount = task.RetryCount
	decision.OriginalPriority = task.Priority

	if task.RetryCount > 0 && task.Priority > retryPriorityPenalty {
		task.Priority -= retryPriorityPenalty
		if task.Priority < 0 {
			task.Priority = 0
		}
	}

	decision.CurrentPriority = task.Priority
	decision.Exhausted = task.RetryCount >= decision.MaxRetries
	return decision
}
