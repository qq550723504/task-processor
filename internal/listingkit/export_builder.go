package listingkit

import (
	"fmt"
	"time"

	previewdomain "task-processor/internal/listing/preview"
	sheinpub "task-processor/internal/publishing/shein"
)

func buildListingKitExport(task *Task, selectedPlatform string) (*ListingKitExport, error) {
	if task == nil {
		return nil, ErrTaskNotFound
	}

	var ok bool
	selectedPlatform, ok = previewdomain.ValidateSelectedPlatform(selectedPlatform)
	if !ok {
		return nil, ErrUnsupportedPreviewPlatform
	}

	export := &ListingKitExport{
		TaskID:           task.ID,
		SelectedPlatform: selectedPlatform,
		Format:           "json",
		MimeType:         "application/json; charset=utf-8",
		FileName:         buildListingKitExportFileName(task.ID, selectedPlatform),
		GeneratedAt:      time.Now(),
		Platforms:        previewPlatforms(task),
	}

	if task.Result == nil {
		return export, nil
	}

	projection := buildListingKitExportProjection(task.Result, selectedPlatform)
	export.CatalogProduct = projection.catalog
	export.AssetBundle = projection.assetBundle
	export.AssetInventorySummary = projection.assetInventory
	export.AssetRenderPreviews = projection.assetRenderPreviews
	export.PlatformAssetRenderPreviews = projection.platformPreviews
	export.AssetGenerationQueue = projection.generationQueue
	export.AssetGenerationOverview = projection.generationOverview
	export.Overview = projection.overview

	if selectedPlatform == "" || selectedPlatform == "amazon" {
		if task.Result.Amazon != nil {
			export.Amazon = buildAmazonExportPayload(amazonExportPayloadInput{
				draft:      task.Result.Amazon.Draft,
				visualBase: buildPlatformVisualExportPayloadInput("amazon", task.Result.Amazon.ImageBundle, task.Result.AssetBundle, export.PlatformAssetRenderPreviews),
			})
		} else if selectedPlatform == "amazon" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	if selectedPlatform == "" || selectedPlatform == "shein" {
		if task.Result.Shein != nil {
			sheinpub.NormalizePackageSemanticFields(task.Result.Shein)
			visualBase := buildPlatformVisualExportBase("shein", task.Result.Shein.ImageBundle, task.Result.AssetBundle, export.PlatformAssetRenderPreviews)
			export.Shein = &SheinExportPayload{
				Inspection:     task.Result.Shein.Inspection,
				ImageBundle:    visualBase.imageBundle,
				RenderPreviews: visualBase.renderPreviews,
				ScenePresets:   visualBase.scenePresets,
				DraftPayload:   task.Result.Shein.DraftPayload,
				PreviewPayload: task.Result.Shein.PreviewPayload,
				ReviewNotes:    append([]string(nil), task.Result.Shein.ReviewNotes...),
			}
			normalizeSheinExportPayloadSemanticFields(export.Shein)
		} else if selectedPlatform == "shein" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	if selectedPlatform == "" || selectedPlatform == "temu" {
		if task.Result.Temu != nil {
			export.Temu = buildTemuExportPayload(
				buildReviewablePlatformExportPayloadInput("temu", task.Result.Temu.ImageBundle, task.Result.AssetBundle, export.PlatformAssetRenderPreviews),
				task.Result.Temu,
			)
		} else if selectedPlatform == "temu" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	if selectedPlatform == "" || selectedPlatform == "walmart" {
		if task.Result.Walmart != nil {
			export.Walmart = buildWalmartExportPayload(
				buildReviewablePlatformExportPayloadInput("walmart", task.Result.Walmart.ImageBundle, task.Result.AssetBundle, export.PlatformAssetRenderPreviews),
				task.Result.Walmart,
			)
		} else if selectedPlatform == "walmart" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	return export, nil
}

func buildListingKitExportMeta(result *ListingKitResult, selectedPlatform string) *ListingKitExportMeta {
	projection := buildListingKitReadProjection(result, selectedPlatform)
	if projection == nil {
		return nil
	}
	return buildListingKitExportMetaFromOverview(projection.Overview)
}

func buildListingKitExportMetaFromOverview(overview *listingKitOverviewData) *ListingKitExportMeta {
	meta := initializeListingKitExportMeta(overview)
	return decorateListingKitExportMeta(overview, meta)
}

func buildListingKitExportFileName(taskID string, selectedPlatform string) string {
	scope := firstNonEmpty(selectedPlatform, "bundle")
	return fmt.Sprintf("listing-kit-%s-%s.json", taskID, scope)
}
