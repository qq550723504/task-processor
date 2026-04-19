package listingkit

import (
	"context"
	"testing"

	assetgeneration "task-processor/internal/asset/generation"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

type stubModelMetadataSceneRenderer struct{}

func (s *stubModelMetadataSceneRenderer) Render(ctx context.Context, asset *productimage.ImageAsset, context *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	return []productimage.ImageAsset{{
		URL:       "file:///tmp/model-scene.jpg",
		Type:      productimage.AssetTypeGalleryImage,
		SourceURL: asset.SourceURL,
		Operations: []string{
			"render_scene_model",
		},
		Metadata: map[string]string{
			"model_provider":    "openai",
			"model_family":      "gpt-image",
			"generation_mode":   "scene_generation",
			"prompt_ref":        "preset:selling_point/default",
			"review_confidence": "0.82",
		},
	}}, nil
}

func TestRunWorkflowPersistsModelBackedGenerationMetadata(t *testing.T) {
	t.Parallel()

	productTask := &productenrich.Task{
		ID: "product-task-model-meta",
		Request: &productenrich.GenerateRequest{
			ImageURLs: []string{"https://example.com/source-4.jpg"},
			Text:      "portable speaker",
		},
	}
	productSvc := &stubWorkflowProductService{
		task: productTask,
		product: &productenrich.ProductJSON{
			Title:       "Portable Speaker",
			Description: "Wireless speaker",
			Category:    []string{"Electronics", "Audio"},
			Images:      []string{"https://example.com/source-4.jpg"},
			Attributes:  map[string]string{"brand": "DemoBrand"},
		},
	}
	assetRepository := assetrepo.NewMemRepository()

	svc := &service{
		productSvc:          productSvc,
		assembler:           NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}),
		assetRepo:           assetRepository,
		assetRecipeResolver: newDefaultAssetRecipeResolver(),
		assetBundleBuilder:  newDefaultAssetBundleBuilder(),
		assetGenerator: assetgeneration.NewService(assetgeneration.Config{
			DeferredRenderer: assetgeneration.NewProductImageDeferredRenderer(&stubModelMetadataSceneRenderer{}),
		}),
	}

	task := &Task{
		ID: "listingkit-task-model-meta",
		Request: &GenerateRequest{
			ImageURLs: []string{"https://example.com/source-4.jpg"},
			Text:      "portable speaker",
			Platforms: []string{"amazon"},
			Country:   "US",
			Language:  "en_US",
			Options:   &GenerateOptions{ProcessImages: false},
		},
	}

	_, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}

	generationTasks, err := assetRepository.ListGenerationTasks(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ListGenerationTasks() error = %v", err)
	}
	if len(generationTasks) == 0 {
		t.Fatal("expected persisted generation tasks")
	}

	var found bool
	for _, item := range generationTasks {
		if item.ExecutionMode != assetgeneration.ExecutionModeRendererBacked || item.ExecutionStatus != "completed" {
			continue
		}
		found = true
		if item.Metadata["model_provider"] != "openai" || item.Metadata["model_family"] != "gpt-image" {
			t.Fatalf("task metadata = %+v", item.Metadata)
		}
		if item.Metadata["generation_mode"] != "scene_generation" || item.Metadata["prompt_ref"] != "preset:selling_point/default" {
			t.Fatalf("task metadata = %+v", item.Metadata)
		}
		if item.ReviewConfidence != 0.82 {
			t.Fatalf("review confidence = %v, want 0.82", item.ReviewConfidence)
		}
	}
	if !found {
		t.Fatalf("generation tasks = %+v, want completed renderer-backed task", generationTasks)
	}
}
