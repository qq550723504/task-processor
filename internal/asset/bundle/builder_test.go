package bundle_test

import (
	"testing"

	"task-processor/internal/asset"
	assetbundle "task-processor/internal/asset/bundle"
	assetrecipe "task-processor/internal/asset/recipe"
)

func TestBuilderBuildSelectsPlatformImageBundle(t *testing.T) {
	t.Parallel()

	builder := assetbundle.NewBuilder()
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://example.com/source.jpg"},
			{ID: "main-1", Kind: asset.KindMainImage, URL: "https://example.com/main.jpg"},
			{ID: "white-1", Kind: asset.KindWhiteBgImage, URL: "https://example.com/white.jpg"},
			{ID: "gallery-1", Kind: asset.KindGalleryImage, URL: "https://example.com/gallery.jpg"},
		},
	}
	recipes := []assetrecipe.AssetRecipe{
		{
			ID:       "amazon-main",
			Platform: "amazon",
			Template: &assetrecipe.Template{
				BundleSlot:     "main",
				Purpose:        "main",
				PreferredKinds: []asset.Kind{asset.KindWhiteBgImage, asset.KindMainImage},
				Optional:       false,
				MaxItems:       1,
			},
		},
		{
			ID:       "amazon-gallery",
			Platform: "amazon",
			Template: &assetrecipe.Template{
				BundleSlot:     "gallery",
				Purpose:        "gallery",
				PreferredKinds: []asset.Kind{asset.KindGalleryImage, asset.KindMainImage, asset.KindSourceImage},
				Optional:       true,
				MaxItems:       3,
			},
		},
	}

	bundle := builder.Build(assetbundle.BuildRequest{
		Platform:  "amazon",
		Inventory: inventory,
		Recipes:   recipes,
	})
	if bundle == nil {
		t.Fatal("expected bundle")
	}
	if bundle.Main == nil || bundle.Main.AssetID != "white-1" {
		t.Fatalf("main slot = %+v, want white-1", bundle.Main)
	}
	if len(bundle.Gallery) != 1 || bundle.Gallery[0].AssetID != "gallery-1" {
		t.Fatalf("gallery slots = %+v, want gallery-1", bundle.Gallery)
	}
	if len(bundle.SelectedAssetIDs) != 2 {
		t.Fatalf("selected asset ids = %+v, want 2", bundle.SelectedAssetIDs)
	}
}

func TestBuilderBuildReportsMissingSlotsAndPendingGeneration(t *testing.T) {
	t.Parallel()

	builder := assetbundle.NewBuilder()
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "source-1", Kind: asset.KindSourceImage, URL: "https://example.com/source.jpg"},
		},
	}
	recipes := []assetrecipe.AssetRecipe{
		{
			ID:        "shein-main-model",
			Platform:  "shein",
			AssetKind: asset.KindModelImage,
			Generated: true,
			Template: &assetrecipe.Template{
				BundleSlot:     "main",
				Purpose:        "main",
				PreferredKinds: []asset.Kind{asset.KindModelImage},
				Optional:       false,
				MaxItems:       1,
			},
		},
		{
			ID:        "shein-gallery-scene",
			Platform:  "shein",
			AssetKind: asset.KindSceneImage,
			Generated: true,
			Template: &assetrecipe.Template{
				BundleSlot:     "gallery",
				Purpose:        "gallery",
				PreferredKinds: []asset.Kind{asset.KindSceneImage},
				Optional:       true,
				MaxItems:       2,
			},
		},
	}

	bundle := builder.Build(assetbundle.BuildRequest{
		Platform:  "shein",
		Inventory: inventory,
		Recipes:   recipes,
	})
	if bundle == nil {
		t.Fatal("expected bundle")
	}
	if bundle.Main != nil {
		t.Fatalf("main slot = %+v, want nil", bundle.Main)
	}
	if len(bundle.MissingSlots) != 1 || bundle.MissingSlots[0].Slot != "main" {
		t.Fatalf("missing slots = %+v, want missing main", bundle.MissingSlots)
	}
	if len(bundle.PendingGeneration) != 2 {
		t.Fatalf("pending generation = %+v, want 2 tasks", bundle.PendingGeneration)
	}
	if bundle.PendingGeneration[0].RecipeID != "shein-main-model" {
		t.Fatalf("first pending task = %+v, want shein-main-model", bundle.PendingGeneration[0])
	}
	if bundle.PendingGeneration[0].CanExecute != true {
		t.Fatalf("first pending task = %+v, want executable task", bundle.PendingGeneration[0])
	}
	if bundle.PendingGeneration[0].ExecutionStatus != "planned" {
		t.Fatalf("first pending task = %+v, want planned execution status", bundle.PendingGeneration[0])
	}
}

func TestBuilderBuildMarksFallbackSlotSemantics(t *testing.T) {
	t.Parallel()

	builder := assetbundle.NewBuilder()
	inventory := &asset.Inventory{
		Records: []asset.AssetRecord{
			{ID: "main-1", Kind: asset.KindMainImage, URL: "https://example.com/main.jpg"},
		},
	}
	recipes := []assetrecipe.AssetRecipe{
		{
			ID:        "shein-main-model",
			Platform:  "shein",
			AssetKind: asset.KindModelImage,
			Generated: true,
			Template: &assetrecipe.Template{
				BundleSlot:     "main",
				Purpose:        "main",
				PreferredKinds: []asset.Kind{asset.KindModelImage, asset.KindMainImage},
				Optional:       false,
				MaxItems:       1,
			},
		},
	}

	bundle := builder.Build(assetbundle.BuildRequest{
		Platform:  "shein",
		Inventory: inventory,
		Recipes:   recipes,
	})
	if bundle == nil || bundle.Main == nil {
		t.Fatalf("bundle = %+v, want main slot", bundle)
	}
	if bundle.Main.SatisfiedBy != "fallback_asset" {
		t.Fatalf("main slot = %+v, want fallback_asset", bundle.Main)
	}
	if bundle.Main.FallbackFrom != string(asset.KindModelImage) {
		t.Fatalf("main slot = %+v, want fallback from model_image", bundle.Main)
	}
	if bundle.Main.ExecutionStatus != "fallback" {
		t.Fatalf("main slot = %+v, want fallback execution status", bundle.Main)
	}
	if bundle.Main.IdealKind != string(asset.KindModelImage) {
		t.Fatalf("main slot = %+v, want ideal kind model_image", bundle.Main)
	}
	if bundle.Main.TemplateLabel == "" {
		t.Fatalf("main slot = %+v, want template label", bundle.Main)
	}
	if len(bundle.PendingGeneration) != 1 {
		t.Fatalf("pending generation = %+v, want 1 task", bundle.PendingGeneration)
	}
	if bundle.PendingGeneration[0].RenderProfile == "" {
		t.Fatalf("pending generation task = %+v, want render profile", bundle.PendingGeneration[0])
	}
	if bundle.PendingGeneration[0].TemplateLabel == "" {
		t.Fatalf("pending generation task = %+v, want template label", bundle.PendingGeneration[0])
	}
}
