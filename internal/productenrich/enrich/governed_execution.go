package enrich

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/aicapability"
	"task-processor/internal/infra/resilience"
	productenrich "task-processor/internal/productenrich"
	"task-processor/internal/shared/aiidentity"
)

type LegacyRouteMetadataResolver interface {
	ResolveLegacyRoute(context.Context, string) (aicapability.RouteDecision, error)
}

var ErrLegacyRouteUnavailable = errors.New("legacy route is unavailable")

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

	routedManager, ok := manager.(productenrich.RoutedLLMManager)
	if !ok {
		return nil, aicapability.RouteDecision{}, aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(request.Operation), errors.New("legacy ProductEnrich manager does not support bound routes"))
	}
	var lastErr error
	var lastDecision aicapability.RouteDecision
	for fallbackIndex, clientName := range normalizedLegacyClients(plan.LegacyClients) {
		decision, resolveErr := legacyMetadata.ResolveLegacyRoute(ctx, clientName)
		decision.Capability = request.Capability
		decision.Operation = request.Operation
		decision.FallbackIndex = fallbackIndex
		lastDecision = decision
		if errors.Is(resolveErr, ErrLegacyRouteUnavailable) {
			lastErr = resolveErr
			continue
		}
		if resolveErr != nil {
			return nil, decision, aicapability.NewError(aicapability.ErrorCredentialUnavailable, string(request.Operation), resolveErr)
		}
		if !validLegacyDecision(decision) {
			return nil, decision, aicapability.NewError(aicapability.ErrorCredentialUnavailable, string(request.Operation), fmt.Errorf("legacy ProductEnrich client %q has incomplete route metadata", clientName))
		}
		client, bindErr := routedManager.GetClientWithRoute(ctx, clientName, productenrich.LLMClientRoute{
			CredentialReference: decision.CredentialReference, ConfigurationVersion: decision.ConfigurationVersion,
		})
		if errors.Is(bindErr, productenrich.ErrLLMClientUnavailable) {
			lastErr = bindErr
			continue
		}
		if bindErr != nil || client == nil {
			if bindErr == nil {
				bindErr = fmt.Errorf("legacy ProductEnrich client %q is nil", clientName)
			}
			return nil, decision, aicapability.NewError(aicapability.ErrorCredentialUnavailable, string(request.Operation), bindErr)
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
	return e.invokeValidated(ctx, cacheStatus, 1, nil)
}

func (e *preparedExecution) invokeValidated(ctx context.Context, cacheStatus aicapability.CacheStatus, maxAttempts int, validate func(string) error) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if e == nil || e.call == nil {
		return "", aicapability.NewError(aicapability.ErrorInvalidInput, "", nil)
	}
	if maxAttempts <= 0 {
		maxAttempts = 1
	}
	startedAt := e.clock()()
	attempts := 0
	var response string
	err := resilience.Retry(ctx, resilience.RetryConfig{
		MaxAttempts:         maxAttempts,
		InitialDelay:        time.Second,
		MaxDelay:            30 * time.Second,
		Multiplier:          2,
		RandomizationFactor: 0,
		IsRetryable:         retryableScoreProviderError,
	}, func(callCtx context.Context) error {
		attempts++
		attemptResponse, callErr := e.call(callCtx)
		response = attemptResponse
		if callErr != nil {
			if aicapability.CategoryOf(callErr) != aicapability.ErrorUnknown {
				return callErr
			}
			return aicapability.NewError(classifyTextError(callErr), string(e.operationName()), callErr)
		}
		if validate == nil {
			return nil
		}
		if validationErr := validate(attemptResponse); validationErr != nil {
			if aicapability.CategoryOf(validationErr) != aicapability.ErrorUnknown {
				return validationErr
			}
			return aicapability.NewError(aicapability.ErrorInvalidProviderResponse, string(e.operationName()), validationErr)
		}
		return nil
	})
	if err != nil {
		terminalErr := err
		if aicapability.CategoryOf(terminalErr) == aicapability.ErrorUnknown && !errors.Is(terminalErr, context.Canceled) {
			terminalErr = aicapability.NewError(classifyTextError(terminalErr), string(e.operationName()), terminalErr)
		}
		e.recordAttempt(ctx, startedAt, response, terminalErr, false, cacheStatus, attempts)
		return response, terminalErr
	}
	e.recordAttempt(ctx, startedAt, response, nil, false, cacheStatus, attempts)
	return response, nil
}

func (e *preparedExecution) Invoke(ctx context.Context, cacheStatus aicapability.CacheStatus) (string, error) {
	return e.invoke(ctx, cacheStatus)
}

func (e *preparedExecution) InvokeValidated(ctx context.Context, cacheStatus aicapability.CacheStatus, maxAttempts int, validate func(string) error) (string, error) {
	return e.invokeValidated(ctx, cacheStatus, maxAttempts, validate)
}

func (e *preparedExecution) RecordCacheHit(ctx context.Context, cachedScore string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if e == nil {
		return aicapability.NewError(aicapability.ErrorInvalidInput, "", nil)
	}
	e.record(ctx, e.clock()(), cachedScore, nil, false, aicapability.CacheStatusHit)
	return nil
}

func (e *preparedExecution) ScoreCacheIdentity(baseScore, inputHash string) productenrich.ScoreCacheIdentity {
	if e == nil {
		return productenrich.ScoreCacheIdentity{}
	}
	capability := e.capability
	if capability == "" {
		capability = e.decision.Capability
	}
	return productenrich.ScoreCacheIdentity{
		Version:  productenrich.ScoreCacheIdentityVersion,
		TenantID: e.identity.TenantID, Capability: capability, Operation: e.operationName(),
		RouteMode: e.plan.Mode, RouteOutcome: e.plan.RouteOutcome,
		ProviderID: e.decision.ProviderID, ModelID: e.decision.ModelID, RoutingKey: e.decision.RoutingKey,
		PolicyVersion: e.decision.PolicyVersion, ConfigurationVersion: e.decision.ConfigurationVersion,
		PromptKey: e.promptKey, PromptVersion: e.promptVersion, PromptScope: e.promptScope,
		PromptHash: e.promptHash(), BaseScore: baseScore, InputHash: inputHash,
	}
}

func (e *preparedExecution) promptHash() string {
	if e == nil {
		return ""
	}
	return hashText(e.prompt)
}

func (e *preparedExecution) setScorePromptIdentity(identity productenrich.ScorePromptIdentity) {
	if e == nil {
		return
	}
	e.promptKey = strings.TrimSpace(identity.PromptKey)
	e.promptVersion = strings.TrimSpace(identity.PromptVersion)
	e.promptScope = strings.TrimSpace(identity.PromptScope)
}

func (g *governedTextGenerator) PrepareText(ctx context.Context, prompt string, identity productenrich.ScorePromptIdentity) (productenrich.GovernedScoreExecution, error) {
	execution, err := g.prepare(ctx, prompt)
	if err != nil {
		return nil, err
	}
	execution.setScorePromptIdentity(identity)
	return execution, nil
}

func (a *governedImageAnalyzer) PrepareImage(ctx context.Context, imageURL, prompt string, identity productenrich.ScorePromptIdentity) (productenrich.GovernedScoreExecution, error) {
	execution, err := a.prepare(ctx, imageURL, prompt)
	if err != nil {
		return nil, err
	}
	execution.setScorePromptIdentity(identity)
	return execution, nil
}

func (e *preparedExecution) recordRejected(ctx context.Context, err error, routeErr bool) {
	e.record(ctx, e.clock()(), "", err, routeErr, aicapability.CacheStatusNotApplicable)
}

func (e *preparedExecution) record(ctx context.Context, startedAt time.Time, response string, callErr error, routeErr bool, cacheStatus aicapability.CacheStatus) {
	e.recordAttempt(ctx, startedAt, response, callErr, routeErr, cacheStatus, 1)
}

func (e *preparedExecution) recordAttempt(ctx context.Context, startedAt time.Time, response string, callErr error, routeErr bool, cacheStatus aicapability.CacheStatus, attempt int) {
	if e == nil || e.recorder == nil {
		return
	}
	if attempt <= 0 {
		attempt = 1
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
		PromptHash: e.promptHash(), InputHash: hashText(e.input), OutputHash: hashText(response),
		StartedAt: startedAt, FinishedAt: finishedAt, LatencyMilliseconds: finishedAt.Sub(startedAt).Milliseconds(),
		Attempt: attempt, FallbackIndex: e.decision.FallbackIndex, Outcome: aicapability.InvocationSucceeded,
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

func retryableScoreProviderError(err error) bool {
	switch aicapability.CategoryOf(err) {
	case aicapability.ErrorRateLimited, aicapability.ErrorProviderTimeout, aicapability.ErrorProviderUnavailable:
		return true
	default:
		return false
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

var _ productenrich.GovernedScoreExecution = (*preparedExecution)(nil)
var _ productenrich.TextExecutionPreparer = (*governedTextGenerator)(nil)
var _ productenrich.ImageExecutionPreparer = (*governedImageAnalyzer)(nil)
