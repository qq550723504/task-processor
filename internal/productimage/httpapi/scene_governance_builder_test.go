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
