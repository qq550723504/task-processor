package generation

import (
	"context"
	"testing"

	"task-processor/internal/asset"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog"
	"task-processor/internal/productimage"
)

func TestAttachGenerationMetadataCopiesPromptObservability(t *testing.T) {
	t.Parallel()

	task := Task{
		Metadata: map[string]string{
			"custom": "keep",
		},
	}

	AttachGenerationMetadata(&task, &productimage.GenerationMetadata{
		Provider:       "openai_compatible",
		ModelFamily:    "nano-banana-fast",
		GenerationMode: "scene_generation",
		PromptRef:      "productimage.scene.default",
		PromptKey:      "productimage.scene.default",
		PromptSource:   "registry",
		PromptVersion:  "default",
	})

	if task.Metadata["custom"] != "keep" {
		t.Fatalf("custom metadata lost: %+v", task.Metadata)
	}
	if task.Metadata["model_provider"] != "openai_compatible" {
		t.Fatalf("model_provider = %q", task.Metadata["model_provider"])
	}
	if task.Metadata["model_family"] != "nano-banana-fast" {
		t.Fatalf("model_family = %q", task.Metadata["model_family"])
	}
	if task.Metadata["generation_mode"] != "scene_generation" {
		t.Fatalf("generation_mode = %q", task.Metadata["generation_mode"])
	}
	if task.Metadata["prompt_ref"] != "productimage.scene.default" {
		t.Fatalf("prompt_ref = %q", task.Metadata["prompt_ref"])
	}
	if task.Metadata["prompt_key"] != "productimage.scene.default" {
		t.Fatalf("prompt_key = %q", task.Metadata["prompt_key"])
	}
	if task.Metadata["prompt_source"] != "registry" {
		t.Fatalf("prompt_source = %q", task.Metadata["prompt_source"])
	}
	if task.Metadata["prompt_version"] != "default" {
		t.Fatalf("prompt_version = %q", task.Metadata["prompt_version"])
	}
}

func TestAttachGenerationMetadataSkipsEmptyFields(t *testing.T) {
	t.Parallel()

	task := Task{
		Metadata: map[string]string{
			"prompt_source":  "registry",
			"prompt_version": "default",
		},
	}

	AttachGenerationMetadata(&task, &productimage.GenerationMetadata{
		Provider:    "openai_compatible",
		ModelFamily: "nano-banana-fast",
	})

	if task.Metadata["model_provider"] != "openai_compatible" {
		t.Fatalf("model_provider = %q", task.Metadata["model_provider"])
	}
	if task.Metadata["model_family"] != "nano-banana-fast" {
		t.Fatalf("model_family = %q", task.Metadata["model_family"])
	}
	if task.Metadata["prompt_source"] != "registry" {
		t.Fatalf("prompt_source = %q", task.Metadata["prompt_source"])
	}
	if task.Metadata["prompt_version"] != "default" {
		t.Fatalf("prompt_version = %q", task.Metadata["prompt_version"])
	}
	if _, ok := task.Metadata["prompt_key"]; ok {
		t.Fatalf("prompt_key should not be set: %+v", task.Metadata)
	}
}

type promptMetadataWhiteBackgroundRenderer struct {
	result *productimage.ImageAsset
}

func (s *promptMetadataWhiteBackgroundRenderer) Render(ctx context.Context, asset *productimage.ImageAsset, productCtx *productimage.ProductContext) (*productimage.ImageAsset, error) {
	return s.result, nil
}

func TestServiceExecuteMirrorsPromptObservabilityIntoTaskMetadata(t *testing.T) {
	t.Parallel()

	service := NewService(Config{
		WhiteBackgroundRenderer: &promptMetadataWhiteBackgroundRenderer{
			result: &productimage.ImageAsset{
				URL:       "file:///tmp/white.png",
				Type:      productimage.AssetTypeWhiteBgImage,
				SourceURL: "https://example.com/source.jpg",
				Metadata: map[string]string{
					"execution_mode":    "pipeline_backed",
					"model_provider":    "openai_compatible",
					"model_family":      "nano-banana-fast",
					"generation_mode":   "white_background",
					"prompt_ref":        "productimage.white_background.default",
					"prompt_key":        "productimage.white_background.default",
					"prompt_source":     "registry",
					"prompt_version":    "default",
					"review_confidence": "0.75",
				},
			},
		},
	})

	result, err := service.Execute(context.Background(), Request{
		TaskID: "task-5-observability",
		Product: &catalog.Product{
			Title:        "Sneaker",
			CategoryPath: []string{"Shoes", "Sneakers"},
		},
		Inventory: &asset.Inventory{
			Records: []asset.AssetRecord{
				{
					ID:     "main-1",
					Kind:   asset.KindMainImage,
					URL:    "file:///tmp/main.png",
					Origin: asset.OriginDerived,
					Metadata: map[string]string{
						"source_url": "https://example.com/source.jpg",
					},
				},
			},
		},
		Recipes: []assetrecipe.AssetRecipe{
			{
				ID:        "base-white-bg-image",
				Platform:  "common",
				AssetKind: asset.KindWhiteBgImage,
				Generated: true,
				Template: &assetrecipe.Template{
					Purpose:        "base_white_bg",
					PreferredKinds: []asset.Kind{asset.KindWhiteBgImage},
					Optional:       true,
					MaxItems:       1,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("tasks = %+v, want 1", result.Tasks)
	}
	task := result.Tasks[0]
	if task.Metadata["prompt_ref"] != "productimage.white_background.default" {
		t.Fatalf("prompt_ref = %q", task.Metadata["prompt_ref"])
	}
	if task.Metadata["prompt_key"] != "productimage.white_background.default" {
		t.Fatalf("prompt_key = %q", task.Metadata["prompt_key"])
	}
	if task.Metadata["prompt_source"] != "registry" {
		t.Fatalf("prompt_source = %q", task.Metadata["prompt_source"])
	}
	if task.Metadata["prompt_version"] != "default" {
		t.Fatalf("prompt_version = %q", task.Metadata["prompt_version"])
	}
	if task.Metadata["model_provider"] != "openai_compatible" {
		t.Fatalf("model_provider = %q", task.Metadata["model_provider"])
	}
	if task.Metadata["model_family"] != "nano-banana-fast" {
		t.Fatalf("model_family = %q", task.Metadata["model_family"])
	}
	if task.Metadata["generation_mode"] != "white_background" {
		t.Fatalf("generation_mode = %q", task.Metadata["generation_mode"])
	}
	if task.ReviewConfidence != 0.75 {
		t.Fatalf("review confidence = %v", task.ReviewConfidence)
	}
}
