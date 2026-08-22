package enrich

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/aicapability"
	productenrich "task-processor/internal/productenrich"
	"task-processor/internal/shared/aiidentity"
)

type LegacyRouteMetadataResolver interface {
	ResolveLegacyRoute(context.Context, string) (aicapability.RouteDecision, error)
}

type preparedExecution struct {
	identity                 aiidentity.Identity
	plan                     aicapability.ExecutionPlan
	decision                 aicapability.RouteDecision
	promptKey, promptVersion string
	promptScope              string
	prompt, input            string
	call                     func(context.Context) (string, error)
	recorder                 aicapability.InvocationRecorder
	capability               aicapability.Capability
	operation                aicapability.Operation
	onRecordError            func(aicapability.InvocationRecord, error)
	now                      func() time.Time
	newID                    func() string
}

func prepareGovernedExecution(
	ctx context.Context,
	execution *preparedExecution,
	manager productenrich.LLMManager,
	planner aicapability.ExecutionPlanner,
	legacyMetadata LegacyRouteMetadataResolver,
	request aicapability.RouteRequest,
	bindCall func(productenrich.LLMClient) func(context.Context) (string, error),
) (*preparedExecution, error) {
	plan, err := planner.Plan(ctx, request)
	if err != nil {
		execution.plan = plan
		if execution.plan.Mode == "" {
			execution.plan = aicapability.ExecutionPlan{Mode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive}
		}
		if aicapability.CategoryOf(err) == aicapability.ErrorUnknown {
			err = aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(request.Operation), err)
		}
		execution.recordRejected(ctx, err, true)
		return nil, err
	}
	execution.plan = plan
	if err := plan.Validate(); err != nil {
		wrapped := aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(request.Operation), err)
		execution.recordRejected(ctx, wrapped, true)
		return nil, wrapped
	}

	client, decision, err := bindExecutionClient(ctx, manager, legacyMetadata, plan, request)
	execution.decision = decision
	if err != nil {
		execution.recordRejected(ctx, err, false)
		return nil, err
	}
	execution.call = bindCall(client)
	return execution, nil
}

func bindExecutionClient(
	ctx context.Context,
	manager productenrich.LLMManager,
	legacyMetadata LegacyRouteMetadataResolver,
	plan aicapability.ExecutionPlan,
	request aicapability.RouteRequest,
) (productenrich.LLMClient, aicapability.RouteDecision, error) {
	if plan.Mode == aicapability.RoutingModeActive {
		decision := plan.Decision
		if !validExecutionDecision(decision, request.Capability, request.Operation) {
			return nil, decision, aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(request.Operation), nil)
		}
		routedManager, ok := manager.(productenrich.RoutedLLMManager)
		if !ok {
			return nil, decision, aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(request.Operation), nil)
		}
		client, err := routedManager.GetClientWithRoute(ctx, decision.CredentialReference, productenrich.LLMClientRoute{
			CredentialReference: decision.CredentialReference, ConfigurationVersion: decision.ConfigurationVersion,
		})
		if err != nil || client == nil {
			if err == nil {
				err = errors.New("routed ProductEnrich client is nil")
			}
			return nil, decision, aicapability.NewError(aicapability.ErrorCredentialUnavailable, string(request.Operation), err)
		}
		return client, decision, nil
	}

	var lastErr error
	var lastDecision aicapability.RouteDecision
	for fallbackIndex, clientName := range normalizedLegacyClients(plan.LegacyClients) {
		client, err := manager.GetClient(clientName)
		if (err != nil || client == nil) && clientName == "default" {
			client = manager.GetDefaultClient()
			if client != nil {
				err = nil
			}
		}
		if err != nil || client == nil {
			if err == nil {
				err = fmt.Errorf("legacy ProductEnrich client %q is nil", clientName)
			}
			lastErr = err
			continue
		}

		decision, resolveErr := legacyMetadata.ResolveLegacyRoute(ctx, clientName)
		decision.Capability = request.Capability
		decision.Operation = request.Operation
		decision.FallbackIndex = fallbackIndex
		lastDecision = decision
		if resolveErr != nil || !validLegacyDecision(decision) {
			if resolveErr == nil {
				resolveErr = fmt.Errorf("legacy ProductEnrich client %q has incomplete route metadata", clientName)
			}
			lastErr = resolveErr
			continue
		}
		return client, decision, nil
	}
	if lastErr == nil {
		lastErr = errors.New("no legacy ProductEnrich client is available")
	}
	return nil, lastDecision, aicapability.NewError(aicapability.ErrorCredentialUnavailable, string(request.Operation), lastErr)
}

func normalizedLegacyClients(clientNames []string) []string {
	result := make([]string, 0, len(clientNames))
	seen := make(map[string]struct{}, len(clientNames))
	for _, clientName := range clientNames {
		clientName = strings.TrimSpace(clientName)
		if clientName == "" {
			continue
		}
		if _, ok := seen[clientName]; ok {
			continue
		}
		seen[clientName] = struct{}{}
		result = append(result, clientName)
	}
	return result
}

func validExecutionDecision(decision aicapability.RouteDecision, capability aicapability.Capability, operation aicapability.Operation) bool {
	return decision.Capability == capability && decision.Operation == operation &&
		strings.TrimSpace(decision.ProviderID) != "" &&
		strings.TrimSpace(decision.ModelID) != "" &&
		strings.TrimSpace(decision.RoutingKey) != "" &&
		strings.TrimSpace(decision.CredentialReference) != ""
}

func validLegacyDecision(decision aicapability.RouteDecision) bool {
	return strings.TrimSpace(decision.ProviderID) != "" &&
		strings.TrimSpace(decision.ModelID) != "" &&
		strings.TrimSpace(decision.RoutingKey) != "" &&
		strings.TrimSpace(decision.CredentialReference) != "" &&
		strings.TrimSpace(decision.ConfigurationVersion) != ""
}

func (e *preparedExecution) invoke(ctx context.Context, cacheStatus aicapability.CacheStatus) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if e == nil || e.call == nil {
		return "", aicapability.NewError(aicapability.ErrorInvalidInput, "", nil)
	}
	startedAt := e.clock()()
	response, err := e.call(ctx)
	if err != nil {
		wrapped := aicapability.NewError(classifyTextError(err), string(e.operationName()), err)
		e.record(ctx, startedAt, response, wrapped, false, cacheStatus)
		return "", wrapped
	}
	e.record(ctx, startedAt, response, nil, false, cacheStatus)
	return response, nil
}

func (e *preparedExecution) recordRejected(ctx context.Context, err error, routeErr bool) {
	e.record(ctx, e.clock()(), "", err, routeErr, aicapability.CacheStatusNotApplicable)
}

func (e *preparedExecution) record(ctx context.Context, startedAt time.Time, response string, callErr error, routeErr bool, cacheStatus aicapability.CacheStatus) {
	if e == nil || e.recorder == nil {
		return
	}
	finishedAt := e.clock()()
	capability := e.capability
	if capability == "" {
		capability = e.decision.Capability
	}
	operation := e.operationName()
	record := aicapability.InvocationRecord{
		InvocationID: e.idFactory()(), TenantID: e.identity.TenantID, UserID: e.identity.UserID,
		BusinessTaskID: e.identity.BusinessTaskID, TraceID: e.identity.TraceID,
		Capability: capability, Operation: operation,
		RouteMode: e.plan.Mode, RouteOutcome: e.plan.RouteOutcome, CacheStatus: cacheStatus,
		ProviderID: e.decision.ProviderID, ModelID: e.decision.ModelID, RoutingKey: e.decision.RoutingKey,
		CredentialReference: e.decision.CredentialReference, PolicyVersion: e.decision.PolicyVersion,
		ConfigurationVersion: e.decision.ConfigurationVersion, PromptKey: e.promptKey,
		PromptVersion: e.promptVersion, PromptScope: e.promptScope,
		PromptHash: hashText(e.prompt), InputHash: hashText(e.input), OutputHash: hashText(response),
		StartedAt: startedAt, FinishedAt: finishedAt, LatencyMilliseconds: finishedAt.Sub(startedAt).Milliseconds(),
		Attempt: 1, FallbackIndex: e.decision.FallbackIndex, Outcome: aicapability.InvocationSucceeded,
	}
	if callErr != nil {
		record.Outcome = aicapability.InvocationFailed
		record.ErrorCategory = classifyTextError(callErr)
		record.ErrorCode = string(record.ErrorCategory)
		if routeErr && aicapability.CategoryOf(callErr) != aicapability.ErrorUnknown {
			record.RouteErrorCategory = aicapability.CategoryOf(callErr)
		}
	}
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), governedTextRecordTimeout)
	defer cancel()
	if err := e.recorder.RecordInvocation(recordCtx, record); err != nil && e.onRecordError != nil {
		e.onRecordError(record, err)
	}
}

func (e *preparedExecution) operationName() aicapability.Operation {
	if e.operation != "" {
		return e.operation
	}
	return e.decision.Operation
}

func (e *preparedExecution) clock() func() time.Time {
	if e.now != nil {
		return e.now
	}
	return time.Now
}

func (e *preparedExecution) idFactory() func() string {
	if e.newID != nil {
		return e.newID
	}
	return uuid.NewString
}
