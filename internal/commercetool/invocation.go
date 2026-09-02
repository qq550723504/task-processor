package commercetool

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const invocationSpanName = "commerce.tool.invoke"

type invocationState struct {
	startedAt  time.Time
	call       Call
	principal  Principal
	definition Definition
	hasTool    bool
}

func (tools *BoundToolSet) Invoke(ctx context.Context, call Call) (Result, error) {
	state := newInvocationState(tools.deps.Now().UTC(), call)
	registered, err := tools.preflight(ctx, state.call, &state)
	if err != nil {
		return tools.finish(ctx, state, nil, "", err)
	}

	callCtx, cancel := context.WithTimeout(ctx, registered.definition.Timeout.Duration)
	defer cancel()
	callCtx, span := tools.startSpan(callCtx, registered.definition, state.call.Metadata)
	defer span.End()

	execution, callErr := registered.executor.Execute(
		callCtx,
		newExecutionEnvelope(state.call.Tool, state.call.Metadata, state.principal),
		cloneRaw(state.call.Arguments),
	)
	output := cloneRaw(execution.Output)
	aiInvocationID, aiInvocationErr := validatedAIInvocationID(registered.definition, execution.AIInvocationID)
	if callCtx.Err() != nil {
		callErr = NewError(ErrorDeadlineExceeded, "tool deadline exceeded", callCtx.Err())
	} else if callErr != nil {
		callErr = normalizeExecutorError(callErr)
	} else if aiInvocationErr != nil {
		callErr = NewError(ErrorInternal, "tool execution failed", aiInvocationErr)
	} else if err := registered.schemas.validateOutput(output); err != nil {
		callErr = err
	} else if err := validateNoReservedAuthorityFields(
		output,
		ErrorOutputInvalid,
		"tool output does not match schema",
	); err != nil {
		callErr = err
	}

	return tools.finish(callCtx, state, output, aiInvocationID, callErr)
}

func newInvocationState(startedAt time.Time, call Call) invocationState {
	call.Arguments = cloneRaw(call.Arguments)
	return invocationState{startedAt: startedAt, call: call}
}

func (tools *BoundToolSet) preflight(ctx context.Context, call Call, state *invocationState) (registeredTool, error) {
	if !completeMetadata(call.Metadata) {
		return registeredTool{}, NewError(ErrorInvalidInput, "call metadata is incomplete", nil)
	}
	if call.Metadata.AgentID != tools.agent.ID || call.Metadata.AgentVersion != tools.agent.Version {
		return registeredTool{}, toolNotAllowed(nil)
	}

	registered, exists := tools.tools[call.Tool]
	if !exists {
		return registeredTool{}, toolNotAllowed(nil)
	}
	state.definition = registered.definition
	state.hasTool = true

	if !invocableRisk(registered.definition.Risk) {
		return registeredTool{}, toolNotAllowed(nil)
	}

	principal, err := tools.deps.PrincipalResolver.ResolvePrincipal(ctx)
	if err != nil {
		return registeredTool{}, NewError(ErrorIdentityIntegrity, "verified principal is unavailable", err)
	}
	principal = clonePrincipal(principal)
	if err := principal.validate(); err != nil {
		return registeredTool{}, NewError(ErrorIdentityIntegrity, "verified principal is unavailable", err)
	}
	state.principal = principal

	if err := tools.deps.Authorizer.Authorize(ctx, clonePrincipal(principal), registered.definition.Permission); err != nil {
		return registeredTool{}, NewError(ErrorPermissionDenied, "tool permission denied", err)
	}

	if registered.definition.Idempotency.Mode == IdempotencyRequiredKey && strings.TrimSpace(call.Metadata.IdempotencyKey) == "" {
		return registeredTool{}, NewError(ErrorInvalidInput, "idempotency key is required", nil)
	}

	if err := registered.schemas.validateInput(call.Arguments); err != nil {
		return registeredTool{}, err
	}
	if err := validateNoReservedAuthorityFields(
		call.Arguments,
		ErrorInvalidInput,
		"tool input does not match schema",
	); err != nil {
		return registeredTool{}, err
	}

	return registered, nil
}

func validateNoReservedAuthorityFields(raw json.RawMessage, code ErrorCode, safeMessage string) error {
	document, err := decodeJSON(raw)
	if err == nil && containsReservedAuthorityField(document) {
		err = errors.New("payload contains a reserved authority field")
	}
	if err != nil {
		return NewError(code, safeMessage, err)
	}

	return nil
}

func containsReservedAuthorityField(document any) bool {
	switch value := document.(type) {
	case map[string]any:
		for name, child := range value {
			if isReservedAuthorityField(name) || containsReservedAuthorityField(child) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if containsReservedAuthorityField(child) {
				return true
			}
		}
	}

	return false
}

func completeMetadata(metadata CallMetadata) bool {
	return strings.TrimSpace(metadata.CallID) != "" &&
		strings.TrimSpace(metadata.AgentID) != "" &&
		strings.TrimSpace(metadata.AgentVersion) != "" &&
		strings.TrimSpace(metadata.AgentRunID) != "" &&
		strings.TrimSpace(metadata.BusinessTaskID) != ""
}

func invocableRisk(risk RiskLevel) bool {
	switch risk {
	case RiskRead, RiskCompute, RiskPropose:
		return true
	default:
		return false
	}
}

func normalizeExecutorError(err error) error {
	var toolError *ToolError
	if errors.As(err, &toolError) && toolError != nil {
		return NewError(toolError.Code, toolError.SafeMessage, err)
	}

	return NewError(ErrorInternal, "tool execution failed", err)
}

func (tools *BoundToolSet) startSpan(
	ctx context.Context,
	definition Definition,
	metadata CallMetadata,
) (context.Context, trace.Span) {
	return tools.deps.Tracer.Start(ctx, invocationSpanName, trace.WithAttributes(
		attribute.String("commerce.tool.id", definition.Ref.ID),
		attribute.String("commerce.tool.version", definition.Ref.Version),
		attribute.String("commerce.agent.id", metadata.AgentID),
		attribute.String("commerce.agent.run_id", metadata.AgentRunID),
		attribute.String("commerce.tool.risk", string(definition.Risk)),
	))
}

func (tools *BoundToolSet) finish(
	ctx context.Context,
	state invocationState,
	output json.RawMessage,
	aiInvocationID string,
	callErr error,
) (Result, error) {
	finishedAt := tools.deps.Now().UTC()
	record := state.auditRecord(finishedAt, output, aiInvocationID, callErr)
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), tools.deps.AuditTimeout)
	defer cancel()

	status := AuditStatusRecorded
	if err := tools.deps.Recorder.RecordToolCall(auditCtx, record); err != nil {
		status = AuditStatusRecordFailed
	}

	result := Result{AIInvocationID: aiInvocationID, AuditStatus: status}
	if callErr == nil {
		result.Output = cloneRaw(output)
	}
	return result, callErr
}

func (state invocationState) auditRecord(
	finishedAt time.Time,
	output json.RawMessage,
	aiInvocationID string,
	callErr error,
) AuditRecord {
	metadata := state.call.Metadata
	record := AuditRecord{
		CallID:         metadata.CallID,
		AgentID:        metadata.AgentID,
		AgentVersion:   metadata.AgentVersion,
		AgentRunID:     metadata.AgentRunID,
		ToolID:         state.call.Tool.ID,
		ToolVersion:    state.call.Tool.Version,
		TenantID:       state.principal.TenantID,
		UserID:         state.principal.UserID,
		BusinessTaskID: metadata.BusinessTaskID,
		TraceID:        metadata.TraceID,
		StartedAt:      state.startedAt,
		FinishedAt:     finishedAt,
		LatencyMillis:  finishedAt.Sub(state.startedAt).Milliseconds(),
		InputHash:      hashRaw(state.call.Arguments),
		OutputHash:     hashRaw(output),
		Outcome:        AuditOutcomeSucceeded,
		AIInvocationID: aiInvocationID,
	}
	if state.hasTool {
		record.ToolID = state.definition.Ref.ID
		record.ToolVersion = state.definition.Ref.Version
		record.Capability = state.definition.Capability
		record.Owner = state.definition.Owner
		record.Risk = state.definition.Risk
		record.Permission = state.definition.Permission.Permission
		record.RetryOwner = state.definition.Retry.Owner
		record.UsageOwner = state.definition.Usage.Owner
	}
	if callErr != nil {
		record.Outcome = AuditOutcomeFailed
		record.ErrorCode = CodeOf(callErr)
	}
	return record
}

func hashRaw(raw json.RawMessage) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
