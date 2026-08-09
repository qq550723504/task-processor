package productimage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"task-processor/internal/aicapability"
)

const governedSceneRecordTimeout = 2 * time.Second

type SceneAIIdentity struct {
	TenantID       string
	UserID         string
	BusinessTaskID string
	TraceID        string
}

type GovernedSceneGeneratorConfig struct {
	Router        aicapability.Router
	Recorder      aicapability.InvocationRecorder
	Provider      SceneGeneratorWithRoute
	Identity      func(context.Context) SceneAIIdentity
	OnRecordError func(aicapability.InvocationRecord, error)
	Now           func() time.Time
	NewID         func() string
}

type governedSceneGenerator struct {
	router        aicapability.Router
	recorder      aicapability.InvocationRecorder
	provider      SceneGeneratorWithRoute
	identity      func(context.Context) SceneAIIdentity
	onRecordError func(aicapability.InvocationRecord, error)
	now           func() time.Time
	newID         func() string
}

func NewGovernedSceneGenerator(config GovernedSceneGeneratorConfig) (SceneGenerator, error) {
	if config.Router == nil || config.Recorder == nil || config.Provider == nil || config.Identity == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductImageSceneGenerate), nil)
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	if config.NewID == nil {
		config.NewID = uuid.NewString
	}
	return &governedSceneGenerator{
		router:        config.Router,
		recorder:      config.Recorder,
		provider:      config.Provider,
		identity:      config.Identity,
		onRecordError: config.OnRecordError,
		now:           config.Now,
		newID:         config.NewID,
	}, nil
}

func (g *governedSceneGenerator) GenerateScene(ctx context.Context, req *SceneGenerationRequest) (*SceneGenerationResult, error) {
	if req == nil {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductImageSceneGenerate), nil)
	}
	identity := g.identity(ctx)
	identity.TenantID = strings.TrimSpace(identity.TenantID)
	identity.UserID = strings.TrimSpace(identity.UserID)
	if identity.TenantID == "" || identity.UserID == "" {
		return nil, aicapability.NewError(aicapability.ErrorInvalidInput, string(aicapability.OperationProductImageSceneGenerate), nil)
	}

	startedAt := g.now()
	inputHash := hashSceneRequest(req)
	promptHash := hashText(buildSceneGenerationResolvedPrompt(req).Text)
	decision, err := g.router.Decide(ctx, aicapability.RouteRequest{
		TenantID:         identity.TenantID,
		UserID:           identity.UserID,
		Capability:       aicapability.CapabilityProductImageScene,
		Operation:        aicapability.OperationProductImageSceneGenerate,
		RequiredFeatures: []aicapability.ModelFeature{aicapability.FeatureImageEdit},
		TraceID:          identity.TraceID,
	})
	if err != nil {
		wrapped := err
		g.record(ctx, g.newRecord(identity, startedAt, inputHash, promptHash, decision, nil, wrapped, true))
		return nil, wrapped
	}
	if decision.Capability != aicapability.CapabilityProductImageScene ||
		decision.Operation != aicapability.OperationProductImageSceneGenerate ||
		strings.TrimSpace(decision.ProviderID) == "" || strings.TrimSpace(decision.RoutingKey) == "" ||
		strings.TrimSpace(decision.ModelID) == "" || strings.TrimSpace(decision.CredentialReference) == "" {
		wrapped := aicapability.NewError(aicapability.ErrorCapabilityUnavailable, string(aicapability.OperationProductImageSceneGenerate), nil)
		g.record(ctx, g.newRecord(identity, startedAt, inputHash, promptHash, decision, nil, wrapped, true))
		return nil, wrapped
	}

	result, providerErr := g.provider.GenerateSceneWithRoute(ctx, req, SceneGenerationRoute{
		RoutingKey: decision.RoutingKey,
		ModelID:    decision.ModelID,
	})
	if providerErr != nil {
		wrapped := aicapability.NewError(classifySceneError(providerErr), string(aicapability.OperationProductImageSceneGenerate), providerErr)
		g.record(ctx, g.newRecord(identity, startedAt, inputHash, promptHash, decision, result, wrapped, false))
		return result, wrapped
	}
	if result == nil || len(result.Assets) == 0 {
		wrapped := aicapability.NewError(aicapability.ErrorInvalidProviderResponse, string(aicapability.OperationProductImageSceneGenerate), nil)
		g.record(ctx, g.newRecord(identity, startedAt, inputHash, promptHash, decision, result, wrapped, false))
		return nil, wrapped
	}

	g.record(ctx, g.newRecord(identity, startedAt, inputHash, promptHash, decision, result, nil, false))
	return result, nil
}

func (g *governedSceneGenerator) newRecord(identity SceneAIIdentity, startedAt time.Time, inputHash, promptHash string, decision aicapability.RouteDecision, result *SceneGenerationResult, callErr error, routeErr bool) aicapability.InvocationRecord {
	finishedAt := g.now()
	record := aicapability.InvocationRecord{
		InvocationID:         g.newID(),
		TenantID:             identity.TenantID,
		UserID:               identity.UserID,
		BusinessTaskID:       identity.BusinessTaskID,
		TraceID:              identity.TraceID,
		Capability:           aicapability.CapabilityProductImageScene,
		Operation:            aicapability.OperationProductImageSceneGenerate,
		RouteMode:            aicapability.RoutingModeActive,
		RouteOutcome:         aicapability.RouteOutcomeActive,
		ProviderID:           decision.ProviderID,
		ModelID:              decision.ModelID,
		RoutingKey:           decision.RoutingKey,
		CredentialReference:  decision.CredentialReference,
		PolicyVersion:        decision.PolicyVersion,
		ConfigurationVersion: decision.ConfigurationVersion,
		PromptHash:           promptHash,
		StartedAt:            startedAt,
		FinishedAt:           finishedAt,
		LatencyMilliseconds:  finishedAt.Sub(startedAt).Milliseconds(),
		Attempt:              1,
		FallbackIndex:        decision.FallbackIndex,
		ImageCount:           sceneResultImageCount(result),
		InputHash:            inputHash,
		OutputHash:           hashSceneResult(result),
		Outcome:              aicapability.InvocationSucceeded,
	}
	if callErr != nil {
		record.Outcome = aicapability.InvocationFailed
		record.ErrorCategory = classifySceneError(callErr)
		record.ErrorCode = string(record.ErrorCategory)
		if routeErr && aicapability.CategoryOf(callErr) != aicapability.ErrorUnknown {
			record.RouteErrorCategory = aicapability.CategoryOf(callErr)
		}
	}
	return record
}

func sceneResultImageCount(result *SceneGenerationResult) int {
	if result == nil {
		return 0
	}
	return len(result.Assets)
}

func (g *governedSceneGenerator) record(ctx context.Context, record aicapability.InvocationRecord) {
	recordCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), governedSceneRecordTimeout)
	defer cancel()
	if err := g.recorder.RecordInvocation(recordCtx, record); err != nil && g.onRecordError != nil {
		g.onRecordError(record, err)
	}
}

func classifySceneError(err error) aicapability.ErrorCategory {
	if errors.Is(err, context.DeadlineExceeded) {
		return aicapability.ErrorProviderTimeout
	}
	if category := aicapability.CategoryOf(err); category != aicapability.ErrorUnknown {
		return category
	}
	return aicapability.ErrorProviderUnavailable
}

func hashSceneRequest(req *SceneGenerationRequest) string {
	payload := struct {
		PromptRef      string
		SceneIntent    string
		SceneCategory  string
		SceneStyle     string
		BackgroundTone string
		Composition    string
		PropsLevel     string
		AudienceHint   string
		CustomHint     string
		ProductContext *ProductContext
	}{
		PromptRef: req.PromptRef, SceneIntent: req.SceneIntent, SceneCategory: req.SceneCategory,
		SceneStyle: req.SceneStyle, BackgroundTone: req.BackgroundTone, Composition: req.Composition,
		PropsLevel: req.PropsLevel, AudienceHint: req.AudienceHint, CustomHint: req.CustomSceneHint,
		ProductContext: req.ProductContext,
	}
	return hashJSON(payload)
}

func hashSceneResult(result *SceneGenerationResult) string {
	if result == nil {
		return ""
	}
	type assetFingerprint struct {
		URL        string
		Type       AssetType
		Operations []string
		Width      int
		Height     int
	}
	assets := make([]assetFingerprint, 0, len(result.Assets))
	for _, asset := range result.Assets {
		assets = append(assets, assetFingerprint{URL: asset.URL, Type: asset.Type, Operations: asset.Operations, Width: asset.Width, Height: asset.Height})
	}
	return hashJSON(assets)
}

func hashJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func hashText(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

var _ SceneGenerator = (*governedSceneGenerator)(nil)
