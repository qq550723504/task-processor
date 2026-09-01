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
	Execute(context.Context, json.RawMessage) (json.RawMessage, error)
}

type ExecutorFunc func(context.Context, json.RawMessage) (json.RawMessage, error)

func (f ExecutorFunc) Execute(ctx context.Context, input json.RawMessage) (json.RawMessage, error) {
	return f(ctx, input)
}

type Principal struct {
	TenantID string
	UserID   string
	Roles    []string
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
