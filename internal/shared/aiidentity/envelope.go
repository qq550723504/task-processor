package aiidentity

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

const CurrentEnvelopeVersion = 1

var (
	ErrMissingIdentity   = errors.New("AI execution identity is missing")
	ErrIdentityIntegrity = errors.New("AI execution identity integrity check failed")
)

// ExecutionEnvelope is the durable, provider-neutral identity for one async
// business task. It contains no credentials or model payloads.
type ExecutionEnvelope struct {
	Version        int    `json:"version"`
	TenantID       string `json:"tenant_id"`
	UserID         string `json:"user_id"`
	BusinessTaskID string `json:"business_task_id"`
	TraceID        string `json:"trace_id,omitempty"`
	SourcePlatform string `json:"source_platform"`
	SourceTaskType string `json:"source_task_type"`
}

// PersistedExecutionEnvelope is embedded in business task models. The
// explicit column names keep the shared contract safe to embed in GORM rows.
type PersistedExecutionEnvelope struct {
	ExecutionIdentityVersion int    `json:"-" gorm:"column:execution_identity_version"`
	ExecutionTenantID        string `json:"-" gorm:"column:execution_tenant_id;index"`
	ExecutionUserID          string `json:"-" gorm:"column:execution_user_id;index"`
	ExecutionTraceID         string `json:"-" gorm:"column:execution_trace_id"`
	ExecutionSourcePlatform  string `json:"-" gorm:"column:execution_source_platform"`
	ExecutionSourceTaskType  string `json:"-" gorm:"column:execution_source_task_type"`
}

type envelopeContextKey struct{}

// ExecutionTenantMatchesContext applies the same tenant authority rule used by
// persisted task repositories: a non-zero execution envelope is authoritative,
// while fully legacy rows may fall back to their legacy tenant column. Contexts
// without a tenant remain available to trusted internal workers.
func ExecutionTenantMatchesContext(ctx context.Context, persisted PersistedExecutionEnvelope, legacyTenantID string) bool {
	tenantID := strings.TrimSpace(FromContext(ctx).TenantID)
	if tenantID == "" {
		return true
	}
	taskTenantID := persisted.ExecutionTenantID
	if persisted.ExecutionIdentityVersion == 0 &&
		taskTenantID == "" &&
		persisted.ExecutionUserID == "" &&
		persisted.ExecutionSourcePlatform == "" &&
		persisted.ExecutionSourceTaskType == "" {
		taskTenantID = legacyTenantID
	}
	return taskTenantID == tenantID
}

func (e ExecutionEnvelope) Validate() error {
	e = normalizeEnvelope(e)
	if e.Version != CurrentEnvelopeVersion {
		return fmt.Errorf("%w: unsupported version %d", ErrIdentityIntegrity, e.Version)
	}
	if e.TenantID == "" || e.UserID == "" || e.BusinessTaskID == "" {
		return fmt.Errorf("%w: tenant, user, and business task are required", ErrIdentityIntegrity)
	}
	if !validSource(e.SourcePlatform, e.SourceTaskType) {
		return fmt.Errorf("%w: unsupported source %q/%q", ErrIdentityIntegrity, e.SourcePlatform, e.SourceTaskType)
	}
	return nil
}

func CaptureExecutionEnvelope(ctx context.Context, taskID, sourcePlatform, sourceTaskType string) (ExecutionEnvelope, error) {
	identity := FromContext(ctx)
	if strings.TrimSpace(identity.TenantID) == "" || strings.TrimSpace(identity.UserID) == "" {
		return ExecutionEnvelope{}, ErrMissingIdentity
	}
	envelope := ExecutionEnvelope{
		Version:        CurrentEnvelopeVersion,
		TenantID:       identity.TenantID,
		UserID:         identity.UserID,
		BusinessTaskID: strings.TrimSpace(taskID),
		TraceID:        identity.TraceID,
		SourcePlatform: sourcePlatform,
		SourceTaskType: sourceTaskType,
	}
	if err := envelope.Validate(); err != nil {
		return ExecutionEnvelope{}, err
	}
	return envelope, nil
}

func WithExecutionEnvelope(ctx context.Context, envelope ExecutionEnvelope) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	envelope = normalizeEnvelope(envelope)
	return context.WithValue(ctx, envelopeContextKey{}, envelope)
}

func ExecutionEnvelopeFromContext(ctx context.Context) (ExecutionEnvelope, bool) {
	if ctx == nil {
		return ExecutionEnvelope{}, false
	}
	envelope, ok := ctx.Value(envelopeContextKey{}).(ExecutionEnvelope)
	if !ok {
		return ExecutionEnvelope{}, false
	}
	return normalizeEnvelope(envelope), true
}

func RestoreExecutionEnvelope(ctx context.Context, envelope ExecutionEnvelope, taskID string) (context.Context, error) {
	envelope = normalizeEnvelope(envelope)
	if envelope.Version == 0 && envelope.TenantID == "" && envelope.UserID == "" && envelope.BusinessTaskID == "" {
		return ctx, ErrMissingIdentity
	}
	if err := envelope.Validate(); err != nil {
		return ctx, err
	}
	if strings.TrimSpace(taskID) == "" || envelope.BusinessTaskID != strings.TrimSpace(taskID) {
		return ctx, fmt.Errorf("%w: envelope task %q does not match task %q", ErrIdentityIntegrity, envelope.BusinessTaskID, taskID)
	}
	identity := Identity{
		TenantID:       envelope.TenantID,
		UserID:         envelope.UserID,
		BusinessTaskID: envelope.BusinessTaskID,
		TraceID:        envelope.TraceID,
	}
	return WithExecutionEnvelope(WithIdentity(ctx, identity), envelope), nil
}

func EnsureExecutionEnvelopeContext(ctx context.Context, expected ExecutionEnvelope) error {
	if expected.Version == 0 && expected.TenantID == "" && expected.UserID == "" && expected.BusinessTaskID == "" {
		return ErrMissingIdentity
	}
	if err := expected.Validate(); err != nil {
		return err
	}
	actual, ok := ExecutionEnvelopeFromContext(ctx)
	if !ok {
		return fmt.Errorf("%w: execution envelope is not present in context", ErrIdentityIntegrity)
	}
	if normalizeEnvelope(actual) != normalizeEnvelope(expected) {
		return fmt.Errorf("%w: context envelope does not match persisted envelope", ErrIdentityIntegrity)
	}
	return nil
}

func PersistedExecutionEnvelopeFrom(envelope ExecutionEnvelope) PersistedExecutionEnvelope {
	envelope = normalizeEnvelope(envelope)
	return PersistedExecutionEnvelope{
		ExecutionIdentityVersion: envelope.Version,
		ExecutionTenantID:        envelope.TenantID,
		ExecutionUserID:          envelope.UserID,
		ExecutionTraceID:         envelope.TraceID,
		ExecutionSourcePlatform:  envelope.SourcePlatform,
		ExecutionSourceTaskType:  envelope.SourceTaskType,
	}
}

func (p PersistedExecutionEnvelope) ExecutionEnvelope(taskID string) (ExecutionEnvelope, error) {
	if p.ExecutionIdentityVersion == 0 && strings.TrimSpace(p.ExecutionTenantID) == "" && strings.TrimSpace(p.ExecutionUserID) == "" && strings.TrimSpace(p.ExecutionSourcePlatform) == "" && strings.TrimSpace(p.ExecutionSourceTaskType) == "" {
		return ExecutionEnvelope{}, nil
	}
	envelope := ExecutionEnvelope{
		Version:        p.ExecutionIdentityVersion,
		TenantID:       p.ExecutionTenantID,
		UserID:         p.ExecutionUserID,
		BusinessTaskID: taskID,
		TraceID:        p.ExecutionTraceID,
		SourcePlatform: p.ExecutionSourcePlatform,
		SourceTaskType: p.ExecutionSourceTaskType,
	}
	if err := envelope.Validate(); err != nil {
		return ExecutionEnvelope{}, err
	}
	return envelope, nil
}

func normalizeEnvelope(envelope ExecutionEnvelope) ExecutionEnvelope {
	envelope.TenantID = strings.TrimSpace(envelope.TenantID)
	envelope.UserID = strings.TrimSpace(envelope.UserID)
	envelope.BusinessTaskID = strings.TrimSpace(envelope.BusinessTaskID)
	envelope.TraceID = strings.TrimSpace(envelope.TraceID)
	envelope.SourcePlatform = strings.ToLower(strings.TrimSpace(envelope.SourcePlatform))
	envelope.SourceTaskType = strings.ToLower(strings.TrimSpace(envelope.SourceTaskType))
	return envelope
}

func validSource(platform, taskType string) bool {
	_, ok := map[string]map[string]struct{}{
		"amazon":        {"listing": {}},
		"productenrich": {"product": {}},
		"productimage":  {"image": {}},
		"listingkit":    {"generation": {}, "studio": {}},
		"shein":         {"listing": {}},
		"temu":          {"listing": {}},
	}[strings.ToLower(strings.TrimSpace(platform))][strings.ToLower(strings.TrimSpace(taskType))]
	return ok
}
