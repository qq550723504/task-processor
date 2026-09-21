// Package store provides private persistence adapters for AI capability data.
package store

import (
	"context"
	"encoding/json"
	"errors"
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
	db           *gorm.DB
	usageSettler aicapability.InvocationUsageSettler
}

// SetUsageSettler attaches the commercial accounting adapter. It is kept as
// an explicit composition seam because invocation observation and commercial
// accounting may use different database pools.
func (r *GormInvocationRecorder) SetUsageSettler(settler aicapability.InvocationUsageSettler) {
	if r != nil {
		r.usageSettler = settler
	}
}

func (r *GormInvocationRecorder) ReserveAIInvocationUsage(ctx context.Context, tenantID, memberID, invocationID string, maximumTokens int64, occurredAt time.Time) error {
	reservation, ok := r.usageSettler.(aicapability.InvocationUsageReservation)
	if !ok {
		return fmt.Errorf("ai invocation usage reservation is unavailable")
	}
	return reservation.ReserveAIInvocationUsage(ctx, tenantID, memberID, invocationID, maximumTokens, occurredAt)
}

func (r *GormInvocationRecorder) ReleaseAIInvocationUsage(ctx context.Context, tenantID, invocationID string) error {
	reservation, ok := r.usageSettler.(aicapability.InvocationUsageReservation)
	if !ok {
		return fmt.Errorf("ai invocation usage reservation is unavailable")
	}
	return reservation.ReleaseAIInvocationUsage(ctx, tenantID, invocationID)
}

func (r *GormInvocationRecorder) FindInvocation(ctx context.Context, tenantID, memberID, invocationID, inputHash string) (aicapability.InvocationRecord, bool, error) {
	if r == nil || r.db == nil || strings.TrimSpace(tenantID) == "" || strings.TrimSpace(memberID) == "" || strings.TrimSpace(invocationID) == "" {
		return aicapability.InvocationRecord{}, false, fmt.Errorf("ai invocation lookup input is invalid")
	}
	var row invocationRow
	if err := r.db.WithContext(ctx).Where("invocation_id = ? AND tenant_id = ? AND member_id = ?", invocationID, tenantID, memberID).Take(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return aicapability.InvocationRecord{}, false, nil
		}
		return aicapability.InvocationRecord{}, false, err
	}
	if inputHash != "" && row.InputHash != strings.TrimSpace(inputHash) {
		return aicapability.InvocationRecord{}, false, fmt.Errorf("ai invocation identity conflict")
	}
	return invocationRecordFromRow(row), true, nil
}

// ResolveDispatchedInvocation closes the only safe recovery gap after a
// provider call has crossed the durable dispatch boundary but its response
// was lost. Resolution is explicit and terminal: a confirmed failure releases
// the reservation, while a confirmed success must carry provider-observed
// token usage and is settled through the commercial owner. It never calls a
// provider and is idempotent for the same terminal fact.
func (r *GormInvocationRecorder) ResolveDispatchedInvocation(ctx context.Context, record aicapability.InvocationRecord) error {
	if r == nil || r.db == nil {
		return fmt.Errorf("ai invocation recorder database is nil")
	}
	if strings.TrimSpace(record.InvocationID) == "" || strings.TrimSpace(record.TenantID) == "" || strings.TrimSpace(record.MemberID) == "" {
		return fmt.Errorf("ai invocation recovery identity is required")
	}
	if record.Outcome != aicapability.InvocationSucceeded && record.Outcome != aicapability.InvocationFailed {
		return fmt.Errorf("ai invocation recovery outcome must be succeeded or failed")
	}
	if record.FinishedAt.IsZero() {
		return fmt.Errorf("ai invocation recovery finished_at is required")
	}
	if record.Outcome == aicapability.InvocationSucceeded && (!record.UsageKnown || record.TotalTokens <= 0) {
		return fmt.Errorf("successful ai invocation recovery requires observed token usage")
	}
	existing, found, err := r.FindInvocation(ctx, record.TenantID, record.MemberID, record.InvocationID, record.InputHash)
	if err != nil {
		return err
	}
	if !found {
		return fmt.Errorf("ai invocation recovery requires a durable dispatched record")
	}
	if existing.Outcome != aicapability.InvocationDispatched && existing.Outcome != record.Outcome {
		return fmt.Errorf("ai invocation recovery outcome conflict")
	}
	if existing.Outcome != aicapability.InvocationDispatched {
		// Re-run only the durable commercial settlement/release for the
		// already-recorded terminal fact. Never ask the provider again.
		return r.RecordInvocation(ctx, existing)
	}
	if existing.Outcome == aicapability.InvocationDispatched {
		// Preserve the original dispatch identity and metadata while applying
		// only the terminal, operator-observed result.
		record.AgentRunID = existing.AgentRunID
		record.UserID = existing.UserID
		record.BusinessTaskID = existing.BusinessTaskID
		record.StartedAt = existing.StartedAt
		record.Capability = existing.Capability
		record.Operation = existing.Operation
		record.ProviderID = existing.ProviderID
		record.ModelID = existing.ModelID
		record.InputHash = existing.InputHash
	}
	return r.RecordInvocation(ctx, record)
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

	row := invocationRowFromRecord(record)
	var existing invocationRow
	lookupErr := r.db.WithContext(ctx).Where("invocation_id = ?", record.InvocationID).Take(&existing).Error
	if errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
			return err
		}
	} else if lookupErr != nil {
		return lookupErr
	} else {
		if existing.TenantID != strings.TrimSpace(record.TenantID) || existing.UserID != strings.TrimSpace(record.UserID) || existing.MemberID != strings.TrimSpace(record.MemberID) || existing.InputHash != strings.TrimSpace(record.InputHash) {
			return fmt.Errorf("ai invocation identity conflict")
		}
		if existing.Outcome == string(aicapability.InvocationSucceeded) && record.Outcome == aicapability.InvocationDispatched {
			return nil
		}
		if existing.Outcome != string(aicapability.InvocationDispatched) && existing.Outcome != strings.TrimSpace(string(record.Outcome)) {
			return fmt.Errorf("ai invocation outcome conflict")
		}
		if err := r.db.WithContext(ctx).Save(&row).Error; err != nil {
			return err
		}
	}
	if r.usageSettler != nil {
		if err := aicapability.SettleSuccessfulInvocation(ctx, record, r.usageSettler); err != nil {
			return err
		}
		// An unknown-usage success retains the pre-dispatch reservation. Releasing
		// it would let repeated successful calls bypass the member and enterprise
		// token limits before the usage fact can be reconciled.
		if record.Outcome != aicapability.InvocationSucceeded && record.Outcome != aicapability.InvocationDispatched {
			if reservation, ok := r.usageSettler.(aicapability.InvocationUsageReservation); ok {
				return reservation.ReleaseAIInvocationUsage(ctx, record.TenantID, record.InvocationID)
			}
		}
	}
	return nil
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
	MemberID             string    `gorm:"column:member_id;size:128"`
	BusinessTaskID       string    `gorm:"column:business_task_id;size:128;index:idx_ai_invocations_business_task_id"`
	TraceID              string    `gorm:"column:trace_id;size:128"`
	Capability           string    `gorm:"column:capability;size:128;index:idx_ai_invocations_capability_started,priority:1"`
	Operation            string    `gorm:"column:operation;size:128"`
	RouteMode            string    `gorm:"column:route_mode;size:32"`
	RouteOutcome         string    `gorm:"column:route_outcome;size:64"`
	CacheStatus          string    `gorm:"column:cache_status;size:32"`
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
	EstimatedCostKnown   bool      `gorm:"column:estimated_cost_known"`
	UsageKnown           bool      `gorm:"column:usage_known"`
	Currency             string    `gorm:"column:currency;size:16"`
	Outcome              string    `gorm:"column:outcome;size:32"`
	ErrorCategory        string    `gorm:"column:error_category;size:64"`
	RouteErrorCategory   string    `gorm:"column:route_error_category;size:64"`
	ErrorCode            string    `gorm:"column:error_code;size:128"`
	ProviderRequestID    string    `gorm:"column:provider_request_id;size:256;index:idx_ai_invocations_provider_request_id"`
	UpstreamJobID        string    `gorm:"column:upstream_job_id;size:256;index:idx_ai_invocations_upstream_job_id"`
	InputHash            string    `gorm:"column:input_hash;size:128"`
	OutputHash           string    `gorm:"column:output_hash;size:128"`
	ReviewScore          float64   `gorm:"column:review_score"`
	ReviewNeedsHuman     bool      `gorm:"column:review_needs_human"`
	ReviewReasonsJSON    string    `gorm:"column:review_reasons;type:text"`
}

func (invocationRow) TableName() string { return "ai_invocations" }

func invocationRowFromRecord(record aicapability.InvocationRecord) invocationRow {
	startedAt := record.StartedAt.UTC()
	finishedAt := record.FinishedAt.UTC()
	latencyMilliseconds := record.LatencyMilliseconds
	if latencyMilliseconds == 0 && !record.StartedAt.IsZero() && !record.FinishedAt.IsZero() {
		latencyMilliseconds = record.FinishedAt.Sub(record.StartedAt).Milliseconds()
	}
	cacheStatus := aicapability.CacheStatus(trim(string(record.CacheStatus)))
	if cacheStatus == "" {
		cacheStatus = aicapability.CacheStatusNotApplicable
	}
	reasons, _ := json.Marshal(record.ReviewReasons)
	return invocationRow{
		EstimatedCostKnown: record.EstimatedCostKnown, UsageKnown: record.UsageKnown,
		InvocationID: trim(record.InvocationID), ParentInvocationID: trim(record.ParentInvocationID), AgentRunID: trim(record.AgentRunID),
		TenantID: trim(record.TenantID), UserID: trim(record.UserID), MemberID: trim(record.MemberID), BusinessTaskID: trim(record.BusinessTaskID), TraceID: trim(record.TraceID),
		Capability: trim(string(record.Capability)), Operation: trim(string(record.Operation)), RouteMode: trim(string(record.RouteMode)), RouteOutcome: trim(string(record.RouteOutcome)), CacheStatus: string(cacheStatus),
		ProviderID: trim(record.ProviderID), ModelID: trim(record.ModelID), RequestedRoutingKey: trim(record.RequestedRoutingKey), RoutingKey: trim(record.RoutingKey), CredentialReference: trim(record.CredentialReference),
		PolicyVersion: trim(record.PolicyVersion), ConfigurationVersion: trim(record.ConfigurationVersion),
		PromptKey: trim(record.PromptKey), PromptVersion: trim(record.PromptVersion), PromptScope: trim(record.PromptScope), PromptHash: trim(record.PromptHash),
		StartedAt: startedAt, FinishedAt: finishedAt, LatencyMilliseconds: latencyMilliseconds,
		Attempt: record.Attempt, FallbackIndex: record.FallbackIndex, PromptTokens: record.PromptTokens, CompletionTokens: record.CompletionTokens, TotalTokens: record.TotalTokens, ImageCount: record.ImageCount, EstimatedCostMicros: record.EstimatedCostMicros, Currency: trim(record.Currency),
		Outcome: trim(string(record.Outcome)), ErrorCategory: trim(string(record.ErrorCategory)), RouteErrorCategory: trim(string(record.RouteErrorCategory)), ErrorCode: trim(record.ErrorCode),
		ProviderRequestID: trim(record.ProviderRequestID), UpstreamJobID: trim(record.UpstreamJobID), InputHash: trim(record.InputHash), OutputHash: trim(record.OutputHash), ReviewScore: record.ReviewScore, ReviewNeedsHuman: record.ReviewNeedsHumanReview, ReviewReasonsJSON: string(reasons),
	}
}

func invocationRecordFromRow(row invocationRow) aicapability.InvocationRecord {
	var reasons []string
	_ = json.Unmarshal([]byte(row.ReviewReasonsJSON), &reasons)
	return aicapability.InvocationRecord{
		InvocationID: row.InvocationID, ParentInvocationID: row.ParentInvocationID, AgentRunID: row.AgentRunID, TenantID: row.TenantID, UserID: row.UserID, MemberID: row.MemberID, BusinessTaskID: row.BusinessTaskID, TraceID: row.TraceID,
		Capability: aicapability.Capability(row.Capability), Operation: aicapability.Operation(row.Operation), RouteMode: aicapability.RoutingMode(row.RouteMode), RouteOutcome: aicapability.RouteOutcome(row.RouteOutcome), CacheStatus: aicapability.CacheStatus(row.CacheStatus), ProviderID: row.ProviderID, ModelID: row.ModelID, RequestedRoutingKey: row.RequestedRoutingKey, RoutingKey: row.RoutingKey, CredentialReference: row.CredentialReference, PolicyVersion: row.PolicyVersion, ConfigurationVersion: row.ConfigurationVersion, PromptKey: row.PromptKey, PromptVersion: row.PromptVersion, PromptScope: row.PromptScope, PromptHash: row.PromptHash, StartedAt: row.StartedAt, FinishedAt: row.FinishedAt, LatencyMilliseconds: row.LatencyMilliseconds, Attempt: row.Attempt, FallbackIndex: row.FallbackIndex, PromptTokens: row.PromptTokens, CompletionTokens: row.CompletionTokens, TotalTokens: row.TotalTokens, ImageCount: row.ImageCount, EstimatedCostMicros: row.EstimatedCostMicros, EstimatedCostKnown: row.EstimatedCostKnown, UsageKnown: row.UsageKnown, Currency: row.Currency, Outcome: aicapability.InvocationOutcome(row.Outcome), ErrorCategory: aicapability.ErrorCategory(row.ErrorCategory), RouteErrorCategory: aicapability.ErrorCategory(row.RouteErrorCategory), ErrorCode: row.ErrorCode, ProviderRequestID: row.ProviderRequestID, UpstreamJobID: row.UpstreamJobID, InputHash: row.InputHash, OutputHash: row.OutputHash, ReviewScore: row.ReviewScore, ReviewNeedsHumanReview: row.ReviewNeedsHuman, ReviewReasons: reasons,
	}
}

func trim(value string) string { return strings.TrimSpace(value) }
