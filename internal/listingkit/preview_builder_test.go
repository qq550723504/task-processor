package listingkit

import (
	"testing"
	"time"

	"task-processor/internal/asset"
	"task-processor/internal/catalog"
	"task-processor/internal/catalog/canonical"
	listingplatform "task-processor/internal/listing/platform"
	common "task-processor/internal/publishing/common"
	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildPreviewHeaderCopiesSummaryFields(t *testing.T) {
	t.Parallel()

	header := buildPreviewHeader(&ListingKitResult{
		Country:  "US",
		Language: "en_US",
		Summary: &GenerationSummary{
			SourceType:   "text",
			ImageCount:   2,
			VariantCount: 3,
			Warnings:     []string{"warn"},
		},
	}, "")
	if header == nil {
		t.Fatal("expected preview header")
	}
	if header.Country != "US" || header.Language != "en_US" {
		t.Fatalf("header locale fields = %+v", header)
	}
	if header.SourceType != "text" || header.ImageCount != 2 || header.VariantCount != 3 {
		t.Fatalf("header summary fields = %+v", header)
	}
	if len(header.Warnings) != 1 || header.Warnings[0] != "warn" {
		t.Fatalf("header warnings = %#v, want [warn]", header.Warnings)
	}
}

func TestNormalizePreviewPlatform(t *testing.T) {
	t.Parallel()

	got, ok := listingplatform.ValidateSelectedPlatform("  SHEIN ")
	if !ok {
		t.Fatal("ValidateSelectedPlatform() = not ok, want ok")
	}
	if got != "shein" {
		t.Fatalf("ValidateSelectedPlatform() = %q, want %q", got, "shein")
	}
}

func TestBuildListingKitPreviewBackfillsSheinSourceProductSDSIdentity(t *testing.T) {
	t.Parallel()

	preview, err := buildListingKitPreview(&Task{
		ID: "task-sds-source-link",
		Request: &GenerateRequest{
			Platforms: []string{"shein"},
			Options: &GenerateOptions{SDS: &SDSSyncOptions{
				ParentProductID: 41661,
				VariantID:       41662,
			}},
		},
		Result: &ListingKitResult{
			TaskID:    "task-sds-source-link",
			Platforms: []string{"shein"},
			CanonicalProduct: &canonical.Product{
				Title: "SDS Pants",
				Attributes: map[string]canonical.Attribute{
					"sku": {Value: "NS6104229008"},
				},
			},
			Shein: &SheinPackage{},
		},
	}, "shein")
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}

	if preview.Shein == nil || preview.Shein.SourceProduct == nil {
		t.Fatalf("source product = %+v", preview.Shein)
	}
	if got := preview.Shein.SourceProduct.ParentProductID; got != "41661" {
		t.Fatalf("source product parent_product_id = %q, want 41661", got)
	}
	if preview.Shein.FinalReview == nil || preview.Shein.FinalReview.SourceProduct == nil {
		t.Fatalf("final review source product = %+v", preview.Shein.FinalReview)
	}
	if got := preview.Shein.FinalReview.SourceProduct.ParentProductID; got != "41661" {
		t.Fatalf("final review source product parent_product_id = %q, want 41661", got)
	}
}

func TestPreviewPlatformsPrefersResultPlatforms(t *testing.T) {
	t.Parallel()

	got := previewPlatforms(&Task{
		Request: &GenerateRequest{Platforms: []string{"amazon"}},
		Result:  &ListingKitResult{Platforms: []string{"shein", "temu"}},
	})
	if len(got) != 2 || got[0] != "shein" || got[1] != "temu" {
		t.Fatalf("previewPlatforms() = %#v, want result platforms", got)
	}
}

func TestBuildListingKitPreviewFiltersSelectedPlatform(t *testing.T) {
	t.Parallel()

	now := time.Now()
	task := &Task{
		ID:        "task-preview-1",
		Status:    TaskStatusCompleted,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
		Request: &GenerateRequest{
			Platforms: []string{"amazon", "shein", "temu"},
		},
		Result: &ListingKitResult{
			TaskID:    "task-preview-1",
			Status:    "completed",
			Platforms: []string{"amazon", "shein", "temu"},
			Country:   "US",
			Language:  "en_US",
			CatalogProduct: &catalog.Product{
				Title: "Wireless Earbuds",
				Brand: "DemoBrand",
			},
			AssetBundle: &asset.Bundle{
				Assets: []asset.Asset{
					{
						ID:   "asset-main",
						Kind: asset.KindMainImage,
						URL:  "https://cdn.example.com/main.jpg",
						Metadata: map[string]string{
							"prompt_key":            "productimage.scene.jewelry",
							"scene_defaults_source": "platform_category",
							"scene_category":        "jewelry",
							"scene_style":           "studio",
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
				SourceType:   "text",
				ImageCount:   2,
				VariantCount: 3,
				NeedsReview:  true,
				Warnings:     []string{"需要确认 SHEIN 销售属性"},
			},
			Shein: &SheinPackage{
				SpuName:      "Wireless Earbuds",
				BrandName:    "DemoBrand",
				CategoryPath: []string{"Electronics", "Headphones"},
				CategoryID:   123,
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
				ReviewNotes: []string{"确认主销售属性"},
				Inspection: &SheinInspection{
					NeedsReview: true,
					Summary:     []string{"销售属性仍需人工确认"},
				},
				RequestDraft: &SheinRequestDraft{
					SpuName: "Wireless Earbuds",
				},
				PreviewProduct: &sheinproduct.Product{
					SPUName: "Wireless Earbuds",
				},
			},
		},
	}

	preview, err := buildListingKitPreview(task, "shein")
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}

	if preview.SelectedPlatform != "shein" {
		t.Fatalf("selected platform = %q, want shein", preview.SelectedPlatform)
	}
	if preview.Catalog == nil || preview.Catalog.Title != "Wireless Earbuds" {
		t.Fatalf("catalog = %+v", preview.Catalog)
	}
	if preview.Assets == nil || len(preview.Assets.Assets) != 1 {
		t.Fatalf("assets = %+v", preview.Assets)
	}
	if preview.AssetInventory == nil || preview.AssetInventory.TotalRecords != 3 {
		t.Fatalf("asset inventory = %+v", preview.AssetInventory)
	}
	if len(preview.AssetRenderPreviews) != 1 || preview.AssetRenderPreviews[0].PreviewFormat != "svg" {
		t.Fatalf("asset render previews = %+v", preview.AssetRenderPreviews)
	}
	if len(preview.PlatformAssetRenderPreviews) != 1 {
		t.Fatalf("platform asset render previews = %+v", preview.PlatformAssetRenderPreviews)
	}
	if preview.PlatformAssetRenderPreviews[0].Platform != "shein" {
		t.Fatalf("platform asset render previews = %+v, want shein platform", preview.PlatformAssetRenderPreviews)
	}
	if preview.PlatformAssetRenderPreviews[0].Main == nil || preview.PlatformAssetRenderPreviews[0].Main.PreviewSVG != "<svg>preview</svg>" {
		t.Fatalf("platform asset render previews main = %+v", preview.PlatformAssetRenderPreviews[0].Main)
	}
	if preview.PlatformAssetRenderPreviews[0].Main.Slot != "main" {
		t.Fatalf("platform asset render previews main = %+v, want main slot", preview.PlatformAssetRenderPreviews[0].Main)
	}
	if preview.PlatformAssetRenderPreviews[0].Main.VisualMode != "selling_point" || preview.PlatformAssetRenderPreviews[0].Main.LayoutEngine != "selling_point_output_v2" {
		t.Fatalf("platform asset render previews main summary = %+v", preview.PlatformAssetRenderPreviews[0].Main)
	}
	if preview.PlatformAssetRenderPreviews[0].Summary == nil {
		t.Fatalf("platform asset render previews summary = %+v", preview.PlatformAssetRenderPreviews[0])
	}
	if preview.PlatformAssetRenderPreviews[0].Summary.TotalPreviews != 1 || !preview.PlatformAssetRenderPreviews[0].Summary.MainAvailable {
		t.Fatalf("platform asset render previews summary = %+v", preview.PlatformAssetRenderPreviews[0].Summary)
	}
	if preview.PlatformAssetRenderPreviews[0].Summary.CapabilityCounts["badge_preview"] != 1 || preview.PlatformAssetRenderPreviews[0].Summary.CapabilityCounts["copy_preview"] != 1 {
		t.Fatalf("platform asset render previews summary = %+v", preview.PlatformAssetRenderPreviews[0].Summary)
	}
	if preview.AssetGenerationQueue == nil || preview.AssetGenerationQueue.Summary == nil || preview.AssetGenerationQueue.Summary.TotalItems != 1 {
		t.Fatalf("asset generation queue = %+v", preview.AssetGenerationQueue)
	}
	if preview.AssetGenerationOverview == nil || preview.AssetGenerationOverview.PrimaryAction != "Upgrade Fallback Assets" {
		t.Fatalf("asset generation overview = %+v, want fallback upgrade CTA", preview.AssetGenerationOverview)
	}
	if preview.AssetGenerationOverview.PrimaryActionKey != "upgrade_fallback_assets" {
		t.Fatalf("asset generation overview = %+v, want fallback action key", preview.AssetGenerationOverview)
	}
	if preview.AssetGenerationOverview.PrimaryActionTarget == nil || preview.AssetGenerationOverview.PrimaryActionTarget.RetryRequest == nil {
		t.Fatalf("asset generation overview = %+v, want executable primary action target", preview.AssetGenerationOverview)
	}
	if preview.Shein == nil {
		t.Fatal("expected shein payload")
	}
	if preview.Shein.ImageBundle == nil {
		t.Fatalf("shein image bundle = %+v", preview.Shein)
	}
	if preview.Shein.ImageBundle.Main == nil || preview.Shein.ImageBundle.Main.ExecutionStatus != "fallback" {
		t.Fatalf("shein image bundle main = %+v, want fallback status", preview.Shein.ImageBundle.Main)
	}
	if preview.Shein.ImageBundle.Main.TemplateLabel != "SHEIN Editorial Main" {
		t.Fatalf("shein image bundle main = %+v, want template label", preview.Shein.ImageBundle.Main)
	}
	if preview.Shein.ImageBundle.Main.StateLabel != "fallback_in_use" {
		t.Fatalf("shein image bundle main = %+v, want state_label", preview.Shein.ImageBundle.Main)
	}
	if preview.Shein.ImageBundle.Main.RetryHint == "" {
		t.Fatalf("shein image bundle main = %+v, want retry hint", preview.Shein.ImageBundle.Main)
	}
	if preview.Shein.RenderPreviews == nil || preview.Shein.RenderPreviews.Main == nil {
		t.Fatalf("shein render previews = %+v", preview.Shein.RenderPreviews)
	}
	if preview.Shein.RenderPreviews.Main.PreviewSVG != "<svg>preview</svg>" {
		t.Fatalf("shein render previews main = %+v", preview.Shein.RenderPreviews.Main)
	}
	if len(preview.Shein.RenderPreviews.Main.LayerTypes) != 3 || preview.Shein.RenderPreviews.Main.StyleTokens[1] != "badge-dark" {
		t.Fatalf("shein render previews main summary = %+v", preview.Shein.RenderPreviews.Main)
	}
	if preview.Shein.RenderPreviews.Summary == nil || preview.Shein.RenderPreviews.Summary.TotalPreviews != 1 {
		t.Fatalf("shein render previews summary = %+v", preview.Shein.RenderPreviews.Summary)
	}
	if len(preview.Shein.ScenePresets) != 1 {
		t.Fatalf("shein scene presets = %+v, want 1 summary", preview.Shein.ScenePresets)
	}
	if preview.Shein.ScenePresets[0].Slot != "main" || preview.Shein.ScenePresets[0].AssetID != "asset-main" {
		t.Fatalf("shein scene preset summary = %+v, want main asset summary", preview.Shein.ScenePresets[0])
	}
	if preview.Shein.ScenePresets[0].ScenePreset == nil || preview.Shein.ScenePresets[0].ScenePreset.PromptKey != "productimage.scene.jewelry" {
		t.Fatalf("shein scene preset summary = %+v, want jewelry scene prompt", preview.Shein.ScenePresets[0])
	}
	if preview.Amazon != nil || preview.Temu != nil || preview.Walmart != nil {
		t.Fatal("expected only selected platform payload")
	}
	if !preview.NeedsReview {
		t.Fatal("expected preview to require review")
	}
	if preview.Overview == nil || len(preview.Overview.PlatformCards) != 1 {
		t.Fatalf("overview cards = %+v, want single shein card", preview.Overview)
	}
	if got, want := preview.Overview.ReviewReasons, []string{"需要确认 SHEIN 销售属性"}; len(got) != len(want) || got[0] != want[0] {
		t.Fatalf("overview review reasons = %#v, want %#v", got, want)
	}
	if preview.Overview.PlatformCards[0].PreviewSummary == nil || preview.Overview.PlatformCards[0].PreviewSummary.TotalPreviews != 1 {
		t.Fatalf("overview card preview summary = %+v", preview.Overview.PlatformCards[0])
	}
	if preview.Overview.PlatformCards[0].PreviewCapabilityCounts["badge_preview"] != 1 {
		t.Fatalf("overview card capability counts = %+v", preview.Overview.PlatformCards[0])
	}
	if preview.Overview.PlatformCards[0].PrimaryActionKey != "upgrade_fallback_assets" {
		t.Fatalf("overview card primary action = %+v", preview.Overview.PlatformCards[0])
	}
	if preview.Overview.PlatformCards[0].PrimaryActionTarget == nil || preview.Overview.PlatformCards[0].PrimaryActionTarget.RetryRequest == nil {
		t.Fatalf("overview card primary action target = %+v", preview.Overview.PlatformCards[0])
	}
	if preview.Shein.PreviewProduct == nil || preview.Shein.PreviewProduct.SPUName != "Wireless Earbuds" {
		t.Fatalf("unexpected shein preview product: %+v", preview.Shein.PreviewProduct)
	}
	if preview.Shein.SubmitReadiness == nil {
		t.Fatal("expected shein submit readiness")
	}
	if preview.Shein.SubmitReadiness.Status != "blocked" {
		t.Fatalf("submit readiness status = %q, want blocked", preview.Shein.SubmitReadiness.Status)
	}
	if preview.Shein.SubmitChecklist == nil {
		t.Fatal("expected shein submit checklist")
	}
	if len(preview.Shein.SubmitChecklist.Required) == 0 {
		t.Fatalf("submit checklist = %+v, want required items", preview.Shein.SubmitChecklist)
	}
	if preview.Shein.RepairCenter == nil || preview.Shein.RepairCenter.Stats == nil || preview.Shein.RepairCenter.Stats.TotalActions == 0 {
		t.Fatalf("repair center = %+v", preview.Shein.RepairCenter)
	}
	if preview.Shein.RepairCenter.PrimaryPlan == nil || preview.Shein.RepairCenter.ApplyQueue == nil || preview.Shein.RepairCenter.Session == nil {
		t.Fatalf("repair center plan = %+v", preview.Shein.RepairCenter)
	}
	if preview.Shein.WorkspaceOverview == nil || preview.Shein.WorkspaceOverview.PrimaryView == "" {
		t.Fatalf("workspace overview = %+v", preview.Shein.WorkspaceOverview)
	}
	if preview.Shein.WorkspaceOverview.ActiveSession == nil {
		t.Fatalf("workspace overview session = %+v", preview.Shein.WorkspaceOverview)
	}
	if preview.Shein.StatusOverview == nil {
		t.Fatal("expected shein status overview")
	}
	if preview.Shein.StatusOverview.Status != "blocked" {
		t.Fatalf("status overview status = %q, want blocked", preview.Shein.StatusOverview.Status)
	}
	if preview.Shein.EditorContext == nil {
		t.Fatal("expected shein editor context")
	}
}

func TestBuildListingKitPreviewRejectsUnsupportedPlatform(t *testing.T) {
	t.Parallel()

	_, err := buildListingKitPreview(&Task{ID: "task-preview-2"}, "ebay")
	if err == nil {
		t.Fatal("expected unsupported platform error")
	}
	if err != ErrUnsupportedPreviewPlatform {
		t.Fatalf("error = %v, want %v", err, ErrUnsupportedPreviewPlatform)
	}
}

func TestBuildListingKitPreviewReturnsPendingHeaderWhenResultMissing(t *testing.T) {
	t.Parallel()

	preview, err := buildListingKitPreview(&Task{
		ID:     "task-preview-pending",
		Status: TaskStatusProcessing,
	}, "")
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}
	if preview == nil || preview.Overview == nil {
		t.Fatalf("preview = %+v, want overview header", preview)
	}
	if preview.Overview.StatusMessage != "任务处理中，预览结果尚未准备完成" {
		t.Fatalf("status message = %q", preview.Overview.StatusMessage)
	}
}

func TestBuildListingKitPreviewIncludesRevisionHistoryMeta(t *testing.T) {
	t.Parallel()

	now := time.Now()
	task := &Task{
		ID:        "task-preview-history",
		Status:    TaskStatusCompleted,
		CreatedAt: now.Add(-time.Minute),
		UpdatedAt: now,
		Result: &ListingKitResult{
			TaskID:               "task-preview-history",
			Status:               "completed",
			RevisionHistoryTotal: maxRevisionHistoryRecords + 3,
			RevisionHistory: []ListingKitRevisionRecord{
				{Platform: "shein", UpdatedAt: now.Add(-2 * time.Minute)},
				{Platform: "shein", UpdatedAt: now.Add(-1 * time.Minute)},
			},
		},
	}

	preview, err := buildListingKitPreview(task, "")
	if err != nil {
		t.Fatalf("build preview: %v", err)
	}
	if preview.RevisionHistoryMeta == nil {
		t.Fatal("expected revision history meta")
	}
	if preview.RevisionHistory[0].RevisionID == "" || preview.RevisionHistory[1].RevisionID == "" {
		t.Fatalf("revision history items missing ids: %+v", preview.RevisionHistory)
	}
	if preview.RevisionHistoryMeta.TotalRecords != maxRevisionHistoryRecords+3 {
		t.Fatalf("revision history meta = %+v", preview.RevisionHistoryMeta)
	}
	if preview.RevisionHistoryMeta.ReturnedRecords != 2 {
		t.Fatalf("returned records = %d, want 2", preview.RevisionHistoryMeta.ReturnedRecords)
	}
	if !preview.RevisionHistoryMeta.HasMore {
		t.Fatalf("revision history meta = %+v, want has_more", preview.RevisionHistoryMeta)
	}
}
