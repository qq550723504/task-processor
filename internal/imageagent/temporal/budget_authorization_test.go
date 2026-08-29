package temporal

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	sdkactivity "go.temporal.io/sdk/activity"
	sdkconverter "go.temporal.io/sdk/converter"
	sdktemporal "go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/store"
)

func TestExecuteSlotV3BudgetQuoteReserveGenerateAndSettle(t *testing.T) {
	repository, input, policy := initializedBudgetedV3Activity(t, "run-budget-success", 1)
	generated := generatedV3Output(input, writeTinyPNG(t))
	quote := budgetActivityQuote("quote-success")
	generated.UsageReceipt = imageagent.SlotUsageReceipt{Actual: quote.Maximum, CostBasis: imageagent.UsageCostReservedUpperBound}
	executor := &budgetedRecordingExecutor{recordingStagedExecutor: &recordingStagedExecutor{generated: generated}, quote: quote}
	artifacts := &recordingArtifactStore{}
	activities := newBudgetV3Activities(t, repository, executor, artifacts)
	input.BudgetAuthorization, input.BudgetPolicy = true, policy

	result, err := activities.ExecuteSlotV3(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	require.Equal(t, 1, executor.QuoteCalls())
	require.Equal(t, 1, executor.GenerateCalls())
	projection, err := repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID})
	require.NoError(t, err)
	require.Equal(t, 1, projection.Run.Usage.Images)
	require.Equal(t, 1, projection.Run.Usage.AgentSteps)
}

func TestExecuteSlotV3ReusesPersistedQuoteAcrossProviderConfigurationChange(t *testing.T) {
	repository, input, policy := initializedBudgetedV3Activity(t, "run-budget-persisted-quote", 1)
	effects := repository.(imageagent.SlotExternalEffectV3Repository)
	persistedQuote := budgetActivityQuote("quote-before-rollout")
	reservation := slotEffectReservationV3(slotExecutionInputV3(input))
	reservation.Policy = policy
	reservation.Quote = persistedQuote
	_, claimed, err := effects.ReserveSlotProviderV3(context.Background(), reservation)
	require.NoError(t, err)
	require.True(t, claimed)
	_, err = effects.SettleSlotProviderV3(context.Background(), reservation, imageagent.SlotUsageReceipt{
		Actual: persistedQuote.Maximum, CostBasis: imageagent.UsageCostReservedUpperBound,
	})
	require.NoError(t, err)
	manifest := v3StagingManifest(input, tinyPNGBytes(t))
	prepared, err := effects.PrepareSlotStagingV3(context.Background(), reservation, manifest)
	require.NoError(t, err)
	_, err = effects.CommitSlotStagedV3(context.Background(), reservation, prepared.StagingManifestFingerprint)
	require.NoError(t, err)

	executor := &budgetedRecordingExecutor{
		recordingStagedExecutor: &recordingStagedExecutor{},
		quote:                   budgetActivityQuote("quote-after-rollout"),
	}
	activities := newBudgetV3Activities(t, repository, executor, &recordingArtifactStore{})
	input.BudgetAuthorization, input.BudgetPolicy = true, policy
	input.DeadlineAt = time.Now().UTC().Add(-time.Minute)

	result, err := activities.ExecuteSlotV3(context.Background(), input)

	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
	require.Zero(t, executor.QuoteCalls(), "recovery must not ask the current router for a new quote")
	require.Zero(t, executor.GenerateCalls(), "recovery must not dispatch the provider again")
}

func TestExecuteSlotV3AccountsProviderUsageWhenOnlyElapsedLimitIsEnabled(t *testing.T) {
	budget := imageagent.Budget{MaxElapsed: time.Hour, EnabledLimits: imageagent.BudgetLimitElapsed}
	repository, input, policy := initializedBudgetedV3ActivityWithBudget(t, "run-budget-elapsed-only", budget)
	generated := generatedV3Output(input, writeTinyPNG(t))
	quote := budgetActivityQuote("quote-elapsed-only")
	generated.UsageReceipt = imageagent.SlotUsageReceipt{Actual: quote.Maximum, CostBasis: imageagent.UsageCostReservedUpperBound}
	executor := &budgetedRecordingExecutor{recordingStagedExecutor: &recordingStagedExecutor{generated: generated}, quote: quote}
	activities := newBudgetV3Activities(t, repository, executor, &recordingArtifactStore{})
	input.BudgetAuthorization, input.BudgetPolicy = true, policy
	input.DeadlineAt = time.Now().UTC().Add(time.Hour)

	_, err := activities.ExecuteSlotV3(context.Background(), input)

	require.NoError(t, err)
	require.Equal(t, 1, executor.QuoteCalls())
	projection, err := repository.GetProjection(context.Background(), imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID})
	require.NoError(t, err)
	require.Equal(t, 1, projection.Run.Usage.Images)
	require.Equal(t, 1, projection.Run.Usage.AgentSteps)
}

func TestExecuteSlotV3PassesAbsoluteRunDeadlineToProvider(t *testing.T) {
	budget := imageagent.Budget{MaxElapsed: time.Hour, EnabledLimits: imageagent.BudgetLimitElapsed}
	repository, input, policy := initializedBudgetedV3ActivityWithBudget(t, "run-budget-provider-deadline", budget)
	base := &budgetedRecordingExecutor{recordingStagedExecutor: &recordingStagedExecutor{}, quote: budgetActivityQuote("quote-provider-deadline")}
	executor := &deadlineBudgetedExecutor{budgetedRecordingExecutor: base}
	activities := newBudgetV3Activities(t, repository, executor, &recordingArtifactStore{})
	input.BudgetAuthorization, input.BudgetPolicy = true, policy
	input.DeadlineAt = time.Now().UTC().Add(50 * time.Millisecond)
	parent, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	_, err := activities.ExecuteSlotV3(parent, input)

	requireV3ApplicationErrorType(t, err, imageagent.SlotProviderOutcomeUnknownCode)
	require.WithinDuration(t, input.DeadlineAt, executor.ProviderDeadline(), 10*time.Millisecond)
}

func TestExecuteSlotV3BudgetDeniesBeforeProvider(t *testing.T) {
	repository, input, policy := initializedBudgetedV3Activity(t, "run-budget-zero", 0)
	executor := &budgetedRecordingExecutor{recordingStagedExecutor: &recordingStagedExecutor{}, quote: budgetActivityQuote("quote-denied")}
	activities := newBudgetV3Activities(t, repository, executor, &recordingArtifactStore{})
	input.BudgetAuthorization, input.BudgetPolicy = true, policy

	_, err := activities.ExecuteSlotV3(context.Background(), input)
	requireV3ApplicationErrorType(t, err, imageagent.BudgetExhaustedCode)
	require.Zero(t, executor.GenerateCalls())
}

func TestExecuteSlotV3BudgetRetainsReservationForAmbiguousProvider(t *testing.T) {
	repository, input, policy := initializedBudgetedV3Activity(t, "run-budget-unknown", 1)
	executor := &budgetedRecordingExecutor{
		recordingStagedExecutor: &recordingStagedExecutor{}, quote: budgetActivityQuote("quote-unknown"),
		generateErr: &imageagent.ProviderDispatchError{State: imageagent.ProviderDispatchedUnknown, Err: errors.New("connection lost after dispatch")},
	}
	activities := newBudgetV3Activities(t, repository, executor, &recordingArtifactStore{})
	input.BudgetAuthorization, input.BudgetPolicy = true, policy

	_, err := activities.ExecuteSlotV3(context.Background(), input)
	requireV3ApplicationErrorType(t, err, imageagent.SlotProviderOutcomeUnknownCode)
	effect, getErr := repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(context.Background(), slotEffectReservationV3(slotExecutionInputV3(input)).Identity)
	require.NoError(t, getErr)
	require.Equal(t, imageagent.SlotBudgetUnknown, effect.BudgetStatus)
	projection, getErr := repository.GetProjection(context.Background(), effect.Identity.RunScope)
	require.NoError(t, getErr)
	require.Zero(t, projection.Run.Usage.Images, "unknown usage is reserved, not falsely committed")
}

func TestExecuteSlotV3MarksProvenPreDispatchFailureRetryable(t *testing.T) {
	repository, input, policy := initializedBudgetedV3Activity(t, "run-budget-pre-dispatch", 1)
	quote := budgetActivityQuote("quote-pre-dispatch")
	generated := generatedV3Output(input, writeTinyPNG(t))
	generated.UsageReceipt = imageagent.SlotUsageReceipt{Actual: quote.Maximum, CostBasis: imageagent.UsageCostReservedUpperBound}
	executor := &budgetedRecordingExecutor{
		recordingStagedExecutor: &recordingStagedExecutor{generated: generated}, quote: quote,
		generateErr: &imageagent.ProviderDispatchError{State: imageagent.ProviderRejectedBeforeEffect, Err: errors.New("rate limited before request")},
	}
	activities := newBudgetV3Activities(t, repository, executor, &recordingArtifactStore{})
	input.BudgetAuthorization, input.BudgetPolicy = true, policy

	_, err := activities.ExecuteSlotV3(context.Background(), input)

	var applicationError *sdktemporal.ApplicationError
	require.ErrorAs(t, err, &applicationError)
	require.Equal(t, "slot_provider_not_dispatched", applicationError.Type())
	require.False(t, applicationError.NonRetryable())
	effect, getErr := repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(context.Background(), slotEffectReservationV3(slotExecutionInputV3(input)).Identity)
	require.NoError(t, getErr)
	require.Equal(t, imageagent.SlotEffectV3Phase("provider_not_dispatched"), effect.Phase)

	executor.generateErr = nil
	result, err := activities.ExecuteSlotV3(context.Background(), input)
	require.NoError(t, err)
	require.Len(t, result.Candidates, 1)
}

func TestImageSlotWorkflowV3BudgetDeadlineBlocksWithoutActivity(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ImageSlotWorkflowV3)
	activityCalls := 0
	env.RegisterActivityWithOptions(func(context.Context, ExecuteSlotV3ActivityInput) (imageagent.SlotEffectV3PublishedResult, error) {
		activityCalls++
		return imageagent.SlotEffectV3PublishedResult{}, nil
	}, sdkactivity.RegisterOptions{Name: activityExecuteSlotV3})
	input := SlotWorkflowV3Input{RunID: "run-deadline", Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"}, PlanRevision: 1, Slot: imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleScene}, Attempt: 1, ExecuteActivityName: activityExecuteSlotV3, BudgetAuthorization: true, DeadlineAt: env.Now().Add(-time.Second)}

	env.ExecuteWorkflow(ImageSlotWorkflowV3, input)
	require.NoError(t, env.GetWorkflowError())
	var result SlotWorkflowV3Result
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, imageagent.BudgetElapsedCode, result.ErrorCode)
	require.Zero(t, activityCalls)
}

func TestImageSlotWorkflowV3ReservesFinalizationGraceAfterBudgetDeadline(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ImageSlotWorkflowV3)
	var observedStartToCloseTimeout time.Duration
	var observedHeartbeatTimeout time.Duration
	env.SetOnActivityStartedListener(func(info *sdkactivity.Info, _ context.Context, _ sdkconverter.EncodedValues) {
		observedStartToCloseTimeout = info.StartToCloseTimeout
		observedHeartbeatTimeout = info.HeartbeatTimeout
	})
	env.RegisterActivityWithOptions(func(context.Context, ExecuteSlotV3ActivityInput) (imageagent.SlotEffectV3PublishedResult, error) {
		return imageagent.SlotEffectV3PublishedResult{}, nil
	}, sdkactivity.RegisterOptions{Name: activityExecuteSlotV3})

	env.ExecuteWorkflow(ImageSlotWorkflowV3, SlotWorkflowV3Input{
		RunID: "run-finalization-grace", Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
		PlanRevision: 1, Slot: imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleScene}, Attempt: 1,
		ExecuteActivityName: activityExecuteSlotV3, BudgetAuthorization: true,
		DeadlineAt: env.Now().Add(2 * time.Minute), ExternalEffectFinalization: true,
	})

	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 3*time.Minute, observedStartToCloseTimeout)
	require.Equal(t, externalEffectHeartbeatTimeout, observedHeartbeatTimeout)
}

func TestImageSlotWorkflowV3UsesProviderWindowPlusGraceBeyondTenMinutes(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ImageSlotWorkflowV3)
	var observedStartToCloseTimeout time.Duration
	var observedHeartbeatTimeout time.Duration
	env.SetOnActivityStartedListener(func(info *sdkactivity.Info, _ context.Context, _ sdkconverter.EncodedValues) {
		observedStartToCloseTimeout = info.StartToCloseTimeout
		observedHeartbeatTimeout = info.HeartbeatTimeout
	})
	env.RegisterActivityWithOptions(func(context.Context, ExecuteSlotV3ActivityInput) (imageagent.SlotEffectV3PublishedResult, error) {
		return imageagent.SlotEffectV3PublishedResult{}, nil
	}, sdkactivity.RegisterOptions{Name: activityExecuteSlotV3})

	env.ExecuteWorkflow(ImageSlotWorkflowV3, SlotWorkflowV3Input{
		RunID: "run-provider-window-plus-grace", Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
		PlanRevision: 1, Slot: imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleScene}, Attempt: 1,
		ExecuteActivityName: activityExecuteSlotV3, BudgetAuthorization: true, ExternalEffectFinalization: true,
		DeadlineAt: env.Now().Add(20 * time.Minute),
	})

	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 21*time.Minute, observedStartToCloseTimeout)
	require.Equal(t, externalEffectHeartbeatTimeout, observedHeartbeatTimeout)
}

func TestImageSlotWorkflowV3UsesProviderWindowPlusGraceBeyondTenMinutesWithoutFinalizationWire(t *testing.T) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(ImageSlotWorkflowV3)
	var observedStartToCloseTimeout time.Duration
	env.SetOnActivityStartedListener(func(info *sdkactivity.Info, _ context.Context, _ sdkconverter.EncodedValues) {
		observedStartToCloseTimeout = info.StartToCloseTimeout
	})
	env.RegisterActivityWithOptions(func(context.Context, ExecuteSlotV3ActivityInput) (imageagent.SlotEffectV3PublishedResult, error) {
		return imageagent.SlotEffectV3PublishedResult{}, nil
	}, sdkactivity.RegisterOptions{Name: activityExecuteSlotV3})

	env.ExecuteWorkflow(ImageSlotWorkflowV3, SlotWorkflowV3Input{
		RunID: "run-provider-window-plus-grace-without-wire", Identity: imageagent.ExecutionIdentity{TenantID: "tenant-a", UserID: "user-a"},
		PlanRevision: 1, Slot: imageagent.Slot{ID: "slot-1", Role: imageagent.SlotRoleScene}, Attempt: 1,
		ExecuteActivityName: activityExecuteSlotV3, BudgetAuthorization: true,
		DeadlineAt: env.Now().Add(20 * time.Minute),
	})

	require.NoError(t, env.GetWorkflowError())
	require.Equal(t, 21*time.Minute, observedStartToCloseTimeout)
}

func TestImageSlotWorkflowV3SaturatesProviderWindowPlusGraceOnDurationOverflow(t *testing.T) {
	const maxDuration = time.Duration(1<<63 - 1)
	require.Equal(t, maxDuration, addFinalizationGrace(maxDuration-30*time.Second))
}

func TestImageSlotWorkflowV3VersionsCancellationDrainActivityOptions(t *testing.T) {
	for _, test := range []struct {
		name                       string
		externalEffectFinalization bool
		wantHeartbeatTimeout       time.Duration
		wantWaitForCancellation    bool
	}{
		{name: "legacy history", externalEffectFinalization: false},
		{name: "new history", externalEffectFinalization: true, wantHeartbeatTimeout: externalEffectHeartbeatTimeout, wantWaitForCancellation: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			options := slotWorkflowV3ActivityOptions(10*time.Minute, test.externalEffectFinalization)
			require.Equal(t, test.wantHeartbeatTimeout, options.HeartbeatTimeout)
			require.Equal(t, test.wantWaitForCancellation, options.WaitForCancellation)
		})
	}
}

func TestImageAgentWorkflowBudgetDeniesRepairBeforeChild(t *testing.T) {
	env := newWorkflowEnv(t)
	env.OnGetVersion(externalEffectFinalizationPatch, workflow.DefaultVersion, 1).Return(workflow.DefaultVersion)
	env.OnGetVersion(budgetAuthorizationPatch, workflow.DefaultVersion, 1).Return(workflow.Version(1))
	plan := sevenSlotPlan()
	for _, slot := range plan.Slots {
		slot := slot
		if slot.ID == "scene-2" {
			env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(imageagent.SlotExecutionResult{}, sdktemporal.NewNonRetryableApplicationError("provider rejected slot", "slot_rejected", nil)).Once()
			continue
		}
		env.OnActivity(activityExecuteSlot, mock.Anything, executeInputForSlot(slot.ID, 1)).Return(successfulSlotResult(slot.ID, 1), nil).Once()
	}
	env.OnActivity(activityPersistSlotResult, mock.Anything, mock.Anything).Return(nil)
	env.OnActivity(activityPersistRunState, mock.Anything, mock.Anything).Return(nil)
	retry := RetrySlotSignal{RunID: "run-1", PlanRevision: 1, SlotID: "scene-2", ActorID: "user-a", ActionID: "retry-over-budget"}
	var retryErr error
	env.RegisterDelayedCallback(func() {
		env.UpdateWorkflow(signalRetrySlot, "retry-over-budget-request", &testsuite.TestUpdateCallback{OnReject: func(err error) { retryErr = err }, OnAccept: func() {}, OnComplete: func(_ interface{}, err error) { retryErr = err }}, retry)
	}, time.Second)
	env.RegisterDelayedCallback(func() {
		env.SignalWorkflow(signalCancel, CancelSignal{RunID: "run-1", PlanRevision: 1, ActorID: "user-a", ActionID: "cancel-budget-test"})
	}, 2*time.Second)
	input := manualWorkflowInput(plan)
	input.WaitForCommands = true
	input.BudgetPolicy = imageagent.BudgetPolicy{RepairAttemptsPerSlot: imageagent.Limit{Enabled: true, Value: 0}}

	env.ExecuteWorkflow(ImageAgentWorkflow, input)
	require.NoError(t, env.GetWorkflowError())
	require.NoError(t, retryErr)
	env.AssertNotCalled(t, activityExecuteSlot, mock.Anything, executeInputForSlot("scene-2", 2))
}

func initializedBudgetedV3Activity(t *testing.T, runID string, maxImages int) (imageagent.Repository, ExecuteSlotV3ActivityInput, imageagent.BudgetPolicy) {
	return initializedBudgetedV3ActivityWithBudget(t, runID, imageagent.Budget{MaxImages: maxImages, EnabledLimits: imageagent.BudgetLimitImages})
}

func initializedBudgetedV3ActivityWithBudget(t *testing.T, runID string, budget imageagent.Budget) (imageagent.Repository, ExecuteSlotV3ActivityInput, imageagent.BudgetPolicy) {
	t.Helper()
	repository := store.NewMemoryRepository()
	run := imageagent.Run{ID: runID, BusinessTaskID: "task-" + runID, TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-" + runID, Status: imageagent.RunStatusExecuting, ActivePlanRevision: 1, Version: 1, Budget: budget, StartedAt: time.Now().UTC()}
	policy, err := run.Budget.Policy()
	require.NoError(t, err)
	plan := imageagent.Plan{Revision: 1, IdempotencyKey: "plan-key-1", SourceAssetIDs: []string{"source-1"}, CreatedBy: "user-a", Slots: []imageagent.Slot{{ID: "slot-1", Role: imageagent.SlotRoleMain, SourceAssetIDs: []string{"source-1"}, IdempotencyKey: "slot-key-1"}}}
	projection := initializeActivityProjection(t, repository, run, plan)
	return repository, ExecuteSlotV3ActivityInput{RunID: run.ID, Identity: imageagent.ExecutionIdentity{TenantID: run.TenantID, UserID: run.UserID, BusinessTaskID: run.BusinessTaskID}, PlanRevision: 1, Slot: plan.Slots[0], Attempt: 1, IdempotencyKey: "slot-key-1:plan:1:attempt:1", AssetCatalog: projection.AssetCatalog}, policy
}

func budgetActivityQuote(fingerprint string) imageagent.SlotUsageQuote {
	maximum := imageagent.UsageVector{Images: 1, AgentSteps: 1}
	return imageagent.SlotUsageQuote{Maximum: maximum, Operations: []imageagent.SlotUsageOperation{{Name: "render_scene", Fingerprint: "operation-" + fingerprint, Maximum: maximum, MaximumOutputs: 1}}, Fingerprint: fingerprint}
}

type budgetedRecordingExecutor struct {
	*recordingStagedExecutor
	mu          sync.Mutex
	quote       imageagent.SlotUsageQuote
	quoteCalls  int
	generateErr error
}

type deadlineBudgetedExecutor struct {
	*budgetedRecordingExecutor
	mu       sync.Mutex
	deadline time.Time
}

func (e *deadlineBudgetedExecutor) GenerateQuotedSlot(ctx context.Context, _ imageagent.SlotExecutionInput, _ imageagent.SlotUsageQuote) (imageagent.SlotGeneratedOutput, error) {
	deadline, _ := ctx.Deadline()
	e.mu.Lock()
	e.deadline = deadline
	e.mu.Unlock()
	<-ctx.Done()
	return imageagent.SlotGeneratedOutput{}, &imageagent.ProviderDispatchError{State: imageagent.ProviderDispatchedUnknown, Err: ctx.Err()}
}

func (e *deadlineBudgetedExecutor) ProviderDeadline() time.Time {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.deadline
}

func (e *budgetedRecordingExecutor) QuoteSlot(context.Context, imageagent.SlotExecutionInput, imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.quoteCalls++
	return e.quote, nil
}

func (e *budgetedRecordingExecutor) GenerateQuotedSlot(ctx context.Context, input imageagent.SlotExecutionInput, quote imageagent.SlotUsageQuote) (imageagent.SlotGeneratedOutput, error) {
	if e.generateErr != nil {
		e.recordingStagedExecutor.mu.Lock()
		e.recordingStagedExecutor.generateCalls++
		e.recordingStagedExecutor.mu.Unlock()
		return imageagent.SlotGeneratedOutput{}, e.generateErr
	}
	if quote.Fingerprint != e.quote.Fingerprint {
		return imageagent.SlotGeneratedOutput{}, imageagent.ErrRevisionConflict
	}
	return e.GenerateSlot(ctx, input)
}

func (e *budgetedRecordingExecutor) QuoteCalls() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.quoteCalls
}

type budgetedSlotExecutor interface {
	imageagent.SlotExecutor
	imageagent.BudgetedStagedSlotExecutor
}

func newBudgetV3Activities(t *testing.T, repository imageagent.Repository, executor budgetedSlotExecutor, artifacts DurableArtifactStore) *Activities {
	t.Helper()
	activities, err := NewActivities(ActivityDependencies{
		Repository: repository, SlotExecutor: executor, StagedSlotExecutor: executor, SlotEffectsV3: repository.(imageagent.SlotExternalEffectV3Repository), ArtifactStore: artifacts,
		Publisher: &identityCheckingPublisher{t: t}, PublisherV3: &identityCheckingPublisher{t: t}, PublicationOwner: func(context.Context) (string, error) { return "workflow-run/activity/1", nil },
	})
	require.NoError(t, err)
	return activities
}
