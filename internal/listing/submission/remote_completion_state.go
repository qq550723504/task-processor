package submission

import "time"

// RemoteCompletionState carries the generic state needed to finish or fail a
// remote submission confirmation while callers keep feature-specific mutation
// and persistence behavior outside the domain package.
type RemoteCompletionState[Task any, Package any, Response any] struct {
	TaskID    string
	Task      Task
	Package   Package
	Action    string
	RequestID string
	StartedAt time.Time
	Response  Response
}

// RemoteCompletionStateInput contains the values used to build a
// RemoteCompletionState.
type RemoteCompletionStateInput[Task any, Package any, Response any] struct {
	TaskID    string
	Task      Task
	Package   Package
	Action    string
	RequestID string
	StartedAt time.Time
	Response  Response
}

// NewRemoteCompletionState builds a generic remote completion state.
func NewRemoteCompletionState[Task any, Package any, Response any](
	in RemoteCompletionStateInput[Task, Package, Response],
) RemoteCompletionState[Task, Package, Response] {
	return RemoteCompletionState[Task, Package, Response]{
		TaskID:    in.TaskID,
		Task:      in.Task,
		Package:   in.Package,
		Action:    in.Action,
		RequestID: in.RequestID,
		StartedAt: in.StartedAt,
		Response:  in.Response,
	}
}
