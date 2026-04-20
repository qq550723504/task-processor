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
				PromptKey:      "productimage.scene.default",
				PromptSource:   "registry",
				PromptVersion:  "default",
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
	if assets[0].Metadata["prompt_key"] != "productimage.scene.default" || assets[0].Metadata["prompt_source"] != "registry" || assets[0].Metadata["prompt_version"] != "default" {
		t.Fatalf("asset metadata = %+v", assets[0].Metadata)
	}
}

func TestModelSceneRendererUsesCategorySpecificPromptRefAndSceneOptions(t *testing.T) {
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
				PromptRef:      "productimage.scene.shoes",
				PromptKey:      "productimage.scene.shoes",
				PromptSource:   "registry",
				PromptVersion:  "default",
			},
		},
	}

	renderer := productimage.NewModelSceneRenderer(generator)
	_, err := renderer.Render(context.Background(), &productimage.ImageAsset{URL: "subject.png"}, &productimage.ProductContext{
		ProductType: "sneaker",
		Attributes: map[string]string{
			"scene_style":       "lifestyle",
			"background_tone":   "warm",
			"composition":       "close_up",
			"props_level":       "light",
			"audience_hint":     "sporty",
			"custom_scene_hint": "show subtle motion energy",
		},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if generator.lastReq == nil {
		t.Fatal("expected scene generation request")
	}
	if generator.lastReq.PromptRef != "productimage.scene.shoes" {
		t.Fatalf("last request = %+v", generator.lastReq)
	}
	if generator.lastReq.SceneStyle != "lifestyle" || generator.lastReq.BackgroundTone != "warm" {
		t.Fatalf("last request = %+v", generator.lastReq)
	}
	if generator.lastReq.Composition != "close_up" || generator.lastReq.PropsLevel != "light" {
		t.Fatalf("last request = %+v", generator.lastReq)
	}
	if generator.lastReq.AudienceHint != "sporty" || generator.lastReq.CustomSceneHint != "show subtle motion energy" {
		t.Fatalf("last request = %+v", generator.lastReq)
	}
}
