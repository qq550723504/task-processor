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
	preferredKinds := deferredBaseKinds(task.AssetKind)
	if len(task.SourceAssetIDs) > 0 {
		if record, ok := preferredRecordByIDs(inventory, task.SourceAssetIDs, preferredKinds); ok {
			return record, true
		}
	}
	return preferredBaseRecord(inventory, preferredKinds...)
}

func deferredBaseKinds(kind asset.Kind) []asset.Kind {
	switch kind {
	case asset.KindModelImage:
		return []asset.Kind{asset.KindCleanImage, asset.KindMainImage, asset.KindSubjectCutout, asset.KindSourceImage}
	case asset.KindSellingPointImage, asset.KindSizeSceneImage, asset.KindDetailCrop:
		return []asset.Kind{asset.KindGalleryImage, asset.KindCleanImage, asset.KindMainImage, asset.KindSubjectCutout, asset.KindSourceImage}
	case asset.KindSceneImage:
		return []asset.Kind{asset.KindSceneImage, asset.KindGalleryImage, asset.KindSourceImage, asset.KindMainImage, asset.KindSubjectCutout}
	default:
		return []asset.Kind{asset.KindCleanImage, asset.KindMainImage, asset.KindSubjectCutout, asset.KindSourceImage, asset.KindGalleryImage}
	}
}

func preferredRecordByIDs(inventory *asset.Inventory, ids []string, kinds []asset.Kind) (asset.AssetRecord, bool) {
	if inventory == nil || len(ids) == 0 || len(kinds) == 0 {
		return asset.AssetRecord{}, false
	}
	allowed := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		allowed[id] = struct{}{}
	}
	for _, kind := range kinds {
		for _, record := range inventory.Records {
			if record.Kind != kind {
				continue
			}
			if _, ok := allowed[record.ID]; ok {
				return record, true
			}
		}
	}
	return asset.AssetRecord{}, false
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
