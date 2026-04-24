package generation

import (
	"fmt"
	"strings"

	"task-processor/internal/asset"
	assetrecipe "task-processor/internal/asset/recipe"
)

func plannedLineage(item assetrecipe.AssetRecipe) []string {
	return []string{item.Platform, item.ID}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func recipeAlreadySatisfied(inventory *asset.Inventory, item assetrecipe.AssetRecipe) bool {
	if inventory == nil {
		return false
	}
	kinds := preferredKinds(item)
	targetSlot := strings.ToLower(strings.TrimSpace(recipeSlot(item)))
	targetRecipeID := strings.TrimSpace(item.ID)
	if targetSlot != "" {
		for _, record := range inventory.Records {
			recordSlot := ""
			if record.Metadata != nil {
				recordSlot = firstNonEmpty(record.Metadata["bundle_slot"], record.Metadata["slot"])
			}
			if strings.ToLower(strings.TrimSpace(recordSlot)) != targetSlot {
				continue
			}
			if len(kinds) == 0 || recordMatchesAnyPreferredKind(record, kinds) {
				return true
			}
		}
	}
	if targetRecipeID != "" {
		for _, record := range inventory.Records {
			if strings.TrimSpace(record.RecipeID) != targetRecipeID {
				continue
			}
			if len(kinds) == 0 || recordMatchesAnyPreferredKind(record, kinds) {
				return true
			}
		}
	}
	if targetSlot != "" || targetRecipeID != "" {
		return false
	}
	for _, record := range inventory.Records {
		if recordMatchesAnyPreferredKind(record, kinds) {
			return true
		}
	}
	return false
}

func recordMatchesAnyPreferredKind(record asset.AssetRecord, kinds []asset.Kind) bool {
	if len(kinds) == 0 {
		return false
	}
	for _, kind := range kinds {
		if record.Kind == kind {
			return true
		}
	}
	return false
}

func preferredKinds(item assetrecipe.AssetRecipe) []asset.Kind {
	if item.Template != nil && len(item.Template.PreferredKinds) > 0 {
		return item.Template.PreferredKinds
	}
	if item.AssetKind != "" {
		return []asset.Kind{item.AssetKind}
	}
	return nil
}

func recipeSlot(item assetrecipe.AssetRecipe) string {
	if item.Template == nil {
		return ""
	}
	return item.Template.BundleSlot
}

func recipePurpose(item assetrecipe.AssetRecipe) string {
	if item.Template == nil {
		return ""
	}
	return item.Template.Purpose
}

func candidateSourceAssetIDs(inventory *asset.Inventory) []string {
	if inventory == nil {
		return nil
	}
	out := make([]string, 0, len(inventory.Records))
	for _, record := range inventory.Records {
		if record.Kind == asset.KindSourceImage || record.Kind == asset.KindMainImage || record.Kind == asset.KindCleanImage || record.Kind == asset.KindSubjectCutout {
			out = append(out, record.ID)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func executeNativeRecipe(taskID string, idx int, inventory *asset.Inventory, item assetrecipe.AssetRecipe) (asset.AssetRecord, bool) {
	switch item.AssetKind {
	case asset.KindCleanImage:
		base, ok := preferredBaseRecord(inventory, asset.KindMainImage, asset.KindSourceImage, asset.KindWhiteBgImage)
		if !ok {
			return asset.AssetRecord{}, false
		}
		return asset.AssetRecord{
			ID:        fmt.Sprintf("generated-clean-%d", idx+1),
			TaskID:    taskID,
			Kind:      asset.KindCleanImage,
			Origin:    asset.OriginGenerated,
			Role:      "clean",
			URL:       base.URL,
			Generator: "asset_generation_native",
			RecipeID:  item.ID,
			Version:   &asset.AssetVersion{Number: 1, Label: "generated"},
			Lineage:   &asset.AssetLineage{ParentAssetIDs: []string{base.ID}, SourceAssetIDs: []string{base.ID}, Step: "clean_alias"},
			Labels:    []string{"clean"},
			Metadata:  map[string]string{"execution_mode": "native_alias", "source_kind": string(base.Kind)},
		}, true
	case asset.KindWhiteBgImage:
		base, ok := preferredBaseRecord(inventory, asset.KindMainImage, asset.KindSourceImage, asset.KindCleanImage)
		if !ok {
			return asset.AssetRecord{}, false
		}
		return asset.AssetRecord{
			ID:        fmt.Sprintf("generated-white-bg-%d", idx+1),
			TaskID:    taskID,
			Kind:      asset.KindWhiteBgImage,
			Origin:    asset.OriginGenerated,
			Role:      "white_bg",
			URL:       base.URL,
			Generator: "asset_generation_native",
			RecipeID:  item.ID,
			Version:   &asset.AssetVersion{Number: 1, Label: "generated"},
			Lineage:   &asset.AssetLineage{ParentAssetIDs: []string{base.ID}, SourceAssetIDs: []string{base.ID}, Step: "white_bg_alias"},
			Labels:    []string{"white_bg"},
			Metadata:  map[string]string{"execution_mode": "native_alias", "source_kind": string(base.Kind)},
		}, true
	case asset.KindSubjectCutout:
		base, ok := preferredBaseRecord(inventory, asset.KindCleanImage, asset.KindMainImage, asset.KindSourceImage)
		if !ok {
			return asset.AssetRecord{}, false
		}
		return asset.AssetRecord{
			ID:        fmt.Sprintf("generated-cutout-%d", idx+1),
			TaskID:    taskID,
			Kind:      asset.KindSubjectCutout,
			Origin:    asset.OriginGenerated,
			Role:      "cutout",
			URL:       base.URL,
			Generator: "asset_generation_native",
			RecipeID:  item.ID,
			Version:   &asset.AssetVersion{Number: 1, Label: "generated"},
			Lineage:   &asset.AssetLineage{ParentAssetIDs: []string{base.ID}, SourceAssetIDs: []string{base.ID}, Step: "subject_cutout_alias"},
			Labels:    []string{"cutout"},
			Metadata:  map[string]string{"execution_mode": "native_alias", "source_kind": string(base.Kind)},
		}, true
	default:
		return asset.AssetRecord{}, false
	}
}

func preferredBaseRecord(inventory *asset.Inventory, kinds ...asset.Kind) (asset.AssetRecord, bool) {
	if inventory == nil {
		return asset.AssetRecord{}, false
	}
	for _, kind := range kinds {
		for _, record := range inventory.Records {
			if record.Kind == kind {
				return record, true
			}
		}
	}
	return asset.AssetRecord{}, false
}

func sourceAssetIDsForRecord(record asset.AssetRecord) []string {
	if record.Lineage == nil {
		return nil
	}
	return append([]string(nil), record.Lineage.SourceAssetIDs...)
}
