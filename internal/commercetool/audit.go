package commercetool

import (
	"context"
	"time"
)

type AuditOutcome string

const (
	AuditOutcomeSucceeded AuditOutcome = "succeeded"
	AuditOutcomeFailed    AuditOutcome = "failed"
)

type AuditRecord struct {
	CallID         string
	AgentID        string
	AgentVersion   string
	AgentRunID     string
	ToolID         string
	ToolVersion    string
	Capability     string
	Owner          string
	TenantID       string
	UserID         string
	BusinessTaskID string
	TraceID        string
	Risk           RiskLevel
	Permission     string
	RetryOwner     RetryOwner
	UsageOwner     UsageOwner
	StartedAt      time.Time
	FinishedAt     time.Time
	LatencyMillis  int64
	InputHash      string
	OutputHash     string
	Outcome        AuditOutcome
	ErrorCode      ErrorCode
	AIInvocationID string
}

type AuditRecorder interface {
	RecordToolCall(context.Context, AuditRecord) error
}
