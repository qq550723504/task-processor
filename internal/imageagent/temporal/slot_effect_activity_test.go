package temporal

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
)

func TestExecuteSlotActivityReplaysPublicationCompleteWithoutRepeatingEffects(t *testing.T) {
	repository, input := initializedSlotEffectActivity(t, "run-activity-replay")
	executor := &recordingRecoverableSlotExecutor{}
	activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotEffects: repository.(imageagent.SlotExternalEffectRepository), SlotExecutor: executor, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)

	first, err := activities.ExecuteSlot(context.Background(), input)
	require.NoError(t, err)
	second, err := activities.ExecuteSlot(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, first, second)
	require.Equal(t, 1, executor.GenerateCalls())
	require.Equal(t, 1, executor.PublishCalls())
	require.Len(t, first.Candidates, 1)
}

func TestExecuteSlotActivityProviderStartedWithoutOutputFailsUnknownWithoutRegeneration(t *testing.T) {
	repository, input := initializedSlotEffectActivity(t, "run-activity-unknown")
	effects := repository.(imageagent.SlotExternalEffectRepository)
	reservation := slotEffectReservationForActivity(input)
	_, claimed, err := effects.ReserveSlotExternalEffect(context.Background(), reservation)
	require.NoError(t, err)
	require.True(t, claimed)
	executor := &recordingRecoverableSlotExecutor{}
	activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotEffects: effects, SlotExecutor: executor, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)

	_, err = activities.ExecuteSlot(context.Background(), input)
	var applicationError *sdktemporal.ApplicationError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, slotProviderOutcomeUnknownErrorType, applicationError.Type())
	require.True(t, applicationError.NonRetryable())
	require.Zero(t, executor.GenerateCalls())
	require.Zero(t, executor.PublishCalls())
}

func TestExecuteSlotActivityResumesGeneratedOutputAtDurablePublicationOnly(t *testing.T) {
	repository, input := initializedSlotEffectActivity(t, "run-activity-resume-publication")
	effects := repository.(imageagent.SlotExternalEffectRepository)
	reservation := slotEffectReservationForActivity(input)
	_, claimed, err := effects.ReserveSlotExternalEffect(context.Background(), reservation)
	require.NoError(t, err)
	require.True(t, claimed)
	_, err = effects.StoreSlotGeneratedOutput(context.Background(), reservation, activityGeneratedOutput())
	require.NoError(t, err)
	executor := &recordingRecoverableSlotExecutor{}
	activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotEffects: effects, SlotExecutor: executor, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)

	result, err := activities.ExecuteSlot(context.Background(), input)
	require.NoError(t, err)
	require.Zero(t, executor.GenerateCalls())
	require.Equal(t, 1, executor.PublishCalls())
	require.Equal(t, "candidate-1", result.Candidates[0].AssetID)
}

func TestExecuteSlotActivityConcurrentExactCallsHaveOneProviderAndOneDurableCandidate(t *testing.T) {
	repository, input := initializedSlotEffectActivity(t, "run-activity-concurrent")
	executor := &recordingRecoverableSlotExecutor{generateStarted: make(chan struct{}), releaseGenerate: make(chan struct{})}
	activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotEffects: repository.(imageagent.SlotExternalEffectRepository), SlotExecutor: executor, Publisher: &identityCheckingPublisher{t: t}})
	require.NoError(t, err)

	type outcome struct {
		result imageagent.SlotExecutionResult
		err    error
	}
	firstDone := make(chan outcome, 1)
	go func() {
		result, err := activities.ExecuteSlot(context.Background(), input)
		firstDone <- outcome{result: result, err: err}
	}()
	<-executor.generateStarted
	_, secondErr := activities.ExecuteSlot(context.Background(), input)
	var applicationError *sdktemporal.ApplicationError
	require.ErrorAs(t, secondErr, &applicationError)
	require.Equal(t, slotProviderOutcomeUnknownErrorType, applicationError.Type())
	close(executor.releaseGenerate)
	first := <-firstDone
	require.NoError(t, first.err)
	require.Equal(t, 1, executor.GenerateCalls())
	require.Equal(t, 1, executor.PublishCalls())

	stored, err := repository.(imageagent.SlotExternalEffectRepository).GetSlotExternalEffect(context.Background(), slotEffectReservationForActivity(input).Identity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotExternalEffectPublicationComplete, stored.Phase)
	require.Equal(t, first.result, stored.Published)
	require.Len(t, stored.Published.Candidates, 1)
}

func TestSlotWorkflowPreservesUnknownProviderOutcomeBlockerCode(t *testing.T) {
	err := sdktemporal.NewNonRetryableApplicationError("unknown", slotProviderOutcomeUnknownErrorType, nil)
	require.Equal(t, "slot_provider_outcome_unknown", slotExecutionErrorCode(err))
	for _, code := range []string{slotProviderOutcomeUnknownCode, slotStagingOutcomeUnknownCode, slotPublicationOutcomeUnknownCode} {
		err = sdktemporal.NewNonRetryableApplicationError("unknown", code, nil)
		require.Equal(t, code, slotExecutionErrorCode(err))
	}
	require.Equal(t, "slot_execution_failed", slotExecutionErrorCode(errors.New("transport failed")))
}

func TestImageSlotWorkflowRejectsInvalidMainCandidateCount(t *testing.T) {
	for _, count := range []int{0, 2} {
		t.Run(fmt.Sprintf("count-%d", count), func(t *testing.T) {
			var suite testsuite.WorkflowTestSuite
			env := suite.NewTestWorkflowEnvironment()
			env.RegisterWorkflow(ImageSlotWorkflow)
			env.RegisterActivityWithOptions(func(_ context.Context, input ExecuteSlotActivityInput) (imageagent.SlotExecutionResult, error) {
				candidates := make([]imageagent.AssetCandidate, count)
				for index := range candidates {
					candidates[index] = imageagent.AssetCandidate{AssetID: fmt.Sprintf("candidate-%d", index), URL: fmt.Sprintf("https://generated.example/%d.png", index)}
				}
				return imageagent.SlotExecutionResult{SlotID: input.Slot.ID, Attempt: input.Attempt, Candidates: candidates}, nil
			}, sdkactivity.RegisterOptions{Name: activityExecuteSlot})
			input := SlotWorkflowInput{RunID: "run-main-count", Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}, PlanRevision: 1, Slot: imageagent.Slot{ID: "main-1", Role: imageagent.SlotRoleMain, IdempotencyKey: "main-key", SourceAssetIDs: []string{"source-1"}}, Attempt: 1}

			env.ExecuteWorkflow(ImageSlotWorkflow, input)
			require.NoError(t, env.GetWorkflowError())
			var result SlotWorkflowResult
			require.NoError(t, env.GetWorkflowResult(&result))
			require.Equal(t, imageagent.SlotStatusBlocked, result.Status)
			require.Equal(t, invalidMainCandidateCountCode, result.ErrorCode)
		})
	}
}

func TestPersistSlotResultCannotAcceptInvalidMainCandidateCount(t *testing.T) {
	for _, count := range []int{0, 2} {
		t.Run(fmt.Sprintf("count-%d", count), func(t *testing.T) {
			repository := store.NewMemoryRepository()
			identity := imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}
			plan := imageagent.Plan{Revision: 1, IdempotencyKey: "plan-main-count", SourceAssetIDs: []string{"source-1"}, CreatedBy: identity.UserID, Slots: []imageagent.Slot{{ID: "main-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "main-key"}}}
			run := imageagent.Run{ID: "run-persist-main-count", BusinessTaskID: "task-main-count", TenantID: identity.TenantID, UserID: identity.UserID, Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-main-count", Status: imageagent.RunStatusExecuting, ActivePlanRevision: 1, Version: 1}
			initializeActivityProjection(t, repository, run, plan)
			activities, err := NewActivities(ActivityDependencies{Repository: repository, SlotExecutor: &recordingRecoverableSlotExecutor{}, Publisher: &identityCheckingPublisher{t: t}})
			require.NoError(t, err)
			candidates := make([]imageagent.AssetCandidate, count)
			for index := range candidates {
				candidates[index] = imageagent.AssetCandidate{AssetID: fmt.Sprintf("candidate-%d", index), URL: fmt.Sprintf("https://generated.example/%d.png", index)}
			}

			err = activities.PersistSlotResult(context.Background(), PersistSlotResultActivityInput{
				RunID: run.ID, Identity: identity, PlanRevision: 1, AttemptKey: "main-key:plan:1:attempt:1",
				Result: SlotWorkflowResult{Execution: imageagent.SlotExecutionResult{SlotID: "main-1", Attempt: 1, Candidates: candidates}, Status: imageagent.SlotStatusAccepted},
			})
			require.NoError(t, err)
			projection, err := repository.GetProjection(context.Background(), imageagent.ScopeForRun(run))
			require.NoError(t, err)
			require.Equal(t, imageagent.SlotStatusBlocked, projection.Slots[0].Slot.Status)
			require.Equal(t, invalidMainCandidateCountCode, projection.Slots[0].ErrorCode)
			require.Empty(t, projection.Slots[0].Candidates)
		})
	}
}

func initializedSlotEffectActivity(t *testing.T, runID string) (imageagent.Repository, ExecuteSlotActivityInput) {
	t.Helper()
	repository := store.NewMemoryRepository()
	run := imageagent.Run{ID: runID, BusinessTaskID: "task-" + runID, TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-" + runID, Status: imageagent.RunStatusPlanning, CurrentNode: "plan", Version: 1, ActivePlanRevision: 1, MaxConcurrentSlots: 1}
	plan := imageagent.Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, CreatedBy: "user-a", Slots: []imageagent.Slot{{ID: "slot-1", Role: imageagent.SlotRoleScene, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1"}}}
	catalog, err := imageagent.NormalizeAssetCatalog(imageagent.AssetCatalog{Assets: []imageagent.AuthorizedAsset{{ID: "source-1", Type: imageagent.AuthorizedAssetSource, URL: "https://source.example/source.png"}}})
	require.NoError(t, err)
	_, err = repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{Scope: imageagent.ScopeForRun(run), Run: run, Plan: plan, Catalog: catalog, Snapshot: imageagent.RunProjection{Run: run, Plan: plan}, CommitID: "start:" + runID, EventType: "run.initialized", EventPayload: []byte(`{}`)})
	require.NoError(t, err)
	return repository, ExecuteSlotActivityInput{RunID: runID, Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}, PlanRevision: 1, Slot: plan.Slots[0], Attempt: 1, IdempotencyKey: "slot-key-1:plan:1:attempt:1", AssetCatalog: catalog}
}

func slotEffectReservationForActivity(input ExecuteSlotActivityInput) imageagent.SlotExternalEffectReservation {
	return imageagent.SlotExternalEffectReservation{Identity: imageagent.SlotExternalEffectIdentity{RunScope: imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID}, PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt}, IdempotencyKey: input.IdempotencyKey, InputFingerprint: imageagent.SlotExecutionFingerprint(imageagent.SlotExecutionInput{RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID, PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt, IdempotencyKey: input.IdempotencyKey, AssetCatalog: input.AssetCatalog})}
}

func activityGeneratedOutput() imageagent.SlotGeneratedOutput {
	return imageagent.SlotGeneratedOutput{SlotID: "slot-1", Attempt: 1, SourceAssetID: "source-1", Assets: []imageagent.GeneratedAsset{{URL: "C:/generated/scene.png", Width: 1200, Height: 1200, Metadata: map[string]string{"local_path": "C:/generated/scene.png"}}}}
}

type recordingRecoverableSlotExecutor struct {
	mu              sync.Mutex
	generateCalls   int
	publishCalls    int
	generateStarted chan struct{}
	releaseGenerate chan struct{}
}

func (e *recordingRecoverableSlotExecutor) GenerateSlot(context.Context, imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	e.mu.Lock()
	e.generateCalls++
	started, release := e.generateStarted, e.releaseGenerate
	e.mu.Unlock()
	if started != nil {
		close(started)
		<-release
	}
	return activityGeneratedOutput(), nil
}

func (e *recordingRecoverableSlotExecutor) PublishSlot(_ context.Context, _ imageagent.SlotExecutionInput, generated imageagent.SlotGeneratedOutput) (imageagent.SlotExecutionResult, error) {
	e.mu.Lock()
	e.publishCalls++
	e.mu.Unlock()
	if len(generated.Assets) != 1 {
		return imageagent.SlotExecutionResult{}, errors.New("generated output missing")
	}
	return imageagent.SlotExecutionResult{SlotID: generated.SlotID, Attempt: generated.Attempt, Candidates: []imageagent.AssetCandidate{{AssetID: "candidate-1", URL: "https://cdn.example.test/scene.png", SourceAssetID: generated.SourceAssetID}}}, nil
}

func (e *recordingRecoverableSlotExecutor) GenerateCalls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.generateCalls
}

func (e *recordingRecoverableSlotExecutor) PublishCalls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.publishCalls
}
