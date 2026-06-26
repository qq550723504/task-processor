package submission

import "time"

// RemoteRefreshExecutionState carries the generic inputs used by a remote
// refresh orchestration while feature packages adapt concrete request fields.
type RemoteRefreshExecutionState[Completion any] struct {
	Completion   Completion
	SupplierCode string
	StartedAt    time.Time
}

// NewRemoteRefreshExecutionState builds a remote refresh execution state.
func NewRemoteRefreshExecutionState[Completion any](completion Completion, supplierCode string, startedAt time.Time) *RemoteRefreshExecutionState[Completion] {
	return &RemoteRefreshExecutionState[Completion]{
		Completion:   completion,
		SupplierCode: supplierCode,
		StartedAt:    startedAt,
	}
}
