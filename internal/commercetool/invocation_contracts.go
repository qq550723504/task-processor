package commercetool

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"
)

type Executor interface {
	Execute(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error)
}

type ExecutorFunc func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error)

func (f ExecutorFunc) Execute(ctx context.Context, envelope ExecutionEnvelope, input json.RawMessage) (ExecutionResult, error) {
	return f(ctx, envelope, input)
}

type CallMetadata struct {
	CallID         string
	AgentID        string
	AgentVersion   string
	AgentRunID     string
	BusinessTaskID string
	TraceID        string
	IdempotencyKey string
}

type Call struct {
	Tool      ToolRef
	Metadata  CallMetadata
	Arguments json.RawMessage
}

type ExecutionEnvelope struct {
	tool      ToolRef
	metadata  CallMetadata
	principal Principal
}

func newExecutionEnvelope(tool ToolRef, metadata CallMetadata, principal Principal) ExecutionEnvelope {
	return ExecutionEnvelope{
		tool:      tool,
		metadata:  metadata,
		principal: clonePrincipal(principal),
	}
}

func (envelope ExecutionEnvelope) Tool() ToolRef {
	return envelope.tool
}

func (envelope ExecutionEnvelope) Metadata() CallMetadata {
	return envelope.metadata
}

func (envelope ExecutionEnvelope) Principal() Principal {
	return clonePrincipal(envelope.principal)
}

type ExecutionResult struct {
	Output         json.RawMessage
	AIInvocationID string
}

type AuditStatus string

const (
	AuditStatusRecorded     AuditStatus = "recorded"
	AuditStatusRecordFailed AuditStatus = "record_failed"
)

type Result struct {
	Output         json.RawMessage
	AIInvocationID string
	AuditStatus    AuditStatus
}

type Principal struct {
	TenantID string
	UserID   string
	Roles    []string
}

func clonePrincipal(principal Principal) Principal {
	principal.Roles = append([]string(nil), principal.Roles...)
	return principal
}

func (principal Principal) validate() error {
	if strings.TrimSpace(principal.TenantID) == "" || strings.TrimSpace(principal.UserID) == "" || len(principal.Roles) == 0 {
		return NewError(ErrorIdentityIntegrity, "principal identity is invalid", nil)
	}
	for _, role := range principal.Roles {
		if strings.TrimSpace(role) == "" {
			return NewError(ErrorIdentityIntegrity, "principal identity is invalid", nil)
		}
	}

	return nil
}

type PrincipalResolver interface {
	ResolvePrincipal(context.Context) (Principal, error)
}

type Authorizer interface {
	Authorize(context.Context, Principal, PermissionRequirement) error
}

type InvocationDependencies struct {
	PrincipalResolver PrincipalResolver
	Authorizer        Authorizer
	Recorder          AuditRecorder
	Tracer            trace.Tracer
	Now               func() time.Time
	AuditTimeout      time.Duration
}

func (dependencies InvocationDependencies) Validate() error {
	if isNilInterface(dependencies.PrincipalResolver) {
		return fmt.Errorf("principal resolver is nil")
	}
	if isNilInterface(dependencies.Authorizer) {
		return fmt.Errorf("authorizer is nil")
	}
	if isNilInterface(dependencies.Recorder) {
		return fmt.Errorf("audit recorder is nil")
	}
	if isNilInterface(dependencies.Tracer) {
		return fmt.Errorf("tracer is nil")
	}
	if dependencies.Now == nil {
		return fmt.Errorf("clock is nil")
	}
	if dependencies.AuditTimeout <= 0 {
		return fmt.Errorf("audit timeout must be greater than zero")
	}

	return nil
}

type Tool struct {
	Definition Definition
	Executor   Executor
}

type AgentDefinition struct {
	ID           string
	Version      string
	AllowedTools []ToolRef
}

func isNilInterface(value any) bool {
	if value == nil {
		return true
	}

	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return reflected.IsNil()
	default:
		return false
	}
}
