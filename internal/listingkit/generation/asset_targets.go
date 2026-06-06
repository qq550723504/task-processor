package generation

import (
	"strings"

	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
)

type TargetKey struct {
	RecipeID string
	Slot     string
}

func TaskRetryable(task assetgeneration.Task) bool {
	if !task.CanExecute {
		return false
	}
	if task.ExecutionStatus != "completed" {
		return true
	}
	switch task.ExecutionMode {
	case assetgeneration.ExecutionModeDeferredStub, assetgeneration.ExecutionModeRendererBacked:
		return true
	default:
		return false
	}
}

func TaskTargets(tasks []assetgeneration.Task) map[TargetKey]struct{} {
	if len(tasks) == 0 {
		return nil
	}
	out := make(map[TargetKey]struct{}, len(tasks))
	for _, item := range tasks {
		recipeID := strings.TrimSpace(item.RecipeID)
		slot := strings.ToLower(strings.TrimSpace(item.Slot))
		if recipeID == "" {
			continue
		}
		out[TargetKey{RecipeID: recipeID, Slot: slot}] = struct{}{}
	}
	return out
}

func ReplaceGeneratedAssetsForTargets(existing []asset.AssetRecord, targets map[TargetKey]struct{}, updates []asset.AssetRecord) []asset.AssetRecord {
	if len(targets) == 0 {
		return append(append([]asset.AssetRecord(nil), existing...), updates...)
	}
	out := make([]asset.AssetRecord, 0, len(existing)+len(updates))
	for _, item := range existing {
		if item.Origin == asset.OriginGenerated {
			if _, ok := targets[assetTargetKey(item)]; ok {
				continue
			}
		}
		out = append(out, item)
	}
	out = append(out, updates...)
	return out
}

func assetTargetKey(item asset.AssetRecord) TargetKey {
	slot := ""
	if item.Metadata != nil {
		slot = firstNonEmpty(item.Metadata["bundle_slot"], item.Metadata["slot"])
	}
	return TargetKey{
		RecipeID: strings.TrimSpace(item.RecipeID),
		Slot:     strings.ToLower(strings.TrimSpace(slot)),
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
