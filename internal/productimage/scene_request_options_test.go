package productimage_test

import (
	"context"
	"testing"

	productimage "task-processor/internal/productimage"
	"task-processor/internal/productimage/store"
)

type recordingSceneRenderer struct {
	lastAsset   *productimage.ImageAsset
	lastContext *productimage.ProductContext
}

func (r *recordingSceneRenderer) Render(_ context.Context, asset *productimage.ImageAsset, context *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	r.lastAsset = asset
	r.lastContext = context
	return []productimage.ImageAsset{{
		URL:       "https://cdn.example.com/gallery-custom.jpg",
		Type:      productimage.AssetTypeGalleryImage,
		SourceURL: asset.SourceURL,
		Metadata: map[string]string{
			"renderer": "recording",
		},
	}}, nil
}

func TestServiceProcessImagesPassesSceneRequestOptionsToRenderer(t *testing.T) {
	repo := store.NewMemTaskRepository()
	sceneRenderer := &recordingSceneRenderer{}
	svc, err := productimage.NewService(&productimage.ServiceConfig{
		TaskRepo:       repo,
		SceneRenderer:  sceneRenderer,
		AssetPublisher: &stubAssetPublisher{},
	})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	task, err := svc.CreateProcessTask(context.Background(), &productimage.ImageProcessRequest{
		ImageURLs:   []string{"https://example.com/source-a.jpg", "https://example.com/source-b.jpg"},
		Marketplace: "amazon",
		Scene: &productimage.SceneGenerationOptions{
			SceneCategory:   "shoes",
			SceneStyle:      "lifestyle",
			BackgroundTone:  "warm",
			Composition:     "close_up",
			PropsLevel:      "light",
			AudienceHint:    "sporty",
			CustomSceneHint: "show subtle motion energy",
		},
	})
	if err != nil {
		t.Fatalf("CreateProcessTask() error = %v", err)
	}

	result, err := svc.ProcessImages(context.Background(), task)
	if err != nil {
		t.Fatalf("ProcessImages() error = %v", err)
	}
	if len(result.GalleryImages) != 1 {
		t.Fatalf("gallery images = %+v", result.GalleryImages)
	}
	if sceneRenderer.lastContext == nil || sceneRenderer.lastContext.Attributes == nil {
		t.Fatalf("scene renderer context = %+v", sceneRenderer.lastContext)
	}
	if sceneRenderer.lastContext.Attributes["scene_category"] != "shoes" {
		t.Fatalf("context attrs = %+v", sceneRenderer.lastContext.Attributes)
	}
	if sceneRenderer.lastContext.Attributes["scene_style"] != "lifestyle" ||
		sceneRenderer.lastContext.Attributes["background_tone"] != "warm" ||
		sceneRenderer.lastContext.Attributes["composition"] != "close_up" ||
		sceneRenderer.lastContext.Attributes["props_level"] != "light" ||
		sceneRenderer.lastContext.Attributes["audience_hint"] != "sporty" ||
		sceneRenderer.lastContext.Attributes["custom_scene_hint"] != "show subtle motion energy" {
		t.Fatalf("context attrs = %+v", sceneRenderer.lastContext.Attributes)
	}
}
