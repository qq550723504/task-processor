package listingkit

import "context"

type TaskResultMutation func(task *Task) error

type TaskResultTransactionRepository interface {
	MutateTaskResult(ctx context.Context, taskID string, mutate TaskResultMutation) (*Task, error)
}
