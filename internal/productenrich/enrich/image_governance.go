package enrich

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/aicapability"
	productenrich "task-processor/internal/productenrich"
	"task-processor/internal/shared/aiidentity"
)

const (
	productEnrichVisionPromptKey     = "productenrich.understanding.analyze_image"
	productEnrichVisionPromptVersion = "v1"
	productEnrichVisionPromptScope   = "product_enrich"
)

// ImageAnalyzer is the narrow model capability used by ProductEnrich image
// understanding. It keeps image routing out of the domain parser and fusion code.
type ImageAnalyzer interface {
	AnalyzeImage(context.Context, string, string) (string, error)
}

type GovernedImageAnalyzerConfig struct {
	Router        aicapability.Router
	Recorder      aicapability.InvocationRecorder
	OnRecordError func(aicapability.InvocationRecord, error)
	Now           func() time.Time
	NewID         func() string
}

type governedImageAnalyzer struct {
	manager       productenrich.LLMManager
	router        aicapability.Router
	recorder      aicapability.InvocationRecorder
	onRecordError func(aicapability.InvocationRecord, error)
	now           func() time.Time
	newID         func() string
}

func NewGovernedImageAnalyzer(manager productenrich.LLMManager, config GovernedImageAnalyzerConfig) (ImageAnalyzer, error) {
	if manager == nil || config.Router == nil || config.Recorder == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductEnrichImageAnalyze), nil)
	}
	if _, ok := manager.(productenrich.RoutedLLMManager); !ok {
		return nil, aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(aicapability.OperationProductEnrichImageAnalyze), nil)
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.NewID == nil {
		config.NewID = uuid.NewString
	}
	return &governedImageAnalyzer{
		manager: manager, router: config.Router, recorder: config.Recorder,
		onRecordError: config.OnRecordError, now: config.Now, newID: config.NewID,
	}, nil
}

func (a *governedImageAnalyzer) AnalyzeImage(ctx context.Context, imageURL, prompt string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if a == nil || a.manager == nil || strings.TrimSpace(imageURL) == "" || strings.TrimSpace(prompt) == "" {
		return "", aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductEnrichImageAnalyze), nil)
	}
	startedAt := a.now()
	identity := aiidentity.FromContext(ctx)
	if identity.TenantID == "" || identity.UserID == "" {
		err := aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductEnrichImageAnalyze), nil)
		a.record(ctx, identity, startedAt, imageURL, prompt, "", aicapability.RouteDecision{}, err, true)
		return "", err
	}
	decision, err := a.router.Decide(ctx, aicapability.RouteRequest{
		TenantID: identity.TenantID, UserID: identity.UserID,
		Capability:       aicapability.CapabilityProductEnrichVision,
		Operation:        aicapability.OperationProductEnrichImageAnalyze,
		RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureVisionAnalyze},
		TraceID:          identity.TraceID,
	})
	if err != nil {
		a.record(ctx, identity, startedAt, imageURL, prompt, "", decision, err, true)
		return "", err
	}
	if !validImageDecision(decision) {
		err = aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(aicapability.OperationProductEnrichImageAnalyze), nil)
		a.record(ctx, identity, startedAt, imageURL, prompt, "", decision, err, true)
		return "", err
	}
	routedManager := a.manager.(productenrich.RoutedLLMManager)
	client, err := routedManager.GetClientWithRoute(ctx, decision.CredentialReference, productenrich.LLMClientRoute{
		CredentialReference: decision.CredentialReference, ConfigurationVersion: decision.ConfigurationVersion,
	})
	if err != nil || client == nil {
		if err == nil {
			err = errors.New("routed vision client is nil")
		}
		wrapped := aicapability.NewError(aicapability.ErrorCredentialUnavailable, string(aicapability.OperationProductEnrichImageAnalyze), err)
		a.record(ctx, identity, startedAt, imageURL, prompt, "", decision, wrapped, false)
		return "", wrapped
	}
	response, err := client.AnalyzeImage(ctx, imageURL, prompt)
	if err != nil {
		wrapped := aicapability.NewError(classifyTextError(err), string(aicapability.OperationProductEnrichImageAnalyze), err)
		a.record(ctx, identity, startedAt, imageURL, prompt, response, decision, wrapped, false)
		return "", wrapped
	}
	a.record(ctx, identity, startedAt, imageURL, prompt, response, decision, nil, false)
	return response, nil
}

func validImageDecision(decision aicapability.RouteDecision) bool {
	return decision.Capability == aicapability.CapabilityProductEnrichVision &&
		decision.Operation == aicapability.OperationProductEnrichImageAnalyze &&
		strings.TrimSpace(decision.ProviderID) != "" && strings.TrimSpace(decision.ModelID) != "" &&
		strings.TrimSpace(decision.RoutingKey) != "" && strings.TrimSpace(decision.CredentialReference) != ""
}

func (a *governedImageAnalyzer) record(ctx context.Context, identity aiidentity.Identity, startedAt time.Time, imageURL, prompt, response string, decision aicapability.RouteDecision, callErr error, routeErr bool) {
	if a == nil || a.recorder == nil {
		return
	}
	finishedAt := a.now()
	record := aicapability.InvocationRecord{
		InvocationID: a.newID(), TenantID: identity.TenantID, UserID: identity.UserID,
		BusinessTaskID: identity.BusinessTaskID, TraceID: identity.TraceID,
		Capability: aicapability.CapabilityProductEnrichVision, Operation: aicapability.OperationProductEnrichImageAnalyze,
		RouteMode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
		ProviderID: decision.ProviderID, ModelID: decision.ModelID, RoutingKey: decision.RoutingKey,
		CredentialReference: decision.CredentialReference, PolicyVersion: decision.PolicyVersion,
		ConfigurationVersion: decision.ConfigurationVersion, PromptKey: productEnrichVisionPromptKey,
		PromptVersion: productEnrichVisionPromptVersion, PromptScope: productEnrichVisionPromptScope,
		PromptHash: hashText(prompt), InputHash: hashText(imageURL), OutputHash: hashText(response),
		StartedAt: startedAt, FinishedAt: finishedAt, LatencyMilliseconds: finishedAt.Sub(startedAt).Milliseconds(),
		Attempt: 1, Outcome: aicapability.InvocationSucceeded,
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
	if err := a.recorder.RecordInvocation(recordCtx, record); err != nil && a.onRecordError != nil {
		a.onRecordError(record, err)
	}
}

var _ ImageAnalyzer = (*governedImageAnalyzer)(nil)
