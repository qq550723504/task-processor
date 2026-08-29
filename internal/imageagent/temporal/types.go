package temporal

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"strings"
	"time"

	"task-processor/internal/imageagent"
	"task-processor/internal/imageagent/objectstore"
)

const (
	// V3WorkflowExecutionTimeout deliberately leaves Temporal's optional server
	// execution deadline unset. Manual runs can wait for operator approval for
	// an unbounded period; configured MaxElapsed remains the business budget.
	V3WorkflowExecutionTimeout         = time.Duration(0)
	V3MinimumStagingLifecycleRetention = 45 * 24 * time.Hour
)

const (
	TaskQueue                           = "image-agent-manual"
	TaskQueueV3                         = "image-agent-manual-v3"
	TaskQueueV3Canary                   = "image-agent-manual-v3-canary"
	EnvTaskQueue                        = "IMAGE_AGENT_TEMPORAL_TASK_QUEUE"
	workflowNameImageAgent              = "ImageAgentWorkflow"
	workflowNameImageSlot               = "ImageSlotWorkflow"
	activityExecuteSlotLegacy           = "imageagent.execute_slot"
	activityPersistSlotResultLegacy     = "imageagent.persist_slot_result"
	activityPersistRunStateLegacy       = "imageagent.persist_run_state"
	activityPersistPlanRevisionLegacy   = "imageagent.persist_plan_revision"
	activityPersistPendingCommandLegacy = "imageagent.persist_pending_command"
	activityPublishApprovedLegacy       = "imageagent.publish_approved"
	activityExecuteSlot                 = "imageagent.execute_slot.v2"
	activityPersistSlotResult           = "imageagent.persist_slot_result.v2"
	activityPersistSlotResultV3         = "imageagent.persist_slot_result.v3"
	activityPersistRunState             = "imageagent.persist_run_state.v2"
	activityPersistWorkflowFailure      = "imageagent.persist_workflow_failure.v1"
	activityPersistWorkflowFailureV2    = "imageagent.persist_workflow_failure.v2"
	activityPersistPlanRevision         = "imageagent.persist_plan_revision.v2"
	activityPersistPendingCommand       = "imageagent.persist_pending_command.v2"
	activityPublishApproved             = "imageagent.publish_approved.v2"
	activityExecuteSlotV3               = "imageagent.execute_slot.v3"
	activityStartEffectRecoveryV3       = "imageagent.start_effect_recovery.v3"
	activityRecoverEffectV3             = "imageagent.recover_effect.v3"
	activityPersistRecoveryBlockedV3    = "imageagent.persist_recovery_blocked.v3"
	activityReconcileEffectRecoveryV3   = "imageagent.reconcile_effect_recovery.v3"
	activityPublishApprovedV3           = "imageagent.publish_approved.v3"
	workflowNameCompatibilityCanary     = "ImageAgentCompatibilityCanaryWorkflow"
	signalApproveResults                = "approve_results"
	signalEffectRecoveryCompleted       = "effect_recovery_completed"
	signalRetrySlot                     = "retry_slot"
	signalReplacePlan                   = "replace_plan"
	signalCancel                        = "cancel"
	updateResumeCommand                 = "resume_command"
	updateErrorRevisionConflict         = "imageagent_revision_conflict"
	updateErrorCommandBlocked           = "imageagent_command_blocked"
	updateErrorRunNotFound              = "imageagent_run_not_found"
	updateErrorLegacyMigrationRequired  = "imageagent_legacy_migration_required"
	defaultMaxConcurrentSlots           = 4
	QueryWorkflowProjection             = "image_agent_projection"
	EffectRecoveryWorkflowName          = "ImageAgentEffectRecoveryWorkflow"
	effectRecoveryBlockedCode           = imageagent.SlotRecoveryBlockedCode
)

type WorkerWireMode string

const (
	WorkerWireModeV2 WorkerWireMode = "v2"
	WorkerWireModeV3 WorkerWireMode = "v3"
)

func (mode WorkerWireMode) DefaultTaskQueue() (string, error) {
	switch mode {
	case WorkerWireModeV2:
		return TaskQueue, nil
	case WorkerWireModeV3:
		return TaskQueueV3, nil
	case "":
		return "", fmt.Errorf("image agent temporal wire mode is required")
	default:
		return "", fmt.Errorf("unsupported image agent temporal wire mode %q", mode)
	}
}

type WorkflowInput struct {
	RunID               string
	Mode                imageagent.RunMode
	Identity            imageagent.ExecutionIdentity
	Plan                imageagent.Plan
	MaxConcurrentSlots  int
	WaitForCommands     bool
	AssetCatalog        imageagent.AssetCatalog
	BudgetPolicy        imageagent.BudgetPolicy
	StartedAt           time.Time
	DeadlineAt          time.Time
	BudgetAuthorization bool
	// externalEffectFinalization is runtime-only versioned behavior selected by
	// the parent workflow before it launches v3 slot children.
	externalEffectFinalization bool
	// enforceIngressPlanPolicy is runtime-only versioned behavior for new
	// workflow histories. It is never serialized from clients.
	enforceIngressPlanPolicy bool
	// projectionExecutionID is runtime-only state populated deterministically
	// from Temporal's workflow execution RunID, never serialized from clients.
	projectionExecutionID string
}

type WorkflowResult struct {
	Status             imageagent.RunStatus
	Block              *imageagent.Block
	Plan               imageagent.Plan
	Slots              []imageagent.SlotProjection
	RecoverableEffects []imageagent.RecoverableEffect
	CompletedSlotIDs   []string
	ResultDigest       string
	PendingCommand     *imageagent.PendingCommandReceipt
	CommandIngress     imageagent.CommandIngress
}

type SlotWorkflowInput struct {
	RunID        string
	Identity     imageagent.ExecutionIdentity
	PlanRevision int64
	Slot         imageagent.Slot
	Attempt      int
	AssetCatalog imageagent.AssetCatalog
}

type SlotWorkflowResult struct {
	Execution imageagent.SlotExecutionResult
	Status    imageagent.SlotStatus
	ErrorCode string
	// EffectPhase is additive v3 evidence used only by the versioned parent
	// cancellation gate. Frozen v2 and old-history results leave it empty.
	EffectPhase imageagent.SlotEffectV3Phase
}

// SlotWorkflowV3Input is additive and is not registered by the Task 4 worker.
// Task 6 owns selecting this child workflow on the production wire.
type SlotWorkflowV3Input struct {
	RunID        string
	Identity     imageagent.ExecutionIdentity
	PlanRevision int64
	Slot         imageagent.Slot
	Attempt      int
	AssetCatalog imageagent.AssetCatalog
	// ExecuteActivityName is supplied by the Task 6 wire-selection gate. Task 4
	// deliberately defines no production v3 activity name or registration.
	ExecuteActivityName        string
	BudgetAuthorization        bool
	BudgetPolicy               imageagent.BudgetPolicy
	DeadlineAt                 time.Time
	ExternalEffectFinalization bool
}

type SlotWorkflowV3Result struct {
	Published imageagent.SlotEffectV3PublishedResult
	Status    imageagent.SlotStatus
	ErrorCode string
	// EffectPhase proves the durable external-effect phase observed by the child.
	// SlotStatusBlocked alone is deliberately not terminalization evidence.
	EffectPhase imageagent.SlotEffectV3Phase
}

type EffectRecoveryWorkflowInput struct {
	RunID        string
	Identity     imageagent.ExecutionIdentity
	PlanRevision int64
	Slot         imageagent.Slot
	Attempt      int
	// ActionID is empty for automatic recovery and historical workflow inputs.
	// Explicit redrives use it to scope a restartable workflow execution.
	ActionID     string
	AssetCatalog imageagent.AssetCatalog
}

type EffectRecoveryOutcome string

const (
	EffectRecoveryOutcomePublished          EffectRecoveryOutcome = "published"
	EffectRecoveryOutcomeProviderUnknown    EffectRecoveryOutcome = "provider_unknown"
	EffectRecoveryOutcomeStagingUnknown     EffectRecoveryOutcome = "staging_unknown"
	EffectRecoveryOutcomePublicationUnknown EffectRecoveryOutcome = "publication_unknown"
	EffectRecoveryOutcomeRecoveryBlocked    EffectRecoveryOutcome = "recovery_blocked"
)

type EffectRecoveryResult struct {
	Outcome     EffectRecoveryOutcome
	Published   imageagent.SlotEffectV3PublishedResult
	EffectPhase imageagent.SlotEffectV3Phase
	BlockedCode string
}

// EffectRecoveryCompletedSignal is emitted after the recovery workflow has
// durably reconciled its external effect. The parent uses the identity tuple
// to ignore stale or duplicate completions while refreshing its in-memory
// projection and pending cancellation intent.
type EffectRecoveryCompletedSignal struct {
	RunID        string
	PlanRevision int64
	SlotID       string
	Attempt      int
	Result       EffectRecoveryResult
}

type ExecuteSlotActivityInput struct {
	RunID          string
	Identity       imageagent.ExecutionIdentity
	PlanRevision   int64
	Slot           imageagent.Slot
	Attempt        int
	IdempotencyKey string
	AssetCatalog   imageagent.AssetCatalog
}

// ExecuteSlotV3ActivityInput is additive until Task 6 selects the v3 wire.
// Keep ExecuteSlotActivityInput frozen for imageagent.execute_slot.v2 replay.
type ExecuteSlotV3ActivityInput struct {
	RunID                      string
	Identity                   imageagent.ExecutionIdentity
	PlanRevision               int64
	Slot                       imageagent.Slot
	Attempt                    int
	IdempotencyKey             string
	AssetCatalog               imageagent.AssetCatalog
	ExternalEffectFinalization bool
	BudgetAuthorization        bool
	BudgetPolicy               imageagent.BudgetPolicy
	DeadlineAt                 time.Time
}

// DurableArtifactStore is the production/recovery boundary around deterministic
// staging and publication objects. Generated bytes are preserved as one
// immutable recovery bundle before the staging phase is persisted, so a fresh
// activity can rehydrate a partially uploaded manifest without regeneration.
type DurableArtifactStore interface {
	imageagent.DurableAssetPublicURLResolver
	PrepareSlotArtifacts(objectstore.PrepareSlotArtifactsInput) (objectstore.PreparedSlotArtifacts, error)
	PreserveSlotArtifacts(context.Context, imageagent.SlotExternalEffectIdentity, objectstore.PreparedSlotArtifacts) error
	RecoverSlotArtifacts(context.Context, imageagent.SlotExternalEffectIdentity, imageagent.StagingManifest) (objectstore.PreparedSlotArtifacts, error)
	EnsureStaged(context.Context, objectstore.PreparedSlotArtifacts) error
	Finalize(context.Context, imageagent.StagingManifest) (imageagent.FinalManifest, error)
	FinalizeWithProgress(context.Context, imageagent.StagingManifest, func(context.Context, int) error) (imageagent.FinalManifest, error)
}

type PersistSlotResultActivityInput struct {
	RunID        string
	Identity     imageagent.ExecutionIdentity
	PlanRevision int64
	Result       SlotWorkflowResult
	AttemptKey   string
}

// PersistSlotResultV3ActivityInput is an additive durable-result contract. It
// is deliberately not registered under imageagent.persist_slot_result.v2.
type PersistSlotResultV3ActivityInput struct {
	RunID        string
	Identity     imageagent.ExecutionIdentity
	PlanRevision int64
	Result       SlotWorkflowV3Result
	AttemptKey   string
}

type PersistRunStateActivityInput struct {
	RunID        string
	Identity     imageagent.ExecutionIdentity
	PlanRevision int64
	Projection   WorkflowResult
	CurrentNode  string
	CommitID     string
}

type PersistWorkflowFailureActivityInput struct {
	RunID          string
	Identity       imageagent.ExecutionIdentity
	FailureCode    string
	FailureMessage string
}

type PersistWorkflowFailureV2ActivityInput struct {
	RunID          string
	Identity       imageagent.ExecutionIdentity
	FailureCode    string
	FailureMessage string
	CommitID       string
}

type PersistPlanRevisionActivityInput struct {
	RunID            string
	Identity         imageagent.ExecutionIdentity
	ExpectedRevision int64
	Plan             imageagent.Plan
}

type PersistPendingCommandActivityInput struct {
	RunID          string
	Identity       imageagent.ExecutionIdentity
	Receipt        *imageagent.PendingCommandReceipt
	CommitID       string
	CommandIngress imageagent.CommandIngress
}

// Legacy activity payloads are frozen wire contracts. Their handlers never
// write owner-scoped v2 state; they fail with an explicit migration error.
type LegacyExecuteSlotActivityInput struct {
	RunID          string
	Identity       imageagent.ExecutionIdentity
	PlanRevision   int64
	Slot           imageagent.Slot
	Attempt        int
	IdempotencyKey string
	AssetCatalog   imageagent.AssetCatalog
}

type LegacyPersistSlotResultActivityInput struct {
	RunID        string
	Identity     imageagent.ExecutionIdentity
	PlanRevision int64
	Result       SlotWorkflowResult
	AttemptKey   string
}

type LegacyPersistRunStateActivityInput struct {
	RunID        string
	Identity     imageagent.ExecutionIdentity
	PlanRevision int64
	Status       imageagent.RunStatus
	CurrentNode  string
	Block        *imageagent.Block
}

type LegacyPersistPlanRevisionActivityInput struct {
	RunID            string
	Identity         imageagent.ExecutionIdentity
	ExpectedRevision int64
	Plan             imageagent.Plan
}

type LegacyPersistPendingCommandActivityInput struct {
	RunID    string
	Identity imageagent.ExecutionIdentity
	Receipt  *imageagent.PendingCommandReceipt
	CommitID string
}

type LegacyPublishApprovedActivityInput struct {
	RunID             string
	Identity          imageagent.ExecutionIdentity
	PlanRevision      int64
	CandidateAssetIDs []string
	IdempotencyKey    string
}

type PublishApprovedActivityInput struct {
	RunID             string
	Identity          imageagent.ExecutionIdentity
	PlanRevision      int64
	CandidateAssetIDs []string
	IdempotencyKey    string
}

// PublishApprovedV3ActivityInput is a distinct durable-approval wire payload.
// Task 6 owns its Temporal registration and workflow routing.
type PublishApprovedV3ActivityInput struct {
	RunID             string
	Identity          imageagent.ExecutionIdentity
	PlanRevision      int64
	CandidateAssetIDs []string
	IdempotencyKey    string
}

type ApproveResultsSignal struct {
	RunID        string
	PlanRevision int64
	ResultDigest string
	ActorID      string
	ActionID     string
}

type RetrySlotSignal struct {
	RunID        string
	PlanRevision int64
	SlotID       string
	ActorID      string
	ActionID     string
}

type ReplacePlanSignal struct {
	RunID            string
	ExpectedRevision int64
	Plan             imageagent.Plan
	ActorID          string
	ActionID         string
}

type CancelSignal struct {
	RunID        string
	PlanRevision int64
	ActorID      string
	ActionID     string
}

type ResumeCommandInput struct {
	RunID    string
	ActorID  string
	ActionID string
}

type CommandAcknowledgement = imageagent.CommandAcknowledgement

func TaskQueueName() string {
	if configured := strings.TrimSpace(os.Getenv(EnvTaskQueue)); configured != "" {
		return configured
	}
	return TaskQueue
}

func WorkflowID(tenantID, ownerUserID, runID string) string {
	return fmt.Sprintf("image-agent:%s:%s:%s", strings.TrimSpace(tenantID), strings.TrimSpace(ownerUserID), strings.TrimSpace(runID))
}

func slotAttemptKey(planRevision int64, slot imageagent.Slot, attempt int) string {
	return fmt.Sprintf("%s:plan:%d:attempt:%d", slot.IdempotencyKey, planRevision, attempt)
}

func publicationKey(runID string, revision int64) string {
	return fmt.Sprintf("image-agent:%s:plan:%d:publication", strings.TrimSpace(runID), revision)
}

func childWorkflowID(input SlotWorkflowInput) string {
	return fmt.Sprintf("%s:plan:%d:slot:%s:attempt:%d", WorkflowID(input.Identity.TenantID, input.Identity.UserID, input.RunID), input.PlanRevision, input.Slot.ID, input.Attempt)
}

// EffectRecoveryWorkflowID keeps the required Task 1 signature while still
// encoding the spec-mandated run and slot identity. The combined string is
// expected to be "<run-id>:<slot-id>" and is split on the last colon.
func EffectRecoveryWorkflowID(identity imageagent.ExecutionIdentity, planRevision int64, runSlot string, attempt int) string {
	runSlot = strings.TrimSpace(runSlot)
	runID, slotID := runSlot, ""
	if separator := strings.LastIndex(runSlot, ":"); separator >= 0 {
		runID = strings.TrimSpace(runSlot[:separator])
		slotID = strings.TrimSpace(runSlot[separator+1:])
	}
	return fmt.Sprintf(
		"image-agent-effect-recovery:%s:%s:%s:%d:%s:%d",
		strings.TrimSpace(identity.TenantID),
		strings.TrimSpace(identity.UserID),
		runID,
		planRevision,
		slotID,
		attempt,
	)
}

// EffectRecoveryWorkflowIDForSlot builds an ID from structured fields. Empty
// action IDs retain the original deterministic ID for automatic handoff and
// historical replay compatibility.
func EffectRecoveryWorkflowIDForSlot(identity imageagent.ExecutionIdentity, planRevision int64, runID, slotID string, attempt int, actionID string) string {
	tenantID := strings.TrimSpace(identity.TenantID)
	userID := strings.TrimSpace(identity.UserID)
	runID = strings.TrimSpace(runID)
	slotID = strings.TrimSpace(slotID)
	base := ""
	if strings.ContainsAny(tenantID+userID+runID+slotID, ":") {
		encode := func(value string) string {
			return base64.RawURLEncoding.EncodeToString([]byte(value))
		}
		base = fmt.Sprintf("image-agent-effect-recovery:v2:%s:%s:%s:%d:%s:%d", encode(tenantID), encode(userID), encode(runID), planRevision, encode(slotID), attempt)
	} else {
		base = fmt.Sprintf("image-agent-effect-recovery:%s:%s:%s:%d:%s:%d", tenantID, userID, runID, planRevision, slotID, attempt)
	}
	if strings.TrimSpace(actionID) == "" {
		return base
	}
	return base + ":action:" + base64.RawURLEncoding.EncodeToString([]byte(actionID))
}
