package httpapi

import (
	"context"
	"errors"
	"testing"

	"task-processor/internal/aicapability"
	"task-processor/internal/core/config"
	openaiclient "task-processor/internal/infra/clients/openai"
	productimage "task-processor/internal/productimage"
)

func TestBuildGovernedProductImageSceneGeneratorKeepsDisabledPathUntouched(t *testing.T) {
	legacy := &sceneGovernanceGeneratorStub{}
	got, err := buildGovernedProductImageSceneGenerator(&config.Config{}, legacy, nil, nil, nil)
	if err != nil {
		t.Fatalf("buildGovernedProductImageSceneGenerator: %v", err)
	}
	if got != legacy {
		t.Fatalf("generator = %T, want legacy generator", got)
	}
}

func TestBuildGovernedProductImageSceneGeneratorRequiresDependenciesWhenEnabled(t *testing.T) {
	_, err := buildGovernedProductImageSceneGenerator(&config.Config{AICapability: config.AICapabilityConfig{ProductImageSceneEnabled: true}}, &sceneGovernanceGeneratorStub{}, nil, nil, nil)
	if err == nil {
		t.Fatal("expected missing governance dependency error")
	}
}

func TestBuildGovernedProductImageSceneGeneratorCarriesTaskAndTraceIdentity(t *testing.T) {
	recorder := &sceneGovernanceRecorderCapture{}
	legacy := &sceneGovernanceGeneratorStub{}
	resolver := &productImageSceneResolver{resolved: &openaiclient.ResolvedClientConfig{
		CacheKey: "image-config-v1",
		Config:   &openaiclient.ClientConfig{APIKey: "key", BaseURL: "https://example.test/v1", Model: "image-model", APIStyle: "openai"},
	}}
	generator, err := buildGovernedProductImageSceneGenerator(&config.Config{AICapability: config.AICapabilityConfig{ProductImageSceneEnabled: true}}, legacy, resolver, recorder, nil)
	if err != nil {
		t.Fatalf("buildGovernedProductImageSceneGenerator: %v", err)
	}
	ctx := productimage.WithAIIdentity(context.Background(), productimage.AIIdentity{TenantID: "tenant-a", UserID: "user-a", BusinessTaskID: "task-a", TraceID: "trace-a"})
	if _, err := generator.GenerateScene(ctx, &productimage.SceneGenerationRequest{}); err != nil {
		t.Fatalf("GenerateScene: %v", err)
	}
	if recorder.record.BusinessTaskID != "task-a" || recorder.record.TraceID != "trace-a" {
		t.Fatalf("record identity = %+v", recorder.record)
	}
}

type sceneGovernanceGeneratorStub struct{}

func (*sceneGovernanceGeneratorStub) GenerateScene(context.Context, *productimage.SceneGenerationRequest) (*productimage.SceneGenerationResult, error) {
	return nil, errors.New("unused")
}

func (*sceneGovernanceGeneratorStub) GenerateSceneWithRoute(context.Context, *productimage.SceneGenerationRequest, productimage.SceneGenerationRoute) (*productimage.SceneGenerationResult, error) {
	return &productimage.SceneGenerationResult{Assets: []productimage.ImageAsset{{URL: "scene.jpg"}}}, nil
}

var _ productimage.SceneGeneratorWithRoute = (*sceneGovernanceGeneratorStub)(nil)
var _ openaiclient.ClientConfigResolver = (*productImageSceneResolver)(nil)
var _ aicapability.InvocationRecorder = (*sceneGovernanceRecorderStub)(nil)

type sceneGovernanceRecorderStub struct{}

func (*sceneGovernanceRecorderStub) RecordInvocation(context.Context, aicapability.InvocationRecord) error {
	return nil
}

type sceneGovernanceRecorderCapture struct {
	record aicapability.InvocationRecord
}

func (r *sceneGovernanceRecorderCapture) RecordInvocation(_ context.Context, record aicapability.InvocationRecord) error {
	r.record = record
	return nil
}
