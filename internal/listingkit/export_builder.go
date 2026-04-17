package listingkit

import (
	"fmt"
	"strings"
	"time"
)

func buildListingKitExport(task *Task, selectedPlatform string) (*ListingKitExport, error) {
	if task == nil {
		return nil, ErrTaskNotFound
	}

	selectedPlatform = strings.ToLower(strings.TrimSpace(selectedPlatform))
	if selectedPlatform != "" && len(normalizePlatforms([]string{selectedPlatform})) == 0 {
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

	export.CatalogProduct = task.Result.CatalogProduct
	export.AssetBundle = task.Result.AssetBundle
	export.Overview = buildListingKitExportMeta(task.Result)

	if selectedPlatform == "" || selectedPlatform == "amazon" {
		if task.Result.Amazon != nil {
			export.Amazon = &AmazonExportPayload{Draft: task.Result.Amazon.Draft}
		} else if selectedPlatform == "amazon" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	if selectedPlatform == "" || selectedPlatform == "shein" {
		if task.Result.Shein != nil {
			export.Shein = &SheinExportPayload{
				Inspection:     task.Result.Shein.Inspection,
				RequestDraft:   task.Result.Shein.RequestDraft,
				PreviewProduct: task.Result.Shein.PreviewProduct,
				ReviewNotes:    append([]string(nil), task.Result.Shein.ReviewNotes...),
			}
		} else if selectedPlatform == "shein" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	if selectedPlatform == "" || selectedPlatform == "temu" {
		if task.Result.Temu != nil {
			export.Temu = &TemuExportPayload{Package: task.Result.Temu}
		} else if selectedPlatform == "temu" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	if selectedPlatform == "" || selectedPlatform == "walmart" {
		if task.Result.Walmart != nil {
			export.Walmart = &WalmartExportPayload{Package: task.Result.Walmart}
		} else if selectedPlatform == "walmart" {
			return nil, ErrPreviewPlatformUnavailable
		}
	}

	return export, nil
}

func buildListingKitExportMeta(result *ListingKitResult) *ListingKitExportMeta {
	if result == nil {
		return nil
	}
	meta := &ListingKitExportMeta{
		Country:  result.Country,
		Language: result.Language,
	}
	if result.Summary != nil {
		meta.SourceType = result.Summary.SourceType
		meta.ImageCount = result.Summary.ImageCount
		meta.VariantCount = result.Summary.VariantCount
		meta.Warnings = append([]string(nil), result.Summary.Warnings...)
	}
	return meta
}

func buildListingKitExportFileName(taskID string, selectedPlatform string) string {
	scope := firstNonEmpty(selectedPlatform, "bundle")
	return fmt.Sprintf("listing-kit-%s-%s.json", taskID, scope)
}
