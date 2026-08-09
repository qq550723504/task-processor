package productimage

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/aicapability"
)

func TestGovernedSceneGeneratorRejectsMissingIdentityWithoutProviderCall(t *testing.T) {
	provider := &governedSceneProvider{}
	generator, err := NewGovernedSceneGenerator(GovernedSceneGeneratorConfig{
		Router:   &governedSceneRouter{decision: governedSceneDecision()},
		Recorder: &governedSceneRecorder{},
		Provider: provider,
		Identity: func(context.Context) SceneAIIdentity { return SceneAIIdentity{} },
	})
	if err != nil {
		t.Fatalf("NewGovernedSceneGenerator: %v", err)
	}

	_, err = generator.GenerateScene(context.Background(), &SceneGenerationRequest{})
	if aicapability.CategoryOf(err) != aicapability.ErrorInvalidInput {
		t.Fatalf("error category = %q, want %q", aicapability.CategoryOf(err), aicapability.ErrorInvalidInput)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
}

func TestGovernedSceneGeneratorRoutesOnceAndRecordsDecision(t *testing.T) {
	provider := &governedSceneProvider{result: &SceneGenerationResult{Assets: []ImageAsset{{URL: "scene.jpg"}}}}
	recorder := &governedSceneRecorder{}
	router := &governedSceneRouter{decision: governedSceneDecision()}
	generator, err := NewGovernedSceneGenerator(GovernedSceneGeneratorConfig{
		Router:   router,
		Recorder: recorder,
		Provider: provider,
		Identity: func(context.Context) SceneAIIdentity {
			return SceneAIIdentity{TenantID: "tenant-a", UserID: "user-a", BusinessTaskID: "task-a", TraceID: "trace-a"}
		},
	})
	if err != nil {
		t.Fatalf("NewGovernedSceneGenerator: %v", err)
	}

	result, err := generator.GenerateScene(context.Background(), &SceneGenerationRequest{PromptRef: "productimage.scene.default", SceneIntent: "gallery_scene"})
	if err != nil {
		t.Fatalf("GenerateScene: %v", err)
	}
	if result == nil || len(result.Assets) != 1 || provider.calls != 1 || router.calls != 1 {
		t.Fatalf("result=%+v provider_calls=%d router_calls=%d", result, provider.calls, router.calls)
	}
	if provider.route.ModelID != "routed-model" || provider.route.RoutingKey != "productimage-image" || provider.route.CredentialReference != "image" || provider.route.ConfigurationVersion != "config-v1" {
		t.Fatalf("provider route = %+v", provider.route)
	}
	if recorder.record.Outcome != aicapability.InvocationSucceeded || recorder.record.ModelID != "routed-model" || recorder.record.TenantID != "tenant-a" || recorder.record.ImageCount != 1 {
		t.Fatalf("record = %+v", recorder.record)
	}
	if recorder.record.PromptHash == "" || recorder.record.InputHash == "" || recorder.record.OutputHash == "" {
		t.Fatalf("record hashes = %+v", recorder.record)
	}
	if recorder.record.PromptHash == recorder.record.InputHash {
		t.Fatalf("prompt hash should be derived from resolved prompt, record = %+v", recorder.record)
	}
}

func TestGovernedSceneGeneratorDoesNotRepeatProviderWhenRecorderFails(t *testing.T) {
	provider := &governedSceneProvider{result: &SceneGenerationResult{Assets: []ImageAsset{{URL: "scene.jpg"}}}}
	recorder := &governedSceneRecorder{err: errors.New("ledger unavailable")}
	recordErrors := 0
	generator, err := NewGovernedSceneGenerator(GovernedSceneGeneratorConfig{
		Router:   &governedSceneRouter{decision: governedSceneDecision()},
		Recorder: recorder,
		Provider: provider,
		Identity: func(context.Context) SceneAIIdentity { return SceneAIIdentity{TenantID: "tenant-a", UserID: "user-a"} },
		OnRecordError: func(aicapability.InvocationRecord, error) {
			recordErrors++
		},
	})
	if err != nil {
		t.Fatalf("NewGovernedSceneGenerator: %v", err)
	}

	if _, err := generator.GenerateScene(context.Background(), &SceneGenerationRequest{}); err != nil {
		t.Fatalf("GenerateScene: %v", err)
	}
	if provider.calls != 1 || recordErrors != 1 {
		t.Fatalf("provider calls = %d, record errors = %d", provider.calls, recordErrors)
	}
}

func TestGovernedSceneGeneratorSeparatesProviderAndRouteErrors(t *testing.T) {
	provider := &governedSceneProvider{err: errors.New("provider down")}
	recorder := &governedSceneRecorder{}
	generator, err := NewGovernedSceneGenerator(GovernedSceneGeneratorConfig{
		Router:   &governedSceneRouter{decision: governedSceneDecision()},
		Recorder: recorder,
		Provider: provider,
		Identity: func(context.Context) SceneAIIdentity { return SceneAIIdentity{TenantID: "tenant-a", UserID: "user-a"} },
	})
	if err != nil {
		t.Fatalf("NewGovernedSceneGenerator: %v", err)
	}

	if _, err := generator.GenerateScene(context.Background(), &SceneGenerationRequest{}); err == nil {
		t.Fatal("expected provider error")
	}
	if recorder.record.ErrorCategory != aicapability.ErrorProviderUnavailable || recorder.record.RouteErrorCategory != "" {
		t.Fatalf("record = %+v", recorder.record)
	}
}

func TestGovernedSceneGeneratorFailsClosedWhenRouteDecisionIsIncomplete(t *testing.T) {
	provider := &governedSceneProvider{result: &SceneGenerationResult{Assets: []ImageAsset{{URL: "scene.jpg"}}}}
	recorder := &governedSceneRecorder{}
	decision := governedSceneDecision()
	decision.ModelID = ""
	generator, err := NewGovernedSceneGenerator(GovernedSceneGeneratorConfig{
		Router:   &governedSceneRouter{decision: decision},
		Recorder: recorder,
		Provider: provider,
		Identity: func(context.Context) SceneAIIdentity { return SceneAIIdentity{TenantID: "tenant-a", UserID: "user-a"} },
	})
	if err != nil {
		t.Fatalf("NewGovernedSceneGenerator: %v", err)
	}

	if _, err := generator.GenerateScene(context.Background(), &SceneGenerationRequest{}); aicapability.CategoryOf(err) != aicapability.ErrorCapabilityUnavailable {
		t.Fatalf("error category = %q, want %q", aicapability.CategoryOf(err), aicapability.ErrorCapabilityUnavailable)
	}
	if provider.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", provider.calls)
	}
}

func TestGovernedSceneGeneratorClassifiesProviderDeadlineAsTimeout(t *testing.T) {
	provider := &governedSceneProvider{err: context.DeadlineExceeded}
	recorder := &governedSceneRecorder{}
	generator, err := NewGovernedSceneGenerator(GovernedSceneGeneratorConfig{
		Router:   &governedSceneRouter{decision: governedSceneDecision()},
		Recorder: recorder,
		Provider: provider,
		Identity: func(context.Context) SceneAIIdentity { return SceneAIIdentity{TenantID: "tenant-a", UserID: "user-a"} },
	})
	if err != nil {
		t.Fatalf("NewGovernedSceneGenerator: %v", err)
	}

	if _, err := generator.GenerateScene(context.Background(), &SceneGenerationRequest{}); aicapability.CategoryOf(err) != aicapability.ErrorProviderTimeout {
		t.Fatalf("error category = %q, want %q", aicapability.CategoryOf(err), aicapability.ErrorProviderTimeout)
	}
	if recorder.record.ErrorCategory != aicapability.ErrorProviderTimeout {
		t.Fatalf("record error category = %q, want %q", recorder.record.ErrorCategory, aicapability.ErrorProviderTimeout)
	}
}

func TestHashSceneRequestIncludesSourceAssetFingerprint(t *testing.T) {
	first := hashSceneRequest(&SceneGenerationRequest{SourceAsset: &ImageAsset{URL: "source-a", SourceURL: "origin-a", Width: 100, Height: 200}})
	second := hashSceneRequest(&SceneGenerationRequest{SourceAsset: &ImageAsset{URL: "source-b", SourceURL: "origin-b", Width: 100, Height: 200}})
	if first == second {
		t.Fatalf("source asset changes must change input hash: %q", first)
	}
}

func governedSceneDecision() aicapability.RouteDecision {
	return aicapability.RouteDecision{
		Capability:           aicapability.CapabilityProductImageScene,
		Operation:            aicapability.OperationProductImageSceneGenerate,
		ProviderID:           "openai",
		ModelID:              "routed-model",
		RoutingKey:           "productimage-image",
		CredentialReference:  "image",
		PolicyVersion:        "productimage-scene-v1",
		ConfigurationVersion: "config-v1",
	}
}

type governedSceneProvider struct {
	calls  int
	route  SceneGenerationRoute
	result *SceneGenerationResult
	err    error
}

func (p *governedSceneProvider) GenerateScene(context.Context, *SceneGenerationRequest) (*SceneGenerationResult, error) {
	return p.result, nil
}

func (p *governedSceneProvider) GenerateSceneWithRoute(_ context.Context, _ *SceneGenerationRequest, route SceneGenerationRoute) (*SceneGenerationResult, error) {
	p.calls++
	p.route = route
	return p.result, p.err
}

type governedSceneRouter struct {
	calls    int
	decision aicapability.RouteDecision
}

func (r *governedSceneRouter) Decide(_ context.Context, _ aicapability.RouteRequest) (aicapability.RouteDecision, error) {
	r.calls++
	return r.decision, nil
}

type governedSceneRecorder struct {
	record aicapability.InvocationRecord
	err    error
}

func (r *governedSceneRecorder) RecordInvocation(_ context.Context, record aicapability.InvocationRecord) error {
	r.record = record
	return r.err
}
