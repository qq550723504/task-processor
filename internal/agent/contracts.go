// Package agent defines bounded inference contracts. It owns neither Product
// facts, provider execution nor durable business workflows. There is no default
// runtime, HTTP registration, credential or persistence implementation here.
package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"task-processor/internal/commercetool"
	"task-processor/internal/product/enrichment"
)

const MaxStateBytes = 2 << 20
const MaxModelOutputBytes = 64 << 10

var ErrConflict = errors.New("agent run conflict")
var ErrInvalid = errors.New("invalid agent request")
var ErrUnavailable = errors.New("agent dependency unavailable")

type StopReason string

const (
	StopSteps         StopReason = "budget_steps"
	StopModelCalls    StopReason = "budget_model_calls"
	StopTokens        StopReason = "budget_tokens"
	StopCost          StopReason = "budget_cost"
	StopRuntime       StopReason = "budget_runtime"
	StopUsageUnknown  StopReason = "usage_unknown"
	StopRepairLimit   StopReason = "repair_limit"
	StopCancelled     StopReason = "cancelled"
	StopUnauthorized  StopReason = "unauthorized"
	StopInvalidOutput StopReason = "invalid_model_output"
	StopTool          StopReason = "tool_error"
	StopAudit         StopReason = "audit_unavailable"
	StopDependency    StopReason = "dependency_unavailable"
	StopModelUnknown  StopReason = "model_outcome_unknown"
	StopTooLarge      StopReason = "state_too_large"
)

type Phase string

const (
	Running             Phase = "running"
	Interrupted         Phase = "interrupted"
	HumanReviewRequired Phase = "human_review_required"
	Stopped             Phase = "stopped"
)

type Binding struct {
	ContextKind, ContextID                                    string
	ProductKey, CatalogVersion, PublicationID, TargetPlatform string
}

func (b Binding) Valid() bool {
	for _, s := range []string{b.ContextKind, b.ContextID, b.ProductKey, b.PublicationID, b.TargetPlatform} {
		if !ValidID(s) {
			return false
		}
	}
	v, err := strconv.ParseUint(b.CatalogVersion, 10, 63)
	return err == nil && v > 0 && strconv.FormatUint(v, 10) == b.CatalogVersion
}

func ValidID(s string) bool {
	return s != "" && len(s) <= 128 && utf8.ValidString(s) && strings.TrimSpace(s) == s && strings.IndexFunc(s, unicode.IsControl) < 0
}

// Scope comes from the current authorizer, never model or browser payload.
type Scope struct{ OrganizationID, ActorID string }
type Request struct {
	Key                          string
	Binding                      Binding
	PolicyVersion, PromptVersion string
	Limits                       Limits
}

// Authorizer freshly resolves current identity/grants AND validates the exact
// owner-qualified binding. A cached principal alone cannot satisfy this port.
type Authorizer interface {
	Authorize(context.Context, Binding) (Scope, error)
}

type Quote struct {
	Tokens, CostMicros int64
	Currency           string
	Known              bool
}
type ObservedUsage struct {
	Tokens, CostMicros int64
	Currency           string
	Known              bool
}
type Action struct {
	// Kind is tool, propose or interrupt. No arbitrary next-node name is accepted.
	Kind       string
	Tool       commercetool.ToolRef
	Candidate  enrichment.Candidate
	Unresolved []string
	Confidence []FieldConfidence
}

// FieldConfidence is model-reported, never an approval or truth score. Missing
// confidence remains Known=false; domain quality is not substituted for it.
type FieldConfidence struct {
	Field string
	Value float64
	Known bool
}
type ModelResult struct {
	Action       Action
	InvocationID string
	Usage        ObservedUsage
}
type ModelInput struct {
	Binding                                    Binding
	PolicyVersion, PromptVersion, InvocationID string
	History                                    []Observation
	Validation                                 *Validation
	UpperBound                                 Quote
	UserFeedback                               string
	AgentRunID, AgentID, AgentVersion, TraceID string
}

// GovernedModel must enforce its quote, honor the deadline and never silently
// retry/fail over. The real text implementation is a separate #130 dependency.
// It returns authoritative invocation/usage facts, not model-authored counters.
type GovernedModel interface {
	Quote(context.Context, ModelInput) (Quote, error)
	Decide(context.Context, ModelInput) (ModelResult, error)
}

type ToolGateway interface {
	Invoke(context.Context, commercetool.ToolRef, commercetool.CallMetadata, Binding) (commercetool.Result, error)
}

type Validation struct {
	Valid                        bool
	CandidateHash, PolicyVersion string
	Unresolved                   []string
}

// Validator uses current domain rules, without invoking a model or saving facts.
// The runtime supplies exact binding and hashes the candidate itself.
type Validator interface {
	Validate(context.Context, Binding, string, enrichment.Candidate) (Validation, error)
}
type Observation struct {
	Step                 int
	Tool                 commercetool.ToolRef
	CallID, InvocationID string
	Output               json.RawMessage
	AuditStatus          commercetool.AuditStatus
	ObservedUsage        *ObservedUsage
}
type State struct {
	RunID                 string
	Scope                 Scope
	Request               Request
	Fingerprint           string
	Revision              uint64
	Phase                 Phase
	StartedAt, Deadline   time.Time
	Usage                 Usage
	Repairs               int
	History               []Observation
	Candidate             enrichment.Candidate
	Validation            *Validation
	Unresolved            []string
	StopReason            StopReason
	HumanReviewRequired   bool
	Confidence            []FieldConfidence
	UserFeedback, TraceID string
	PendingInvocationID   string
}
type Record struct {
	State      State
	Checkpoint []byte
}

// Store is a single run/control/checkpoint authority. Claim must atomically
// enforce (scope, context kind/ID, key) uniqueness and fingerprint equality.
// expected=0 starts a run (an existing match is read-only); expected>0 may claim
// exactly that INTERRUPTED revision. It increments Revision and sets Running.
// Commit atomically CAS-writes state AND opaque checkpoint at claimed Revision.
// Failed/ambiguous writes must not grant execution; Running has no automatic
// recovery/retry here. Only an admitted durable adapter can enable HTTP/workers.
// No in-memory production implementation is provided by this package.
type Store interface {
	Claim(context.Context, Record, uint64) (record Record, acquired bool, err error)
	Commit(context.Context, Record, uint64) (Record, error)
}
