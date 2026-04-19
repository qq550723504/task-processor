package productimage_test

import (
	"context"
	"testing"

	"task-processor/internal/productimage"
)

type sceneGeneratorStub struct {
	lastReq *productimage.SceneGenerationRequest
	result  *productimage.SceneGenerationResult
}

func (s *sceneGeneratorStub) GenerateScene(_ context.Context, req *productimage.SceneGenerationRequest) (*productimage.SceneGenerationResult, error) {
	s.lastReq = req
	return s.result, nil
}

func TestModelSceneRendererUsesSceneGenerator(t *testing.T) {
	generator := &sceneGeneratorStub{
		result: &productimage.SceneGenerationResult{
			Assets: []productimage.ImageAsset{{
				URL:      "scene.jpg",
				Type:     productimage.AssetTypeGalleryImage,
				Metadata: map[string]string{},
			}},
			Metadata: &productimage.GenerationMetadata{
				Provider:       "openai",
				ModelFamily:    "gpt-image",
				GenerationMode: "scene_generation",
				PromptRef:      "productimage.scene.default",
			},
		},
	}

	renderer := productimage.NewModelSceneRenderer(generator)
	assets, err := renderer.Render(context.Background(), &productimage.ImageAsset{URL: "subject.png"}, &productimage.ProductContext{ProductType: "dress"})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if generator.lastReq == nil || generator.lastReq.SceneIntent != "gallery_scene" {
		t.Fatalf("last request = %+v", generator.lastReq)
	}
	if generator.lastReq.PromptRef != "productimage.scene.default" {
		t.Fatalf("last request = %+v", generator.lastReq)
	}
	if len(assets) != 1 || assets[0].URL != "scene.jpg" {
		t.Fatalf("assets = %+v", assets)
	}
	if assets[0].Metadata["generation_mode"] != "scene_generation" || assets[0].Metadata["model_family"] != "gpt-image" {
		t.Fatalf("asset metadata = %+v", assets[0].Metadata)
	}
	if assets[0].Metadata["prompt_ref"] != "productimage.scene.default" {
		t.Fatalf("asset metadata = %+v", assets[0].Metadata)
	}
}
