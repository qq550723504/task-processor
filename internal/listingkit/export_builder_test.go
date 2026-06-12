package listingkit

import (
	"strings"
	"testing"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	common "task-processor/internal/publishing/common"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildListingKitExportForSelectedPlatform(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID: "task-export-1",
		Request: &GenerateRequest{
			Platforms: []string{"amazon", "shein", "temu", "walmart"},
		},
		Result: &ListingKitResult{
			Platforms: []string{"amazon", "shein", "temu", "walmart"},
			Country:   "US",
			Language:  "en_US",
			CatalogProduct: &catalog.Product{
				Title: "Travel Bottle",
				Brand: "DemoBrand",
			},
			AssetBundle: &asset.Bundle{
				Assets: []asset.Asset{
					{
						ID:   "asset-main",
						Kind: asset.KindMainImage,
						URL:  "https://cdn.example.com/main.jpg",
						Metadata: map[string]string{
							"prompt_key":            "productimage.scene.bags",
							"scene_defaults_source": "platform_category",
							"scene_category":        "bags",
							"scene_style":           "studio",
							"background_tone":       "bright",
						},
					},
				},
			},
			AssetInventorySummary: &asset.InventorySummary{
				TotalRecords:  3,
				SelectedCount: 2,
			},
			AssetRenderPreviews: []AssetRenderPreview{
				{
					AssetID:             "asset-main",
					Kind:                asset.KindSellingPointImage,
					RenderProfile:       "shein_selling_point",
					TemplateLabel:       "SHEIN Editorial Main",
					PreviewFormat:       "svg",
					PreviewSVG:          "<svg>preview</svg>",
					VisualMode:          "selling_point",
					LayoutEngine:        "selling_point_output_v2",
					RenderOutputVersion: "v2",
					DrawOutputVersion:   "v1",
					DrawPreviewVersion:  "v1",
					LayerTypes:          []string{"background", "badge", "text"},
					Regions:             []string{"full_canvas", "title_band", "body_copy"},
					StyleTokens:         []string{"bg-soft", "badge-dark", "copy-primary"},
				},
			},
			AssetGenerationQueue: &GenerationWorkQueue{
				Summary: &GenerationWorkQueueSummary{
					TotalItems:         1,
					QualityGradeCounts: map[string]int{"provisional": 1},
				},
				Items: []GenerationWorkQueueItem{{
					Platform:          "shein",
					Slot:              "main",
					State:             "fallback_in_use",
					Retryable:         true,
					QualityGrade:      "provisional",
					QualityGradeLabel: "Provisional",
				}},
			},
			AssetGenerationOverview: &AssetGenerationOverview{
				PrimaryAction:    "Upgrade Fallback Assets",
				PrimaryActionKey: "upgrade_fallback_assets",
				PrimaryActionTarget: &AssetGenerationActionTarget{
					ActionKey: "upgrade_fallback_assets",
					Filters: &AssetGenerationRecommendedFilters{
						QualityGrade:      "provisional",
						QualityGradeLabel: "Provisional",
						Platforms:         []string{"shein"},
						RetryableOnly:     true,
					},
					QueueQuery: &GenerationQueueQuery{
						QualityGrade:      "provisional",
						QualityGradeLabel: "Provisional",
						Retryable:         true,
						RetryablePresent:  true,
						SortBy:            "quality_grade",
						SortOrder:         "asc",
					},
					RetryRequest: &RetryGenerationTasksRequest{
						QualityGrade:      "provisional",
						QualityGradeLabel: "Provisional",
					},
				},
				PrimaryActionReason:       "1 asset slots are still using fallback outputs.",
				SecondaryActionKeys:       []string{"retry_provisional_slots"},
				DominantQualityGrade:      "provisional",
				DominantQualityGradeLabel: "Provisional",
				BlockingPlatforms:         []string{"shein"},
				BlockingQualityGrades:     []string{"provisional"},
				RecommendedFilters: &AssetGenerationRecommendedFilters{
					QualityGrade:  "provisional",
					Platforms:     []string{"shein"},
					RetryableOnly: true,
				},
				RetryableCount: 1,
			},
			Summary: &GenerationSummary{
				SourceType:   "1688_url",
				ImageCount:   4,
				VariantCount: 2,
				Warnings:     []string{"请确认类目", "请确认类目"},
			},
			Shein: &SheinPackage{
				ImageBundle: &common.PublishImageBundle{
					Platform: "shein",
					Main: &common.BundleSlot{
						AssetID:         "asset-main",
						URL:             "https://cdn.example.com/main.jpg",
						TemplateLabel:   "SHEIN Editorial Main",
						IdealKind:       string(asset.KindModelImage),
						StateLabel:      "fallback_in_use",
						RetryHint:       "retry generation for this slot to replace the fallback asset",
						SatisfiedBy:     "fallback_asset",
						FallbackFrom:    string(asset.KindModelImage),
						ExecutionStatus: "fallback",
					},
				},
				RequestDraft: &SheinRequestDraft{
					SpuName: "Travel Bottle",
				},
				PreviewProduct: &sheinproduct.Product{
					SPUName: "Travel Bottle",
				},
				Inspection: &SheinInspection{
					NeedsReview: true,
					Summary:     []string{"请确认类目"},
				},
				ReviewNotes: []string{"请确认类目"},
			},
		},
	}

	export, err := buildListingKitExport(task, "shein")
	if err != nil {
		t.Fatalf("build export: %v", err)
	}

	if export.SelectedPlatform != "shein" {
		t.Fatalf("selected platform = %q, want shein", export.SelectedPlatform)
	}
	if export.CatalogProduct == nil || export.CatalogProduct.Title != "Travel Bottle" {
		t.Fatalf("catalog product = %+v", export.CatalogProduct)
	}
	if export.AssetBundle == nil || len(export.AssetBundle.Assets) != 1 {
		t.Fatalf("asset bundle = %+v", export.AssetBundle)
	}
	if export.AssetInventorySummary == nil || export.AssetInventorySummary.TotalRecords != 3 {
		t.Fatalf("asset inventory summary = %+v", export.AssetInventorySummary)
	}
	if len(export.AssetRenderPreviews) != 1 || export.AssetRenderPreviews[0].PreviewFormat != "svg" {
		t.Fatalf("asset render previews = %+v", export.AssetRenderPreviews)
	}
	if len(export.PlatformAssetRenderPreviews) != 1 {
		t.Fatalf("platform asset render previews = %+v", export.PlatformAssetRenderPreviews)
	}
	if export.PlatformAssetRenderPreviews[0].Platform != "shein" {
		t.Fatalf("platform asset render previews = %+v, want shein platform", export.PlatformAssetRenderPreviews)
	}
	if export.PlatformAssetRenderPreviews[0].Main == nil || export.PlatformAssetRenderPreviews[0].Main.PreviewSVG != "<svg>preview</svg>" {
		t.Fatalf("platform asset render previews main = %+v", export.PlatformAssetRenderPreviews[0].Main)
	}
	if export.PlatformAssetRenderPreviews[0].Main.Slot != "main" {
		t.Fatalf("platform asset render previews main = %+v, want main slot", export.PlatformAssetRenderPreviews[0].Main)
	}
	if export.PlatformAssetRenderPreviews[0].Main.VisualMode != "selling_point" || export.PlatformAssetRenderPreviews[0].Main.LayoutEngine != "selling_point_output_v2" {
		t.Fatalf("platform asset render previews main summary = %+v", export.PlatformAssetRenderPreviews[0].Main)
	}
	if export.PlatformAssetRenderPreviews[0].Summary == nil {
		t.Fatalf("platform asset render previews summary = %+v", export.PlatformAssetRenderPreviews[0])
	}
	if export.PlatformAssetRenderPreviews[0].Summary.TotalPreviews != 1 || !export.PlatformAssetRenderPreviews[0].Summary.MainAvailable {
		t.Fatalf("platform asset render previews summary = %+v", export.PlatformAssetRenderPreviews[0].Summary)
	}
	if export.PlatformAssetRenderPreviews[0].Summary.CapabilityCounts["badge_preview"] != 1 || export.PlatformAssetRenderPreviews[0].Summary.CapabilityCounts["copy_preview"] != 1 {
		t.Fatalf("platform asset render previews summary = %+v", export.PlatformAssetRenderPreviews[0].Summary)
	}
	if export.AssetGenerationQueue == nil || export.AssetGenerationQueue.Summary == nil || export.AssetGenerationQueue.Summary.TotalItems != 1 {
		t.Fatalf("asset generation queue = %+v", export.AssetGenerationQueue)
	}
	if export.AssetGenerationOverview == nil || export.AssetGenerationOverview.PrimaryAction != "Upgrade Fallback Assets" {
		t.Fatalf("asset generation overview = %+v, want fallback upgrade CTA", export.AssetGenerationOverview)
	}
	if export.AssetGenerationOverview.PrimaryActionKey != "upgrade_fallback_assets" {
		t.Fatalf("asset generation overview = %+v, want fallback action key", export.AssetGenerationOverview)
	}
	if export.AssetGenerationOverview.PrimaryActionTarget == nil || export.AssetGenerationOverview.PrimaryActionTarget.RetryRequest == nil {
		t.Fatalf("asset generation overview = %+v, want executable primary action target", export.AssetGenerationOverview)
	}
	if export.Shein == nil {
		t.Fatal("expected shein export payload")
	}
	if export.Shein.ImageBundle == nil {
		t.Fatalf("shein image bundle = %+v", export.Shein)
	}
	if export.Shein.ImageBundle.Main == nil || export.Shein.ImageBundle.Main.ExecutionStatus != "fallback" {
		t.Fatalf("shein image bundle main = %+v, want fallback status", export.Shein.ImageBundle.Main)
	}
	if export.Shein.ImageBundle.Main.TemplateLabel != "SHEIN Editorial Main" {
		t.Fatalf("shein image bundle main = %+v, want template label", export.Shein.ImageBundle.Main)
	}
	if export.Shein.ImageBundle.Main.StateLabel != "fallback_in_use" {
		t.Fatalf("shein image bundle main = %+v, want state label", export.Shein.ImageBundle.Main)
	}
	if export.Shein.ImageBundle.Main.RetryHint == "" {
		t.Fatalf("shein image bundle main = %+v, want retry hint", export.Shein.ImageBundle.Main)
	}
	if export.Shein.RenderPreviews == nil || export.Shein.RenderPreviews.Main == nil {
		t.Fatalf("shein render previews = %+v", export.Shein.RenderPreviews)
	}
	if export.Shein.RenderPreviews.Main.PreviewSVG != "<svg>preview</svg>" {
		t.Fatalf("shein render previews main = %+v", export.Shein.RenderPreviews.Main)
	}
	if len(export.Shein.RenderPreviews.Main.Regions) != 3 || export.Shein.RenderPreviews.Main.StyleTokens[2] != "copy-primary" {
		t.Fatalf("shein render previews main summary = %+v", export.Shein.RenderPreviews.Main)
	}
	if export.Shein.RenderPreviews.Summary == nil || export.Shein.RenderPreviews.Summary.TotalPreviews != 1 {
		t.Fatalf("shein render previews summary = %+v", export.Shein.RenderPreviews.Summary)
	}
	if len(export.Shein.ScenePresets) != 1 {
		t.Fatalf("shein scene presets = %+v, want 1 summary", export.Shein.ScenePresets)
	}
	if export.Shein.ScenePresets[0].Slot != "main" || export.Shein.ScenePresets[0].AssetID != "asset-main" {
		t.Fatalf("shein scene preset summary = %+v, want main slot summary", export.Shein.ScenePresets[0])
	}
	if export.Shein.ScenePresets[0].ScenePreset == nil || export.Shein.ScenePresets[0].ScenePreset.PromptKey != "productimage.scene.bags" {
		t.Fatalf("shein scene preset summary = %+v, want bag scene prompt", export.Shein.ScenePresets[0])
	}
	if export.Shein.ScenePresets[0].ScenePreset.DefaultsSource != "platform_category" || export.Shein.ScenePresets[0].ScenePreset.SceneStyle != "studio" {
		t.Fatalf("shein scene preset summary = %+v, want preserved scene metadata", export.Shein.ScenePresets[0])
	}
	if export.Amazon != nil || export.Temu != nil || export.Walmart != nil {
		t.Fatal("expected only shein export payload")
	}
	if export.Shein.RequestDraft == nil || export.Shein.RequestDraft.SpuName != "Travel Bottle" {
		t.Fatalf("unexpected shein request draft: %+v", export.Shein.RequestDraft)
	}
	if export.Overview == nil || len(export.Overview.PlatformCards) != 1 {
		t.Fatalf("export overview cards = %+v", export.Overview)
	}
	if got, want := export.Overview.ReviewReasons, []string{"请确认类目"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("export overview review reasons = %#v, want %#v", got, want)
	}
	if export.Overview.PlatformCards[0].PreviewSummary == nil || export.Overview.PlatformCards[0].PreviewSummary.TotalPreviews != 1 {
		t.Fatalf("export overview preview summary = %+v", export.Overview.PlatformCards[0])
	}
	if export.Overview.PlatformCards[0].PrimaryActionKey != "upgrade_fallback_assets" {
		t.Fatalf("export overview card primary action = %+v", export.Overview.PlatformCards[0])
	}
	if export.Overview.PlatformCards[0].PrimaryActionTarget == nil || export.Overview.PlatformCards[0].PrimaryActionTarget.RetryRequest == nil {
		t.Fatalf("export overview card action target = %+v", export.Overview.PlatformCards[0])
	}
	if !strings.Contains(export.FileName, "shein") {
		t.Fatalf("file name = %q, want platform suffix", export.FileName)
	}
}

func TestBuildListingKitExportReturnsBundleByDefault(t *testing.T) {
	t.Parallel()

	task := &Task{
		ID: "task-export-2",
		Request: &GenerateRequest{
			Platforms: []string{"temu", "walmart"},
		},
		Result: &ListingKitResult{
			Platforms: []string{"temu", "walmart"},
			Temu: &TemuPackage{
				GoodsName: "Bottle",
			},
			Walmart: &WalmartPackage{
				ProductName: "Bottle",
			},
		},
	}

	export, err := buildListingKitExport(task, "")
	if err != nil {
		t.Fatalf("build export bundle: %v", err)
	}

	if export.Temu == nil || export.Walmart == nil {
		t.Fatal("expected bundle export to include available platforms")
	}
	if !strings.Contains(export.FileName, "bundle") {
		t.Fatalf("file name = %q, want bundle suffix", export.FileName)
	}
}

func TestBuildListingKitExportRejectsUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	_, err := buildListingKitExport(&Task{ID: "task-export-invalid"}, " ebay ")
	if err != ErrUnsupportedPreviewPlatform {
		t.Fatalf("error = %v, want %v", err, ErrUnsupportedPreviewPlatform)
	}
}

func TestBuildListingKitExportMetaCopiesOverviewFields(t *testing.T) {
	t.Parallel()

	meta := buildListingKitExportMeta(&ListingKitResult{
		Country:  "US",
		Language: "en_US",
		Summary: &GenerationSummary{
			SourceType:   "text",
			ImageCount:   2,
			VariantCount: 3,
			Warnings:     []string{"warn"},
		},
	}, "")
	if meta == nil {
		t.Fatal("expected export meta")
	}
	if meta.Country != "US" || meta.Language != "en_US" {
		t.Fatalf("meta locale fields = %+v", meta)
	}
	if meta.SourceType != "text" || meta.ImageCount != 2 || meta.VariantCount != 3 {
		t.Fatalf("meta summary fields = %+v", meta)
	}
	if len(meta.Warnings) != 1 || meta.Warnings[0] != "warn" {
		t.Fatalf("meta warnings = %#v, want [warn]", meta.Warnings)
	}
}
