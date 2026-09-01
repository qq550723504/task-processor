package commercetool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type stubExecutor struct {
	output json.RawMessage
	err    error
	calls  *int
}

func (s *stubExecutor) Execute(context.Context, json.RawMessage) (json.RawMessage, error) {
	if s.calls != nil {
		*s.calls = *s.calls + 1
	}
	return cloneRaw(s.output), s.err
}

type resolverStub struct {
	principal Principal
	err       error
}

func (s resolverStub) ResolvePrincipal(context.Context) (Principal, error) {
	return s.principal, s.err
}

type authorizerStub struct{ err error }

func (s authorizerStub) Authorize(context.Context, Principal, PermissionRequirement) error {
	return s.err
}

type recordingAuditStub struct {
	records []AuditRecord
	err     error
}

func (s *recordingAuditStub) RecordToolCall(_ context.Context, record AuditRecord) error {
	s.records = append(s.records, record)
	return s.err
}

func verifiedResolver() PrincipalResolver {
	return resolverStub{principal: Principal{
		TenantID: "tenant-1",
		UserID:   "user-1",
		Roles:    []string{"listingkit_admin"},
	}}
}

func validTool() Tool {
	return Tool{
		Definition: validDefinition(),
		Executor: &stubExecutor{
			output: json.RawMessage(`{"task_id":"task-1"}`),
		},
	}
}

func agentAllowing(ref ToolRef) AgentDefinition {
	return AgentDefinition{
		ID:           "fake.product-agent",
		Version:      "v1.0.0",
		AllowedTools: []ToolRef{ref},
	}
}

func validInvocationDependencies() InvocationDependencies {
	return InvocationDependencies{
		PrincipalResolver: verifiedResolver(),
		Authorizer:        authorizerStub{},
		Recorder:          &recordingAuditStub{},
		Tracer:            otel.Tracer("commercetool-test"),
		Now:               time.Now,
		AuditTimeout:      time.Second,
	}
}

func TestNewRegistryRejectsDuplicateExactToolRef(t *testing.T) {
	tool := validTool()

	_, err := NewRegistry(tool, tool)

	require.ErrorContains(t, err, "duplicate tool")
}

func TestNewRegistryRejectsTypedNilExecutor(t *testing.T) {
	var executor *stubExecutor

	_, err := NewRegistry(Tool{Definition: validDefinition(), Executor: executor})

	require.ErrorContains(t, err, "executor is nil")
}

func TestNewRegistryValidatesDefinitionBeforeExecutor(t *testing.T) {
	var executor *stubExecutor
	definition := validDefinition()
	definition.Ref.ID = ""

	_, err := NewRegistry(Tool{Definition: definition, Executor: executor})

	require.ErrorContains(t, err, "invalid tool ID")
	require.NotContains(t, err.Error(), "executor is nil")
}

func TestNewRegistryCompilesSchemasBeforeCheckingDuplicateRef(t *testing.T) {
	first := validTool()
	duplicate := validTool()
	duplicate.Definition.InputSchema = json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"task_id":{"type":"not-a-json-schema-type"}}}`)

	_, err := NewRegistry(first, duplicate)

	require.ErrorContains(t, err, "compile input schema")
	require.NotContains(t, err.Error(), "duplicate tool")
}

func TestRegistryBindRequiresExactToolVersion(t *testing.T) {
	tool := validTool()
	registry, err := NewRegistry(tool)
	require.NoError(t, err)
	agent := agentAllowing(ToolRef{ID: tool.Definition.Ref.ID, Version: "v2.0.0"})

	_, err = registry.Bind(agent, validInvocationDependencies())

	require.Equal(t, ErrorToolNotAllowed, CodeOf(err))
	require.NotContains(t, err.Error(), "v2.0.0")
}

func TestRegistryBindRejectsMissingToolWithoutLeakingItsIdentity(t *testing.T) {
	registry, err := NewRegistry(validTool())
	require.NoError(t, err)
	agent := agentAllowing(ToolRef{ID: "secret.missing.tool", Version: "v9.0.0"})

	_, err = registry.Bind(agent, validInvocationDependencies())

	require.Equal(t, ErrorToolNotAllowed, CodeOf(err))
	require.Equal(t, "tool_not_allowed: requested tool is not allowed", err.Error())
	require.NotContains(t, err.Error(), "secret")
}

func TestRegistryBindRejectsDuplicateAllowlistRef(t *testing.T) {
	tool := validTool()
	registry, err := NewRegistry(tool)
	require.NoError(t, err)
	agent := agentAllowing(tool.Definition.Ref)
	agent.AllowedTools = append(agent.AllowedTools, tool.Definition.Ref)

	_, err = registry.Bind(agent, validInvocationDependencies())

	require.Equal(t, ErrorToolNotAllowed, CodeOf(err))
	require.Equal(t, "tool_not_allowed: requested tool is not allowed", err.Error())
}

func TestRegistryBindRejectsWriteAndPublishTools(t *testing.T) {
	tests := []struct {
		name        string
		risk        RiskLevel
		sideEffects SideEffectMode
	}{
		{name: "write", risk: RiskWrite, sideEffects: SideEffectBusinessMutation},
		{name: "publish", risk: RiskPublish, sideEffects: SideEffectExternalMutation},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool := validTool()
			tool.Definition.Risk = tt.risk
			tool.Definition.SideEffects.Mode = tt.sideEffects
			tool.Definition.Idempotency.Mode = IdempotencyRequiredKey
			registry, err := NewRegistry(tool)
			require.NoError(t, err)

			_, err = registry.Bind(agentAllowing(tool.Definition.Ref), validInvocationDependencies())

			require.Equal(t, ErrorToolNotAllowed, CodeOf(err))
			require.Equal(t, "tool_not_allowed: requested tool is not allowed", err.Error())
		})
	}
}

func TestRegistryBindValidatesAgentIdentity(t *testing.T) {
	registry, err := NewRegistry(validTool())
	require.NoError(t, err)
	agent := agentAllowing(validDefinition().Ref)
	agent.Version = "1.0.0"

	_, err = registry.Bind(agent, validInvocationDependencies())

	require.Equal(t, ErrorIdentityIntegrity, CodeOf(err))
	require.Equal(t, "identity_integrity: agent identity is invalid", err.Error())
}

func TestRegistryBindContainsOnlyExplicitlyAllowedTools(t *testing.T) {
	first := validTool()
	second := validTool()
	second.Definition.Ref = ToolRef{ID: "catalog.offer.inspect", Version: "v1.0.0"}
	registry, err := NewRegistry(first, second)
	require.NoError(t, err)

	bound, err := registry.Bind(agentAllowing(second.Definition.Ref), validInvocationDependencies())

	require.NoError(t, err)
	require.Equal(t, []Definition{second.Definition}, bound.Definitions())
}

func TestBoundToolSetDefinitionsAreSortedByIDAndVersion(t *testing.T) {
	tools := []Tool{validTool(), validTool(), validTool()}
	tools[0].Definition.Ref = ToolRef{ID: "product.offer.inspect", Version: "v2.0.0"}
	tools[1].Definition.Ref = ToolRef{ID: "catalog.offer.inspect", Version: "v1.0.0"}
	tools[2].Definition.Ref = ToolRef{ID: "product.offer.inspect", Version: "v1.0.0"}
	registry, err := NewRegistry(tools...)
	require.NoError(t, err)
	agent := agentAllowing(tools[0].Definition.Ref)
	agent.AllowedTools = []ToolRef{
		tools[0].Definition.Ref,
		tools[1].Definition.Ref,
		tools[2].Definition.Ref,
	}

	bound, err := registry.Bind(agent, validInvocationDependencies())

	require.NoError(t, err)
	definitions := bound.Definitions()
	require.Equal(t, []ToolRef{
		{ID: "catalog.offer.inspect", Version: "v1.0.0"},
		{ID: "product.offer.inspect", Version: "v1.0.0"},
		{ID: "product.offer.inspect", Version: "v2.0.0"},
	}, []ToolRef{definitions[0].Ref, definitions[1].Ref, definitions[2].Ref})
}

func TestRegistryDefensivelyCopiesOriginalSchemas(t *testing.T) {
	tool := validTool()
	registry, err := NewRegistry(tool)
	require.NoError(t, err)
	mutateSchemaStringTypeToNumber(t, tool.Definition.InputSchema)
	mutateSchemaStringTypeToNumber(t, tool.Definition.OutputSchema)

	bound, err := registry.Bind(agentAllowing(tool.Definition.Ref), validInvocationDependencies())

	require.NoError(t, err)
	definition := bound.Definitions()[0]
	require.Contains(t, string(definition.InputSchema), `"type":"string"`)
	require.Contains(t, string(definition.OutputSchema), `"type":"string"`)
	registered := bound.tools[tool.Definition.Ref]
	require.NoError(t, registered.schemas.validateInput(json.RawMessage(`{"task_id":"task-1"}`)))
	require.NoError(t, registered.schemas.validateOutput(json.RawMessage(`{"task_id":"task-1"}`)))
	require.Equal(t, ErrorInvalidInput, CodeOf(registered.schemas.validateInput(json.RawMessage(`{"task_id":1}`))))
	require.Equal(t, ErrorOutputInvalid, CodeOf(registered.schemas.validateOutput(json.RawMessage(`{"task_id":1}`))))
}

func TestBoundToolSetDefinitionsReturnsDefensiveSchemaCopies(t *testing.T) {
	tool := validTool()
	registry, err := NewRegistry(tool)
	require.NoError(t, err)
	bound, err := registry.Bind(agentAllowing(tool.Definition.Ref), validInvocationDependencies())
	require.NoError(t, err)
	returned := bound.Definitions()
	mutateSchemaStringTypeToNumber(t, returned[0].InputSchema)
	mutateSchemaStringTypeToNumber(t, returned[0].OutputSchema)

	definition := bound.Definitions()[0]

	require.Contains(t, string(definition.InputSchema), `"type":"string"`)
	require.Contains(t, string(definition.OutputSchema), `"type":"string"`)
	registered := bound.tools[tool.Definition.Ref]
	require.NoError(t, registered.schemas.validateInput(json.RawMessage(`{"task_id":"task-1"}`)))
	require.NoError(t, registered.schemas.validateOutput(json.RawMessage(`{"task_id":"task-1"}`)))
}

func TestInvocationDependenciesValidateRejectsTypedNilInterfaces(t *testing.T) {
	var resolver *resolverStub
	var authorizer *authorizerStub
	var recorder *recordingAuditStub
	var tracer *tracerStub
	tests := []struct {
		name   string
		mutate func(*InvocationDependencies)
		want   string
	}{
		{name: "resolver", mutate: func(d *InvocationDependencies) { d.PrincipalResolver = resolver }, want: "principal resolver is nil"},
		{name: "authorizer", mutate: func(d *InvocationDependencies) { d.Authorizer = authorizer }, want: "authorizer is nil"},
		{name: "recorder", mutate: func(d *InvocationDependencies) { d.Recorder = recorder }, want: "audit recorder is nil"},
		{name: "tracer", mutate: func(d *InvocationDependencies) { d.Tracer = tracer }, want: "tracer is nil"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dependencies := validInvocationDependencies()
			tt.mutate(&dependencies)

			err := dependencies.Validate()

			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestInvocationDependenciesValidateRejectsMissingClockAndNonPositiveAuditTimeout(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*InvocationDependencies)
		want   string
	}{
		{name: "clock", mutate: func(d *InvocationDependencies) { d.Now = nil }, want: "clock is nil"},
		{name: "zero timeout", mutate: func(d *InvocationDependencies) { d.AuditTimeout = 0 }, want: "audit timeout must be greater than zero"},
		{name: "negative timeout", mutate: func(d *InvocationDependencies) { d.AuditTimeout = -time.Second }, want: "audit timeout must be greater than zero"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dependencies := validInvocationDependencies()
			tt.mutate(&dependencies)

			err := dependencies.Validate()

			require.ErrorContains(t, err, tt.want)
		})
	}
}

func TestInvocationDependenciesValidateAcceptsCompleteDependencies(t *testing.T) {
	require.NoError(t, validInvocationDependencies().Validate())
}

func TestPrincipalValidateRequiresTenantUserAndNonEmptyRoles(t *testing.T) {
	tests := []struct {
		name      string
		principal Principal
	}{
		{name: "tenant", principal: Principal{UserID: "user-1", Roles: []string{"admin"}}},
		{name: "user", principal: Principal{TenantID: "tenant-1", Roles: []string{"admin"}}},
		{name: "roles", principal: Principal{TenantID: "tenant-1", UserID: "user-1"}},
		{name: "blank role", principal: Principal{TenantID: "tenant-1", UserID: "user-1", Roles: []string{" "}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.principal.validate()

			require.Equal(t, ErrorIdentityIntegrity, CodeOf(err))
			require.Equal(t, "identity_integrity: principal identity is invalid", err.Error())
		})
	}
}

func TestPrincipalValidateAcceptsCompleteIdentity(t *testing.T) {
	require.NoError(t, (Principal{TenantID: "tenant-1", UserID: "user-1", Roles: []string{"admin"}}).validate())
}

type tracerStub struct {
	trace.Tracer
}

func mutateSchemaStringTypeToNumber(t *testing.T, schema json.RawMessage) {
	t.Helper()
	offset := bytes.Index(schema, []byte(`"string"`))
	require.GreaterOrEqual(t, offset, 0)
	copy(schema[offset:offset+len(`"string"`)], []byte(`"number"`))
}

func TestExecutorFuncDelegatesToFunction(t *testing.T) {
	want := json.RawMessage(`{"task_id":"task-1"}`)
	executor := ExecutorFunc(func(context.Context, json.RawMessage) (json.RawMessage, error) {
		return want, errors.New("executor failure")
	})

	output, err := executor.Execute(context.Background(), nil)

	require.Equal(t, want, output)
	require.EqualError(t, err, "executor failure")
}
