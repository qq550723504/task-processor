package generation

import (
	"fmt"

	"task-processor/internal/asset"
)

func executeDeferredTask(taskID string, idx int, inventory *asset.Inventory, task Task) (asset.AssetRecord, bool) {
	base, ok := preferredDeferredBaseRecord(inventory, task)
	if !ok {
		return asset.AssetRecord{}, false
	}
	role := deferredRole(task.AssetKind, task.Purpose)
	return asset.AssetRecord{
		ID:        fmt.Sprintf("generated-%s-%d", role, idx+1),
		TaskID:    taskID,
		Kind:      task.AssetKind,
		Origin:    asset.OriginGenerated,
		Role:      role,
		URL:       base.URL,
		Generator: "asset_generation_stub",
		RecipeID:  task.RecipeID,
		Version:   &asset.AssetVersion{Number: 1, Label: "generated"},
		Lineage:   &asset.AssetLineage{ParentAssetIDs: []string{base.ID}, SourceAssetIDs: []string{base.ID}, Step: "deferred_generation"},
		Labels:    []string{role, task.Platform},
		Width:     base.Width,
		Height:    base.Height,
		Metadata: map[string]string{
			"execution_mode": ExecutionModeDeferredStub,
			"source_kind":    string(base.Kind),
			"platform":       task.Platform,
			"purpose":        task.Purpose,
			"slot":           task.Slot,
			"bundle_slot":    task.Slot,
		},
	}, true
}

func preferredDeferredBaseRecord(inventory *asset.Inventory, task Task) (asset.AssetRecord, bool) {
	if inventory == nil {
		return asset.AssetRecord{}, false
	}
	if len(task.SourceAssetIDs) > 0 {
		for _, sourceID := range task.SourceAssetIDs {
			for _, record := range inventory.Records {
				if record.ID == sourceID {
					return record, true
				}
			}
		}
	}
	switch task.AssetKind {
	case asset.KindModelImage:
		return preferredBaseRecord(inventory, asset.KindCleanImage, asset.KindMainImage, asset.KindSourceImage)
	case asset.KindSellingPointImage, asset.KindSizeSceneImage, asset.KindDetailCrop:
		return preferredBaseRecord(inventory, asset.KindGalleryImage, asset.KindCleanImage, asset.KindMainImage, asset.KindSourceImage)
	case asset.KindSceneImage:
		return preferredBaseRecord(inventory, asset.KindSceneImage, asset.KindGalleryImage, asset.KindSourceImage, asset.KindMainImage)
	default:
		return preferredBaseRecord(inventory, asset.KindCleanImage, asset.KindMainImage, asset.KindSourceImage, asset.KindGalleryImage)
	}
}

func deferredRole(kind asset.Kind, purpose string) string {
	if purpose != "" {
		return purpose
	}
	switch kind {
	case asset.KindModelImage:
		return "model"
	case asset.KindSellingPointImage:
		return "selling_point"
	case asset.KindSizeSceneImage:
		return "size_scene"
	case asset.KindSceneImage:
		return "scene"
	case asset.KindDetailCrop:
		return "detail"
	default:
		return "generated"
	}
}
