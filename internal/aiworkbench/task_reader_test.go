package aiworkbench

import (
	"context"
	"encoding/json"
	"errors"
	"task-processor/internal/agent"
	"testing"
	"time"
)

func TestAuthorizedTaskReaderChecksIdentityAndExactRunBeforeReview(t *testing.T) {
	scope := Scope{OrganizationID: "org-a", ActorID: "actor-a"}
	req := agent.Request{Key: "key-a", Binding: agent.Binding{ContextKind: "acquisition", ContextID: "operation-a", ProductKey: "product-a", TargetPlatform: "shein"}}
	raw, _ := json.Marshal(req)
	task := BusinessTask{Scope: scope, OperationID: req.Binding.ContextID, ProductKey: req.Binding.ProductKey, TargetPlatform: req.Binding.TargetPlatform, ExecutionRequestKey: req.Key, ExecutionRequest: raw}
	lookups, reviews := 0, 0
	reader := AuthorizedTaskProjectionReader{Authorize: func(context.Context, Scope) error { return ErrNotFound }, LookupRun: func(context.Context, agent.Scope, agent.Binding, string) (agent.Record, bool, error) {
		lookups++
		return agent.Record{State: agent.State{Scope: agent.Scope{OrganizationID: "org-a", ActorID: "other-actor"}, Request: req, Phase: agent.Running, Deadline: time.Now().Add(time.Hour)}}, true, nil
	}, ReadReview: func(context.Context, string) (TaskReviewState, bool, error) {
		reviews++
		return TaskReviewState{}, false, nil
	}}
	if _, err := reader.Read(context.Background(), scope, task); !errors.Is(err, ErrNotFound) || lookups != 0 || reviews != 0 {
		t.Fatal("owner facts read before authorization")
	}
	reader.Authorize = func(context.Context, Scope) error { return nil }
	if _, err := reader.Read(context.Background(), scope, task); !errors.Is(err, ErrUnavailable) || reviews != 0 {
		t.Fatalf("review read before exact run check: %v reads=%d", err, reviews)
	}
}
