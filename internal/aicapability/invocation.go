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
	InvocationSucceeded InvocationOutcome = "succeeded"
	InvocationFailed    InvocationOutcome = "failed"
)

type InvocationRecord struct {
	InvocationID         string
	ParentInvocationID   string
	AgentRunID           string
	TenantID             string
	UserID               string
	BusinessTaskID       string
	TraceID              string
	Capability           Capability
	Operation            Operation
	RouteMode            RoutingMode
	RouteOutcome         RouteOutcome
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
	Currency             string
	Outcome              InvocationOutcome
	ErrorCategory        ErrorCategory
	RouteErrorCategory   ErrorCategory
	ErrorCode            string
	ProviderRequestID    string
	UpstreamJobID        string
	InputHash            string
	OutputHash           string
}

type InvocationRecorder interface {
	RecordInvocation(context.Context, InvocationRecord) error
}
