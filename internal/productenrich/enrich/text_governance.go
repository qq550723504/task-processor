package enrich

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
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
	Planner             aicapability.ExecutionPlanner
	LegacyRouteMetadata LegacyRouteMetadataResolver
	Recorder            aicapability.InvocationRecorder
	Capability          aicapability.Capability
	Operation           aicapability.Operation
	RequiredFeature     aicapability.ModelFeature
	PromptKey           string
	PromptVersion       string
	PromptScope         string
	OnRecordError       func(aicapability.InvocationRecord, error)
	Now                 func() time.Time
	NewID               func() string
}

type governedTextGenerator struct {
	manager             productenrich.LLMManager
	planner             aicapability.ExecutionPlanner
	legacyRouteMetadata LegacyRouteMetadataResolver
	recorder            aicapability.InvocationRecorder
	onRecordError       func(aicapability.InvocationRecord, error)
	now                 func() time.Time
	newID               func() string
	capability          aicapability.Capability
	operation           aicapability.Operation
	requiredFeature     aicapability.ModelFeature
	promptKey           string
	promptVersion       string
	promptScope         string
}

func NewGovernedTextGenerator(manager productenrich.LLMManager, config GovernedTextGeneratorConfig) (TextGenerator, error) {
	if manager == nil || config.Planner == nil || config.LegacyRouteMetadata == nil || config.Recorder == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductEnrichTextExtract), nil)
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.NewID == nil {
		config.NewID = uuid.NewString
	}
	if config.Capability == "" {
		config.Capability = aicapability.CapabilityProductEnrichText
	}
	if config.Operation == "" {
		config.Operation = aicapability.OperationProductEnrichTextExtract
	}
	if config.RequiredFeature == "" {
		config.RequiredFeature = aicapability.FeatureTextGenerate
	}
	if config.PromptKey == "" {
		config.PromptKey = productEnrichTextPromptKey
	}
	if config.PromptVersion == "" {
		config.PromptVersion = productEnrichTextPromptVersion
	}
	if config.PromptScope == "" {
		config.PromptScope = productEnrichTextPromptScope
	}
	return &governedTextGenerator{
		manager: manager, planner: config.Planner, legacyRouteMetadata: config.LegacyRouteMetadata, recorder: config.Recorder,
		onRecordError: config.OnRecordError, now: config.Now, newID: config.NewID,
		capability: config.Capability, operation: config.Operation, requiredFeature: config.RequiredFeature,
		promptKey: config.PromptKey, promptVersion: config.PromptVersion, promptScope: config.PromptScope,
	}, nil
}

func (g *governedTextGenerator) Generate(ctx context.Context, prompt string) (string, error) {
	execution, err := g.prepare(ctx, prompt)
	if err != nil {
		return "", err
	}
	return execution.invoke(ctx, aicapability.CacheStatusNotApplicable)
}

func (g *governedTextGenerator) prepare(ctx context.Context, prompt string) (*preparedExecution, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if g == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductEnrichTextExtract), nil)
	}
	if g.manager == nil || strings.TrimSpace(prompt) == "" {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(g.operation), nil)
	}

	identity := aiidentity.FromContext(ctx)
	execution := &preparedExecution{
		identity: identity, capability: g.capability, operation: g.operation,
		promptKey: g.promptKey, promptVersion: g.promptVersion, promptScope: g.promptScope,
		prompt: prompt, input: prompt, recorder: g.recorder, onRecordError: g.onRecordError,
		now: g.now, newID: g.newID,
	}
	if identity.TenantID == "" || identity.UserID == "" {
		err := aicapability.NewError(aicapability.ErrorIdentityIntegrity, string(g.operation), nil)
		execution.plan = aicapability.ExecutionPlan{Mode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive}
		execution.recordRejected(ctx, err, true)
		return nil, err
	}

	request := aicapability.RouteRequest{
		TenantID: identity.TenantID, UserID: identity.UserID,
		Capability: g.capability, Operation: g.operation,
		RequiredFeatures: []aicapability.ModelFeature{g.requiredFeature}, TraceID: identity.TraceID,
	}
	return prepareGovernedExecution(ctx, execution, g.manager, g.planner, g.legacyRouteMetadata, request, func(client productenrich.LLMClient) func(context.Context) (string, error) {
		return func(callCtx context.Context) (string, error) {
			return client.Generate(callCtx, prompt)
		}
	})
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

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

var _ TextGenerator = (*governedTextGenerator)(nil)
