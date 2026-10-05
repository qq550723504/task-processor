// Package aiworkbench owns private Chat conversations, planning receipts and
// the business intent handed to the existing Product Agent owner.
package aiworkbench

import (
	"errors"
	"time"

	"task-processor/internal/agent"
)

var (
	ErrInvalid              = errors.New("invalid workbench request")
	ErrNotFound             = errors.New("workbench item not found")
	ErrUnavailable          = errors.New("workbench unavailable")
	ErrIdempotencyConflict  = errors.New("workbench idempotency conflict")
	ErrConversationArchived = errors.New("conversation archived")
	ErrRevisionMismatch     = errors.New("conversation revision mismatch")
	ErrProposalStale        = errors.New("workbench proposal stale")
	ErrTaskOutcomeUnknown   = errors.New("workbench task action outcome unknown")
)

type Scope struct {
	OrganizationID string
	ActorID        string
}

type Conversation struct {
	ID               string
	Scope            Scope
	Title            string
	Favorite         bool
	Archived         bool
	MetadataRevision uint64
	NextSequence     uint64
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type CreateInput struct {
	Favorite bool
}

type MetadataChange struct {
	Title    *string
	Favorite *bool
	Archived *bool
}

type MessageAuthor string

const (
	AuthorUser      MessageAuthor = "USER"
	AuthorAssistant MessageAuthor = "ASSISTANT"
)

type Message struct {
	ID             string
	ConversationID string
	Sequence       uint64
	Author         MessageAuthor
	Content        string
	CreatedAt      time.Time
}

type MessageInput struct {
	Content          string
	OperationID      string
	TargetPlatform   string
	TemplateID       string
	TemplateRevision string
	KnowledgeBaseID  string
}

// WorkScope is the exact server-authorized selection frozen with a planning
// command. It contains no mutable provider or model-selected execution facts.
type WorkScope struct {
	OperationID      string `json:"operation_id"`
	TargetPlatform   string `json:"target_platform"`
	TemplateID       string `json:"template_id,omitempty"`
	TemplateRevision string `json:"template_revision,omitempty"`
	KnowledgeBaseID  string `json:"knowledge_base_id,omitempty"`
}

func (m MessageInput) WorkScope() WorkScope {
	return WorkScope{OperationID: m.OperationID, TargetPlatform: m.TargetPlatform,
		TemplateID: m.TemplateID, TemplateRevision: m.TemplateRevision, KnowledgeBaseID: m.KnowledgeBaseID}
}

type PreparedPlan struct {
	MemberID     string
	InputHash    string
	ModelProfile []byte
	Deadline     time.Time
	Unavailable  bool
}

// PlanPreparer is a pure calculation over the exact sequence prefix selected
// under the Conversation row lock. It must not perform external owner I/O.
type PlanPreparer func(history []Message, invocationID string) (PreparedPlan, error)

type PlanningState string

const (
	PlanningReadyToDispatch      PlanningState = "READY_TO_DISPATCH"
	PlanningComplete             PlanningState = "COMPLETE"
	PlanningFailedBeforeDispatch PlanningState = "FAILED_BEFORE_DISPATCH"
	PlanningInvalidOutput        PlanningState = "PLANNER_INVALID_OUTPUT"
	PlanningUnknown              PlanningState = "PLANNER_UNKNOWN"
)

type PlanMode string

const (
	PlanClarify PlanMode = "CLARIFY"
	PlanReady   PlanMode = "READY"
)

type PlanningCommand struct {
	Scope               Scope
	IdempotencyKey      string
	Operation           string
	ConversationID      string
	RequestFingerprint  string
	UserMessageID       string
	SourceSequence      uint64
	PlannerInvocationID string
	MemberID            string
	InputHash           string
	WorkScope           WorkScope
	ModelProfile        []byte
	StartedAt           time.Time
	Deadline            time.Time
	State               PlanningState
	AssistantMessageID  string
	ProposalID          string
	TerminalDigest      string
	Mode                PlanMode
	CommittedAt         *time.Time
}

type PlanTerminal struct {
	AssistantText string
	Mode          PlanMode
	GoalSummary   string
	Proposal      *ExecutionProposal
}

// ExecutionProposal is immutable business intent for one Product title
// suggestion. The planning model supplies only GoalSummary; all remaining
// facts come from freshly authorized Product, AgentConfig and Knowledge owners.
type ExecutionProposal struct {
	ID                         string
	Digest                     string
	Scope                      Scope
	ConversationID             string
	SourceUserMessageID        string
	AssistantMessageID         string
	SourceSequence             uint64
	Kind                       string
	GoalSummary                string
	OperationID                string
	ProductKey                 string
	CatalogVersion             string
	PublicationID              string
	TargetPlatform             string
	AgentID                    string
	AgentVersion               string
	ObservedAgentRevision      string
	ObservedActivationEpoch    string
	TemplateID                 string
	TemplateRevision           string
	KnowledgeBaseID            string
	KnowledgeRevisionSetDigest string
	ExecutionModelProfile      []byte
	CreatedAt                  time.Time
}

// BusinessTask is the durable confirmed intent and exact Agent handoff. Its
// visible state is projected from Agent and Review owners, not stored here.
type BusinessTask struct {
	ID                       string
	Scope                    Scope
	ConversationID           string
	SourceMessageID          string
	ProposalID               string
	ProposalDigest           string
	ConfirmationFingerprint  string
	Kind                     string
	Title                    string
	GoalSummary              string
	OperationID              string
	ProductKey               string
	TargetPlatform           string
	AgentID                  string
	AgentVersion             string
	ExecutionRequestKey      string
	ConfigurationSnapshotRef agent.ConfigurationSnapshotRef
	ContextSnapshotRef       agent.ContextSnapshotRef
	ExecutionRequestDigest   string
	ExecutionRequest         []byte
	CreatedAt                time.Time
}

type PreparedTask struct {
	ProposalDigest           string
	ConfigurationSnapshotRef agent.ConfigurationSnapshotRef
	ContextSnapshotRef       agent.ContextSnapshotRef
	ExecutionRequest         []byte
}

type TaskActionKind string

const (
	TaskActionStart  TaskActionKind = "start"
	TaskActionResume TaskActionKind = "resume"
	TaskActionReview TaskActionKind = "review"
)

type TaskActionState string

const (
	TaskActionClaimed  TaskActionState = "CLAIMED"
	TaskActionComplete TaskActionState = "COMPLETE"
	TaskActionFailed   TaskActionState = "FAILED"
	TaskActionUnknown  TaskActionState = "UNKNOWN"
)

type TaskActionInput struct {
	TaskID, Key string
	Action      TaskActionKind
	Revision    uint64
	Feedback    string
}

type TaskActionReceipt struct {
	Scope       Scope
	TaskID, Key string
	Action      TaskActionKind
	Revision    uint64
	Fingerprint string
	State       TaskActionState
	ErrorCode   string
	CreatedAt   time.Time
	FinishedAt  *time.Time
}
