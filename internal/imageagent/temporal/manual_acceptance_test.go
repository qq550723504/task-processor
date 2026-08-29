package temporal_test

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	enumspb "go.temporal.io/api/enums/v1"
	sdkactivity "go.temporal.io/sdk/activity"
	sdkclient "go.temporal.io/sdk/client"
	sdkconverter "go.temporal.io/sdk/converter"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
	imageagentstore "task-processor/internal/imageagent/store"
	imageagenttemporal "task-processor/internal/imageagent/temporal"
	imageagenttools "task-processor/internal/imageagent/tools"
	"task-processor/internal/infra/storage"
	"task-processor/internal/productimage"
)

type podLossRecoveryAcceptanceResult struct {
	MainAssetID         string
	GalleryAssetIDs     []string
	PersistedJSON       []byte
	FirstPodDiscarded   bool
	RecoveryGenerations int
	ApprovedAssetIDs    []string
	PublishedObjects    int
}

type manualRecoveryWorkflowRestartAcceptanceResult struct {
	FirstActivityOwnerDiscarded bool
	RecoveryProviderCalls       int
	RecoveryWorkflowStarts      int
	RecoveryAttachCalls         int
	RecoveryWorkflowName        string
	RecoveryTaskQueue           string
	RecoveryConflictPolicy      enumspb.WorkflowIdConflictPolicy
	RecoveryReusePolicy         enumspb.WorkflowIdReusePolicy
	ExpectedRecoveryWorkflowID  string
	StartedRecoveryWorkflowID   string
	AttachedRecoveryWorkflowID  string
	RecoveredEffectPhase        imageagent.SlotEffectV3Phase
	RecoveredEffectAttempt      int
	RecoveredCandidateAssetIDs  []string
	ProjectionStatus            imageagent.RunStatus
	ProjectionBlockCode         string
	ProjectionOwnerCount        int
	ProjectionSlotStatus        imageagent.SlotStatus
}

func TestManualWorkflowRecoveryOwnerCompletesAfterWorkerRestart(t *testing.T) {
	result := executeManualRecoveryWorkflowRestartAcceptance(t)

	require.True(t, result.FirstActivityOwnerDiscarded)
	require.Zero(t, result.RecoveryProviderCalls)
	require.Equal(t, 2, result.RecoveryWorkflowStarts)
	require.Equal(t, 1, result.RecoveryAttachCalls)
	require.Equal(t, imageagenttemporal.EffectRecoveryWorkflowName, result.RecoveryWorkflowName)
	require.Equal(t, imageagenttemporal.TaskQueueV3, result.RecoveryTaskQueue)
	require.Equal(t, enumspb.WORKFLOW_ID_CONFLICT_POLICY_USE_EXISTING, result.RecoveryConflictPolicy)
	require.Equal(t, enumspb.WORKFLOW_ID_REUSE_POLICY_ALLOW_DUPLICATE_FAILED_ONLY, result.RecoveryReusePolicy)
	require.Equal(t, result.ExpectedRecoveryWorkflowID, result.StartedRecoveryWorkflowID)
	require.Equal(t, result.ExpectedRecoveryWorkflowID, result.AttachedRecoveryWorkflowID)
	require.Equal(t, result.StartedRecoveryWorkflowID, result.AttachedRecoveryWorkflowID)
	require.Equal(t, imageagent.SlotEffectV3PublicationComplete, result.RecoveredEffectPhase)
	require.Equal(t, 1, result.RecoveredEffectAttempt)
	require.NotEmpty(t, result.RecoveredCandidateAssetIDs)
	require.Equal(t, imageagent.RunStatusBlocked, result.ProjectionStatus)
	require.Empty(t, result.ProjectionBlockCode)
	require.Zero(t, result.ProjectionOwnerCount)
	require.Equal(t, imageagent.SlotStatusAccepted, result.ProjectionSlotStatus)
}

func executeManualRecoveryWorkflowRestartAcceptance(t *testing.T) manualRecoveryWorkflowRestartAcceptanceResult {
	t.Helper()

	ctx := context.Background()
	repository := imageagentstore.NewMemoryRepository()
	plan := acceptancePlan(1, 1)
	run := imageagent.Run{
		ID: "run-recovery-restart", BusinessTaskID: "task-recovery-restart", TenantID: "tenant-a", UserID: "user-a",
		Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-recovery-restart", Status: imageagent.RunStatusExecuting,
		CurrentNode: "execute_slots", ActivePlanRevision: plan.Revision, Version: 1,
	}
	scope := imageagent.RunScope{TenantID: run.TenantID, OwnerUserID: run.UserID, RunID: run.ID}
	catalog, err := imageagent.NormalizeAssetCatalog(acceptanceAssetCatalog(1))
	require.NoError(t, err)
	slots := make([]imageagent.SlotProjection, len(plan.Slots))
	for index, slot := range plan.Slots {
		slots[index] = imageagent.SlotProjection{Slot: slot}
	}
	_, err = repository.InitializeRun(ctx, imageagent.ProjectionInitialization{
		Scope: scope, Run: run, Plan: plan, Catalog: catalog,
		Snapshot: imageagent.RunProjection{Run: run, Plan: plan, Slots: slots, AssetCatalog: catalog, ProjectionVersion: 1, LastEventID: 1},
		CommitID: "start:run-key-recovery-restart", EventType: "run.created", EventPayload: json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	currentProjection, err := repository.GetProjection(ctx, scope)
	require.NoError(t, err)
	catalog = currentProjection.AssetCatalog

	transientDir := t.TempDir()
	transientPath := filepath.Join(transientDir, "generated.png")
	require.NoError(t, os.WriteFile(transientPath, acceptancePNG, 0o600))
	firstExecutor := newAcceptanceProductImageExecutor(transientPath)
	durableAPI := &podLossAcceptanceS3{objects: map[string]podLossAcceptanceS3Object{}}
	uploader := storage.NewS3UploaderWithAPI(durableAPI, storage.S3UploaderOptions{
		Bucket: "acceptance-assets", PublicBase: "https://cdn.example.test",
		ArtifactCapabilities: storage.ArtifactStorageCapabilities{Mode: storage.ArtifactStorageModeAWS},
	})
	durableStore, err := objectstore.NewS3DurableArtifactStore(uploader, objectstore.S3DurableArtifactStoreConfig{
		MaxArtifactBytes: 1 << 20, OperationTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	podLost := errors.New("original activity process lost after artifact_staged")
	firstActivities := newPodLossAcceptanceActivities(t, repository, firstExecutor, durableStore, acceptancePublisher{}, func(context.Context) (string, error) {
		return "", podLost
	})
	firstActivityIdentity := fmt.Sprintf("%p", firstActivities)
	recoveryInput := podLossAcceptanceActivityInput(run, plan, catalog, plan.Slots[0])
	_, err = firstActivities.ExecuteSlotV3(ctx, recoveryInput)
	require.ErrorIs(t, err, podLost)

	recoveredEffectIdentity := imageagent.SlotExternalEffectIdentity{
		RunScope: scope, PlanRevision: plan.Revision, SlotID: recoveryInput.Slot.ID, Attempt: recoveryInput.Attempt,
	}
	storedBeforeRecovery, err := repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(ctx, recoveredEffectIdentity)
	require.NoError(t, err)
	require.Equal(t, imageagent.SlotEffectV3ArtifactStaged, storedBeforeRecovery.Phase)
	expectedReservation := manualAcceptanceReservation(recoveryInput)
	require.Equal(t, expectedReservation.IdempotencyKey, storedBeforeRecovery.IdempotencyKey)
	require.Equal(t, expectedReservation.InputFingerprint, storedBeforeRecovery.InputFingerprint)
	require.Equal(t, expectedReservation.Quote.Fingerprint, storedBeforeRecovery.Quote.Fingerprint)
	effects := repository.(imageagent.SlotExternalEffectV3Repository)

	blockedProjection := currentProjection
	blockedProjection.Run.Status = imageagent.RunStatusBlocked
	blockedProjection.Run.CurrentNode = "recover_effect"
	blockedProjection.Run.Version++
	blockedProjection.Run.Block = &imageagent.Block{Code: "recovery_requested", Message: "recovery_requested", SlotID: recoveryInput.Slot.ID}
	blockedProjection.Slots[0].Slot.Status = imageagent.SlotStatusBlocked
	blockedProjection.Slots[0].Attempt = recoveryInput.Attempt
	blockedProjection.Slots[0].ErrorCode = "recovery_requested"
	blockedProjection.RecoverableEffects = []imageagent.RecoverableEffect{{
		SlotID: recoveryInput.Slot.ID, Attempt: recoveryInput.Attempt, Code: "recovery_requested",
	}}
	_, err = repository.CommitProjection(ctx, imageagent.ProjectionCommit{
		Scope: scope, CommitID: "test:recovery-requested", ExpectedProjectionVersion: currentProjection.ProjectionVersion,
		Snapshot: blockedProjection, EventType: "run.updated", EventPayload: json.RawMessage(`{}`), ExpectedRunVersion: currentProjection.Run.Version,
		RunMutation: &imageagent.RunMutation{
			Status: blockedProjection.Run.Status, CurrentNode: blockedProjection.Run.CurrentNode,
			ActivePlanRevision: blockedProjection.Run.ActivePlanRevision, Block: blockedProjection.Run.Block,
		},
		SlotMutation: &imageagent.SlotProjectionMutation{
			PlanRevision: plan.Revision,
			Result: imageagent.SlotResult{
				SlotID: recoveryInput.Slot.ID, Attempt: recoveryInput.Attempt, Status: imageagent.SlotStatusBlocked, ErrorCode: "recovery_requested",
			},
			Projection: blockedProjection.Slots[0],
			Attempt: imageagent.StepAttempt{
				TenantID: run.TenantID, OwnerUserID: run.UserID, RunID: run.ID,
				SlotID: recoveryInput.Slot.ID, PlanRevision: plan.Revision, Node: "execute_slot_v3",
				IdempotencyKey: recoveryInput.IdempotencyKey, Attempt: recoveryInput.Attempt, Outcome: "blocked", ErrorCategory: "recovery_requested",
			},
		},
	})
	require.NoError(t, err)

	require.NoError(t, os.RemoveAll(transientDir))
	_, statErr := os.Stat(transientPath)
	require.ErrorIs(t, statErr, os.ErrNotExist)
	firstActivities = nil
	firstExecutor = nil

	recoveryExecutor := newAcceptanceProductImageExecutor(transientPath)
	blockingStore := newBlockingFinalizeArtifactStore(durableStore)
	recoveryActivities, err := imageagenttemporal.NewActivities(imageagenttemporal.ActivityDependencies{
		Repository: repository, SlotEffects: repository.(imageagent.SlotExternalEffectRepository), SlotExecutor: recoveryExecutor,
		Publisher: acceptancePublisher{}, PublisherV3: acceptancePublisher{}, SlotEffectsV3: effects,
		StagedSlotExecutor: recoveryExecutor, ArtifactStore: blockingStore,
		PublicationOwner:         func(context.Context) (string, error) { return "recovered-workflow-run/execute-slot-v3/2", nil },
		PublicationLeaseDuration: time.Minute,
	})
	require.NoError(t, err)
	workflowClient := newRecordingRecoveryWorkflowClient(t, recoveryActivities)
	client := imageagenttemporal.NewClient(workflowClient)
	command := imageagent.RecoverEffectCommand{
		RunID: run.ID, PlanRevision: plan.Revision, SlotID: recoveryInput.Slot.ID, Attempt: recoveryInput.Attempt,
		ActionID:   "recover-effect-restart-1",
		Identity:   imageagent.ExecutionIdentity{TenantID: run.TenantID, UserID: run.UserID, BusinessTaskID: run.BusinessTaskID},
		Projection: blockedProjection,
	}
	expectedRecoveryWorkflowID := imageagenttemporal.EffectRecoveryWorkflowIDForSlot(
		command.Identity, command.PlanRevision, command.RunID, command.SlotID, command.Attempt, command.ActionID,
	)

	require.NoError(t, client.RecoverEffect(ctx, command))
	startedID, _, _ := workflowClient.attachSummary()
	recordedStart := workflowClient.firstStart()
	recordedReservation := manualAcceptanceReservationFromRecoveryInput(recordedStart.input)
	require.Equal(t, storedBeforeRecovery.IdempotencyKey, recordedReservation.IdempotencyKey)
	require.Equal(t, storedBeforeRecovery.InputFingerprint, recordedReservation.InputFingerprint)
	require.Equal(t, storedBeforeRecovery.Quote.Fingerprint, recordedReservation.Quote.Fingerprint)
	select {
	case <-blockingStore.started:
	case <-workflowClient.executionDone(startedID):
		execution := workflowClient.waitForExecution(t, startedID, time.Second)
		require.NoError(t, execution.err)
		t.Fatal("recovery workflow completed before finalization block was reached")
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for recovery finalization to start")
	}
	// Reusing the same action ID models a worker restart: it must attach to the
	// still-running execution instead of opening a concurrent recovery.
	command.ActionID = "recover-effect-restart-1"
	require.NoError(t, client.RecoverEffect(ctx, command))
	blockingStore.releaseFinalize()

	startedID, attachedID, attachCalls := workflowClient.attachSummary()
	execution := workflowClient.waitForExecution(t, startedID, 5*time.Second)
	require.NoError(t, execution.err)

	recoveredEffect, err := effects.GetSlotExternalEffectV3(ctx, recoveredEffectIdentity)
	require.NoError(t, err)
	projected, err := repository.GetProjection(ctx, scope)
	require.NoError(t, err)

	return manualRecoveryWorkflowRestartAcceptanceResult{
		FirstActivityOwnerDiscarded: firstActivityIdentity != fmt.Sprintf("%p", recoveryActivities),
		RecoveryProviderCalls:       len(recoveryExecutor.calledIDs()),
		RecoveryWorkflowStarts:      workflowClient.startCount(),
		RecoveryAttachCalls:         attachCalls,
		RecoveryWorkflowName:        workflowClient.workflowName(),
		RecoveryTaskQueue:           workflowClient.taskQueue(),
		RecoveryConflictPolicy:      workflowClient.conflictPolicy(),
		RecoveryReusePolicy:         workflowClient.reusePolicy(),
		ExpectedRecoveryWorkflowID:  expectedRecoveryWorkflowID,
		StartedRecoveryWorkflowID:   startedID,
		AttachedRecoveryWorkflowID:  attachedID,
		RecoveredEffectPhase:        recoveredEffect.Phase,
		RecoveredEffectAttempt:      recoveredEffect.Identity.Attempt,
		RecoveredCandidateAssetIDs:  candidateAssetIDs(execution.result.Published),
		ProjectionStatus:            projected.Run.Status,
		ProjectionBlockCode: func() string {
			if projected.Run.Block == nil {
				return ""
			}
			return projected.Run.Block.Code
		}(),
		ProjectionOwnerCount: len(projected.RecoverableEffects),
		ProjectionSlotStatus: projected.Slots[0].Slot.Status,
	}
}

func TestManualImageAgentAcceptanceRecoversAfterPodLossAndApprovesAllAssets(t *testing.T) {
	result := executePodLossRecoveryAcceptance(t)

	require.True(t, result.FirstPodDiscarded, "acceptance must remove the original temp directory and activity/executor instance")
	require.Zero(t, result.RecoveryGenerations, "the replacement activity must not invoke the provider again")
	require.NotEmpty(t, result.MainAssetID)
	require.Len(t, result.GalleryAssetIDs, 2)
	require.Equal(t, append([]string{result.MainAssetID}, result.GalleryAssetIDs...), result.ApprovedAssetIDs)
	require.Equal(t, 3, result.PublishedObjects)
	require.NotContains(t, string(result.PersistedJSON), "local_path")
	require.NotContains(t, string(result.PersistedJSON), "authorization")
}

func executePodLossRecoveryAcceptance(t *testing.T) podLossRecoveryAcceptanceResult {
	t.Helper()
	ctx := context.Background()
	repository := imageagentstore.NewMemoryRepository()
	plan := acceptancePlan(3, 3)
	run := imageagent.Run{
		ID: "run-pod-loss", BusinessTaskID: "task-pod-loss", TenantID: "tenant-a", UserID: "user-a",
		Mode: imageagent.RunModeManual, IdempotencyKey: "run-key-pod-loss", Status: imageagent.RunStatusExecuting,
		CurrentNode: "execute_slots", ActivePlanRevision: plan.Revision, Version: 1,
	}
	scope := imageagent.RunScope{TenantID: run.TenantID, OwnerUserID: run.UserID, RunID: run.ID}
	catalog, err := imageagent.NormalizeAssetCatalog(acceptanceAssetCatalog(3))
	require.NoError(t, err)
	slots := make([]imageagent.SlotProjection, len(plan.Slots))
	for index, slot := range plan.Slots {
		slots[index] = imageagent.SlotProjection{Slot: slot}
	}
	_, err = repository.InitializeRun(ctx, imageagent.ProjectionInitialization{
		Scope: scope, Run: run, Plan: plan, Catalog: catalog,
		Snapshot: imageagent.RunProjection{Run: run, Plan: plan, Slots: slots, AssetCatalog: catalog, ProjectionVersion: 1, LastEventID: 1},
		CommitID: "start:run-key-pod-loss", EventType: "run.created", EventPayload: json.RawMessage(`{}`),
	})
	require.NoError(t, err)
	currentProjection, err := repository.GetProjection(ctx, scope)
	require.NoError(t, err)
	catalog = currentProjection.AssetCatalog

	transientDir := t.TempDir()
	transientPath := filepath.Join(transientDir, "generated.png")
	require.NoError(t, os.WriteFile(transientPath, acceptancePNG, 0o600))
	firstExecutor := newAcceptanceProductImageExecutor(transientPath)
	durableAPI := &podLossAcceptanceS3{objects: map[string]podLossAcceptanceS3Object{}}
	uploader := storage.NewS3UploaderWithAPI(durableAPI, storage.S3UploaderOptions{
		Bucket: "acceptance-assets", PublicBase: "https://cdn.example.test",
		ArtifactCapabilities: storage.ArtifactStorageCapabilities{Mode: storage.ArtifactStorageModeAWS},
	})
	durableStore, err := objectstore.NewS3DurableArtifactStore(uploader, objectstore.S3DurableArtifactStoreConfig{
		MaxArtifactBytes: 1 << 20, OperationTimeout: 5 * time.Second,
	})
	require.NoError(t, err)
	podLost := errors.New("original activity process lost after artifact_staged")
	firstActivities := newPodLossAcceptanceActivities(t, repository, firstExecutor, durableStore, acceptancePublisher{}, func(context.Context) (string, error) {
		return "", podLost
	})
	firstActivityIdentity := fmt.Sprintf("%p", firstActivities)

	for _, slot := range plan.Slots {
		input := podLossAcceptanceActivityInput(run, plan, catalog, slot)
		_, executeErr := firstActivities.ExecuteSlotV3(ctx, input)
		require.ErrorIs(t, executeErr, podLost)
		effect, effectErr := repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(ctx, imageagent.SlotExternalEffectIdentity{
			RunScope: scope, PlanRevision: plan.Revision, SlotID: slot.ID, Attempt: 1,
		})
		require.NoError(t, effectErr)
		require.Equal(t, imageagent.SlotEffectV3ArtifactStaged, effect.Phase)
	}

	require.NoError(t, os.RemoveAll(transientDir))
	_, statErr := os.Stat(transientPath)
	require.ErrorIs(t, statErr, os.ErrNotExist)
	firstActivities = nil
	firstExecutor = nil

	recoveryExecutor := newAcceptanceProductImageExecutor(transientPath)
	approvalPublisher := &recordingAcceptanceApprovalPublisher{}
	recoveryActivities := newPodLossAcceptanceActivities(t, repository, recoveryExecutor, durableStore, approvalPublisher, func(context.Context) (string, error) {
		return "recovered-workflow-run/execute-slot-v3/2", nil
	})
	require.NotEqual(t, firstActivityIdentity, fmt.Sprintf("%p", recoveryActivities))

	for _, slot := range plan.Slots {
		input := podLossAcceptanceActivityInput(run, plan, catalog, slot)
		published, executeErr := recoveryActivities.ExecuteSlotV3(ctx, input)
		require.NoError(t, executeErr)
		require.Len(t, published.Candidates, 1)
		require.NoError(t, recoveryActivities.PersistSlotResultV3(ctx, imageagenttemporal.PersistSlotResultV3ActivityInput{
			RunID: run.ID, Identity: input.Identity, PlanRevision: plan.Revision,
			Result:     imageagenttemporal.SlotWorkflowV3Result{Published: published, Status: imageagent.SlotStatusAccepted},
			AttemptKey: input.IdempotencyKey,
		}))
	}

	projection, err := repository.GetProjection(ctx, scope)
	require.NoError(t, err)
	approvedIDs := make([]string, 0, len(projection.Slots))
	mainID := ""
	galleryIDs := make([]string, 0, len(projection.Slots)-1)
	for _, slot := range projection.Slots {
		require.Equal(t, imageagent.SlotStatusAccepted, slot.Slot.Status)
		require.Len(t, slot.Candidates, 1)
		candidateID := slot.Candidates[0].AssetID
		approvedIDs = append(approvedIDs, candidateID)
		if slot.Slot.Role == imageagent.SlotRoleMain {
			require.Empty(t, mainID, "acceptance plan must have exactly one main candidate")
			mainID = candidateID
		} else {
			galleryIDs = append(galleryIDs, candidateID)
		}
	}
	require.NotEmpty(t, mainID)
	require.NoError(t, recoveryActivities.PublishApprovedV3(ctx, imageagenttemporal.PublishApprovedV3ActivityInput{
		RunID: run.ID, Identity: imageagent.ExecutionIdentity{TenantID: run.TenantID, UserID: run.UserID},
		PlanRevision: plan.Revision, CandidateAssetIDs: approvedIDs, IdempotencyKey: "approve-pod-loss-v3",
	}))
	require.Equal(t, approvedIDs, approvalPublisher.input.CandidateAssetIDs)

	effects := make([]imageagent.SlotEffectV3Attempt, 0, len(plan.Slots))
	for _, slot := range plan.Slots {
		effect, effectErr := repository.(imageagent.SlotExternalEffectV3Repository).GetSlotExternalEffectV3(ctx, imageagent.SlotExternalEffectIdentity{
			RunScope: scope, PlanRevision: plan.Revision, SlotID: slot.ID, Attempt: 1,
		})
		require.NoError(t, effectErr)
		require.Equal(t, imageagent.SlotEffectV3PublicationComplete, effect.Phase)
		effects = append(effects, effect)
	}
	persistedJSON, err := json.Marshal(struct {
		Projection imageagent.RunProjection         `json:"projection"`
		Effects    []imageagent.SlotEffectV3Attempt `json:"effects"`
	}{Projection: projection, Effects: effects})
	require.NoError(t, err)

	return podLossRecoveryAcceptanceResult{
		MainAssetID: mainID, GalleryAssetIDs: galleryIDs, PersistedJSON: persistedJSON,
		FirstPodDiscarded: true, RecoveryGenerations: len(recoveryExecutor.calledIDs()),
		ApprovedAssetIDs: append([]string(nil), approvalPublisher.input.CandidateAssetIDs...),
		PublishedObjects: durableAPI.countPrefix("image-agent/public/"),
	}
}

func newAcceptanceProductImageExecutor(artifactPath string) *recordingAcceptanceExecutor {
	return &recordingAcceptanceExecutor{delegate: imageagenttools.NewProductImageSlotExecutor(imageagenttools.Dependencies{
		SubjectExtractor: acceptanceSubjectExtractor{}, WhiteBackgroundRenderer: acceptanceWhiteRenderer{artifactPath: artifactPath},
		SceneRenderer: acceptanceSceneRenderer{artifactPath: artifactPath},
	})}
}

func newPodLossAcceptanceActivities(
	t *testing.T,
	repository imageagent.Repository,
	executor *recordingAcceptanceExecutor,
	durableStore imageagenttemporal.DurableArtifactStore,
	publisherV3 imageagent.ApprovedAssetPublisherV3,
	publicationOwner func(context.Context) (string, error),
) *imageagenttemporal.Activities {
	t.Helper()
	activities, err := imageagenttemporal.NewActivities(imageagenttemporal.ActivityDependencies{
		Repository: repository, SlotExecutor: executor, Publisher: acceptancePublisher{}, PublisherV3: publisherV3,
		SlotEffectsV3: repository.(imageagent.SlotExternalEffectV3Repository), StagedSlotExecutor: executor,
		ArtifactStore: durableStore, PublicationOwner: publicationOwner, PublicationLeaseDuration: time.Minute,
	})
	require.NoError(t, err)
	return activities
}

func podLossAcceptanceActivityInput(run imageagent.Run, plan imageagent.Plan, catalog imageagent.AssetCatalog, slot imageagent.Slot) imageagenttemporal.ExecuteSlotV3ActivityInput {
	return podLossAcceptanceActivityInputForAttempt(run, plan, catalog, slot, 1)
}

func podLossAcceptanceActivityInputForAttempt(run imageagent.Run, plan imageagent.Plan, catalog imageagent.AssetCatalog, slot imageagent.Slot, attempt int) imageagenttemporal.ExecuteSlotV3ActivityInput {
	return imageagenttemporal.ExecuteSlotV3ActivityInput{
		RunID: run.ID, Identity: imageagent.ExecutionIdentity{TenantID: run.TenantID, UserID: run.UserID},
		PlanRevision: plan.Revision, Slot: slot, Attempt: attempt,
		IdempotencyKey: fmt.Sprintf("%s:plan:%d:attempt:%d", slot.IdempotencyKey, plan.Revision, attempt), AssetCatalog: catalog,
	}
}

func TestManualImageAgentAcceptancePreservesSixSuccessfulSlotsWhenOneBlocks(t *testing.T) {
	plan := acceptancePlan(7, 9)
	result, calledSlotIDs, events := executeAcceptanceWorkflow(t, plan, "scene-2")

	require.Equal(t, imageagent.RunStatusBlocked, result.Status)
	require.NotNil(t, result.Block)
	require.Equal(t, "scene-2", result.Block.SlotID)
	require.Len(t, result.CompletedSlotIDs, 6)
	require.Len(t, result.Slots, 7)
	require.Equal(t, sortedSlotIDs(plan), sortedStrings(calledSlotIDs))
	require.Equal(t, 6, acceptedSlotEventCount(t, events))
}

func TestManualImageAgentAcceptanceAllowsElevenStandardSlotsThroughWorkflow(t *testing.T) {
	plan := acceptancePlan(11, 9)
	require.NoError(t, imageagent.ValidatePlan(plan))

	result, calledSlotIDs, _ := executeAcceptanceWorkflow(t, plan, "scene-10")

	require.Equal(t, imageagent.RunStatusBlocked, result.Status)
	require.Len(t, result.Slots, 11)
	require.Len(t, result.CompletedSlotIDs, 10)
	require.Equal(t, sortedSlotIDs(plan), sortedStrings(calledSlotIDs))
}

func executeAcceptanceWorkflow(t *testing.T, plan imageagent.Plan, invalidSlotID string) (imageagenttemporal.WorkflowResult, []string, []imageagent.RunEvent) {
	t.Helper()
	repository := imageagentstore.NewMemoryRepository()
	run := &imageagent.Run{
		ID: "run-acceptance", BusinessTaskID: "task-acceptance",
		TenantID: "tenant-a", UserID: "user-a", Mode: imageagent.RunModeManual,
		IdempotencyKey: "run-key-acceptance", Status: imageagent.RunStatusPlanning,
		CurrentNode: "plan", Version: 1,
	}
	scope := imageagent.RunScope{TenantID: run.TenantID, OwnerUserID: run.UserID, RunID: run.ID}
	catalog := acceptanceAssetCatalog(9)
	normalizedCatalog, err := imageagent.NormalizeAssetCatalog(catalog)
	require.NoError(t, err)
	run.ActivePlanRevision = plan.Revision
	initialSlots := make([]imageagent.SlotProjection, len(plan.Slots))
	for index, slot := range plan.Slots {
		initialSlots[index] = imageagent.SlotProjection{Slot: slot}
	}
	_, err = repository.InitializeRun(context.Background(), imageagent.ProjectionInitialization{
		Scope: scope, Run: *run, Plan: plan, Catalog: normalizedCatalog,
		Snapshot: imageagent.RunProjection{Run: *run, Plan: plan, Slots: initialSlots, AssetCatalog: normalizedCatalog, ProjectionVersion: 1, LastEventID: 1},
		CommitID: "start:run-key-acceptance", EventType: "run.created", EventPayload: json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	artifactPath := filepath.Join(t.TempDir(), "generated.png")
	require.NoError(t, os.WriteFile(artifactPath, acceptancePNG, 0o600))
	delegate := imageagenttools.NewProductImageSlotExecutor(imageagenttools.Dependencies{
		SubjectExtractor:        acceptanceSubjectExtractor{},
		WhiteBackgroundRenderer: acceptanceWhiteRenderer{artifactPath: artifactPath},
		SceneRenderer:           acceptanceSceneRenderer{artifactPath: artifactPath},
	})
	executor := &recordingAcceptanceExecutor{delegate: delegate, invalidSlotID: invalidSlotID}
	activities, err := imageagenttemporal.NewActivities(imageagenttemporal.ActivityDependencies{
		Repository: repository, SlotExecutor: executor, Publisher: acceptancePublisher{}, PublisherV3: acceptancePublisher{},
		SlotEffectsV3: repository.(imageagent.SlotExternalEffectV3Repository), StagedSlotExecutor: executor,
		ArtifactStore: acceptanceDurableArtifactStore{},
	})
	require.NoError(t, err)

	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.SetWorkerOptions(worker.Options{DeadlockDetectionTimeout: 5 * time.Second})
	env.RegisterWorkflow(imageagenttemporal.ImageSlotWorkflowV3)
	require.NoError(t, imageagenttemporal.RegisterActivitiesForMode(env, activities, imageagenttemporal.WorkerWireModeV3))
	env.ExecuteWorkflow(imageagenttemporal.ImageAgentWorkflow, imageagenttemporal.WorkflowInput{
		RunID: run.ID, Mode: imageagent.RunModeManual,
		Identity: imageagent.ExecutionIdentity{TenantID: run.TenantID, UserID: run.UserID},
		Plan:     plan, AssetCatalog: normalizedCatalog, MaxConcurrentSlots: 4, WaitForCommands: false,
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result imageagenttemporal.WorkflowResult
	require.NoError(t, env.GetWorkflowResult(&result))
	events, err := repository.ListEvents(context.Background(), scope, 0, 100)
	require.NoError(t, err)
	return result, executor.calledIDs(), events
}

func acceptanceAssetCatalog(count int) imageagent.AssetCatalog {
	assets := make([]imageagent.AuthorizedAsset, 0, count+1)
	for index := 1; index <= count; index++ {
		id := fmt.Sprintf("source-%d", index)
		assets = append(assets, imageagent.AuthorizedAsset{
			ID: id, Type: imageagent.AuthorizedAssetSource,
			DisplayURL: fmt.Sprintf("https://source.example/%s.png", id),
			Width:      1200, Height: 1200,
		})
	}
	assets = append(assets, imageagent.AuthorizedAsset{ID: "style-modern", Type: imageagent.AuthorizedAssetStyle, URL: "https://style.example/style-modern.png"})
	return imageagent.AssetCatalog{Assets: assets}
}

func acceptancePlan(slotCount, sourceCount int) imageagent.Plan {
	sourceIDs := make([]string, sourceCount)
	for index := range sourceIDs {
		sourceIDs[index] = fmt.Sprintf("source-%d", index+1)
	}
	slots := make([]imageagent.Slot, slotCount)
	for index := range slots {
		id := fmt.Sprintf("scene-%d", index)
		role := imageagent.SlotRoleScene
		if index == 0 {
			id = "main-1"
			role = imageagent.SlotRoleMain
		}
		slots[index] = imageagent.Slot{
			ID: id, Role: role,
			SourceAssetIDs:    []string{sourceIDs[index%len(sourceIDs)]},
			StyleReferenceIDs: []string{"style-modern"},
			Brief:             id,
			IdempotencyKey:    "slot-key-" + id,
			Status:            imageagent.SlotStatusPending,
		}
	}
	return imageagent.Plan{
		Revision: 1, IdempotencyKey: "plan-key-acceptance",
		SourceAssetIDs: sourceIDs, StyleReferenceIDs: []string{"style-modern"},
		Slots: slots, CreatedBy: "user-a",
	}
}

func manualAcceptanceReservation(input imageagenttemporal.ExecuteSlotV3ActivityInput) imageagent.SlotEffectV3Reservation {
	execution := imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: input.IdempotencyKey, AssetCatalog: input.AssetCatalog,
	}
	return imageagent.SlotEffectV3Reservation{
		Identity: imageagent.SlotExternalEffectIdentity{
			RunScope:     imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID},
			PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt,
		},
		IdempotencyKey:   input.IdempotencyKey,
		InputFingerprint: imageagent.SlotExecutionFingerprint(execution),
	}
}

func manualAcceptanceReservationFromRecoveryInput(input imageagenttemporal.EffectRecoveryWorkflowInput) imageagent.SlotEffectV3Reservation {
	execution := imageagent.SlotExecutionInput{
		RunID: input.RunID, TenantID: input.Identity.TenantID, UserID: input.Identity.UserID,
		PlanRevision: input.PlanRevision, Slot: input.Slot, Attempt: input.Attempt,
		IdempotencyKey: fmt.Sprintf("%s:plan:%d:attempt:%d", input.Slot.IdempotencyKey, input.PlanRevision, input.Attempt),
		AssetCatalog:   input.AssetCatalog, ProductContext: input.AssetCatalog.ProductContext,
	}
	return imageagent.SlotEffectV3Reservation{
		Identity: imageagent.SlotExternalEffectIdentity{
			RunScope:     imageagent.RunScope{TenantID: input.Identity.TenantID, OwnerUserID: input.Identity.UserID, RunID: input.RunID},
			PlanRevision: input.PlanRevision, SlotID: input.Slot.ID, Attempt: input.Attempt,
		},
		IdempotencyKey:   execution.IdempotencyKey,
		InputFingerprint: imageagent.SlotExecutionFingerprint(execution),
	}
}

func candidateAssetIDs(result imageagent.SlotEffectV3PublishedResult) []string {
	ids := make([]string, 0, len(result.Candidates))
	for _, candidate := range result.Candidates {
		ids = append(ids, candidate.AssetID)
	}
	return ids
}

type blockingFinalizeArtifactStore struct {
	delegate    imageagenttemporal.DurableArtifactStore
	started     chan struct{}
	release     chan struct{}
	startedOnce sync.Once
	releaseOnce sync.Once
}

func newBlockingFinalizeArtifactStore(delegate imageagenttemporal.DurableArtifactStore) *blockingFinalizeArtifactStore {
	return &blockingFinalizeArtifactStore{
		delegate: delegate,
		started:  make(chan struct{}),
		release:  make(chan struct{}),
	}
}

func (s *blockingFinalizeArtifactStore) PublicURL(key string) string {
	return s.delegate.PublicURL(key)
}

func (s *blockingFinalizeArtifactStore) PrepareSlotArtifacts(input objectstore.PrepareSlotArtifactsInput) (objectstore.PreparedSlotArtifacts, error) {
	return s.delegate.PrepareSlotArtifacts(input)
}

func (s *blockingFinalizeArtifactStore) PreserveSlotArtifacts(ctx context.Context, identity imageagent.SlotExternalEffectIdentity, prepared objectstore.PreparedSlotArtifacts) error {
	return s.delegate.PreserveSlotArtifacts(ctx, identity, prepared)
}

func (s *blockingFinalizeArtifactStore) RecoverSlotArtifacts(ctx context.Context, identity imageagent.SlotExternalEffectIdentity, manifest imageagent.StagingManifest) (objectstore.PreparedSlotArtifacts, error) {
	return s.delegate.RecoverSlotArtifacts(ctx, identity, manifest)
}

func (s *blockingFinalizeArtifactStore) EnsureStaged(ctx context.Context, prepared objectstore.PreparedSlotArtifacts) error {
	return s.delegate.EnsureStaged(ctx, prepared)
}

func (s *blockingFinalizeArtifactStore) Finalize(ctx context.Context, manifest imageagent.StagingManifest) (imageagent.FinalManifest, error) {
	return s.FinalizeWithProgress(ctx, manifest, nil)
}

func (s *blockingFinalizeArtifactStore) FinalizeWithProgress(ctx context.Context, manifest imageagent.StagingManifest, progress func(context.Context, int) error) (imageagent.FinalManifest, error) {
	s.startedOnce.Do(func() { close(s.started) })
	<-s.release
	return s.delegate.FinalizeWithProgress(ctx, manifest, progress)
}

func (s *blockingFinalizeArtifactStore) waitForFinalizeStart(t *testing.T, timeout time.Duration) {
	t.Helper()
	select {
	case <-s.started:
	case <-time.After(timeout):
		t.Fatal("timed out waiting for recovery finalization to start")
	}
}

func (s *blockingFinalizeArtifactStore) releaseFinalize() {
	s.releaseOnce.Do(func() { close(s.release) })
}

type recordingRecoveryWorkflowClient struct {
	t          *testing.T
	activities *imageagenttemporal.Activities
	mu         sync.Mutex
	starts     []recordedRecoveryStart
	executions map[string]*recordedRecoveryExecution
}

type recordedRecoveryStart struct {
	options      sdkclient.StartWorkflowOptions
	workflowName string
	input        imageagenttemporal.EffectRecoveryWorkflowInput
}

type recordedRecoveryExecution struct {
	workflowID  string
	runID       string
	result      imageagenttemporal.EffectRecoveryResult
	err         error
	attachCalls int
	done        chan struct{}
	run         *recordingWorkflowRun
}

func newRecordingRecoveryWorkflowClient(t *testing.T, activities *imageagenttemporal.Activities) *recordingRecoveryWorkflowClient {
	t.Helper()
	return &recordingRecoveryWorkflowClient{
		t:          t,
		activities: activities,
		executions: make(map[string]*recordedRecoveryExecution),
	}
}

func (c *recordingRecoveryWorkflowClient) ExecuteWorkflow(_ context.Context, options sdkclient.StartWorkflowOptions, workflow interface{}, args ...interface{}) (sdkclient.WorkflowRun, error) {
	name, ok := workflow.(string)
	if !ok {
		return nil, fmt.Errorf("unexpected workflow name payload %T", workflow)
	}
	if len(args) != 1 {
		return nil, fmt.Errorf("unexpected recovery workflow arg count %d", len(args))
	}
	input, ok := args[0].(imageagenttemporal.EffectRecoveryWorkflowInput)
	if !ok {
		return nil, fmt.Errorf("unexpected recovery workflow input %T", args[0])
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.starts = append(c.starts, recordedRecoveryStart{options: options, workflowName: name, input: input})
	if execution, exists := c.executions[options.ID]; exists {
		execution.attachCalls++
		return execution.run, nil
	}
	execution := &recordedRecoveryExecution{
		workflowID: options.ID,
		runID:      fmt.Sprintf("recorded-recovery-run-%d", len(c.starts)),
		done:       make(chan struct{}),
	}
	execution.run = &recordingWorkflowRun{execution: execution}
	c.executions[options.ID] = execution
	go c.executeRecoveryWorkflow(execution, input)
	return execution.run, nil
}

func (c *recordingRecoveryWorkflowClient) executeRecoveryWorkflow(execution *recordedRecoveryExecution, input imageagenttemporal.EffectRecoveryWorkflowInput) {
	var suite testsuite.WorkflowTestSuite
	env := suite.NewTestWorkflowEnvironment()
	env.RegisterWorkflow(imageagenttemporal.ImageAgentEffectRecoveryWorkflow)
	env.RegisterActivityWithOptions(c.activities.RecoverEffectV3, sdkactivity.RegisterOptions{Name: "imageagent.recover_effect.v3"})
	env.RegisterActivityWithOptions(c.activities.PersistRecoveryBlockedEffectV3, sdkactivity.RegisterOptions{Name: "imageagent.persist_recovery_blocked.v3"})
	env.RegisterActivityWithOptions(c.activities.ReconcileEffectRecoveryV3, sdkactivity.RegisterOptions{Name: "imageagent.reconcile_effect_recovery.v3"})
	env.OnSignalExternalWorkflow(mock.Anything, mock.Anything, mock.Anything, "effect_recovery_completed", mock.Anything).Return(nil)
	env.ExecuteWorkflow(imageagenttemporal.ImageAgentEffectRecoveryWorkflow, input)

	var (
		result imageagenttemporal.EffectRecoveryResult
		err    error
	)
	if workflowErr := env.GetWorkflowError(); workflowErr != nil {
		err = workflowErr
	} else {
		err = env.GetWorkflowResult(&result)
	}

	c.mu.Lock()
	execution.result = result
	execution.err = err
	close(execution.done)
	c.mu.Unlock()
}

func (*recordingRecoveryWorkflowClient) QueryWorkflow(context.Context, string, string, string, ...interface{}) (sdkconverter.EncodedValue, error) {
	return nil, fmt.Errorf("unexpected QueryWorkflow call in recovery acceptance")
}

func (*recordingRecoveryWorkflowClient) SignalWorkflow(context.Context, string, string, string, interface{}) error {
	return fmt.Errorf("unexpected SignalWorkflow call in recovery acceptance")
}

func (*recordingRecoveryWorkflowClient) UpdateWorkflow(context.Context, sdkclient.UpdateWorkflowOptions) (sdkclient.WorkflowUpdateHandle, error) {
	return nil, fmt.Errorf("unexpected UpdateWorkflow call in recovery acceptance")
}

func (c *recordingRecoveryWorkflowClient) startCount() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.starts)
}

func (c *recordingRecoveryWorkflowClient) attachSummary() (string, string, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.starts) == 0 {
		return "", "", 0
	}
	startedID := c.starts[0].options.ID
	attachedID := ""
	if len(c.starts) > 1 {
		attachedID = c.starts[1].options.ID
	}
	attachCalls := 0
	if execution := c.executions[startedID]; execution != nil {
		attachCalls = execution.attachCalls
	}
	return startedID, attachedID, attachCalls
}

func (c *recordingRecoveryWorkflowClient) firstStart() recordedRecoveryStart {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.starts) == 0 {
		return recordedRecoveryStart{}
	}
	return c.starts[0]
}

func (c *recordingRecoveryWorkflowClient) workflowName() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.starts) == 0 {
		return ""
	}
	return c.starts[0].workflowName
}

func (c *recordingRecoveryWorkflowClient) taskQueue() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.starts) == 0 {
		return ""
	}
	return c.starts[0].options.TaskQueue
}

func (c *recordingRecoveryWorkflowClient) conflictPolicy() enumspb.WorkflowIdConflictPolicy {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.starts) == 0 {
		return enumspb.WORKFLOW_ID_CONFLICT_POLICY_UNSPECIFIED
	}
	return c.starts[0].options.WorkflowIDConflictPolicy
}

func (c *recordingRecoveryWorkflowClient) reusePolicy() enumspb.WorkflowIdReusePolicy {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.starts) == 0 {
		return enumspb.WORKFLOW_ID_REUSE_POLICY_UNSPECIFIED
	}
	return c.starts[0].options.WorkflowIDReusePolicy
}

func (c *recordingRecoveryWorkflowClient) waitForExecution(t *testing.T, workflowID string, timeout time.Duration) recordedRecoveryExecution {
	t.Helper()
	c.mu.Lock()
	execution := c.executions[workflowID]
	c.mu.Unlock()
	if execution == nil {
		t.Fatalf("workflow %q was not recorded", workflowID)
	}
	select {
	case <-execution.done:
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for recovery workflow %q", workflowID)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return *execution
}

func (c *recordingRecoveryWorkflowClient) executionDone(workflowID string) <-chan struct{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	if execution := c.executions[workflowID]; execution != nil {
		return execution.done
	}
	done := make(chan struct{})
	close(done)
	return done
}

type recordingWorkflowRun struct {
	execution *recordedRecoveryExecution
}

func (r *recordingWorkflowRun) GetID() string {
	return r.execution.workflowID
}

func (r *recordingWorkflowRun) GetRunID() string {
	return r.execution.runID
}

func (r *recordingWorkflowRun) Get(ctx context.Context, valuePtr interface{}) error {
	return r.GetWithOptions(ctx, valuePtr, sdkclient.WorkflowRunGetOptions{})
}

func (r *recordingWorkflowRun) GetWithOptions(ctx context.Context, valuePtr interface{}, _ sdkclient.WorkflowRunGetOptions) error {
	select {
	case <-r.execution.done:
	case <-ctx.Done():
		return ctx.Err()
	}
	if r.execution.err != nil {
		return r.execution.err
	}
	switch typed := valuePtr.(type) {
	case nil:
		return nil
	case *imageagenttemporal.EffectRecoveryResult:
		*typed = r.execution.result
		return nil
	default:
		return fmt.Errorf("unsupported recovery workflow result target %T", valuePtr)
	}
}

type recordingAcceptanceExecutor struct {
	delegate      acceptanceExecutorDelegate
	invalidSlotID string
	mu            sync.Mutex
	calls         []string
}

type acceptanceExecutorDelegate interface {
	imageagent.StagedSlotExecutor
	imageagent.RecoverableSlotExecutor
}

func (e *recordingAcceptanceExecutor) QuoteSlot(context.Context, imageagent.SlotExecutionInput, imageagent.BudgetPolicy) (imageagent.SlotUsageQuote, error) {
	maximum := imageagent.UsageVector{Images: 1, AgentSteps: 1}
	return imageagent.SlotUsageQuote{Maximum: maximum, Operations: []imageagent.SlotUsageOperation{{Name: "acceptance_provider", Fingerprint: "acceptance-provider-v1", Maximum: maximum, MaximumOutputs: 1}}, Fingerprint: "acceptance-slot-quote-v1"}, nil
}

func (e *recordingAcceptanceExecutor) GenerateQuotedSlot(ctx context.Context, input imageagent.SlotExecutionInput, _ imageagent.SlotUsageQuote) (imageagent.SlotGeneratedOutput, error) {
	generated, err := e.GenerateSlot(ctx, input)
	if err == nil {
		generated.UsageReceipt = imageagent.SlotUsageReceipt{Actual: imageagent.UsageVector{Images: int64(len(generated.Assets)), AgentSteps: 1}, CostBasis: imageagent.UsageCostReservedUpperBound}
	}
	return generated, err
}

func (e *recordingAcceptanceExecutor) ExecuteSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	generated, err := e.GenerateSlot(ctx, input)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	return e.PublishSlot(ctx, input, generated)
}

func (e *recordingAcceptanceExecutor) GenerateSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotGeneratedOutput, error) {
	result, err := e.delegate.GenerateSlot(ctx, input)
	e.mu.Lock()
	e.calls = append(e.calls, input.Slot.ID)
	e.mu.Unlock()
	if err == nil && input.Slot.ID == e.invalidSlotID {
		return imageagent.SlotGeneratedOutput{}, fmt.Errorf("intentional acceptance provider failure for %s", input.Slot.ID)
	}
	return result, err
}

func (e *recordingAcceptanceExecutor) PublishSlot(ctx context.Context, input imageagent.SlotExecutionInput, generated imageagent.SlotGeneratedOutput) (imageagent.SlotExecutionResult, error) {
	result, err := e.delegate.PublishSlot(ctx, input, generated)
	if err == nil && input.Slot.ID == e.invalidSlotID {
		return imageagent.SlotExecutionResult{SlotID: input.Slot.ID, Attempt: input.Attempt}, nil
	}
	return result, err
}

func (e *recordingAcceptanceExecutor) BuildSlotResult(ctx context.Context, input imageagent.SlotExecutionInput, published imageagent.PublishedSlotOutput) (imageagent.SlotExecutionResult, error) {
	return e.delegate.BuildSlotResult(ctx, input, published)
}

func (e *recordingAcceptanceExecutor) calledIDs() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]string(nil), e.calls...)
}

type acceptanceSubjectExtractor struct{}

func (acceptanceSubjectExtractor) Extract(_ context.Context, imageURL string, _ *productimage.ProductContext) (*productimage.ImageAsset, error) {
	return &productimage.ImageAsset{
		URL: imageURL + "?subject=1", SourceURL: imageURL,
		Type: productimage.AssetTypeSubjectCutout,
	}, nil
}

type acceptanceWhiteRenderer struct{ artifactPath string }

func (r acceptanceWhiteRenderer) Render(_ context.Context, _ *productimage.ImageAsset, context *productimage.ProductContext) (*productimage.ImageAsset, error) {
	return &productimage.ImageAsset{
		URL:  r.artifactPath,
		Type: productimage.AssetTypeWhiteBgImage,
	}, nil
}

type acceptanceSceneRenderer struct{ artifactPath string }

func (r acceptanceSceneRenderer) Render(_ context.Context, _ *productimage.ImageAsset, context *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	return []productimage.ImageAsset{{
		URL:  r.artifactPath,
		Type: productimage.AssetTypeGalleryImage,
	}}, nil
}

type acceptancePublisher struct{}

func (acceptancePublisher) PublishApproved(context.Context, imageagent.PublishApprovedInput) (imageagent.PublicationAcknowledgement, error) {
	return imageagent.PublicationAcknowledgement{}, nil
}

func (acceptancePublisher) PublishApprovedV3(context.Context, imageagent.PublishApprovedV3Input) (imageagent.PublicationAcknowledgement, error) {
	return imageagent.PublicationAcknowledgement{}, nil
}

type recordingAcceptanceApprovalPublisher struct {
	input imageagent.PublishApprovedV3Input
}

func (p *recordingAcceptanceApprovalPublisher) PublishApprovedV3(_ context.Context, input imageagent.PublishApprovedV3Input) (imageagent.PublicationAcknowledgement, error) {
	p.input = input
	return imageagent.PublicationAcknowledgement{IdempotencyKey: input.IdempotencyKey}, nil
}

type podLossAcceptanceS3Object struct {
	data        []byte
	contentType string
	metadata    map[string]string
	checksum    string
}

type podLossAcceptanceS3 struct {
	mu      sync.Mutex
	objects map[string]podLossAcceptanceS3Object
}

func (s *podLossAcceptanceS3) PutObject(_ context.Context, input *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	data, err := io.ReadAll(input.Body)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	key := aws.ToString(input.Key)
	if _, exists := s.objects[key]; exists {
		return nil, objectstore.ErrObjectConflict
	}
	s.objects[key] = podLossAcceptanceS3Object{
		data: append([]byte(nil), data...), contentType: aws.ToString(input.ContentType),
		metadata: cloneAcceptanceMetadata(input.Metadata), checksum: aws.ToString(input.ChecksumSHA256),
	}
	return &s3.PutObjectOutput{}, nil
}

func (s *podLossAcceptanceS3) HeadObject(_ context.Context, input *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	object, exists := s.objects[aws.ToString(input.Key)]
	if !exists {
		return nil, &types.NotFound{}
	}
	return &s3.HeadObjectOutput{
		ContentLength: aws.Int64(int64(len(object.data))), ContentType: aws.String(object.contentType),
		Metadata: cloneAcceptanceMetadata(object.metadata), ChecksumSHA256: aws.String(object.checksum),
	}, nil
}

func (s *podLossAcceptanceS3) GetObject(_ context.Context, input *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	object, exists := s.objects[aws.ToString(input.Key)]
	if !exists {
		return nil, &types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{
		Body: io.NopCloser(strings.NewReader(string(object.data))), ContentLength: aws.Int64(int64(len(object.data))),
		ContentType: aws.String(object.contentType), Metadata: cloneAcceptanceMetadata(object.metadata), ChecksumSHA256: aws.String(object.checksum),
	}, nil
}

func (s *podLossAcceptanceS3) CopyObject(_ context.Context, input *s3.CopyObjectInput, _ ...func(*s3.Options)) (*s3.CopyObjectOutput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sourceKey := strings.TrimPrefix(aws.ToString(input.CopySource), "acceptance-assets/")
	source, exists := s.objects[sourceKey]
	if !exists {
		return nil, &types.NotFound{}
	}
	destinationKey := aws.ToString(input.Key)
	if _, exists := s.objects[destinationKey]; !exists {
		source.contentType = aws.ToString(input.ContentType)
		source.metadata = cloneAcceptanceMetadata(input.Metadata)
		s.objects[destinationKey] = source
	}
	return &s3.CopyObjectOutput{}, nil
}

func (*podLossAcceptanceS3) DeleteObject(context.Context, *s3.DeleteObjectInput, ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	return nil, errors.New("acceptance does not delete durable recovery objects")
}

func (s *podLossAcceptanceS3) countPrefix(prefix string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for key := range s.objects {
		if strings.HasPrefix(key, prefix) {
			count++
		}
	}
	return count
}

func cloneAcceptanceMetadata(metadata map[string]string) map[string]string {
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

var acceptancePNG, _ = base64.StdEncoding.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNk+A8AAQUBAScY42YAAAAASUVORK5CYII=")

type acceptanceDurableArtifactStore struct{}

func (acceptanceDurableArtifactStore) PublicURL(key string) string {
	return "https://cdn.example.test/" + key
}

func (acceptanceDurableArtifactStore) PrepareSlotArtifacts(input objectstore.PrepareSlotArtifactsInput) (objectstore.PreparedSlotArtifacts, error) {
	assets := make([]imageagent.StagedAssetRef, len(input.Assets))
	ownerKey, err := imageagent.ArtifactOwnerKey(input.Identity.OwnerUserID)
	if err != nil {
		return objectstore.PreparedSlotArtifacts{}, err
	}
	for index, asset := range input.Assets {
		operations, err := imageagent.NormalizeArtifactOperations(asset.Operations)
		if err != nil {
			return objectstore.PreparedSlotArtifacts{}, err
		}
		sum := sha256.Sum256(asset.Bytes)
		hash := hex.EncodeToString(sum[:])
		assets[index] = imageagent.StagedAssetRef{
			ObjectKey: fmt.Sprintf("image-agent/staging/%s/%s/%s/%d/%s/%d/%d-%s.png", input.Identity.TenantID, ownerKey, input.Identity.RunID, input.Identity.PlanRevision, input.Identity.SlotID, input.Identity.Attempt, index, hash),
			SHA256:    hash, SizeBytes: int64(len(asset.Bytes)), ContentType: asset.ContentType, Width: asset.Width, Height: asset.Height,
			SourceAssetID: asset.SourceAssetID, Operations: operations, ProviderReceiptID: asset.ProviderReceiptID,
		}
	}
	manifest, err := imageagent.NormalizeStagingManifest(imageagent.StagingManifest{Assets: assets})
	if err != nil {
		return objectstore.PreparedSlotArtifacts{}, err
	}
	return objectstore.PreparedSlotArtifacts{Manifest: manifest}, nil
}

func (acceptanceDurableArtifactStore) PreserveSlotArtifacts(context.Context, imageagent.SlotExternalEffectIdentity, objectstore.PreparedSlotArtifacts) error {
	return nil
}

func (acceptanceDurableArtifactStore) RecoverSlotArtifacts(_ context.Context, _ imageagent.SlotExternalEffectIdentity, expected imageagent.StagingManifest) (objectstore.PreparedSlotArtifacts, error) {
	if len(expected.Assets) == 0 {
		return objectstore.PreparedSlotArtifacts{}, objectstore.ErrArtifactUnavailable
	}
	return objectstore.PreparedSlotArtifacts{Manifest: expected}, nil
}

func (acceptanceDurableArtifactStore) EnsureStaged(_ context.Context, prepared objectstore.PreparedSlotArtifacts) error {
	return imageagent.ValidateStagingManifest(prepared.Manifest)
}

func (acceptanceDurableArtifactStore) Finalize(ctx context.Context, manifest imageagent.StagingManifest) (imageagent.FinalManifest, error) {
	return acceptanceDurableArtifactStore{}.FinalizeWithProgress(ctx, manifest, nil)
}

func (acceptanceDurableArtifactStore) FinalizeWithProgress(ctx context.Context, manifest imageagent.StagingManifest, progress func(context.Context, int) error) (imageagent.FinalManifest, error) {
	manifest, err := imageagent.NormalizeStagingManifest(manifest)
	if err != nil {
		return imageagent.FinalManifest{}, err
	}
	assets := make([]imageagent.PublishedAssetRef, len(manifest.Assets))
	for index, asset := range manifest.Assets {
		if progress != nil {
			if err := progress(ctx, index); err != nil {
				return imageagent.FinalManifest{}, err
			}
		}
		assets[index] = imageagent.PublishedAssetRef{
			ObjectKey: "image-agent/public/" + asset.ObjectKey[len("image-agent/staging/"):],
			SHA256:    asset.SHA256, SizeBytes: asset.SizeBytes, ContentType: asset.ContentType, Width: asset.Width, Height: asset.Height,
			SourceAssetID: asset.SourceAssetID, Operations: asset.Operations, ProviderReceiptID: asset.ProviderReceiptID,
		}
	}
	return imageagent.NormalizeFinalManifest(imageagent.FinalManifest{Assets: assets})
}

func acceptedSlotEventCount(t *testing.T, events []imageagent.RunEvent) int {
	t.Helper()
	count := 0
	for _, event := range events {
		if event.Type != "slot.result.persisted" {
			continue
		}
		var payload struct {
			Status imageagent.SlotStatus `json:"status"`
		}
		require.NoError(t, json.Unmarshal(event.Payload, &payload))
		if payload.Status == imageagent.SlotStatusAccepted {
			count++
		}
	}
	return count
}

func sortedSlotIDs(plan imageagent.Plan) []string {
	ids := make([]string, len(plan.Slots))
	for index, slot := range plan.Slots {
		ids[index] = slot.ID
	}
	return sortedStrings(ids)
}

func sortedStrings(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
