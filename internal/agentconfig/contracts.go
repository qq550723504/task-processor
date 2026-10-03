// Package agentconfig owns enterprise activation and versioned configuration.
// Runtime, Knowledge, provider usage and Product review retain their owners.
package agentconfig

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"strings"
	"task-processor/internal/agent"
	"task-processor/internal/aicapability"
	"task-processor/internal/commercetool"
	"time"
	"unicode"
	"unicode/utf8"
)

var (
	ErrInvalid      = errors.New("INVALID_REQUEST")
	ErrForbidden    = errors.New("FORBIDDEN")
	ErrNotFound     = errors.New("NOT_FOUND")
	ErrConflict     = errors.New("IDEMPOTENCY_CONFLICT")
	ErrChanged      = errors.New("CONFIGURATION_CHANGED")
	ErrNotEnabled   = errors.New("AGENT_NOT_ENABLED")
	ErrArchived     = errors.New("TEMPLATE_ARCHIVED")
	ErrDefault      = errors.New("TEMPLATE_IS_DEFAULT")
	ErrDefinition   = errors.New("AGENT_DEFINITION_UNAVAILABLE")
	ErrRevision     = errors.New("REVISION_MISMATCH")
	ErrPrecondition = errors.New("PRECONDITION_REQUIRED")
	ErrUnavailable  = errors.New("DEPENDENCY_UNAVAILABLE")
)

const SnapshotKind = "agent-configuration-v1"
const ParameterSchema = "title-config-v1"

func UUID(s string) bool {
	id, err := uuid.Parse(s)
	return err == nil && id != uuid.Nil && id.String() == s
}
func Platform(s string) bool { return s == "shein" || s == "temu" || s == "amazon" }
func LimitsAdmissible(f, c agent.Limits) bool {
	return f.Valid() && c.Valid() && f.Currency == c.Currency && f.Steps <= c.Steps && f.ModelCalls <= c.ModelCalls && f.Tokens <= c.Tokens && f.CostMicros <= c.CostMicros && f.Runtime <= c.Runtime
}

type TemplateInput struct {
	Name                   string `json:"name"`
	TargetPlatform         string `json:"targetPlatform"`
	DefaultKnowledgeBaseID string `json:"defaultKnowledgeBaseId,omitempty"`
}

func (p TemplateInput) Valid() bool {
	n := strings.TrimSpace(p.Name)
	return len(n) <= 512 && utf8.ValidString(n) && utf8.RuneCountInString(n) > 0 && utf8.RuneCountInString(n) <= 120 && strings.IndexFunc(n, unicode.IsControl) < 0 && Platform(p.TargetPlatform) && (p.DefaultKnowledgeBaseID == "" || UUID(p.DefaultKnowledgeBaseID))
}

type TemplateRef struct {
	TemplateID string `json:"templateId"`
	Revision   string `json:"revision"`
}
type OrganizationAgent struct {
	AgentID         string       `json:"agentId"`
	Activation      string       `json:"activation"`
	Revision        string       `json:"revision"`
	ActivationEpoch string       `json:"activationEpoch"`
	DefaultTemplate *TemplateRef `json:"defaultTemplate"`
	UpdatedAt       time.Time    `json:"updatedAt"`
}
type Template struct {
	TemplateID    string `json:"templateId"`
	AgentID       string `json:"agentId"`
	Lifecycle     string `json:"lifecycle"`
	Revision      string `json:"revision"`
	Version       string `json:"version"`
	SchemaVersion string `json:"schemaVersion"`
	TemplateInput
	CreatedAt time.Time `json:"createdAt"`
}
type Command struct {
	Scope                               agent.Scope
	Key, AgentID, Operation, TemplateID string
	Expected                            uint64
	Absent                              bool
	Input                               TemplateInput
	Default                             *TemplateRef
}
type Receipt struct {
	CommandID      string    `json:"commandId"`
	Operation      string    `json:"operation"`
	AgentID        string    `json:"agentId"`
	TemplateID     string    `json:"templateId,omitempty"`
	BeforeRevision string    `json:"beforeRevision,omitempty"`
	Revision       string    `json:"revision"`
	Version        string    `json:"version,omitempty"`
	Noop           bool      `json:"noop"`
	CommittedAt    time.Time `json:"committedAt"`
}
type StartCommand struct {
	Scope                                  agent.Scope
	AgentID, AgentVersion, KnowledgeBaseID string
	Template                               *TemplateRef
	Request                                agent.Request
	ExecutionModelProfile                  aicapability.ModelProfile
}
type Snapshot struct {
	ID, Digest                                                   string
	Scope                                                        agent.Scope
	AgentID, AgentVersion, KnowledgeBaseID, Epoch, AgentRevision string
	Template                                                     *TemplateRef
	Request                                                      agent.Request
	ExecutionModelProfile                                        aicapability.ModelProfile
}
type Capability struct {
	ID         string    `json:"id"`
	Support    string    `json:"support"`
	Readiness  string    `json:"readiness"`
	Reason     string    `json:"reason"`
	ObservedAt time.Time `json:"observedAt"`
}
type CatalogEntry struct {
	Definition                         commercetool.AgentDefinition
	Name, Description, ParameterSchema string
}
type CatalogReader interface {
	ReadCatalog(context.Context) ([]CatalogEntry, error)
}
type Repository interface {
	ReadAgent(context.Context, agent.Scope, string) (OrganizationAgent, error)
	ListAgents(context.Context, agent.Scope, string, string, int) ([]OrganizationAgent, string, error)
	Templates(context.Context, agent.Scope, string, string, string, int) ([]Template, string, error)
	ReadTemplate(context.Context, agent.Scope, string, string, uint64) (Template, error)
	Execute(context.Context, Command, ...func(context.Context) error) (Receipt, error)
	Prepare(context.Context, StartCommand) (Snapshot, error)
	LoadSnapshot(context.Context, agent.Scope, agent.ConfigurationSnapshotRef) (Snapshot, error)
}
