package podapp

import (
	"context"
	"task-processor/internal/listing/submission"
	"task-processor/internal/product/collection"
	"task-processor/internal/product/pod"
	"testing"

	"github.com/stretchr/testify/require"
)

type startAuthorization struct {
	scope   collection.Scope
	revoked bool
}

func (a startAuthorization) Authorize(context.Context, string) (collection.Scope, error) {
	if a.revoked {
		return collection.Scope{}, pod.ErrForbidden
	}
	return a.scope, nil
}

type startIntents struct{ found bool }

func (i startIntents) ReadIntent(context.Context, submission.ExecutionScope, string) (submission.ExecutionAttempt, error) {
	if !i.found {
		return submission.ExecutionAttempt{}, submission.ErrExecutionNotFound
	}
	return submission.ExecutionAttempt{Status: submission.ExecutionOutcomeUnknown}, nil
}

type startRecorder struct {
	inputs []Execution
	err    error
}

func (s *startRecorder) Ensure(_ context.Context, in Execution) error {
	s.inputs = append(s.inputs, in)
	return s.err
}

func TestClosedOrUnavailableWorkflowIsNotReportedAsQueued(t *testing.T) {
	o := pod.Operation{Plan: pod.Plan{OperationID: "12752596-6056-4316-9f2f-380c97df9675", Scope: collection.Scope{"org", "actor", "member"}}}
	starter := &startRecorder{err: pod.ErrUnknown}
	s := Service{Authorization: startAuthorization{scope: o.Plan.Scope}, Intents: startIntents{}, Starter: starter}
	require.Equal(t, "UNKNOWN", s.project(context.Background(), o).State)
	require.Len(t, starter.inputs, 1)
}
func TestQueryRecoveryEnsuresOnlyUnstartedOriginalWithLiveDesignGrant(t *testing.T) {
	o := pod.Operation{Plan: pod.Plan{OperationID: "12752596-6056-4316-9f2f-380c97df9675", Scope: collection.Scope{"org", "actor", "member"}}}
	for _, tc := range []struct {
		name                    string
		sent, revoked, finished bool
		count                   int
	}{{"not started", false, false, false, 1}, {"unknown is read only", true, false, false, 0}, {"revoked", false, true, false, 0}, {"finished", false, false, true, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			input := o
			if tc.finished {
				input.Finished = &pod.FinishedReference{ID: "70"}
			}
			starter := &startRecorder{}
			s := Service{Authorization: startAuthorization{o.Plan.Scope, tc.revoked}, Intents: startIntents{tc.sent}, Starter: starter}
			s.ensureUnstarted(context.Background(), input)
			require.Len(t, starter.inputs, tc.count)
			if tc.count == 1 {
				require.Equal(t, Execution{o.Plan.Scope, o.Plan.OperationID}, starter.inputs[0])
			}
		})
	}
}
