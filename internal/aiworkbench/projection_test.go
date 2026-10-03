package aiworkbench

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"task-processor/internal/product/enrichment"
)

func TestTaskProjectionUsesExactAgentAndReviewFacts(t *testing.T) {
	now := time.Now().UTC()
	task := BusinessTask{Scope: Scope{OrganizationID: "org-1", ActorID: "actor-1"},
		OperationID: "operation-1", ProductKey: "product-1", TargetPlatform: "shein", ExecutionRequestKey: "key-1"}
	run := agent.Record{State: agent.State{Scope: agent.Scope{OrganizationID: "org-1", ActorID: "actor-1"},
		Request: agent.Request{Key: "key-1", Binding: agent.Binding{ContextKind: "acquisition", ContextID: "operation-1", ProductKey: "product-1", TargetPlatform: "shein"}},
		Phase:   agent.Running, Deadline: now.Add(time.Minute)}}
	projected, err := ProjectTask(task, nil, "", now)
	require.NoError(t, err)
	require.Equal(t, TaskError, projected.State)
	require.Equal(t, "START_NOT_CLAIMED", projected.Reason)
	require.True(t, projected.CanStart)
	projected, err = ProjectTask(task, &run, "", now)
	require.NoError(t, err)
	require.Equal(t, TaskRunning, projected.State)
	projected, err = ProjectTask(task, &run, "", now.Add(2*time.Minute))
	require.NoError(t, err)
	require.Equal(t, TaskError, projected.State)
	require.Equal(t, "EXECUTION_OUTCOME_UNKNOWN", projected.Reason)
	require.False(t, projected.CanStart, "an expired RUNNING execution cannot be started again")
	require.True(t, projected.CanReconcile, "the existing Agent owner can finalize the expired RUNNING fact")
	projected, err = ProjectTask(task, &run, "pending", now)
	require.NoError(t, err)
	require.Equal(t, TaskWaitingConfirmation, projected.State)
	projected, err = ProjectTask(task, &run, "applied", now)
	require.NoError(t, err)
	require.Equal(t, TaskCompleted, projected.State)
	run.State.Scope.ActorID = "another-actor"
	_, err = ProjectTask(task, &run, "", now)
	require.ErrorIs(t, err, ErrUnavailable)
}

func TestTaskProjectionDoesNotOfferActionsForInvalidReviewOrExpiredInterruption(t *testing.T) {
	now := time.Now().UTC()
	task := BusinessTask{Scope: Scope{OrganizationID: "org-1", ActorID: "actor-1"},
		OperationID: "operation-1", ProductKey: "product-1", TargetPlatform: "shein", ExecutionRequestKey: "key-1"}
	run := agent.Record{State: agent.State{Scope: agent.Scope{OrganizationID: "org-1", ActorID: "actor-1"},
		Request: agent.Request{Key: "key-1", Binding: agent.Binding{ContextKind: "acquisition", ContextID: "operation-1", ProductKey: "product-1", TargetPlatform: "shein"}},
		Phase:   agent.HumanReviewRequired, StopReason: agent.StopRepairLimit, Validation: &agent.Validation{Valid: false},
		Candidate: enrichment.Candidate{Changes: []enrichment.FieldChange{{Field: "title", Value: "candidate"}}}}}
	invalid, err := ProjectTask(task, &run, "", now)
	require.NoError(t, err)
	require.Equal(t, TaskWaitingConfirmation, invalid.State)
	require.False(t, invalid.CanReview, "repair-limit output is not admissible to Product Review")
	run.State.StopReason = ""
	run.State.Validation.Valid = true
	reviewable, err := ProjectTask(task, &run, "", now)
	require.NoError(t, err)
	require.True(t, reviewable.CanReview)
	pending, err := ProjectTask(task, &run, "pending", now)
	require.NoError(t, err)
	require.False(t, pending.CanReview, "an existing Review must not offer a second submission")

	run.State.Phase = agent.Interrupted
	run.State.Deadline = now.Add(time.Minute)
	active, err := ProjectTask(task, &run, "", now)
	require.NoError(t, err)
	require.True(t, active.CanResume)
	expired, err := ProjectTask(task, &run, "", now.Add(time.Minute))
	require.NoError(t, err)
	require.False(t, expired.CanResume)
	require.Equal(t, "AGENT_INTERRUPTED_DEADLINE_EXPIRED", expired.Reason)
}
