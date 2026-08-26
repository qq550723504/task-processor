package temporal

import (
	"fmt"
	"os"
	"strings"

	"task-processor/internal/imageagent"
)

const (
	TaskQueue                   = "image-agent-manual"
	EnvTaskQueue                = "IMAGE_AGENT_TEMPORAL_TASK_QUEUE"
	workflowNameImageAgent      = "ImageAgentWorkflow"
	workflowNameImageSlot       = "ImageSlotWorkflow"
	activityExecuteSlot         = "imageagent.execute_slot"
	activityPersistSlotResult   = "imageagent.persist_slot_result"
	activityPersistRunState     = "imageagent.persist_run_state"
	activityPersistPlanRevision = "imageagent.persist_plan_revision"
	activityPublishApproved     = "imageagent.publish_approved"
	signalApproveResults        = "approve_results"
	signalRetrySlot             = "retry_slot"
	signalReplacePlan           = "replace_plan"
	signalCancel                = "cancel"
	updateErrorRevisionConflict = "imageagent_revision_conflict"
	updateErrorCommandBlocked   = "imageagent_command_blocked"
	updateErrorRunNotFound      = "imageagent_run_not_found"
	defaultMaxConcurrentSlots   = 4
	QueryWorkflowProjection     = "image_agent_projection"
)

type WorkflowInput struct {
	RunID              string
	Mode               imageagent.RunMode
	Identity           imageagent.ExecutionIdentity
	Plan               imageagent.Plan
	MaxConcurrentSlots int
	WaitForCommands    bool
}

type WorkflowResult struct {
	Status           imageagent.RunStatus
	Block            *imageagent.Block
	Plan             imageagent.Plan
	Slots            []imageagent.SlotProjection
	CompletedSlotIDs []string
	ResultDigest     string
}

type SlotWorkflowInput struct {
	RunID        string
	Identity     imageagent.ExecutionIdentity
	PlanRevision int64
	Slot         imageagent.Slot
	Attempt      int
}

type SlotWorkflowResult struct {
	Execution imageagent.SlotExecutionResult
	Status    imageagent.SlotStatus
	ErrorCode string
}

type ExecuteSlotActivityInput struct {
	RunID          string
	Identity       imageagent.ExecutionIdentity
	PlanRevision   int64
	Slot           imageagent.Slot
	Attempt        int
	IdempotencyKey string
}

type PersistSlotResultActivityInput struct {
	RunID        string
	Identity     imageagent.ExecutionIdentity
	PlanRevision int64
	Result       SlotWorkflowResult
	AttemptKey   string
}

type PersistRunStateActivityInput struct {
	RunID        string
	Identity     imageagent.ExecutionIdentity
	PlanRevision int64
	Status       imageagent.RunStatus
	CurrentNode  string
	Block        *imageagent.Block
}

type PersistPlanRevisionActivityInput struct {
	RunID            string
	Identity         imageagent.ExecutionIdentity
	ExpectedRevision int64
	Plan             imageagent.Plan
}

type PublishApprovedActivityInput struct {
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

type CommandAcknowledgement struct {
	RunID        string
	PlanRevision int64
	ActionID     string
	Status       imageagent.RunStatus
}

func TaskQueueName() string {
	if configured := strings.TrimSpace(os.Getenv(EnvTaskQueue)); configured != "" {
		return configured
	}
	return TaskQueue
}

func WorkflowID(tenantID, runID string) string {
	return fmt.Sprintf("image-agent:%s:%s", strings.TrimSpace(tenantID), strings.TrimSpace(runID))
}

func slotAttemptKey(slot imageagent.Slot, attempt int) string {
	return fmt.Sprintf("%s:attempt:%d", slot.IdempotencyKey, attempt)
}

func publicationKey(runID string, revision int64) string {
	return fmt.Sprintf("image-agent:%s:plan:%d:publication", strings.TrimSpace(runID), revision)
}

func childWorkflowID(input SlotWorkflowInput) string {
	return fmt.Sprintf("%s:plan:%d:slot:%s:attempt:%d", WorkflowID(input.Identity.TenantID, input.RunID), input.PlanRevision, input.Slot.ID, input.Attempt)
}
