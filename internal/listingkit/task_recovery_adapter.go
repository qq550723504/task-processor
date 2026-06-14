package listingkit

type taskRecoverySubmitterFunc func(taskID string) error

func (f taskRecoverySubmitterFunc) Submit(taskID string) error { return f(taskID) }
