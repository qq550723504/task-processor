package agentconfigpersistence

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"task-processor/internal/agent"
	"task-processor/internal/agentconfig"
)

func imageStartFixture(t *testing.T, store *Store) agentconfig.ImageStartCommand {
	t.Helper()
	scope := agent.Scope{OrganizationID: "image-org", ActorID: "image-actor"}
	enable := command(scope, "enable", 0)
	enable.AgentID, enable.Absent = agentconfig.ImageAgentID, true
	_, err := store.Execute(context.Background(), enable)
	require.NoError(t, err)
	parameters := imageTemplateFixture()
	create := command(scope, "create-template", 0)
	create.AgentID = agentconfig.ImageAgentID
	create.Input = agentconfig.TemplateInput{Name: "图片模板", TargetPlatform: "product", Image: &parameters}
	receipt, err := store.Execute(context.Background(), create)
	require.NoError(t, err)
	return agentconfig.ImageStartCommand{Scope: scope, MemberID: "image-member", RequestKey: uuid.NewString(), ContextID: "source-operation", RunID: uuid.NewString(), TargetPlatform: "product", SourceDigest: digest([]byte("source-v1")), InputDigest: digest([]byte("input-v1")), Template: &agentconfig.TemplateRef{TemplateID: receipt.TemplateID, Revision: "1"}, HardLimits: agentconfig.ImageRunLimits{Images: 32, Points: 1000, ElapsedSeconds: 300}}
}

func TestImageAdmissionIsWriteOnceAndOrdersDisable(t *testing.T) {
	_, store, _ := fixture(t)
	ctx := context.Background()
	start := imageStartFixture(t, store)
	snapshot, err := store.PrepareImageConfiguration(ctx, start)
	require.NoError(t, err)
	admission := agentconfig.ImageRunAdmissionCommand{Scope: start.Scope, Snapshot: snapshot.Ref(), MemberID: start.MemberID, RunID: start.RunID, ConfirmActionID: uuid.NewString(), SourceDigest: start.SourceDigest, InputDigest: start.InputDigest, PlanDigest: digest([]byte("plan")), QuoteDigest: digest([]byte("quote")), Limits: agentconfig.ImageRunLimits{Images: 2, Points: 30, ElapsedSeconds: 180}}
	original, err := store.AdmitImageRun(ctx, admission, start.HardLimits)
	require.NoError(t, err)
	require.Equal(t, 180*time.Second, original.Deadline.Sub(original.AdmittedAt))
	disable := command(start.Scope, "disable", 1)
	disable.AgentID = agentconfig.ImageAgentID
	_, err = store.Execute(ctx, disable)
	require.NoError(t, err)
	replay, err := store.AdmitImageRun(ctx, admission, agentconfig.ImageRunLimits{Images: 1, Points: 1, ElapsedSeconds: 1})
	require.NoError(t, err, "original admitted work retains its budget and deadline after disable or ceiling drift")
	require.Equal(t, original, replay)
	loaded, err := store.LoadImageConfiguration(ctx, start.Scope, snapshot.Ref())
	require.NoError(t, err)
	require.Equal(t, snapshot, loaded, "admission must not rewrite immutable configuration")
	changed := admission
	changed.ConfirmActionID = uuid.NewString()
	_, err = store.AdmitImageRun(ctx, changed, start.HardLimits)
	require.ErrorIs(t, err, agentconfig.ErrConflict, "a new action must not refresh an already admitted Run")
	foreign := admission
	foreign.Scope.OrganizationID = "foreign-org"
	_, err = store.AdmitImageRun(ctx, foreign, start.HardLimits)
	require.Error(t, err)
}

func TestImageDisableBeforeAdmissionRejectsAndRetainsPreparedSnapshot(t *testing.T) {
	_, store, _ := fixture(t)
	ctx := context.Background()
	start := imageStartFixture(t, store)
	snapshot, err := store.PrepareImageConfiguration(ctx, start)
	require.NoError(t, err)
	disable := command(start.Scope, "disable", 1)
	disable.AgentID = agentconfig.ImageAgentID
	_, err = store.Execute(ctx, disable)
	require.NoError(t, err)
	admission := agentconfig.ImageRunAdmissionCommand{Scope: start.Scope, Snapshot: snapshot.Ref(), MemberID: start.MemberID, RunID: start.RunID, ConfirmActionID: uuid.NewString(), SourceDigest: start.SourceDigest, InputDigest: start.InputDigest, PlanDigest: digest([]byte("plan")), QuoteDigest: digest([]byte("quote")), Limits: agentconfig.ImageRunLimits{Images: 2, Points: 30, ElapsedSeconds: 180}}
	_, err = store.AdmitImageRun(ctx, admission, start.HardLimits)
	require.ErrorIs(t, err, agentconfig.ErrNotEnabled)
	replay, err := store.PrepareImageConfiguration(ctx, start)
	require.NoError(t, err, "retrying preparation reads the original snapshot without creating permission to dispatch")
	require.Equal(t, snapshot, replay)
	start.RequestKey = uuid.NewString()
	_, err = store.PrepareImageConfiguration(ctx, start)
	require.ErrorIs(t, err, agentconfig.ErrNotEnabled)
}

func TestConcurrentImageAdmissionAndDisableShareOneCommitBoundary(t *testing.T) {
	_, store, _ := fixture(t)
	ctx := context.Background()
	start := imageStartFixture(t, store)
	snapshot, err := store.PrepareImageConfiguration(ctx, start)
	require.NoError(t, err)
	admission := agentconfig.ImageRunAdmissionCommand{Scope: start.Scope, Snapshot: snapshot.Ref(), MemberID: start.MemberID, RunID: start.RunID, ConfirmActionID: uuid.NewString(), SourceDigest: start.SourceDigest, InputDigest: start.InputDigest, PlanDigest: digest([]byte("plan")), QuoteDigest: digest([]byte("quote")), Limits: agentconfig.ImageRunLimits{Images: 2, Points: 30, ElapsedSeconds: 180}}
	disable := command(start.Scope, "disable", 1)
	disable.AgentID = agentconfig.ImageAgentID
	const count = 12
	results := make([]agentconfig.ImageRunAdmissionReceipt, count)
	errors := make([]error, count)
	gate := make(chan struct{})
	var group sync.WaitGroup
	for i := 0; i < count; i++ {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			<-gate
			results[index], errors[index] = store.AdmitImageRun(ctx, admission, start.HardLimits)
		}(i)
	}
	var disableErr error
	group.Add(1)
	go func() { defer group.Done(); <-gate; _, disableErr = store.Execute(ctx, disable) }()
	close(gate)
	group.Wait()
	require.NoError(t, disableErr)
	var admitted *agentconfig.ImageRunAdmissionReceipt
	for i := range results {
		if errors[i] == nil {
			admitted = &results[i]
			break
		}
	}
	if admitted == nil {
		for _, err := range errors {
			require.ErrorIs(t, err, agentconfig.ErrNotEnabled)
		}
		_, err = store.ReadImageRunAdmission(ctx, start.Scope, snapshot.Ref())
		require.ErrorIs(t, err, agentconfig.ErrNotFound)
		return
	}
	// Every contender observes the same original receipt even if disable won
	// the lock after the first admission, never a new budget/deadline.
	for i := range results {
		require.NoError(t, errors[i])
		require.Equal(t, *admitted, results[i])
	}
	read, err := store.ReadImageRunAdmission(ctx, start.Scope, snapshot.Ref())
	require.NoError(t, err)
	require.Equal(t, *admitted, read)
}
