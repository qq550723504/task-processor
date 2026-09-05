package commercetool

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func validCall() Call {
	return Call{
		Tool: validDefinition().Ref,
		Metadata: CallMetadata{
			CallID:         "call-1",
			AgentID:        "fake.product-agent",
			AgentVersion:   "v1.0.0",
			AgentRunID:     "run-1",
			BusinessTaskID: "task-1",
			TraceID:        "trace-1",
		},
		Arguments: json.RawMessage(`{"task_id":"task-1"}`),
	}
}

func validAICapabilityDefinition() Definition {
	definition := validDefinition()
	definition.Risk = RiskPropose
	definition.Usage.Owner = UsageOwnerAICapability
	return definition
}

func bindToolForTest(t *testing.T, executor Executor, deps InvocationDependencies) *BoundToolSet {
	t.Helper()
	return bindDefinitionForTest(t, validDefinition(), executor, deps)
}

func bindDefinitionForTest(t *testing.T, definition Definition, executor Executor, deps InvocationDependencies) *BoundToolSet {
	t.Helper()
	definition.Timeout.Duration = 25 * time.Millisecond
	return bindExactDefinitionForTest(t, definition, executor, deps)
}

func bindExactDefinitionForTest(t *testing.T, definition Definition, executor Executor, deps InvocationDependencies) *BoundToolSet {
	t.Helper()
	registry, err := NewRegistry(Tool{Definition: definition, Executor: executor})
	require.NoError(t, err)
	bound, err := registry.Bind(agentAllowing(definition.Ref), deps)
	require.NoError(t, err)
	return bound
}

func boundToolSetForTest(t *testing.T, calls *int, resolver PrincipalResolver) *BoundToolSet {
	t.Helper()
	executor := ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		*calls = *calls + 1
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	})
	deps := validInvocationDependencies()
	deps.PrincipalResolver = resolver
	return bindToolForTest(t, executor, deps)
}

func boundToolSetWithAuthorizer(t *testing.T, calls *int, authorizer Authorizer) *BoundToolSet {
	t.Helper()
	executor := ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		*calls = *calls + 1
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	})
	deps := validInvocationDependencies()
	deps.Authorizer = authorizer
	return bindToolForTest(t, executor, deps)
}

func boundToolSetWithExecutor(t *testing.T, executor Executor) *BoundToolSet {
	t.Helper()
	return bindToolForTest(t, executor, validInvocationDependencies())
}

type countingResolver struct {
	principal Principal
	err       error
	calls     int
}

func (s *countingResolver) ResolvePrincipal(context.Context) (Principal, error) {
	s.calls++
	return s.principal, s.err
}

type recordingAuthorizer struct {
	err         error
	calls       int
	principal   Principal
	requirement PermissionRequirement
}

func (s *recordingAuthorizer) Authorize(_ context.Context, principal Principal, requirement PermissionRequirement) error {
	s.calls++
	s.principal = principal
	s.requirement = requirement
	return s.err
}

type authorizerFunc func(context.Context, Principal, PermissionRequirement) error

func (f authorizerFunc) Authorize(ctx context.Context, principal Principal, requirement PermissionRequirement) error {
	return f(ctx, principal, requirement)
}

type contextRecordingAuditStub struct {
	records      []AuditRecord
	err          error
	contextErr   error
	hasDeadline  bool
	deadlineLeft time.Duration
}

func (s *contextRecordingAuditStub) RecordToolCall(ctx context.Context, record AuditRecord) error {
	s.records = append(s.records, record)
	s.contextErr = ctx.Err()
	deadline, ok := ctx.Deadline()
	s.hasDeadline = ok
	if ok {
		s.deadlineLeft = time.Until(deadline)
	}
	return s.err
}

func TestInvokeRejectsMissingVerifiedPrincipalBeforeExecution(t *testing.T) {
	calls := 0
	deps := validInvocationDependencies()
	recorder := &recordingAuditStub{}
	deps.PrincipalResolver = resolverStub{}
	deps.Recorder = recorder
	bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		calls++
		return ExecutionResult{}, nil
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorIdentityIntegrity, CodeOf(err))
	require.Equal(t, "identity_integrity: verified principal is unavailable", err.Error())
	require.Equal(t, 0, calls)
	require.Equal(t, AuditStatusRecorded, result.AuditStatus)
	require.Len(t, recorder.records, 1)
}

func TestInvokeRejectsNonCanonicalPrincipalWhitespace(t *testing.T) {
	tests := []Principal{
		{TenantID: " tenant-1", UserID: "user-1", Roles: []string{"listingkit_operator"}},
		{TenantID: "tenant-1", UserID: "user-1 ", Roles: []string{"listingkit_operator"}},
		{TenantID: "tenant-1", UserID: "user-1", Roles: []string{" listingkit_operator"}},
	}
	for _, principal := range tests {
		calls := 0
		resolver := &countingResolver{principal: principal}
		bound := boundToolSetForTest(t, &calls, resolver)

		_, err := bound.Invoke(context.Background(), validCall())

		require.Equal(t, ErrorIdentityIntegrity, CodeOf(err))
		require.Equal(t, 1, resolver.calls)
		require.Equal(t, 0, calls)
	}
}

func TestInvokeBoundsRawArgumentsBeforeCloneOrPreflightDependencies(t *testing.T) {
	valid := validCall().Arguments
	exact := append(cloneRaw(valid), bytesOf(' ', MaxInvocationArgumentsBytes-len(valid))...)
	over := append(cloneRaw(exact), ' ')

	t.Run("exact limit", func(t *testing.T) {
		call := validCall()
		call.Arguments = exact
		calls := 0
		bound := boundToolSetForTest(t, &calls, verifiedResolver())
		_, err := bound.Invoke(context.Background(), call)
		require.NoError(t, err)
		require.Equal(t, 1, calls)
	})

	t.Run("over limit", func(t *testing.T) {
		call := validCall()
		call.Arguments = over
		recorder := &recordingAuditStub{}
		resolver := &countingResolver{principal: verifiedPrincipal()}
		authorizer := &recordingAuthorizer{}
		executorCalls := 0
		deps := validInvocationDependencies()
		deps.PrincipalResolver = resolver
		deps.Authorizer = authorizer
		deps.Recorder = recorder
		bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
			executorCalls++
			return ExecutionResult{}, nil
		}), deps)

		_, err := bound.Invoke(context.Background(), call)

		require.Equal(t, ErrorInvalidInput, CodeOf(err))
		require.Equal(t, 0, resolver.calls)
		require.Equal(t, 0, authorizer.calls)
		require.Equal(t, 0, executorCalls)
		require.Len(t, recorder.records, 1)
		require.Equal(t, oversizedInvocationInputHash, recorder.records[0].InputHash)
	})
}

func TestOversizedInvocationStateDoesNotRetainRawArguments(t *testing.T) {
	call := validCall()
	call.Arguments = bytesOf('x', MaxInvocationArgumentsBytes+1)

	state := newOversizedInvocationState(time.Now(), call)

	require.Nil(t, state.call.Arguments)
	require.Equal(t, oversizedInvocationInputHash, state.inputHash)
	require.Equal(t, call.Tool, state.call.Tool)
	require.Equal(t, call.Metadata, state.call.Metadata)
}

func bytesOf(value byte, count int) []byte {
	if count <= 0 {
		return nil
	}
	result := make([]byte, count)
	for index := range result {
		result[index] = value
	}
	return result
}

func TestInvokeRejectsResolverErrorsWithoutLeakingThem(t *testing.T) {
	calls := 0
	bound := boundToolSetForTest(t, &calls, resolverStub{err: errors.New("credential backend secret")})

	_, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorIdentityIntegrity, CodeOf(err))
	require.Equal(t, "identity_integrity: verified principal is unavailable", err.Error())
	require.NotContains(t, err.Error(), "credential")
	require.Equal(t, 0, calls)
}

func TestInvokeRejectsPermissionDeniedBeforeExecution(t *testing.T) {
	calls := 0
	bound := boundToolSetWithAuthorizer(t, &calls, authorizerStub{
		err: NewError(ErrorPermissionDenied, "dependency permission detail", nil),
	})

	_, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorPermissionDenied, CodeOf(err))
	require.Equal(t, "permission_denied: tool permission denied", err.Error())
	require.NotContains(t, err.Error(), "dependency")
	require.Equal(t, 0, calls)
}

func TestInvokeMapsAnyAuthorizerErrorToSafePermissionDenied(t *testing.T) {
	calls := 0
	bound := boundToolSetWithAuthorizer(t, &calls, authorizerStub{err: errors.New("casbin database password")})

	_, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorPermissionDenied, CodeOf(err))
	require.Equal(t, "permission_denied: tool permission denied", err.Error())
	require.NotContains(t, err.Error(), "casbin")
	require.Equal(t, 0, calls)
}

func TestInvokeAuthorizesTrustedPrincipalForRegisteredPermission(t *testing.T) {
	authorizer := &recordingAuthorizer{}
	deps := validInvocationDependencies()
	deps.Authorizer = authorizer
	bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	}), deps)

	_, err := bound.Invoke(context.Background(), validCall())

	require.NoError(t, err)
	require.Equal(t, 1, authorizer.calls)
	require.Equal(t, verifiedPrincipal(), authorizer.principal)
	require.Equal(t, validDefinition().Permission, authorizer.requirement)
}

func TestInvokeProtectsTrustedPrincipalSnapshotFromSynchronousAuthorizerMutation(t *testing.T) {
	sourceRoles := []string{"listingkit_admin"}
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.PrincipalResolver = resolverStub{principal: Principal{
		TenantID: "tenant-1",
		UserID:   "user-1",
		Roles:    sourceRoles,
	}}
	deps.Authorizer = authorizerFunc(func(_ context.Context, principal Principal, requirement PermissionRequirement) error {
		require.Equal(t, validDefinition().Permission, requirement)
		principal.Roles[0] = "authorizer-mutated"
		return nil
	})
	deps.Recorder = recorder
	var executorPrincipal Principal
	bound := bindToolForTest(t, ExecutorFunc(func(_ context.Context, envelope ExecutionEnvelope, _ json.RawMessage) (ExecutionResult, error) {
		executorPrincipal = envelope.Principal()
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.NoError(t, err)
	require.Equal(t, []string{"listingkit_admin"}, sourceRoles)
	require.Equal(t, Principal{
		TenantID: "tenant-1",
		UserID:   "user-1",
		Roles:    []string{"listingkit_admin"},
	}, executorPrincipal)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
	require.Len(t, recorder.records, 1)
	require.Equal(t, "tenant-1", recorder.records[0].TenantID)
	require.Equal(t, "user-1", recorder.records[0].UserID)
	require.Equal(t, validDefinition().Permission.Permission, recorder.records[0].Permission)
	require.Equal(t, AuditOutcomeSucceeded, recorder.records[0].Outcome)
}

func TestPreflightProtectsTrustedPrincipalStateFromAuthorizerRetainedSliceMutation(t *testing.T) {
	sourceRoles := []string{"listingkit_admin"}
	var retainedRoles []string
	var executorPrincipal Principal
	deps := validInvocationDependencies()
	deps.PrincipalResolver = resolverStub{principal: Principal{
		TenantID: "tenant-1",
		UserID:   "user-1",
		Roles:    sourceRoles,
	}}
	deps.Authorizer = authorizerFunc(func(_ context.Context, principal Principal, _ PermissionRequirement) error {
		retainedRoles = principal.Roles
		return nil
	})
	bound := bindToolForTest(t, ExecutorFunc(func(_ context.Context, envelope ExecutionEnvelope, _ json.RawMessage) (ExecutionResult, error) {
		executorPrincipal = envelope.Principal()
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	}), deps)
	state := newInvocationState(time.Unix(0, 0), validCall())

	registered, err := bound.preflight(context.Background(), state.call, &state)
	require.NoError(t, err)
	require.NotEmpty(t, retainedRoles)
	retainedRoles[0] = "authorizer-late-mutation"

	require.Equal(t, []string{"listingkit_admin"}, sourceRoles)
	require.Equal(t, []string{"listingkit_admin"}, state.principal.Roles)
	_, err = registered.executor.Execute(
		context.Background(),
		newExecutionEnvelope(state.call.Tool, state.call.Metadata, state.principal),
		cloneRaw(state.call.Arguments),
	)
	require.NoError(t, err)
	require.Equal(t, []string{"listingkit_admin"}, executorPrincipal.Roles)
}

func TestInvokeProtectsTrustedPrincipalSnapshotFromResolverSourceMutationAfterResolve(t *testing.T) {
	sourceRoles := []string{"listingkit_admin"}
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.PrincipalResolver = resolverStub{principal: Principal{
		TenantID: "tenant-1",
		UserID:   "user-1",
		Roles:    sourceRoles,
	}}
	deps.Authorizer = authorizerFunc(func(_ context.Context, _ Principal, _ PermissionRequirement) error {
		sourceRoles[0] = "resolver-source-mutated"
		return nil
	})
	deps.Recorder = recorder
	var executorPrincipal Principal
	bound := bindToolForTest(t, ExecutorFunc(func(_ context.Context, envelope ExecutionEnvelope, _ json.RawMessage) (ExecutionResult, error) {
		executorPrincipal = envelope.Principal()
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.NoError(t, err)
	require.Equal(t, []string{"resolver-source-mutated"}, sourceRoles)
	require.Equal(t, []string{"listingkit_admin"}, executorPrincipal.Roles)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
	require.Len(t, recorder.records, 1)
	require.Equal(t, "tenant-1", recorder.records[0].TenantID)
	require.Equal(t, "user-1", recorder.records[0].UserID)
	require.Equal(t, validDefinition().Permission.Permission, recorder.records[0].Permission)
	require.Equal(t, AuditOutcomeSucceeded, recorder.records[0].Outcome)
}

func TestInvokeRejectsAuthorityFieldsOutsideInputSchema(t *testing.T) {
	calls := 0
	bound := boundToolSetForTest(t, &calls, verifiedResolver())
	call := validCall()
	call.Arguments = json.RawMessage(`{"task_id":"task-1","tenant_id":"attacker"}`)

	_, err := bound.Invoke(context.Background(), call)

	require.Equal(t, ErrorInvalidInput, CodeOf(err))
	require.Equal(t, "invalid_input: tool input does not match schema", err.Error())
	require.Equal(t, 0, calls)
}

func TestInvokeRecursivelyRejectsReservedAuthorityInputWhenCompiledSchemaAllowsIt(t *testing.T) {
	tests := []struct {
		name      string
		arguments json.RawMessage
	}{
		{name: "root", arguments: json.RawMessage(`{"tenant_id":"attacker"}`)},
		{name: "nested array", arguments: json.RawMessage(`{"payload":[{"trace_id":"attacker"}]}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			recorder := &recordingAuditStub{}
			deps := validInvocationDependencies()
			deps.Recorder = recorder
			bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
				calls++
				return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
			}), deps)
			registered := bound.tools[validDefinition().Ref]
			registered.schemas.input = permissiveRuntimeObjectSchemaForTest(t)
			bound.tools[validDefinition().Ref] = registered
			call := validCall()
			call.Arguments = tt.arguments

			result, err := bound.Invoke(context.Background(), call)

			require.Equal(t, ErrorInvalidInput, CodeOf(err))
			require.Equal(t, "invalid_input: tool input does not match schema", err.Error())
			require.Equal(t, 0, calls)
			require.Nil(t, result.Output)
			require.Len(t, recorder.records, 1)
			require.Equal(t, ErrorInvalidInput, recorder.records[0].ErrorCode)
		})
	}
}

func TestInvokeRecursivelyRejectsReservedAuthorityOutputWhenCompiledSchemaAllowsIt(t *testing.T) {
	tests := []struct {
		name   string
		output json.RawMessage
	}{
		{name: "root", output: json.RawMessage(`{"tool_version":"attacker"}`)},
		{name: "nested array", output: json.RawMessage(`{"payload":[{"permission":"attacker"}]}`)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			recorder := &recordingAuditStub{}
			deps := validInvocationDependencies()
			deps.Recorder = recorder
			bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
				calls++
				return ExecutionResult{Output: tt.output}, nil
			}), deps)
			registered := bound.tools[validDefinition().Ref]
			registered.schemas.output = permissiveRuntimeObjectSchemaForTest(t)
			bound.tools[validDefinition().Ref] = registered

			result, err := bound.Invoke(context.Background(), validCall())

			require.Equal(t, ErrorOutputInvalid, CodeOf(err))
			require.Equal(t, "output_invalid: tool output does not match schema", err.Error())
			require.Equal(t, 1, calls)
			require.Nil(t, result.Output)
			require.Len(t, recorder.records, 1)
			require.Equal(t, ErrorOutputInvalid, recorder.records[0].ErrorCode)
		})
	}
}

func permissiveRuntimeObjectSchemaForTest(t *testing.T) *jsonschema.Schema {
	t.Helper()
	const location = "urn:task-processor:commerce-tool:runtime-authority-test"
	compiler := jsonschema.NewCompiler()
	require.NoError(t, compiler.AddResource(location, map[string]any{"type": "object"}))
	schema, err := compiler.Compile(location)
	require.NoError(t, err)
	return schema
}

func TestInvokeRequiresCompleteCallMetadataBeforeExecution(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CallMetadata)
	}{
		{name: "call ID", mutate: func(metadata *CallMetadata) { metadata.CallID = "" }},
		{name: "agent ID", mutate: func(metadata *CallMetadata) { metadata.AgentID = "" }},
		{name: "agent version", mutate: func(metadata *CallMetadata) { metadata.AgentVersion = "" }},
		{name: "agent run ID", mutate: func(metadata *CallMetadata) { metadata.AgentRunID = "" }},
		{name: "business task ID", mutate: func(metadata *CallMetadata) { metadata.BusinessTaskID = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			bound := boundToolSetForTest(t, &calls, verifiedResolver())
			call := validCall()
			tt.mutate(&call.Metadata)

			_, err := bound.Invoke(context.Background(), call)

			require.Equal(t, ErrorInvalidInput, CodeOf(err))
			require.Equal(t, "invalid_input: call metadata is incomplete", err.Error())
			require.Equal(t, 0, calls)
		})
	}
}

func TestInvokeRejectsAgentIdentityDifferentFromBoundAgent(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*CallMetadata)
	}{
		{name: "ID", mutate: func(metadata *CallMetadata) { metadata.AgentID = "other.product-agent" }},
		{name: "version", mutate: func(metadata *CallMetadata) { metadata.AgentVersion = "v2.0.0" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			bound := boundToolSetForTest(t, &calls, verifiedResolver())
			call := validCall()
			tt.mutate(&call.Metadata)

			_, err := bound.Invoke(context.Background(), call)

			require.Equal(t, ErrorToolNotAllowed, CodeOf(err))
			require.Equal(t, 0, calls)
		})
	}
}

func TestInvokeRejectsToolOutsideBoundSetBeforeExecution(t *testing.T) {
	calls := 0
	bound := boundToolSetForTest(t, &calls, verifiedResolver())
	call := validCall()
	call.Tool = ToolRef{ID: "catalog.secret.inspect", Version: "v9.0.0"}

	_, err := bound.Invoke(context.Background(), call)

	require.Equal(t, ErrorToolNotAllowed, CodeOf(err))
	require.Equal(t, "tool_not_allowed: requested tool is not allowed", err.Error())
	require.NotContains(t, err.Error(), "secret")
	require.Equal(t, 0, calls)
}

func TestInvokeRequiresIdempotencyKeyWhenDefinitionRequiresIt(t *testing.T) {
	calls := 0
	definition := validDefinition()
	definition.Idempotency.Mode = IdempotencyRequiredKey
	bound := bindDefinitionForTest(t, definition, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		calls++
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	}), validInvocationDependencies())

	_, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorInvalidInput, CodeOf(err))
	require.Equal(t, "invalid_input: idempotency key is required", err.Error())
	require.Equal(t, 0, calls)
}

func TestInvokePreflightUsesFixedFailClosedOrder(t *testing.T) {
	t.Run("metadata before bound agent and dependencies", func(t *testing.T) {
		resolver := &countingResolver{principal: verifiedPrincipal()}
		authorizer := &recordingAuthorizer{}
		recorder := &recordingAuditStub{}
		deps := validInvocationDependencies()
		deps.PrincipalResolver = resolver
		deps.Authorizer = authorizer
		deps.Recorder = recorder
		calls := 0
		bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
			calls++
			return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
		}), deps)
		call := validCall()
		call.Metadata.CallID = ""
		call.Metadata.AgentID = "other.agent"

		result, err := bound.Invoke(context.Background(), call)

		require.Equal(t, ErrorInvalidInput, CodeOf(err))
		require.Equal(t, 0, resolver.calls)
		require.Equal(t, 0, authorizer.calls)
		require.Equal(t, 0, calls)
		require.Len(t, recorder.records, 1)
		require.Equal(t, AuditStatusRecorded, result.AuditStatus)
	})

	t.Run("bound agent before dependencies", func(t *testing.T) {
		resolver := &countingResolver{principal: verifiedPrincipal()}
		authorizer := &recordingAuthorizer{}
		recorder := &recordingAuditStub{}
		deps := validInvocationDependencies()
		deps.PrincipalResolver = resolver
		deps.Authorizer = authorizer
		deps.Recorder = recorder
		calls := 0
		bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
			calls++
			return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
		}), deps)
		call := validCall()
		call.Metadata.AgentID = "other.agent"

		result, err := bound.Invoke(context.Background(), call)

		require.Equal(t, ErrorToolNotAllowed, CodeOf(err))
		require.Equal(t, 0, resolver.calls)
		require.Equal(t, 0, authorizer.calls)
		require.Equal(t, 0, calls)
		require.Len(t, recorder.records, 1)
		require.Equal(t, AuditStatusRecorded, result.AuditStatus)
	})

	t.Run("bound tool before principal", func(t *testing.T) {
		resolver := &countingResolver{principal: verifiedPrincipal()}
		calls := 0
		bound := boundToolSetForTest(t, &calls, resolver)
		call := validCall()
		call.Tool = ToolRef{ID: "missing.tool", Version: "v1.0.0"}

		_, err := bound.Invoke(context.Background(), call)

		require.Equal(t, ErrorToolNotAllowed, CodeOf(err))
		require.Equal(t, 0, resolver.calls)
		require.Equal(t, 0, calls)
	})

	t.Run("risk ceiling before principal", func(t *testing.T) {
		resolver := &countingResolver{principal: verifiedPrincipal()}
		calls := 0
		bound := boundToolSetForTest(t, &calls, resolver)
		registered := bound.tools[validDefinition().Ref]
		registered.definition.Risk = RiskWrite
		bound.tools[validDefinition().Ref] = registered

		_, err := bound.Invoke(context.Background(), validCall())

		require.Equal(t, ErrorToolNotAllowed, CodeOf(err))
		require.Equal(t, 0, resolver.calls)
		require.Equal(t, 0, calls)
	})

	t.Run("principal before authorization", func(t *testing.T) {
		resolver := &countingResolver{err: errors.New("identity store failure")}
		authorizer := &recordingAuthorizer{}
		deps := validInvocationDependencies()
		deps.PrincipalResolver = resolver
		deps.Authorizer = authorizer
		calls := 0
		bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
			calls++
			return ExecutionResult{}, nil
		}), deps)

		_, err := bound.Invoke(context.Background(), validCall())

		require.Equal(t, ErrorIdentityIntegrity, CodeOf(err))
		require.Equal(t, 1, resolver.calls)
		require.Equal(t, 0, authorizer.calls)
		require.Equal(t, 0, calls)
	})

	t.Run("authorization before idempotency", func(t *testing.T) {
		definition := validDefinition()
		definition.Idempotency.Mode = IdempotencyRequiredKey
		authorizer := &recordingAuthorizer{err: errors.New("denied")}
		deps := validInvocationDependencies()
		deps.Authorizer = authorizer
		calls := 0
		bound := bindDefinitionForTest(t, definition, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
			calls++
			return ExecutionResult{}, nil
		}), deps)
		call := validCall()
		call.Arguments = json.RawMessage(`{"bad":true}`)

		_, err := bound.Invoke(context.Background(), call)

		require.Equal(t, ErrorPermissionDenied, CodeOf(err))
		require.Equal(t, 1, authorizer.calls)
		require.Equal(t, 0, calls)
	})

	t.Run("idempotency before input schema", func(t *testing.T) {
		definition := validDefinition()
		definition.Idempotency.Mode = IdempotencyRequiredKey
		calls := 0
		bound := bindDefinitionForTest(t, definition, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
			calls++
			return ExecutionResult{}, nil
		}), validInvocationDependencies())
		call := validCall()
		call.Arguments = json.RawMessage(`{"bad":true}`)

		_, err := bound.Invoke(context.Background(), call)

		require.Equal(t, "invalid_input: idempotency key is required", err.Error())
		require.Equal(t, 0, calls)
	})
}

func TestInvokeRejectsIncompletePrincipalBeforeAuthorization(t *testing.T) {
	tests := []struct {
		name      string
		principal Principal
	}{
		{name: "missing tenant", principal: Principal{UserID: "user-1", Roles: []string{"admin"}}},
		{name: "missing user", principal: Principal{TenantID: "tenant-1", Roles: []string{"admin"}}},
		{name: "missing roles", principal: Principal{TenantID: "tenant-1", UserID: "user-1"}},
		{name: "blank role", principal: Principal{TenantID: "tenant-1", UserID: "user-1", Roles: []string{" "}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver := &countingResolver{principal: tt.principal}
			authorizer := &recordingAuthorizer{}
			recorder := &recordingAuditStub{}
			deps := validInvocationDependencies()
			deps.PrincipalResolver = resolver
			deps.Authorizer = authorizer
			deps.Recorder = recorder
			calls := 0
			bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
				calls++
				return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
			}), deps)

			result, err := bound.Invoke(context.Background(), validCall())

			require.Equal(t, ErrorIdentityIntegrity, CodeOf(err))
			require.Equal(t, 1, resolver.calls)
			require.Equal(t, 0, authorizer.calls)
			require.Equal(t, 0, calls)
			require.Len(t, recorder.records, 1)
			require.Equal(t, ErrorIdentityIntegrity, recorder.records[0].ErrorCode)
			require.Equal(t, AuditStatusRecorded, result.AuditStatus)
		})
	}
}

func TestInvokeAuditsEveryPreflightFailureWithoutCallingExecutor(t *testing.T) {
	tests := []struct {
		name     string
		wantCode ErrorCode
		mutate   func(*BoundToolSet, *Call)
	}{
		{
			name:     "metadata",
			wantCode: ErrorInvalidInput,
			mutate:   func(_ *BoundToolSet, call *Call) { call.Metadata.CallID = "" },
		},
		{
			name:     "bound agent",
			wantCode: ErrorToolNotAllowed,
			mutate:   func(_ *BoundToolSet, call *Call) { call.Metadata.AgentID = "other.agent" },
		},
		{
			name:     "bound tool",
			wantCode: ErrorToolNotAllowed,
			mutate: func(_ *BoundToolSet, call *Call) {
				call.Tool = ToolRef{ID: "missing.tool", Version: "v1.0.0"}
			},
		},
		{
			name:     "risk ceiling",
			wantCode: ErrorToolNotAllowed,
			mutate: func(bound *BoundToolSet, _ *Call) {
				registered := bound.tools[validDefinition().Ref]
				registered.definition.Risk = RiskPublish
				bound.tools[validDefinition().Ref] = registered
			},
		},
		{
			name:     "principal",
			wantCode: ErrorIdentityIntegrity,
			mutate: func(bound *BoundToolSet, _ *Call) {
				bound.deps.PrincipalResolver = resolverStub{err: errors.New("resolver failed")}
			},
		},
		{
			name:     "authorization",
			wantCode: ErrorPermissionDenied,
			mutate: func(bound *BoundToolSet, _ *Call) {
				bound.deps.Authorizer = authorizerStub{err: errors.New("authorizer failed")}
			},
		},
		{
			name:     "idempotency",
			wantCode: ErrorInvalidInput,
			mutate: func(bound *BoundToolSet, _ *Call) {
				registered := bound.tools[validDefinition().Ref]
				registered.definition.Idempotency.Mode = IdempotencyRequiredKey
				bound.tools[validDefinition().Ref] = registered
			},
		},
		{
			name:     "input schema",
			wantCode: ErrorInvalidInput,
			mutate:   func(_ *BoundToolSet, call *Call) { call.Arguments = json.RawMessage(`{"bad":true}`) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			recorder := &recordingAuditStub{}
			deps := validInvocationDependencies()
			deps.Recorder = recorder
			bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
				calls++
				return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
			}), deps)
			call := validCall()
			tt.mutate(bound, &call)

			result, err := bound.Invoke(context.Background(), call)

			require.Equal(t, tt.wantCode, CodeOf(err))
			require.Equal(t, 0, calls)
			require.Len(t, recorder.records, 1)
			require.Equal(t, AuditOutcomeFailed, recorder.records[0].Outcome)
			require.Equal(t, tt.wantCode, recorder.records[0].ErrorCode)
			require.Equal(t, AuditStatusRecorded, result.AuditStatus)
		})
	}
}

func TestInvokeExecutesToolExactlyOnce(t *testing.T) {
	calls := 0
	bound := boundToolSetForTest(t, &calls, verifiedResolver())

	result, err := bound.Invoke(context.Background(), validCall())

	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
	require.Equal(t, AuditStatusRecorded, result.AuditStatus)
}

func TestInvokePassesImmutableTrustedEnvelopeSeparateFromArguments(t *testing.T) {
	sourceRoles := []string{"listingkit_admin"}
	deps := validInvocationDependencies()
	deps.PrincipalResolver = resolverStub{principal: Principal{
		TenantID: "tenant-1",
		UserID:   "user-1",
		Roles:    sourceRoles,
	}}
	definition := validDefinition()
	definition.Idempotency.Mode = IdempotencyRequiredKey
	call := validCall()
	call.Metadata.IdempotencyKey = "idem-1"
	wantArguments := cloneRaw(call.Arguments)
	var gotMetadata CallMetadata
	var gotPrincipal Principal
	var gotTool ToolRef
	var gotArguments json.RawMessage
	bound := bindDefinitionForTest(t, definition, ExecutorFunc(func(
		_ context.Context,
		envelope ExecutionEnvelope,
		arguments json.RawMessage,
	) (ExecutionResult, error) {
		sourceRoles[0] = "caller-mutated"
		call.Arguments[0] = '['
		gotMetadata = envelope.Metadata()
		gotPrincipal = envelope.Principal()
		gotTool = envelope.Tool()
		gotArguments = cloneRaw(arguments)
		mutatedPrincipal := envelope.Principal()
		mutatedPrincipal.Roles[0] = "executor-mutated"
		require.Equal(t, []string{"listingkit_admin"}, envelope.Principal().Roles)
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	}), deps)

	result, err := bound.Invoke(context.Background(), call)

	require.NoError(t, err)
	require.Equal(t, CallMetadata{
		CallID:         "call-1",
		AgentID:        "fake.product-agent",
		AgentVersion:   "v1.0.0",
		AgentRunID:     "run-1",
		BusinessTaskID: "task-1",
		TraceID:        "trace-1",
		IdempotencyKey: "idem-1",
	}, gotMetadata)
	require.Equal(t, Principal{
		TenantID: "tenant-1",
		UserID:   "user-1",
		Roles:    []string{"listingkit_admin"},
	}, gotPrincipal)
	require.Equal(t, ToolRef{ID: "product.canonical.inspect", Version: "v1.0.0"}, gotTool)
	require.Equal(t, wantArguments, gotArguments)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
	require.NotContains(t, string(gotArguments), "idem-1")
	require.NotContains(t, string(gotArguments), "call-1")
}

func TestInvokeReturnsAndAuditsApplicableAIInvocationIDOutsideModelOutput(t *testing.T) {
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindDefinitionForTest(t, validAICapabilityDefinition(), ExecutorFunc(func(
		context.Context,
		ExecutionEnvelope,
		json.RawMessage,
	) (ExecutionResult, error) {
		return ExecutionResult{
			Output:         json.RawMessage(`{"task_id":"task-1"}`),
			AIInvocationID: "ai-invocation-1",
		}, nil
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.NoError(t, err)
	require.Equal(t, "ai-invocation-1", result.AIInvocationID)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
	require.NotContains(t, string(result.Output), "ai-invocation-1")
	require.Len(t, recorder.records, 1)
	require.Equal(t, "ai-invocation-1", recorder.records[0].AIInvocationID)
}

func TestInvokePreservesApplicableAIInvocationIDWhenExecutionFails(t *testing.T) {
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindDefinitionForTest(t, validAICapabilityDefinition(), ExecutorFunc(func(
		context.Context,
		ExecutionEnvelope,
		json.RawMessage,
	) (ExecutionResult, error) {
		return ExecutionResult{
			Output:         json.RawMessage(`{"unexpected":true}`),
			AIInvocationID: "ai-invocation-1",
		}, errors.New("provider details")
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorInternal, CodeOf(err))
	require.Nil(t, result.Output)
	require.Equal(t, "ai-invocation-1", result.AIInvocationID)
	require.Len(t, recorder.records, 1)
	require.Equal(t, "ai-invocation-1", recorder.records[0].AIInvocationID)
	require.Equal(t, "e7f5f6e5a781d62f599ff51c8cfdbe866f8ff4a971b5bbb569bf22f69e5295f2", recorder.records[0].OutputHash)
}

func TestInvokeFailsClosedAndScrubsNonApplicableAIInvocationIDOnSuccess(t *testing.T) {
	tests := []struct {
		name              string
		risk              RiskLevel
		usageOwner        UsageOwner
		aiInvocationID    string
		sensitiveFragment string
	}{
		{name: "read unmetered forged ledger ID", risk: RiskRead, usageOwner: UsageOwnerUnmetered, aiInvocationID: "forged-ai-invocation", sensitiveFragment: "forged"},
		{name: "propose unmetered forged ledger ID", risk: RiskPropose, usageOwner: UsageOwnerUnmetered, aiInvocationID: "forged-propose-invocation", sensitiveFragment: "forged"},
		{name: "unmetered credential-shaped token", risk: RiskRead, usageOwner: UsageOwnerUnmetered, aiInvocationID: "sk-live-secret-token", sensitiveFragment: "sk-live-secret"},
		{name: "unmetered provider error with newline", risk: RiskRead, usageOwner: UsageOwnerUnmetered, aiInvocationID: "provider error\ncredential=secret", sensitiveFragment: "provider error"},
		{name: "propose domain ledger forged ID", risk: RiskPropose, usageOwner: UsageOwnerDomainLedger, aiInvocationID: "forged-domain-ledger-id", sensitiveFragment: "forged"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &recordingAuditStub{}
			deps := validInvocationDependencies()
			deps.Recorder = recorder
			definition := validDefinition()
			definition.Risk = tt.risk
			definition.Usage.Owner = tt.usageOwner
			bound := bindDefinitionForTest(t, definition, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
				return ExecutionResult{
					Output:         json.RawMessage(`{"task_id":"task-1"}`),
					AIInvocationID: tt.aiInvocationID,
				}, nil
			}), deps)

			result, err := bound.Invoke(context.Background(), validCall())

			require.Error(t, err)
			require.Equal(t, ErrorInternal, CodeOf(err))
			require.Equal(t, "internal: tool execution failed", err.Error())
			require.NotContains(t, err.Error(), tt.sensitiveFragment)
			require.Nil(t, result.Output)
			require.Empty(t, result.AIInvocationID)
			require.Len(t, recorder.records, 1)
			require.Equal(t, AuditOutcomeFailed, recorder.records[0].Outcome)
			require.Equal(t, ErrorInternal, recorder.records[0].ErrorCode)
			require.Empty(t, recorder.records[0].AIInvocationID)
			require.Equal(t, "8076f8fc7f4e66be2f5b3ebd07dba6328ab7043e838b3d7b50037ec25021c811", recorder.records[0].OutputHash)
			encoded, marshalErr := json.Marshal(struct {
				Result Result
				Audit  AuditRecord
			}{Result: result, Audit: recorder.records[0]})
			require.NoError(t, marshalErr)
			require.NotContains(t, string(encoded), tt.sensitiveFragment)
		})
	}
}

func TestInvokeFailsClosedWhenPostBindAIDefinitionRiskInvariantIsCorrupted(t *testing.T) {
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	definition := validAICapabilityDefinition()
	bound := bindDefinitionForTest(t, definition, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		return ExecutionResult{
			Output:         json.RawMessage(`{"task_id":"task-1"}`),
			AIInvocationID: "ai-invocation-1",
		}, nil
	}), deps)

	// NewRegistry rejects this inconsistent matrix through Definition.Validate.
	// This package-internal copy/write simulates post-Bind invariant corruption so
	// the runtime defense-in-depth cannot regress without exposing a mutation API.
	registered := bound.tools[definition.Ref]
	registered.definition.Risk = RiskRead
	bound.tools[definition.Ref] = registered

	result, err := bound.Invoke(context.Background(), validCall())

	require.Error(t, err)
	require.Equal(t, ErrorInternal, CodeOf(err))
	require.Equal(t, "internal: tool execution failed", err.Error())
	require.Nil(t, result.Output)
	require.Empty(t, result.AIInvocationID)
	require.Len(t, recorder.records, 1)
	require.Equal(t, AuditOutcomeFailed, recorder.records[0].Outcome)
	require.Equal(t, ErrorInternal, recorder.records[0].ErrorCode)
	require.Empty(t, recorder.records[0].AIInvocationID)
	require.Equal(t, "8076f8fc7f4e66be2f5b3ebd07dba6328ab7043e838b3d7b50037ec25021c811", recorder.records[0].OutputHash)
}

func TestInvokeScrubsNonApplicableAIInvocationIDWithoutOverridingExecutorError(t *testing.T) {
	tests := []struct {
		name           string
		risk           RiskLevel
		usageOwner     UsageOwner
		aiInvocationID string
	}{
		{name: "read unmetered forged ledger ID", risk: RiskRead, usageOwner: UsageOwnerUnmetered, aiInvocationID: "forged-ai-invocation"},
		{name: "propose unmetered forged ledger ID", risk: RiskPropose, usageOwner: UsageOwnerUnmetered, aiInvocationID: "forged-propose-invocation"},
		{name: "unmetered credential-shaped token", risk: RiskRead, usageOwner: UsageOwnerUnmetered, aiInvocationID: "sk-live-secret-token"},
		{name: "unmetered provider error with newline", risk: RiskRead, usageOwner: UsageOwnerUnmetered, aiInvocationID: "provider error\ncredential=secret"},
		{name: "propose domain ledger forged ID", risk: RiskPropose, usageOwner: UsageOwnerDomainLedger, aiInvocationID: "forged-domain-ledger-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &recordingAuditStub{}
			deps := validInvocationDependencies()
			deps.Recorder = recorder
			definition := validDefinition()
			definition.Risk = tt.risk
			definition.Usage.Owner = tt.usageOwner
			bound := bindDefinitionForTest(t, definition, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
				return ExecutionResult{
					Output:         json.RawMessage(`{"unexpected":true}`),
					AIInvocationID: tt.aiInvocationID,
				}, NewError(ErrorConflict, "safe conflict", errors.New("provider credential"))
			}), deps)

			result, err := bound.Invoke(context.Background(), validCall())

			require.Equal(t, ErrorConflict, CodeOf(err))
			require.Equal(t, "conflict: safe conflict", err.Error())
			require.NotContains(t, err.Error(), tt.aiInvocationID)
			require.Nil(t, result.Output)
			require.Empty(t, result.AIInvocationID)
			require.Len(t, recorder.records, 1)
			require.Equal(t, AuditOutcomeFailed, recorder.records[0].Outcome)
			require.Equal(t, ErrorConflict, recorder.records[0].ErrorCode)
			require.Empty(t, recorder.records[0].AIInvocationID)
			require.Equal(t, "e7f5f6e5a781d62f599ff51c8cfdbe866f8ff4a971b5bbb569bf22f69e5295f2", recorder.records[0].OutputHash)
		})
	}
}

func TestInvokeFailsClosedAndScrubsInvalidApplicableAIInvocationIDOnSuccess(t *testing.T) {
	tests := []struct {
		name           string
		aiInvocationID string
	}{
		{name: "whitespace only", aiInvocationID: " \t"},
		{name: "leading whitespace", aiInvocationID: " ai-invocation-1"},
		{name: "trailing whitespace", aiInvocationID: "ai-invocation-1 "},
		{name: "over 128 bytes", aiInvocationID: strings.Repeat("a", 129)},
		{name: "unsafe slash", aiInvocationID: "ai/invocation"},
		{name: "unsafe equals", aiInvocationID: "ai=credential"},
		{name: "newline", aiInvocationID: "ai-invocation\ncredential"},
		{name: "unsafe first character", aiInvocationID: "-ai-invocation"},
		{name: "non ASCII", aiInvocationID: "ai-invocación"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &recordingAuditStub{}
			deps := validInvocationDependencies()
			deps.Recorder = recorder
			bound := bindDefinitionForTest(t, validAICapabilityDefinition(), ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
				return ExecutionResult{
					Output:         json.RawMessage(`{"task_id":"task-1"}`),
					AIInvocationID: tt.aiInvocationID,
				}, nil
			}), deps)

			result, err := bound.Invoke(context.Background(), validCall())

			require.Error(t, err)
			require.Equal(t, ErrorInternal, CodeOf(err))
			require.Equal(t, "internal: tool execution failed", err.Error())
			require.Nil(t, result.Output)
			require.Empty(t, result.AIInvocationID)
			require.Len(t, recorder.records, 1)
			require.Equal(t, AuditOutcomeFailed, recorder.records[0].Outcome)
			require.Equal(t, ErrorInternal, recorder.records[0].ErrorCode)
			require.Empty(t, recorder.records[0].AIInvocationID)
			require.Equal(t, "8076f8fc7f4e66be2f5b3ebd07dba6328ab7043e838b3d7b50037ec25021c811", recorder.records[0].OutputHash)
		})
	}
}

func TestInvokeScrubsInvalidApplicableAIInvocationIDWithoutOverridingExecutorError(t *testing.T) {
	tests := []struct {
		name           string
		aiInvocationID string
	}{
		{name: "whitespace", aiInvocationID: " ai-invocation-1"},
		{name: "over 128 bytes", aiInvocationID: strings.Repeat("a", 129)},
		{name: "newline", aiInvocationID: "ai-invocation\ncredential"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &recordingAuditStub{}
			deps := validInvocationDependencies()
			deps.Recorder = recorder
			bound := bindDefinitionForTest(t, validAICapabilityDefinition(), ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
				return ExecutionResult{
					Output:         json.RawMessage(`{"unexpected":true}`),
					AIInvocationID: tt.aiInvocationID,
				}, NewError(ErrorConflict, "safe conflict", errors.New("provider credential"))
			}), deps)

			result, err := bound.Invoke(context.Background(), validCall())

			require.Equal(t, ErrorConflict, CodeOf(err))
			require.Equal(t, "conflict: safe conflict", err.Error())
			require.Nil(t, result.Output)
			require.Empty(t, result.AIInvocationID)
			require.Len(t, recorder.records, 1)
			require.Equal(t, AuditOutcomeFailed, recorder.records[0].Outcome)
			require.Equal(t, ErrorConflict, recorder.records[0].ErrorCode)
			require.Empty(t, recorder.records[0].AIInvocationID)
			require.Equal(t, "e7f5f6e5a781d62f599ff51c8cfdbe866f8ff4a971b5bbb569bf22f69e5295f2", recorder.records[0].OutputHash)
		})
	}
}

func TestInvokeAIInvocationIDContractPrecedesOutputSchemaValidation(t *testing.T) {
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindDefinitionForTest(t, validAICapabilityDefinition(), ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		return ExecutionResult{
			Output:         json.RawMessage(`{"unexpected":true}`),
			AIInvocationID: " invalid-ai-invocation",
		}, nil
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorInternal, CodeOf(err))
	require.Equal(t, "internal: tool execution failed", err.Error())
	require.Nil(t, result.Output)
	require.Empty(t, result.AIInvocationID)
	require.Len(t, recorder.records, 1)
	require.Equal(t, AuditOutcomeFailed, recorder.records[0].Outcome)
	require.Equal(t, ErrorInternal, recorder.records[0].ErrorCode)
	require.Empty(t, recorder.records[0].AIInvocationID)
	require.Equal(t, "e7f5f6e5a781d62f599ff51c8cfdbe866f8ff4a971b5bbb569bf22f69e5295f2", recorder.records[0].OutputHash)
}

func TestInvokeDeadlinePrecedesAIInvocationIDContractAndScrubsInvalidID(t *testing.T) {
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindDefinitionForTest(t, validAICapabilityDefinition(), ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		return ExecutionResult{
			Output:         json.RawMessage(`{"unexpected":true}`),
			AIInvocationID: " invalid-ai-invocation",
		}, nil
	}), deps)
	expiredCtx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()

	result, err := bound.Invoke(expiredCtx, validCall())

	require.Equal(t, ErrorDeadlineExceeded, CodeOf(err))
	require.Equal(t, "deadline_exceeded: tool deadline exceeded", err.Error())
	require.Nil(t, result.Output)
	require.Empty(t, result.AIInvocationID)
	require.Len(t, recorder.records, 1)
	require.Equal(t, AuditOutcomeFailed, recorder.records[0].Outcome)
	require.Equal(t, ErrorDeadlineExceeded, recorder.records[0].ErrorCode)
	require.Empty(t, recorder.records[0].AIInvocationID)
	require.Equal(t, "e7f5f6e5a781d62f599ff51c8cfdbe866f8ff4a971b5bbb569bf22f69e5295f2", recorder.records[0].OutputHash)
}

func TestInvokeAcceptsSafeApplicableAIInvocationIDBoundaries(t *testing.T) {
	tests := []struct {
		name           string
		aiInvocationID string
	}{
		{name: "one byte", aiInvocationID: "a"},
		{name: "128 bytes", aiInvocationID: strings.Repeat("a", 128)},
		{name: "safe token characters", aiInvocationID: "AI_9.test:+-id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := &recordingAuditStub{}
			deps := validInvocationDependencies()
			deps.Recorder = recorder
			bound := bindDefinitionForTest(t, validAICapabilityDefinition(), ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
				return ExecutionResult{
					Output:         json.RawMessage(`{"task_id":"task-1"}`),
					AIInvocationID: tt.aiInvocationID,
				}, nil
			}), deps)

			result, err := bound.Invoke(context.Background(), validCall())

			require.NoError(t, err)
			require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
			require.Equal(t, tt.aiInvocationID, result.AIInvocationID)
			require.Len(t, recorder.records, 1)
			require.Equal(t, AuditOutcomeSucceeded, recorder.records[0].Outcome)
			require.Empty(t, recorder.records[0].ErrorCode)
			require.Equal(t, tt.aiInvocationID, recorder.records[0].AIInvocationID)
			require.Equal(t, "8076f8fc7f4e66be2f5b3ebd07dba6328ab7043e838b3d7b50037ec25021c811", recorder.records[0].OutputHash)
		})
	}
}

func TestInvokeAllowsEmptyAIInvocationIDForAICapabilityUsage(t *testing.T) {
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindDefinitionForTest(t, validAICapabilityDefinition(), ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.NoError(t, err)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
	require.Empty(t, result.AIInvocationID)
	require.Len(t, recorder.records, 1)
	require.Equal(t, AuditOutcomeSucceeded, recorder.records[0].Outcome)
	require.Empty(t, recorder.records[0].AIInvocationID)
}

func TestInvokeRecorderFailurePreservesValidatedAIInvocationID(t *testing.T) {
	recorder := &recordingAuditStub{err: errors.New("audit store failed")}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindDefinitionForTest(t, validAICapabilityDefinition(), ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		return ExecutionResult{
			Output:         json.RawMessage(`{"task_id":"task-1"}`),
			AIInvocationID: "ai-invocation-1",
		}, nil
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.NoError(t, err)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
	require.Equal(t, "ai-invocation-1", result.AIInvocationID)
	require.Equal(t, AuditStatusRecordFailed, result.AuditStatus)
	require.Len(t, recorder.records, 1)
	require.Equal(t, "ai-invocation-1", recorder.records[0].AIInvocationID)
	require.Equal(t, AuditOutcomeSucceeded, recorder.records[0].Outcome)
}

func TestInvokeClonesArgumentsBeforeCallingExecutor(t *testing.T) {
	call := validCall()
	wantArguments := cloneRaw(call.Arguments)
	bound := boundToolSetWithExecutor(t, ExecutorFunc(func(_ context.Context, _ ExecutionEnvelope, arguments json.RawMessage) (ExecutionResult, error) {
		require.Equal(t, wantArguments, arguments)
		arguments[0] = '['
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	}))

	_, err := bound.Invoke(context.Background(), call)

	require.NoError(t, err)
	require.Equal(t, wantArguments, call.Arguments)
}

func TestInvokeReturnsDefensiveOutputCopy(t *testing.T) {
	tests := []struct {
		name          string
		recorderError error
		wantStatus    AuditStatus
	}{
		{name: "audit recorded", wantStatus: AuditStatusRecorded},
		{name: "audit failed", recorderError: errors.New("audit unavailable"), wantStatus: AuditStatusRecordFailed},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executorOutput := json.RawMessage(`{"task_id":"task-1"}`)
			deps := validInvocationDependencies()
			deps.Recorder = &recordingAuditStub{err: tt.recorderError}
			bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
				return ExecutionResult{Output: executorOutput}, nil
			}), deps)

			result, err := bound.Invoke(context.Background(), validCall())
			require.NoError(t, err)
			executorOutput[0] = '['

			require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
			require.Equal(t, tt.wantStatus, result.AuditStatus)
		})
	}
}

func TestInvokeSnapshotsExecutorOutputBeforeAuditRecorderCanMutateOriginalSlice(t *testing.T) {
	executorOutput := json.RawMessage(`{"task_id":"task-1"}`)
	recorder := &recordingAuditStub{
		onRecord: func() {
			copy(executorOutput, json.RawMessage(`{"task_id":"task-X"}`))
		},
	}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		return ExecutionResult{Output: executorOutput}, nil
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.NoError(t, err)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
	require.Len(t, recorder.records, 1)
	require.Equal(t, "8076f8fc7f4e66be2f5b3ebd07dba6328ab7043e838b3d7b50037ec25021c811", recorder.records[0].OutputHash)
}

func TestInvokeMapsDeadlineWithoutRetrying(t *testing.T) {
	calls := 0
	bound := boundToolSetWithExecutor(t, ExecutorFunc(func(ctx context.Context, _ ExecutionEnvelope, _ json.RawMessage) (ExecutionResult, error) {
		calls++
		<-ctx.Done()
		return ExecutionResult{}, ctx.Err()
	}))

	result, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorDeadlineExceeded, CodeOf(err))
	require.Equal(t, "deadline_exceeded: tool deadline exceeded", err.Error())
	require.Equal(t, 1, calls)
	require.Nil(t, result.Output)
	require.Equal(t, AuditStatusRecorded, result.AuditStatus)
}

func TestInvokePreservesCanceledParentContextForExecutor(t *testing.T) {
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	calls := 0
	var executorContextErr error
	var executorContextDone bool
	bound := bindToolForTest(t, ExecutorFunc(func(ctx context.Context, _ ExecutionEnvelope, _ json.RawMessage) (ExecutionResult, error) {
		calls++
		executorContextErr = ctx.Err()
		select {
		case <-ctx.Done():
			executorContextDone = true
		default:
		}
		return ExecutionResult{Output: json.RawMessage(`{"unexpected":true}`)}, nil
	}), deps)
	parentCtx, cancelParent := context.WithCancel(context.Background())
	cancelParent()

	result, err := bound.Invoke(parentCtx, validCall())

	require.Equal(t, ErrorDeadlineExceeded, CodeOf(err))
	require.ErrorIs(t, executorContextErr, context.Canceled)
	require.True(t, executorContextDone)
	require.Equal(t, 1, calls)
	require.Nil(t, result.Output)
	require.Len(t, recorder.records, 1)
	require.Equal(t, ErrorDeadlineExceeded, recorder.records[0].ErrorCode)
	require.Equal(t, "e7f5f6e5a781d62f599ff51c8cfdbe866f8ff4a971b5bbb569bf22f69e5295f2", recorder.records[0].OutputHash)
}

func TestInvokeUsesEarlierParentDeadlineWithoutRebuildingContext(t *testing.T) {
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	definition := validDefinition()
	definition.Timeout.Duration = 2 * time.Hour
	parentDeadline := time.Now().Add(time.Hour)
	parentCtx, cancelParent := context.WithDeadline(context.Background(), parentDeadline)
	defer cancelParent()
	calls := 0
	var executorDeadline time.Time
	var executorHasDeadline bool
	var executorContextErr error
	bound := bindExactDefinitionForTest(t, definition, ExecutorFunc(func(ctx context.Context, _ ExecutionEnvelope, _ json.RawMessage) (ExecutionResult, error) {
		calls++
		executorDeadline, executorHasDeadline = ctx.Deadline()
		cancelParent()
		select {
		case <-ctx.Done():
		default:
		}
		executorContextErr = ctx.Err()
		return ExecutionResult{Output: json.RawMessage(`{"unexpected":true}`)}, nil
	}), deps)

	result, err := bound.Invoke(parentCtx, validCall())

	require.Equal(t, ErrorDeadlineExceeded, CodeOf(err))
	require.True(t, executorHasDeadline)
	require.True(t, executorDeadline.Equal(parentDeadline))
	require.ErrorIs(t, executorContextErr, context.Canceled)
	require.Equal(t, 1, calls)
	require.Nil(t, result.Output)
	require.Len(t, recorder.records, 1)
	require.Equal(t, ErrorDeadlineExceeded, recorder.records[0].ErrorCode)
}

func TestInvokeDiscardsOutputWhenExecutorReturnsAfterDeadline(t *testing.T) {
	calls := 0
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindToolForTest(t, ExecutorFunc(func(ctx context.Context, _ ExecutionEnvelope, _ json.RawMessage) (ExecutionResult, error) {
		calls++
		<-ctx.Done()
		return ExecutionResult{Output: json.RawMessage(`{"unexpected":true}`)}, nil
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorDeadlineExceeded, CodeOf(err))
	require.Equal(t, 1, calls)
	require.Nil(t, result.Output)
	require.Len(t, recorder.records, 1)
	require.Equal(t, ErrorDeadlineExceeded, recorder.records[0].ErrorCode)
}

func TestInvokeRejectsInvalidExecutorOutput(t *testing.T) {
	calls := 0
	bound := boundToolSetWithExecutor(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		calls++
		return ExecutionResult{Output: json.RawMessage(`{"unexpected":true}`)}, nil
	}))

	result, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorOutputInvalid, CodeOf(err))
	require.Equal(t, "output_invalid: tool output does not match schema", err.Error())
	require.Equal(t, 1, calls)
	require.Nil(t, result.Output)
}

func TestInvokeMapsOrdinaryExecutorErrorToSafeInternal(t *testing.T) {
	calls := 0
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		calls++
		return ExecutionResult{Output: json.RawMessage(`{"unexpected":true}`)}, errors.New("database password leaked")
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorInternal, CodeOf(err))
	require.Equal(t, "internal: tool execution failed", err.Error())
	require.NotContains(t, err.Error(), "password")
	require.Equal(t, 1, calls)
	require.Nil(t, result.Output)
	require.Len(t, recorder.records, 1)
	require.Equal(t, ErrorInternal, recorder.records[0].ErrorCode)
	require.Equal(t, "e7f5f6e5a781d62f599ff51c8cfdbe866f8ff4a971b5bbb569bf22f69e5295f2", recorder.records[0].OutputHash)
}

func TestInvokePreservesToolErrorCodeAndSafeMessageOnly(t *testing.T) {
	calls := 0
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		calls++
		return ExecutionResult{Output: json.RawMessage(`{"unexpected":true}`)}, NewError(ErrorConflict, "safe conflict", errors.New("credential secret"))
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorConflict, CodeOf(err))
	require.Equal(t, "conflict: safe conflict", err.Error())
	require.NotContains(t, err.Error(), "credential")
	require.Equal(t, 1, calls)
	require.Nil(t, result.Output)
	require.Len(t, recorder.records, 1)
	require.Equal(t, ErrorConflict, recorder.records[0].ErrorCode)
}

func TestInvokeAuditUsesTrustedPrincipalHashesPayloadsAndRecordsTiming(t *testing.T) {
	recorder := &recordingAuditStub{}
	times := []time.Time{
		time.Date(2026, time.August, 31, 10, 0, 0, 0, time.FixedZone("SGT", 8*60*60)),
		time.Date(2026, time.August, 31, 10, 0, 0, int(17*time.Millisecond), time.FixedZone("SGT", 8*60*60)),
	}
	nowCalls := 0
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	deps.Now = func() time.Time {
		value := times[nowCalls]
		nowCalls++
		return value
	}
	bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"output-secret"}`)}, nil
	}), deps)
	call := validCall()
	call.Arguments = json.RawMessage(`{"task_id":"payload-secret"}`)

	result, err := bound.Invoke(context.Background(), call)

	require.NoError(t, err)
	require.Equal(t, AuditStatusRecorded, result.AuditStatus)
	require.Len(t, recorder.records, 1)
	record := recorder.records[0]
	require.Equal(t, "call-1", record.CallID)
	require.Equal(t, "fake.product-agent", record.AgentID)
	require.Equal(t, "v1.0.0", record.AgentVersion)
	require.Equal(t, "run-1", record.AgentRunID)
	require.Equal(t, validDefinition().Ref.ID, record.ToolID)
	require.Equal(t, validDefinition().Ref.Version, record.ToolVersion)
	require.Equal(t, validDefinition().Capability, record.Capability)
	require.Equal(t, validDefinition().Owner, record.Owner)
	require.Equal(t, "tenant-1", record.TenantID)
	require.Equal(t, "user-1", record.UserID)
	require.Equal(t, "task-1", record.BusinessTaskID)
	require.Equal(t, RiskRead, record.Risk)
	require.Equal(t, validDefinition().Permission.Permission, record.Permission)
	require.Equal(t, RetryOwnerCaller, record.RetryOwner)
	require.Equal(t, UsageOwnerUnmetered, record.UsageOwner)
	require.Equal(t, "0905ec74d2fee9f56c507b11621c1b459c803f5e09262284b2356e2d6b360fa0", record.InputHash)
	require.Equal(t, "7b6d9a3196a583cb81ae34ff6a33897516cb1b67f2b35154c8b6a531b61a178a", record.OutputHash)
	require.Regexp(t, `^[0-9a-f]{64}$`, record.InputHash)
	require.Regexp(t, `^[0-9a-f]{64}$`, record.OutputHash)
	require.Equal(t, times[0].UTC(), record.StartedAt)
	require.Equal(t, times[1].UTC(), record.FinishedAt)
	require.Equal(t, int64(17), record.LatencyMillis)
	require.Equal(t, AuditOutcomeSucceeded, record.Outcome)
	require.Empty(t, record.ErrorCode)
	require.Empty(t, record.AIInvocationID)
	require.Equal(t, "trace-1", record.TraceID)

	encoded, marshalErr := json.Marshal(record)
	require.NoError(t, marshalErr)
	require.NotContains(t, string(encoded), "payload-secret")
	require.NotContains(t, string(encoded), "output-secret")
	for i := 0; i < reflect.TypeOf(record).NumField(); i++ {
		name := strings.ToLower(reflect.TypeOf(record).Field(i).Name)
		if strings.Contains(name, "input") || strings.Contains(name, "output") {
			require.True(t, strings.HasSuffix(name, "hash"), "raw payload field %q must not exist", name)
		}
	}
}

func TestInvokeAllowsEmptyTraceIDWithoutForgingAuditCorrelation(t *testing.T) {
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	}), deps)
	call := validCall()
	call.Metadata.TraceID = ""

	result, err := bound.Invoke(context.Background(), call)

	require.NoError(t, err)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
	require.Len(t, recorder.records, 1)
	require.Empty(t, recorder.records[0].TraceID)
}

func TestInvokeAuditsSuccessAndFailureExactlyOnce(t *testing.T) {
	tests := []struct {
		name        string
		executorErr error
		wantOutcome AuditOutcome
		wantCode    ErrorCode
	}{
		{name: "success", wantOutcome: AuditOutcomeSucceeded},
		{name: "failure", executorErr: errors.New("executor failed"), wantOutcome: AuditOutcomeFailed, wantCode: ErrorInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			recorder := &recordingAuditStub{}
			deps := validInvocationDependencies()
			deps.Recorder = recorder
			bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
				calls++
				return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, tt.executorErr
			}), deps)

			_, _ = bound.Invoke(context.Background(), validCall())

			require.Equal(t, 1, calls)
			require.Len(t, recorder.records, 1)
			require.Equal(t, tt.wantOutcome, recorder.records[0].Outcome)
			require.Equal(t, tt.wantCode, recorder.records[0].ErrorCode)
		})
	}
}

func TestInvokeRecorderFailureDoesNotReplayOrOverrideSuccessfulOutput(t *testing.T) {
	calls := 0
	recorder := &recordingAuditStub{err: errors.New("audit store failed")}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		calls++
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.NoError(t, err)
	require.Equal(t, 1, calls)
	require.Len(t, recorder.records, 1)
	require.JSONEq(t, `{"task_id":"task-1"}`, string(result.Output))
	require.Equal(t, AuditStatusRecordFailed, result.AuditStatus)
}

func TestInvokeRecorderFailureDoesNotOverrideToolError(t *testing.T) {
	recorder := &recordingAuditStub{err: errors.New("audit store failed")}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		return ExecutionResult{}, NewError(ErrorConflict, "safe conflict", nil)
	}), deps)

	result, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorConflict, CodeOf(err))
	require.Equal(t, AuditStatusRecordFailed, result.AuditStatus)
	require.Len(t, recorder.records, 1)
}

func TestInvokeRecorderContextSurvivesCallCancellationAndHasAuditDeadline(t *testing.T) {
	recorder := &contextRecordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Recorder = recorder
	deps.AuditTimeout = 50 * time.Millisecond
	bound := bindToolForTest(t, ExecutorFunc(func(ctx context.Context, _ ExecutionEnvelope, _ json.RawMessage) (ExecutionResult, error) {
		<-ctx.Done()
		return ExecutionResult{}, ctx.Err()
	}), deps)

	_, err := bound.Invoke(context.Background(), validCall())

	require.Equal(t, ErrorDeadlineExceeded, CodeOf(err))
	require.NoError(t, recorder.contextErr)
	require.True(t, recorder.hasDeadline)
	require.Greater(t, recorder.deadlineLeft, time.Duration(0))
	require.LessOrEqual(t, recorder.deadlineLeft, deps.AuditTimeout)
}

func TestInvokeCreatesGovernedOpenTelemetrySpanWithoutRawArguments(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Tracer = provider.Tracer("commercetool-test")
	deps.Recorder = recorder
	bound := bindToolForTest(t, ExecutorFunc(func(ctx context.Context, _ ExecutionEnvelope, _ json.RawMessage) (ExecutionResult, error) {
		require.True(t, trace.SpanFromContext(ctx).SpanContext().IsValid())
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	}), deps)
	call := validCall()
	call.Arguments = json.RawMessage(`{"task_id":"span-payload-secret"}`)

	_, err := bound.Invoke(context.Background(), call)

	require.NoError(t, err)
	spans := spanRecorder.Ended()
	require.Len(t, spans, 1)
	span := spans[0]
	require.Equal(t, "commerce.tool.invoke", span.Name())
	attributes := attributeMap(span.Attributes())
	require.Len(t, attributes, 5)
	require.Equal(t, validDefinition().Ref.ID, attributes["commerce.tool.id"])
	require.Equal(t, validDefinition().Ref.Version, attributes["commerce.tool.version"])
	require.Equal(t, "fake.product-agent", attributes["commerce.agent.id"])
	require.Equal(t, "run-1", attributes["commerce.agent.run_id"])
	require.Equal(t, string(RiskRead), attributes["commerce.tool.risk"])
	for key, value := range attributes {
		require.NotContains(t, strings.ToLower(key), "argument")
		require.NotContains(t, value, "span-payload-secret")
	}
	require.Len(t, recorder.records, 1)
	require.Equal(t, "trace-1", recorder.records[0].TraceID)
	require.NotEqual(t, recorder.records[0].TraceID, span.SpanContext().TraceID().String())
}

func TestInvokePreflightRejectionDoesNotCreateInvocationSpan(t *testing.T) {
	spanRecorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(spanRecorder))
	t.Cleanup(func() { require.NoError(t, provider.Shutdown(context.Background())) })
	recorder := &recordingAuditStub{}
	deps := validInvocationDependencies()
	deps.Tracer = provider.Tracer("commercetool-test")
	deps.Recorder = recorder
	calls := 0
	bound := bindToolForTest(t, ExecutorFunc(func(context.Context, ExecutionEnvelope, json.RawMessage) (ExecutionResult, error) {
		calls++
		return ExecutionResult{Output: json.RawMessage(`{"task_id":"task-1"}`)}, nil
	}), deps)
	call := validCall()
	call.Metadata.CallID = ""

	result, err := bound.Invoke(context.Background(), call)

	require.Equal(t, ErrorInvalidInput, CodeOf(err))
	require.Equal(t, 0, calls)
	require.Len(t, recorder.records, 1)
	require.Equal(t, AuditStatusRecorded, result.AuditStatus)
	require.Empty(t, spanRecorder.Started())
	require.Empty(t, spanRecorder.Ended())
}

func verifiedPrincipal() Principal {
	return Principal{TenantID: "tenant-1", UserID: "user-1", Roles: []string{"listingkit_admin"}}
}

func attributeMap(attributes []attribute.KeyValue) map[string]string {
	result := make(map[string]string, len(attributes))
	for _, item := range attributes {
		result[string(item.Key)] = item.Value.AsString()
	}
	return result
}
