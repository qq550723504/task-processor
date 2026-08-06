// Package store provides private persistence adapters for AI capability data.
package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"task-processor/internal/aicapability"
)

// GormInvocationRecorder persists non-sensitive invocation metadata.
// It deliberately has no retry policy: callers decide whether recorder failures
// should affect their request path.
type GormInvocationRecorder struct {
	db *gorm.DB
}

// NewGormInvocationRecorder creates a recorder backed by db.
func NewGormInvocationRecorder(db *gorm.DB) *GormInvocationRecorder {
	return &GormInvocationRecorder{db: db}
}

// AutoMigrateInvocationLedger creates or updates the invocation ledger schema.
func AutoMigrateInvocationLedger(db *gorm.DB) error {
	if db == nil {
		return fmt.Errorf("ai invocation ledger database is nil")
	}
	return db.AutoMigrate(&invocationRow{})
}

// RecordInvocation stores safe operational metadata only. It never accepts or
// creates columns for prompts, responses, credentials, cookies, or image bytes.
func (r *GormInvocationRecorder) RecordInvocation(ctx context.Context, record aicapability.InvocationRecord) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("ai invocation recorder database is nil")
	}
	if strings.TrimSpace(record.InvocationID) == "" {
		return fmt.Errorf("invocation_id is required")
	}
	if err := validateUsage(record); err != nil {
		return err
	}

	return r.db.WithContext(ctx).Create(invocationRowFromRecord(record)).Error
}

func validateUsage(record aicapability.InvocationRecord) error {
	if record.PromptTokens < 0 || record.CompletionTokens < 0 || record.TotalTokens < 0 || record.ImageCount < 0 || record.EstimatedCostMicros < 0 {
		return fmt.Errorf("invocation usage and cost counters must not be negative")
	}
	return nil
}

type invocationRow struct {
	InvocationID         string    `gorm:"column:invocation_id;primaryKey;size:128"`
	ParentInvocationID   string    `gorm:"column:parent_invocation_id;size:128"`
	AgentRunID           string    `gorm:"column:agent_run_id;size:128"`
	TenantID             string    `gorm:"column:tenant_id;size:128;index:idx_ai_invocations_tenant_started,priority:1"`
	UserID               string    `gorm:"column:user_id;size:128"`
	BusinessTaskID       string    `gorm:"column:business_task_id;size:128;index:idx_ai_invocations_business_task_id"`
	TraceID              string    `gorm:"column:trace_id;size:128"`
	Capability           string    `gorm:"column:capability;size:128;index:idx_ai_invocations_capability_started,priority:1"`
	Operation            string    `gorm:"column:operation;size:128"`
	RouteMode            string    `gorm:"column:route_mode;size:32"`
	RouteOutcome         string    `gorm:"column:route_outcome;size:64"`
	ProviderID           string    `gorm:"column:provider_id;size:128;index:idx_ai_invocations_provider_model_started,priority:1"`
	ModelID              string    `gorm:"column:model_id;size:256;index:idx_ai_invocations_provider_model_started,priority:2"`
	RequestedRoutingKey  string    `gorm:"column:requested_routing_key;size:256"`
	RoutingKey           string    `gorm:"column:routing_key;size:256"`
	CredentialReference  string    `gorm:"column:credential_reference;size:256"`
	PolicyVersion        string    `gorm:"column:policy_version;size:128"`
	ConfigurationVersion string    `gorm:"column:configuration_version;size:128"`
	PromptKey            string    `gorm:"column:prompt_key;size:256"`
	PromptVersion        string    `gorm:"column:prompt_version;size:128"`
	PromptScope          string    `gorm:"column:prompt_scope;size:128"`
	PromptHash           string    `gorm:"column:prompt_hash;size:128"`
	StartedAt            time.Time `gorm:"column:started_at;index:idx_ai_invocations_tenant_started,priority:2;index:idx_ai_invocations_capability_started,priority:2;index:idx_ai_invocations_provider_model_started,priority:3"`
	FinishedAt           time.Time `gorm:"column:finished_at"`
	LatencyMilliseconds  int64     `gorm:"column:latency_milliseconds"`
	Attempt              int       `gorm:"column:attempt"`
	FallbackIndex        int       `gorm:"column:fallback_index"`
	PromptTokens         int       `gorm:"column:prompt_tokens"`
	CompletionTokens     int       `gorm:"column:completion_tokens"`
	TotalTokens          int       `gorm:"column:total_tokens"`
	ImageCount           int       `gorm:"column:image_count"`
	EstimatedCostMicros  int64     `gorm:"column:estimated_cost_micros"`
	Currency             string    `gorm:"column:currency;size:16"`
	Outcome              string    `gorm:"column:outcome;size:32"`
	ErrorCategory        string    `gorm:"column:error_category;size:64"`
	RouteErrorCategory   string    `gorm:"column:route_error_category;size:64"`
	ErrorCode            string    `gorm:"column:error_code;size:128"`
	ProviderRequestID    string    `gorm:"column:provider_request_id;size:256;index:idx_ai_invocations_provider_request_id"`
	UpstreamJobID        string    `gorm:"column:upstream_job_id;size:256;index:idx_ai_invocations_upstream_job_id"`
	InputHash            string    `gorm:"column:input_hash;size:128"`
	OutputHash           string    `gorm:"column:output_hash;size:128"`
}

func (invocationRow) TableName() string { return "ai_invocations" }

func invocationRowFromRecord(record aicapability.InvocationRecord) invocationRow {
	startedAt := record.StartedAt.UTC()
	finishedAt := record.FinishedAt.UTC()
	latencyMilliseconds := record.LatencyMilliseconds
	if latencyMilliseconds == 0 && !record.StartedAt.IsZero() && !record.FinishedAt.IsZero() {
		latencyMilliseconds = record.FinishedAt.Sub(record.StartedAt).Milliseconds()
	}

	return invocationRow{
		InvocationID: trim(record.InvocationID), ParentInvocationID: trim(record.ParentInvocationID), AgentRunID: trim(record.AgentRunID),
		TenantID: trim(record.TenantID), UserID: trim(record.UserID), BusinessTaskID: trim(record.BusinessTaskID), TraceID: trim(record.TraceID),
		Capability: trim(string(record.Capability)), Operation: trim(string(record.Operation)), RouteMode: trim(string(record.RouteMode)), RouteOutcome: trim(string(record.RouteOutcome)),
		ProviderID: trim(record.ProviderID), ModelID: trim(record.ModelID), RequestedRoutingKey: trim(record.RequestedRoutingKey), RoutingKey: trim(record.RoutingKey), CredentialReference: trim(record.CredentialReference),
		PolicyVersion: trim(record.PolicyVersion), ConfigurationVersion: trim(record.ConfigurationVersion),
		PromptKey: trim(record.PromptKey), PromptVersion: trim(record.PromptVersion), PromptScope: trim(record.PromptScope), PromptHash: trim(record.PromptHash),
		StartedAt: startedAt, FinishedAt: finishedAt, LatencyMilliseconds: latencyMilliseconds,
		Attempt: record.Attempt, FallbackIndex: record.FallbackIndex, PromptTokens: record.PromptTokens, CompletionTokens: record.CompletionTokens, TotalTokens: record.TotalTokens, ImageCount: record.ImageCount, EstimatedCostMicros: record.EstimatedCostMicros, Currency: trim(record.Currency),
		Outcome: trim(string(record.Outcome)), ErrorCategory: trim(string(record.ErrorCategory)), RouteErrorCategory: trim(string(record.RouteErrorCategory)), ErrorCode: trim(record.ErrorCode),
		ProviderRequestID: trim(record.ProviderRequestID), UpstreamJobID: trim(record.UpstreamJobID), InputHash: trim(record.InputHash), OutputHash: trim(record.OutputHash),
	}
}

func trim(value string) string { return strings.TrimSpace(value) }
