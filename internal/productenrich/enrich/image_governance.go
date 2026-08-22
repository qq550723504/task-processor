package enrich

import (
	"context"
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
	Planner             aicapability.ExecutionPlanner
	LegacyRouteMetadata LegacyRouteMetadataResolver
	Recorder            aicapability.InvocationRecorder
	Capability          aicapability.Capability
	Operation           aicapability.Operation
	RequiredFeature     aicapability.ModelFeature
	OnRecordError       func(aicapability.InvocationRecord, error)
	Now                 func() time.Time
	NewID               func() string
}

type governedImageAnalyzer struct {
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
}

func NewGovernedImageAnalyzer(manager productenrich.LLMManager, config GovernedImageAnalyzerConfig) (ImageAnalyzer, error) {
	if manager == nil || config.Planner == nil || config.LegacyRouteMetadata == nil || config.Recorder == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductEnrichImageAnalyze), nil)
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.NewID == nil {
		config.NewID = uuid.NewString
	}
	if config.Capability == "" {
		config.Capability = aicapability.CapabilityProductEnrichVision
	}
	if config.Operation == "" {
		config.Operation = aicapability.OperationProductEnrichImageAnalyze
	}
	if config.RequiredFeature == "" {
		config.RequiredFeature = aicapability.FeatureVisionAnalyze
	}
	return &governedImageAnalyzer{
		manager: manager, planner: config.Planner, legacyRouteMetadata: config.LegacyRouteMetadata, recorder: config.Recorder,
		onRecordError: config.OnRecordError, now: config.Now, newID: config.NewID,
		capability: config.Capability, operation: config.Operation, requiredFeature: config.RequiredFeature,
	}, nil
}

func (a *governedImageAnalyzer) AnalyzeImage(ctx context.Context, imageURL, prompt string) (string, error) {
	execution, err := a.prepare(ctx, imageURL, prompt)
	if err != nil {
		return "", err
	}
	return execution.invoke(ctx, aicapability.CacheStatusNotApplicable)
}

func (a *governedImageAnalyzer) prepare(ctx context.Context, imageURL, prompt string) (*preparedExecution, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if a == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductEnrichImageAnalyze), nil)
	}
	if a.manager == nil || strings.TrimSpace(imageURL) == "" || strings.TrimSpace(prompt) == "" {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(a.operation), nil)
	}

	identity := aiidentity.FromContext(ctx)
	execution := &preparedExecution{
		identity: identity, capability: a.capability, operation: a.operation,
		promptKey: productEnrichVisionPromptKey, promptVersion: productEnrichVisionPromptVersion, promptScope: productEnrichVisionPromptScope,
		prompt: prompt, input: imageURL, recorder: a.recorder, onRecordError: a.onRecordError,
		now: a.now, newID: a.newID,
	}
	if identity.TenantID == "" || identity.UserID == "" {
		err := aicapability.NewError(aicapability.ErrorIdentityIntegrity, string(a.operation), nil)
		execution.plan = aicapability.ExecutionPlan{Mode: aicapability.RoutingModeActive, RouteOutcome: aicapability.RouteOutcomeActive}
		execution.recordRejected(ctx, err, true)
		return nil, err
	}

	request := aicapability.RouteRequest{
		TenantID: identity.TenantID, UserID: identity.UserID,
		Capability: a.capability, Operation: a.operation,
		RequiredFeatures: []aicapability.ModelFeature{a.requiredFeature}, TraceID: identity.TraceID,
	}
	return prepareGovernedExecution(ctx, execution, a.manager, a.planner, a.legacyRouteMetadata, request, func(client productenrich.LLMClient) func(context.Context) (string, error) {
		return func(callCtx context.Context) (string, error) {
			return client.AnalyzeImage(callCtx, imageURL, prompt)
		}
	})
}

var _ ImageAnalyzer = (*governedImageAnalyzer)(nil)
