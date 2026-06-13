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
			"prompt_ref":        "productimage.scene.default",
			"prompt_key":        "productimage.scene.default",
			"prompt_source":     "registry",
			"prompt_version":    "default",
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

	svc := &service{mirrors: serviceDependencyMirrors{productSvc: productSvc, assembler: NewAssemblerWithConfig(AssemblerConfig{AmazonBuilder: stubAmazonDraftBuilder{}}), assetRepo: assetRepository, assetRecipeResolver: newDefaultAssetRecipeResolver(), assetBundleBuilder: newDefaultAssetBundleBuilder(), assetGenerator: assetgeneration.NewService(assetgeneration.Config{
		DeferredRenderer: assetgeneration.NewProductImageDeferredRenderer(&stubModelMetadataSceneRenderer{}),
	})},
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

	result, err := svc.runWorkflow(context.Background(), task)
	if err != nil {
		t.Fatalf("runWorkflow() error = %v", err)
	}

	generationTasks, err := assetRepository.ListGenerationTasks(context.Background(), task.ID)
	if err != nil {
		t.Fatalf("ListGenerationTasks() error = %v", err)
	}
	if len(generationTasks) != 0 {
		t.Fatalf("generation tasks = %+v, want no generation tasks when process_images=false", generationTasks)
	}
	if len(result.AssetGenerationTasks) != 0 {
		t.Fatalf("workflow result generation tasks = %+v, want none when process_images=false", result.AssetGenerationTasks)
	}
}
