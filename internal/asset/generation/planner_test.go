package generation_test

import (
	"context"
	"testing"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog"
	"task-processor/internal/productimage"
)

func TestNoopServicePlansOnlyMissingGeneratedAssets(t *testing.T) {
	t.Parallel()

	service := assetgeneration.NewNoopService()
	result, err := service.Plan(context.Background(), assetgeneration.Request{
		TaskID: "task-1",
		Inventory: &asset.Inventory{
			Records: []asset.AssetRecord{
				{ID: "white-1", Kind: asset.KindWhiteBgImage, URL: "https://example.com/white.jpg"},
			},
		},
		Recipes: []assetrecipe.AssetRecipe{
			{
				ID:        "amazon-main-white-bg",
				Platform:  "amazon",
				AssetKind: asset.KindWhiteBgImage,
				Generated: false,
				Template: &assetrecipe.Template{
					BundleSlot:     "main",
					Purpose:        "main",
					PreferredKinds: []asset.Kind{asset.KindWhiteBgImage},
					Optional:       false,
					MaxItems:       1,
				},
			},
			{
				ID:        "amazon-lifestyle",
				Platform:  "amazon",
				AssetKind: asset.KindSceneImage,
				Generated: true,
				Template: &assetrecipe.Template{
					BundleSlot:     "auxiliary",
					Purpose:        "scene",
					PreferredKinds: []asset.Kind{asset.KindSceneImage},
					Optional:       true,
					MaxItems:       1,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Plan() error = %v", err)
	}
	if len(result.Tasks) != 1 {
		t.Fatalf("tasks = %+v, want 1 missing generated task", result.Tasks)
	}
	if result.Tasks[0].RecipeID != "amazon-lifestyle" {
		t.Fatalf("task = %+v, want amazon-lifestyle", result.Tasks[0])
	}
	if result.Tasks[0].Status != "planned" {
		t.Fatalf("task status = %q, want planned", result.Tasks[0].Status)
	}
}

func TestNoopServiceExecuteMaterializesCleanImageFromMainAsset(t *testing.T) {
	t.Parallel()

	service := assetgeneration.NewNoopService()
	result, err := service.Execute(context.Background(), assetgeneration.Request{
		TaskID: "task-1",
		Inventory: &asset.Inventory{
			Records: []asset.AssetRecord{
				{ID: "main-1", Kind: asset.KindMainImage, URL: "https://example.com/main.jpg", Generator: "productimage_pipeline"},
			},
		},
		Recipes: []assetrecipe.AssetRecipe{
			{
				ID:        "base-clean-image",
				Platform:  "common",
				AssetKind: asset.KindCleanImage,
				Generated: true,
				Template: &assetrecipe.Template{
					Purpose:        "base_clean",
					PreferredKinds: []asset.Kind{asset.KindCleanImage},
					Optional:       true,
					MaxItems:       1,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.Assets) != 1 {
		t.Fatalf("assets = %+v, want 1 clean image", result.Assets)
	}
	if result.Assets[0].Kind != asset.KindCleanImage {
		t.Fatalf("asset kind = %q, want clean_image", result.Assets[0].Kind)
	}
	if result.Assets[0].URL != "https://example.com/main.jpg" {
		t.Fatalf("asset url = %q, want main image url", result.Assets[0].URL)
	}
	if result.Assets[0].Lineage == nil || len(result.Assets[0].Lineage.SourceAssetIDs) == 0 || result.Assets[0].Lineage.SourceAssetIDs[0] != "main-1" {
		t.Fatalf("asset lineage = %+v, want source main-1", result.Assets[0].Lineage)
	}
}

func TestNoopServiceExecuteMaterializesWhiteBgAndCutoutFromSourceAsset(t *testing.T) {
	t.Parallel()

	service := assetgeneration.NewNoopService()
	result, err := service.Execute(context.Background(), assetgeneration.Request{
		TaskID: "task-2",
		Inventory: &asset.Inventory{
			Records: []asset.AssetRecord{
				{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://example.com/source.jpg", Generator: "canonical_product"},
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
			{
				ID:        "base-subject-cutout",
				Platform:  "common",
				AssetKind: asset.KindSubjectCutout,
				Generated: true,
				Template: &assetrecipe.Template{
					Purpose:        "base_cutout",
					PreferredKinds: []asset.Kind{asset.KindSubjectCutout},
					Optional:       true,
					MaxItems:       1,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.Assets) != 2 {
		t.Fatalf("assets = %+v, want 2 generated assets", result.Assets)
	}
	if result.Assets[0].Kind != asset.KindWhiteBgImage {
		t.Fatalf("first asset kind = %q, want white_bg_image", result.Assets[0].Kind)
	}
	if result.Assets[1].Kind != asset.KindSubjectCutout {
		t.Fatalf("second asset kind = %q, want subject_cutout", result.Assets[1].Kind)
	}
	if result.Assets[0].Lineage == nil || result.Assets[0].Lineage.SourceAssetIDs[0] != "source-1" {
		t.Fatalf("white bg lineage = %+v, want source-1", result.Assets[0].Lineage)
	}
	if result.Assets[1].Lineage == nil || result.Assets[1].Lineage.SourceAssetIDs[0] != "source-1" {
		t.Fatalf("cutout lineage = %+v, want source-1", result.Assets[1].Lineage)
	}
}

type stubSubjectExtractor struct {
	lastImageURL string
	lastContext  *productimage.ProductContext
	result       *productimage.ImageAsset
}

func (s *stubSubjectExtractor) Extract(ctx context.Context, imageURL string, context *productimage.ProductContext) (*productimage.ImageAsset, error) {
	s.lastImageURL = imageURL
	s.lastContext = context
	return s.result, nil
}

type stubWhiteBackgroundRenderer struct {
	lastAsset   *productimage.ImageAsset
	lastContext *productimage.ProductContext
	result      *productimage.ImageAsset
}

func (s *stubWhiteBackgroundRenderer) Render(ctx context.Context, asset *productimage.ImageAsset, context *productimage.ProductContext) (*productimage.ImageAsset, error) {
	s.lastAsset = asset
	s.lastContext = context
	return s.result, nil
}

type stubDeferredRenderer struct {
	lastRequest assetgeneration.DeferredRenderRequest
	result      *asset.AssetRecord
}

func (s *stubDeferredRenderer) Render(ctx context.Context, req assetgeneration.DeferredRenderRequest) (*asset.AssetRecord, error) {
	s.lastRequest = req
	return s.result, nil
}

type stubProductImageSceneRenderer struct {
	lastAsset   *productimage.ImageAsset
	lastContext *productimage.ProductContext
	results     []productimage.ImageAsset
}

func (s *stubProductImageSceneRenderer) Render(ctx context.Context, asset *productimage.ImageAsset, context *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	s.lastAsset = asset
	s.lastContext = context
	return append([]productimage.ImageAsset(nil), s.results...), nil
}

func TestServiceExecuteUsesPipelineBackedWhiteBgAndCutout(t *testing.T) {
	t.Parallel()

	subjectExtractor := &stubSubjectExtractor{
		result: &productimage.ImageAsset{
			URL:       "file:///tmp/cutout.png",
			Type:      productimage.AssetTypeSubjectCutout,
			SourceURL: "https://example.com/source.jpg",
			Width:     900,
			Height:    900,
			Operations: []string{
				"extract_subject_segmenter",
			},
			Metadata: map[string]string{
				"mode": "segmenter",
			},
		},
	}
	whiteBgRenderer := &stubWhiteBackgroundRenderer{
		result: &productimage.ImageAsset{
			URL:       "file:///tmp/white.png",
			Type:      productimage.AssetTypeWhiteBgImage,
			SourceURL: "https://example.com/source.jpg",
			Width:     1600,
			Height:    1600,
			Operations: []string{
				"render_white_bg_model",
			},
			Metadata: map[string]string{
				"background": "white",
			},
		},
	}

	service := assetgeneration.NewService(assetgeneration.Config{
		SubjectExtractor:        subjectExtractor,
		WhiteBackgroundRenderer: whiteBgRenderer,
	})
	result, err := service.Execute(context.Background(), assetgeneration.Request{
		TaskID: "task-3",
		Product: &catalog.Product{
			Title:        "Women Dress",
			CategoryPath: []string{"Fashion", "Dresses"},
			Attributes: []catalog.Attribute{
				{Name: "material", Value: "cotton"},
			},
		},
		Inventory: &asset.Inventory{
			Records: []asset.AssetRecord{
				{
					ID:     "source-1",
					Kind:   asset.KindSourceImage,
					URL:    "https://example.com/source.jpg",
					Origin: asset.OriginSource,
				},
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
			{
				ID:        "base-subject-cutout",
				Platform:  "common",
				AssetKind: asset.KindSubjectCutout,
				Generated: true,
				Template: &assetrecipe.Template{
					Purpose:        "base_cutout",
					PreferredKinds: []asset.Kind{asset.KindSubjectCutout},
					Optional:       true,
					MaxItems:       1,
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(result.Assets) != 2 {
		t.Fatalf("assets = %+v, want 2 generated assets", result.Assets)
	}
	if result.Assets[0].Generator != "productimage_pipeline" {
		t.Fatalf("first generator = %q, want productimage_pipeline", result.Assets[0].Generator)
	}
	if result.Assets[0].Metadata["execution_mode"] != "pipeline_backed" {
		t.Fatalf("first metadata = %+v, want pipeline_backed", result.Assets[0].Metadata)
	}
	if result.Assets[1].Generator != "productimage_pipeline" {
		t.Fatalf("second generator = %q, want productimage_pipeline", result.Assets[1].Generator)
	}
	if result.Assets[1].Metadata["execution_mode"] != "pipeline_backed" {
		t.Fatalf("second metadata = %+v, want pipeline_backed", result.Assets[1].Metadata)
	}
	if whiteBgRenderer.lastAsset == nil || whiteBgRenderer.lastAsset.URL != "file:///tmp/main.png" {
		t.Fatalf("renderer asset = %+v, want main image asset", whiteBgRenderer.lastAsset)
	}
	if subjectExtractor.lastImageURL != "https://example.com/source.jpg" {
		t.Fatalf("extractor image url = %q, want source url fallback", subjectExtractor.lastImageURL)
	}
	if subjectExtractor.lastContext == nil || subjectExtractor.lastContext.ProductType != "Dresses" {
		t.Fatalf("extractor context = %+v, want product type from catalog", subjectExtractor.lastContext)
	}
}

func TestServiceDispatchCompletesDeferredGenerationTasks(t *testing.T) {
	t.Parallel()

	service := assetgeneration.NewService(assetgeneration.Config{})
	result, err := service.Dispatch(context.Background(), assetgeneration.DispatchRequest{
		TaskID: "task-4",
		Product: &catalog.Product{
			Title:        "Winter Coat",
			CategoryPath: []string{"Fashion", "Coats"},
		},
		Inventory: &asset.Inventory{
			Records: []asset.AssetRecord{
				{ID: "clean-1", Kind: asset.KindCleanImage, URL: "file:///tmp/clean.jpg", Origin: asset.OriginGenerated},
				{ID: "gallery-1", Kind: asset.KindGalleryImage, URL: "file:///tmp/gallery.jpg", Origin: asset.OriginDerived},
			},
		},
		Tasks: []assetgeneration.Task{
			{
				ID:              "shein:shein-main-model",
				TaskID:          "task-4",
				Platform:        "shein",
				RecipeID:        "shein-main-model",
				AssetKind:       asset.KindModelImage,
				Slot:            "main",
				Purpose:         "main",
				Status:          "planned",
				ExecutionStatus: "planned",
				ExecutionMode:   "deferred_generation",
				CanExecute:      true,
				SourceAssetIDs:  []string{"clean-1"},
			},
			{
				ID:              "shein:shein-selling-point",
				TaskID:          "task-4",
				Platform:        "shein",
				RecipeID:        "shein-selling-point",
				AssetKind:       asset.KindSellingPointImage,
				Slot:            "auxiliary",
				Purpose:         "selling_point",
				Status:          "planned",
				ExecutionStatus: "planned",
				ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
				CanExecute:      true,
				SourceAssetIDs:  []string{"gallery-1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if len(result.Assets) != 2 {
		t.Fatalf("assets = %+v, want 2 generated assets", result.Assets)
	}
	if result.Assets[0].Generator != "asset_generation_stub" {
		t.Fatalf("first asset = %+v, want stub generator", result.Assets[0])
	}
	if result.Assets[0].Metadata["execution_mode"] != "deferred_stub" {
		t.Fatalf("first asset metadata = %+v, want deferred_stub", result.Assets[0].Metadata)
	}
	if len(result.Tasks) != 2 {
		t.Fatalf("tasks = %+v, want 2 updated tasks", result.Tasks)
	}
	if result.Tasks[0].ExecutionStatus != "completed" || result.Tasks[0].SatisfiedBy != "generated_asset" {
		t.Fatalf("first task = %+v, want completed generated asset", result.Tasks[0])
	}
	if result.Tasks[0].ExecutionMode != assetgeneration.ExecutionModeDeferredStub {
		t.Fatalf("first task = %+v, want deferred_stub fallback", result.Tasks[0])
	}
}

func TestServiceDispatchUsesDeferredRendererForSceneAndSellingPoint(t *testing.T) {
	t.Parallel()

	renderer := &stubDeferredRenderer{
		result: &asset.AssetRecord{
			ID:        "renderer-scene-1",
			Kind:      asset.KindSceneImage,
			Origin:    asset.OriginGenerated,
			Role:      "scene",
			URL:       "https://cdn.example.com/scene.jpg",
			Generator: "asset_generation_renderer",
			Metadata: map[string]string{
				"renderer": "stub",
			},
		},
	}
	service := assetgeneration.NewService(assetgeneration.Config{
		DeferredRenderer: renderer,
	})

	result, err := service.Dispatch(context.Background(), assetgeneration.DispatchRequest{
		TaskID: "task-5",
		Product: &catalog.Product{
			Title:        "Portable Speaker",
			CategoryPath: []string{"Electronics", "Audio"},
		},
		Inventory: &asset.Inventory{
			Records: []asset.AssetRecord{
				{ID: "gallery-1", Kind: asset.KindGalleryImage, URL: "file:///tmp/gallery.jpg", Origin: asset.OriginDerived},
			},
		},
		Tasks: []assetgeneration.Task{
			{
				ID:              "amazon:amazon-lifestyle",
				TaskID:          "task-5",
				Platform:        "amazon",
				RecipeID:        "amazon-lifestyle",
				AssetKind:       asset.KindSceneImage,
				Slot:            "auxiliary",
				Purpose:         "scene",
				Status:          "planned",
				ExecutionStatus: "planned",
				ExecutionMode:   assetgeneration.ExecutionModeRendererBacked,
				CanExecute:      true,
				SourceAssetIDs:  []string{"gallery-1"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Dispatch() error = %v", err)
	}
	if len(result.Assets) != 1 {
		t.Fatalf("assets = %+v, want 1 rendered asset", result.Assets)
	}
	if result.Assets[0].Metadata["execution_mode"] != assetgeneration.ExecutionModeRendererBacked {
		t.Fatalf("asset metadata = %+v, want renderer_backed", result.Assets[0].Metadata)
	}
	if result.Tasks[0].ExecutionMode != assetgeneration.ExecutionModeRendererBacked {
		t.Fatalf("task = %+v, want renderer_backed", result.Tasks[0])
	}
	if renderer.lastRequest.BaseAsset.ID != "gallery-1" {
		t.Fatalf("renderer request = %+v, want gallery base asset", renderer.lastRequest)
	}
}

func TestProductImageDeferredRendererMapsSceneAssets(t *testing.T) {
	t.Parallel()

	sceneRenderer := &stubProductImageSceneRenderer{
		results: []productimage.ImageAsset{
			{
				URL:       "file:///tmp/scene-rendered.jpg",
				Type:      productimage.AssetTypeGalleryImage,
				SourceURL: "file:///tmp/gallery.jpg",
				Width:     1400,
				Height:    1400,
				Operations: []string{
					"render_scene_local",
				},
				Metadata: map[string]string{
					"renderer": "productimage",
				},
			},
		},
	}
	renderer := assetgeneration.NewProductImageDeferredRenderer(sceneRenderer)

	record, err := renderer.Render(context.Background(), assetgeneration.DeferredRenderRequest{
		TaskID: "task-renderer-1",
		Product: &catalog.Product{
			Title:        "Portable Speaker",
			CategoryPath: []string{"Electronics", "Audio"},
			Attributes: []catalog.Attribute{
				{Name: "material", Value: "plastic"},
			},
		},
		Task: assetgeneration.Task{
			Platform:      "amazon",
			RecipeID:      "amazon-lifestyle",
			AssetKind:     asset.KindSceneImage,
			Slot:          "auxiliary",
			Purpose:       "scene",
			RenderProfile: "amazon_lifestyle_scene",
			TemplateLabel: "Amazon Lifestyle Scene",
		},
		BaseAsset: asset.AssetRecord{
			ID:     "gallery-1",
			Kind:   asset.KindGalleryImage,
			URL:    "file:///tmp/gallery.jpg",
			Origin: asset.OriginDerived,
			Metadata: map[string]string{
				"source_url": "https://example.com/gallery.jpg",
			},
		},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if record.Kind != asset.KindSceneImage {
		t.Fatalf("record kind = %q, want scene_image", record.Kind)
	}
	if record.Generator != "productimage_scene_renderer" {
		t.Fatalf("record generator = %q, want productimage_scene_renderer", record.Generator)
	}
	if record.Metadata["execution_mode"] != assetgeneration.ExecutionModeRendererBacked {
		t.Fatalf("record metadata = %+v, want renderer_backed execution mode", record.Metadata)
	}
	if record.Lineage == nil || len(record.Lineage.SourceAssetIDs) == 0 || record.Lineage.SourceAssetIDs[0] != "gallery-1" {
		t.Fatalf("record lineage = %+v, want gallery-1 source lineage", record.Lineage)
	}
	if sceneRenderer.lastAsset == nil || sceneRenderer.lastAsset.SourceURL != "https://example.com/gallery.jpg" {
		t.Fatalf("scene renderer asset = %+v, want source_url propagated", sceneRenderer.lastAsset)
	}
	if sceneRenderer.lastAsset == nil || sceneRenderer.lastAsset.Metadata["render_profile"] != "amazon_lifestyle_scene" {
		t.Fatalf("scene renderer asset = %+v, want render_profile propagated", sceneRenderer.lastAsset)
	}
	if sceneRenderer.lastAsset == nil || sceneRenderer.lastAsset.Metadata["template_label"] != "Amazon Lifestyle Scene" {
		t.Fatalf("scene renderer asset = %+v, want template_label propagated", sceneRenderer.lastAsset)
	}
	if sceneRenderer.lastContext == nil || sceneRenderer.lastContext.ProductType != "Audio" {
		t.Fatalf("scene renderer context = %+v, want product context", sceneRenderer.lastContext)
	}
	if record.Metadata["render_profile"] != "amazon_lifestyle_scene" {
		t.Fatalf("record metadata = %+v, want render_profile", record.Metadata)
	}
	if record.Metadata["template_label"] != "Amazon Lifestyle Scene" {
		t.Fatalf("record metadata = %+v, want template_label", record.Metadata)
	}
	if record.Metadata["background_template"] == "" || record.Metadata["overlay_template"] == "" || record.Metadata["visual_mode"] == "" {
		t.Fatalf("record metadata = %+v, want preset resource metadata", record.Metadata)
	}
	if record.Metadata["max_copy_lines"] == "" || record.Metadata["max_badges"] == "" || record.Metadata["measurement_mode"] == "" || record.Metadata["detail_anchor_mode"] == "" {
		t.Fatalf("record metadata = %+v, want preset constraint metadata", record.Metadata)
	}
}

func TestProductImageDeferredRendererPropagatesSellingPointSlotPlan(t *testing.T) {
	t.Parallel()

	sceneRenderer := &stubProductImageSceneRenderer{
		results: []productimage.ImageAsset{
			{
				URL:       "file:///tmp/selling-point-rendered.jpg",
				Type:      productimage.AssetTypeGalleryImage,
				SourceURL: "file:///tmp/gallery.jpg",
				Width:     1400,
				Height:    1400,
				Metadata:  map[string]string{},
			},
		},
	}
	renderer := assetgeneration.NewProductImageDeferredRenderer(sceneRenderer)

	record, err := renderer.Render(context.Background(), assetgeneration.DeferredRenderRequest{
		TaskID: "task-renderer-2",
		Product: &catalog.Product{
			Title:        "Portable Speaker",
			CategoryPath: []string{"Electronics", "Audio"},
		},
		Task: assetgeneration.Task{
			Platform:      "shein",
			RecipeID:      "shein-selling-point",
			AssetKind:     asset.KindSellingPointImage,
			Slot:          "auxiliary",
			Purpose:       "selling_point",
			RenderProfile: "shein_selling_point",
			TemplateLabel: "SHEIN Selling Point",
		},
		BaseAsset: asset.AssetRecord{
			ID:     "gallery-1",
			Kind:   asset.KindGalleryImage,
			URL:    "file:///tmp/gallery.jpg",
			Origin: asset.OriginDerived,
		},
	})
	if err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	if record.Metadata["layout_slots.copy"] == "" || record.Metadata["layout_slots.badges"] == "" {
		t.Fatalf("record metadata = %+v, want selling-point slot plan metadata", record.Metadata)
	}
	if record.Metadata["layout_constraints.max_copy_lines"] == "" || record.Metadata["layout_constraints.detail_anchor_mode"] == "" {
		t.Fatalf("record metadata = %+v, want selling-point constraint plan metadata", record.Metadata)
	}
	if record.Metadata["layout_engine_version"] != "v1" || record.Metadata["slot_plan_version"] != "v1" {
		t.Fatalf("record metadata = %+v, want plan version metadata", record.Metadata)
	}
	if record.Metadata["layout_content.copy"] == "" || record.Metadata["layout_content.badges"] == "" {
		t.Fatalf("record metadata = %+v, want selling-point content plan metadata", record.Metadata)
	}
	if record.Metadata["content_plan_version"] != "v1" {
		t.Fatalf("record metadata = %+v, want content plan version metadata", record.Metadata)
	}
	if record.Metadata["layout_fill_input"] == "" || record.Metadata["layout_fill_input_version"] != "v1" {
		t.Fatalf("record metadata = %+v, want unified fill input metadata", record.Metadata)
	}
	if record.Metadata["layout_render_blocks"] == "" || record.Metadata["render_block_plan_version"] != "v1" {
		t.Fatalf("record metadata = %+v, want render blocks metadata", record.Metadata)
	}
	if record.Metadata["layout_render_plan"] == "" || record.Metadata["render_plan_version"] != "v1" {
		t.Fatalf("record metadata = %+v, want render plan metadata", record.Metadata)
	}
	if record.Metadata["layout_render_output"] == "" || record.Metadata["render_output_version"] != "v2" {
		t.Fatalf("record metadata = %+v, want render output metadata", record.Metadata)
	}
	if record.Metadata["layout_draw_output"] == "" || record.Metadata["draw_output_version"] != "v1" {
		t.Fatalf("record metadata = %+v, want draw output metadata", record.Metadata)
	}
	if record.Metadata["layout_draw_preview_svg"] == "" || record.Metadata["draw_preview_version"] != "v1" {
		t.Fatalf("record metadata = %+v, want draw preview metadata", record.Metadata)
	}
}
