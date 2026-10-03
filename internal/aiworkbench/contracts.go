// Package aiworkbench owns private Chat conversations, planning receipts and
// the business intent handed to the existing Product Agent owner.
package aiworkbench

import (
	"errors"
	"time"
)

var (
	ErrInvalid              = errors.New("invalid workbench request")
	ErrNotFound             = errors.New("workbench item not found")
	ErrUnavailable          = errors.New("workbench unavailable")
	ErrIdempotencyConflict  = errors.New("workbench idempotency conflict")
	ErrConversationArchived = errors.New("conversation archived")
	ErrRevisionMismatch     = errors.New("conversation revision mismatch")
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

type PreparedPlan struct {
	InputHash    string
	ModelProfile []byte
	Deadline     time.Time
}

type PlanningState string

const (
	PlanningReadyToDispatch      PlanningState = "READY_TO_DISPATCH"
	PlanningComplete             PlanningState = "COMPLETE"
	PlanningFailedBeforeDispatch PlanningState = "FAILED_BEFORE_DISPATCH"
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
	InputHash           string
	ModelProfile        []byte
	StartedAt           time.Time
	Deadline            time.Time
	State               PlanningState
	AssistantMessageID  string
	TerminalDigest      string
	Mode                PlanMode
	CommittedAt         *time.Time
}

type PlanTerminal struct {
	AssistantText string
	Mode          PlanMode
	GoalSummary   string
}
