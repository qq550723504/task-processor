package listingkit

import (
	"strings"

	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

func buildPlatformScenePresetSummaries(bundle *common.PublishImageBundle, assetBundle *asset.Bundle) []PlatformScenePresetSummary {
	if bundle == nil || assetBundle == nil {
		return nil
	}
	summaries := make([]PlatformScenePresetSummary, 0, 8)
	appendSlot := func(slot *common.BundleSlot, fallbackSlot string) {
		if slot == nil {
			return
		}
		scenePreset := buildGenerationScenePresetSummary(assetBundle, strings.TrimSpace(slot.AssetID))
		if scenePreset == nil {
			return
		}
		summaries = append(summaries, PlatformScenePresetSummary{
			Slot:        firstNonEmpty(strings.TrimSpace(slot.Key), fallbackSlot),
			Purpose:     strings.TrimSpace(slot.Purpose),
			AssetID:     strings.TrimSpace(slot.AssetID),
			ScenePreset: scenePreset,
		})
	}
	appendSlot(bundle.Main, "main")
	for i := range bundle.Gallery {
		appendSlot(&bundle.Gallery[i], "gallery")
	}
	for i := range bundle.Auxiliary {
		appendSlot(&bundle.Auxiliary[i], "auxiliary")
	}
	if len(summaries) == 0 {
		return nil
	}
	return summaries
}
