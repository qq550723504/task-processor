package aiworkbench

import (
	"context"
	"encoding/json"
	"task-processor/internal/agent"
	"time"
)

// TaskReviewState contains only metadata the original Review owner authorizes
// for this task reader. ID is empty when only task-state access is granted.
type TaskReviewState struct {
	State, ID  string
	Revision   uint64
	OccurredAt *time.Time
}
type AuthorizedTaskProjectionReader struct {
	Authorize  func(context.Context, Scope) error
	LookupRun  func(context.Context, agent.Scope, agent.Binding, string) (agent.Record, bool, error)
	ReadReview func(context.Context, string) (TaskReviewState, bool, error)
	Now        func() time.Time
}
type AuthorizedTaskProjection struct {
	TaskProjection
	Run    *agent.Record
	Review TaskReviewState
}

func (r AuthorizedTaskProjectionReader) Read(ctx context.Context, scope Scope, task BusinessTask) (AuthorizedTaskProjection, error) {
	if task.Scope != scope || r.Authorize == nil || r.LookupRun == nil || r.ReadReview == nil {
		return AuthorizedTaskProjection{}, ErrNotFound
	}
	if e := r.Authorize(ctx, scope); e != nil {
		return AuthorizedTaskProjection{}, e
	}
	var request agent.Request
	if json.Unmarshal(task.ExecutionRequest, &request) != nil || request.Key != task.ExecutionRequestKey {
		return AuthorizedTaskProjection{}, ErrUnavailable
	}
	run, found, e := r.LookupRun(ctx, agent.Scope{OrganizationID: scope.OrganizationID, ActorID: scope.ActorID}, request.Binding, request.Key)
	if e != nil {
		return AuthorizedTaskProjection{}, ErrUnavailable
	}
	out := AuthorizedTaskProjection{}
	if found {
		if run.State.Scope.OrganizationID != scope.OrganizationID || run.State.Scope.ActorID != scope.ActorID || run.State.Request != request {
			return out, ErrUnavailable
		}
		out.Run = &run
		out.Review, _, e = r.ReadReview(ctx, run.State.RunID)
		if e != nil {
			return out, ErrUnavailable
		}
	}
	now := time.Now()
	if r.Now != nil {
		now = r.Now()
	}
	out.TaskProjection, e = ProjectTask(task, out.Run, out.Review.State, now)
	return out, e
}
