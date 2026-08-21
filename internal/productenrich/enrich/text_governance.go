package enrich

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/aicapability"
	productenrich "task-processor/internal/productenrich"
	"task-processor/internal/shared/aiidentity"
)

const (
	productEnrichTextPromptKey     = "productenrich.understanding.extract_text"
	productEnrichTextPromptVersion = "v1"
	productEnrichTextPromptScope   = "product_enrich"
	governedTextRecordTimeout      = 2 * time.Second
)

// TextGenerator is the narrow model capability used by text attribute extraction.
// It deliberately excludes image operations so the ProductEnrich domain does not
// gain a second provider-routing abstraction.
type TextGenerator interface {
	Generate(context.Context, string) (string, error)
}

type GovernedTextGeneratorConfig struct {
	Router        aicapability.Router
	Recorder      aicapability.InvocationRecorder
	OnRecordError func(aicapability.InvocationRecord, error)
	Now           func() time.Time
	NewID         func() string
}

type governedTextGenerator struct {
	manager       productenrich.LLMManager
	router        aicapability.Router
	recorder      aicapability.InvocationRecorder
	onRecordError func(aicapability.InvocationRecord, error)
	now           func() time.Time
	newID         func() string
}

func NewGovernedTextGenerator(manager productenrich.LLMManager, config GovernedTextGeneratorConfig) (TextGenerator, error) {
	if manager == nil || config.Router == nil || config.Recorder == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductEnrichTextExtract), nil)
	}
	if _, ok := manager.(productenrich.RoutedLLMManager); !ok {
		return nil, aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(aicapability.OperationProductEnrichTextExtract), nil)
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.NewID == nil {
		config.NewID = uuid.NewString
	}
	return &governedTextGenerator{
		manager: manager, router: config.Router, recorder: config.Recorder,
		onRecordError: config.OnRecordError, now: config.Now, newID: config.NewID,
	}, nil
}

func (g *governedTextGenerator) Generate(ctx context.Context, prompt string) (string, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if g == nil || g.manager == nil || strings.TrimSpace(prompt) == "" {
		return "", aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductEnrichTextExtract), nil)
	}
	startedAt := g.now()
	identity := aiidentity.FromContext(ctx)
	if identity.TenantID == "" || identity.UserID == "" {
		err := aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductEnrichTextExtract), nil)
		g.record(ctx, identity, startedAt, prompt, "", aicapability.RouteDecision{}, err, true)
		return "", err
	}

	decision, err := g.router.Decide(ctx, aicapability.RouteRequest{
		TenantID: identity.TenantID, UserID: identity.UserID,
		Capability:       aicapability.CapabilityProductEnrichText,
		Operation:        aicapability.OperationProductEnrichTextExtract,
		RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureTextGenerate},
		TraceID:          identity.TraceID,
	})
	if err != nil {
		g.record(ctx, identity, startedAt, prompt, "", decision, err, true)
		return "", err
	}
	if !validTextDecision(decision) {
		err = aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(aicapability.OperationProductEnrichTextExtract), nil)
		g.record(ctx, identity, startedAt, prompt, "", decision, err, true)
		return "", err
	}
	routedManager, ok := g.manager.(productenrich.RoutedLLMManager)
	if !ok {
		err = aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(aicapability.OperationProductEnrichTextExtract), nil)
		g.record(ctx, identity, startedAt, prompt, "", decision, err, true)
		return "", err
	}
	client, err := routedManager.GetClientWithRoute(ctx, decision.CredentialReference, productenrich.LLMClientRoute{
		CredentialReference: decision.CredentialReference, ConfigurationVersion: decision.ConfigurationVersion,
	})
	if err != nil || client == nil {
		if err == nil {
			err = fmt.Errorf("routed text client is nil")
		}
		wrapped := aicapability.NewError(aicapability.ErrorCredentialUnavailable, string(aicapability.OperationProductEnrichTextExtract), err)
		g.record(ctx, identity, startedAt, prompt, "", decision, wrapped, false)
		return "", wrapped
	}
	response, err := client.Generate(ctx, prompt)
	if err != nil {
		wrapped := aicapability.NewError(classifyTextError(err), string(aicapability.OperationProductEnrichTextExtract), err)
		g.record(ctx, identity, startedAt, prompt, response, decision, wrapped, false)
		return "", wrapped
	}
	g.record(ctx, identity, startedAt, prompt, response, decision, nil, false)
	return response, nil
}

func validTextDecision(decision aicapability.RouteDecision) bool {
	return decision.Capability == aicapability.CapabilityProductEnrichText &&
		decision.Operation == aicapability.OperationProductEnrichTextExtract &&
		strings.TrimSpace(decision.ProviderID) != "" && strings.TrimSpace(decision.ModelID) != "" &&
		strings.TrimSpace(decision.RoutingKey) != "" && strings.TrimSpace(decision.CredentialReference) != ""
}

func classifyTextError(err error) aicapability.ErrorCategory {
	if errors.Is(err, context.DeadlineExceeded) {
		return aicapability.ErrorProviderTimeout
	}
	if category := aicapability.CategoryOf(err); category != aicapability.ErrorUnknown {
		return category
	}
	return aicapability.ErrorProviderUnavailable
}

func (g *governedTextGenerator) record(ctx context.Context, identity aiidentity.Identity, startedAt time.Time, prompt, response string, decision aicapability.RouteDecision, callErr error, routeErr bool) {
	if g == nil || g.recorder == nil {
		return
	}
	finishedAt := g.now()
	record := aicapability.InvocationRecord{
		InvocationID: g.newID(), TenantID: identity.TenantID, UserID: identity.UserID,
		BusinessTaskID: identity.BusinessTaskID, TraceID: identity.TraceID,
		Capability: aicapability.CapabilityProductEnrichText, Operation: aicapability.OperationProductEnrichTextExtract,
		RouteMode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive,
		ProviderID: decision.ProviderID, ModelID: decision.ModelID, RoutingKey: decision.RoutingKey,
		CredentialReference: decision.CredentialReference, PolicyVersion: decision.PolicyVersion,
		ConfigurationVersion: decision.ConfigurationVersion, PromptKey: productEnrichTextPromptKey,
		PromptVersion: productEnrichTextPromptVersion, PromptScope: productEnrichTextPromptScope,
		PromptHash: hashText(prompt), InputHash: hashText(prompt), OutputHash: hashText(response),
		StartedAt: startedAt, FinishedAt: finishedAt, LatencyMilliseconds: finishedAt.Sub(startedAt).Milliseconds(),
		Attempt: 1, FallbackIndex: decision.FallbackIndex, Outcome: aicapability.InvocationSucceeded,
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
	if err := g.recorder.RecordInvocation(recordCtx, record); err != nil && g.onRecordError != nil {
		g.onRecordError(record, err)
	}
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

var _ TextGenerator = (*governedTextGenerator)(nil)
