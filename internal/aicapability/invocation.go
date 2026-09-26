package aicapability

import (
	"context"
	"time"
)

type RouteOutcome string

const (
	RouteOutcomeLegacy           RouteOutcome = "legacy"
	RouteOutcomeShadowDecided    RouteOutcome = "shadow_decided"
	RouteOutcomeShadowRouteError RouteOutcome = "shadow_route_error"
	RouteOutcomeActive           RouteOutcome = "active"
)

type InvocationOutcome string

const (
	// InvocationDispatched is a durable provider-dispatch boundary. Once this
	// fact exists, retries must not issue the provider call again without a
	// provider-side idempotency contract.
	InvocationDispatched InvocationOutcome = "dispatched"
	InvocationSucceeded  InvocationOutcome = "succeeded"
	InvocationFailed     InvocationOutcome = "failed"
	// InvocationUsageObservedFailed is limited to image Review output failures
	// with trustworthy provider-observed tokens. It bills consumption without
	// claiming that QA succeeded or that an image may be approved.
	InvocationUsageObservedFailed InvocationOutcome = "usage_observed_failed"
)

type InvocationRecord struct {
	InvocationID       string
	ParentInvocationID string
	AgentRunID         string
	TenantID           string
	UserID             string
	// MemberID is the canonical membership-grant identity used by commercial
	// allocation. It is intentionally distinct from the identity-provider user
	// subject.
	MemberID             string
	BusinessTaskID       string
	TraceID              string
	Capability           Capability
	Operation            Operation
	RouteMode            RoutingMode
	RouteOutcome         RouteOutcome
	CacheStatus          CacheStatus
	ProviderID           string
	ModelID              string
	RequestedRoutingKey  string
	RoutingKey           string
	CredentialReference  string
	PolicyVersion        string
	ConfigurationVersion string
	PromptKey            string
	PromptVersion        string
	PromptScope          string
	PromptHash           string
	StartedAt            time.Time
	FinishedAt           time.Time
	LatencyMilliseconds  int64
	Attempt              int
	FallbackIndex        int
	PromptTokens         int
	CompletionTokens     int
	TotalTokens          int
	ImageCount           int
	EstimatedCostMicros  int64
	EstimatedCostKnown   bool
	UsageKnown           bool
	Currency             string
	Outcome              InvocationOutcome
	ErrorCategory        ErrorCategory
	RouteErrorCategory   ErrorCategory
	ErrorCode            string
	ProviderRequestID    string
	UpstreamJobID        string
	InputHash            string
	OutputHash           string
	// Review result is kept as a small, typed replay envelope so a Temporal
	// retry can return a completed review without dispatching the provider a
	// second time. It is not a prompt or provider response payload.
	ReviewScore            float64
	ReviewNeedsHumanReview bool
	ReviewReasons          []string
}

type InvocationRecorder interface {
	RecordInvocation(context.Context, InvocationRecord) error
}

// InvocationReplayReader is optional for non-durable test recorders. The
// production recorder implements it to close the provider retry boundary.
type InvocationReplayReader interface {
	FindInvocation(context.Context, string, string, string, string) (InvocationRecord, bool, error)
}

// InvocationRecovery is an owner-triggered terminal resolution for a durable
// dispatched invocation whose worker lost the provider response. The caller
// must supply an externally observed provider outcome; this interface never
// redispatches a provider call and never invents usage.
type InvocationRecovery interface {
	ResolveDispatchedInvocation(context.Context, InvocationRecord) error
}

// InvocationUsageReservation is the commercial owner boundary used before a
// provider dispatch. It reserves the caller-supplied conservative token upper
// bound; settlement releases the unused reservation and commits observed tokens.
type InvocationUsageReservation interface {
	ReserveAIInvocationUsage(context.Context, string, string, string, int64, time.Time) error
	ReleaseAIInvocationUsage(context.Context, string, string) error
}
