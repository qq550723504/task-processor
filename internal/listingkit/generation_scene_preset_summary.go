package listingkit

import (
	"task-processor/internal/asset"
	listinggeneration "task-processor/internal/listingkit/generation"
)

type GenerationScenePresetSummary struct {
	PromptKey       string `json:"prompt_key,omitempty"`
	DefaultsSource  string `json:"defaults_source,omitempty"`
	SceneCategory   string `json:"scene_category,omitempty"`
	SceneStyle      string `json:"scene_style,omitempty"`
	BackgroundTone  string `json:"background_tone,omitempty"`
	Composition     string `json:"composition,omitempty"`
	PropsLevel      string `json:"props_level,omitempty"`
	AudienceHint    string `json:"audience_hint,omitempty"`
	CustomSceneHint string `json:"custom_scene_hint,omitempty"`
}

func buildGenerationScenePresetSummary(bundle *asset.Bundle, assetID string) *GenerationScenePresetSummary {
	if bundle == nil || assetID == "" {
		return nil
	}
	for _, item := range bundle.Assets {
		if item.ID != assetID {
			continue
		}
		return buildGenerationScenePresetSummaryFromMetadata(item.Metadata)
	}
	return nil
}

func buildGenerationScenePresetSummaryFromMetadata(metadata map[string]string) *GenerationScenePresetSummary {
	summary := listinggeneration.ScenePresetSummaryFromMetadata(metadata)
	if summary == nil {
		return nil
	}
	return &GenerationScenePresetSummary{
		PromptKey:       summary.PromptKey,
		DefaultsSource:  summary.DefaultsSource,
		SceneCategory:   summary.SceneCategory,
		SceneStyle:      summary.SceneStyle,
		BackgroundTone:  summary.BackgroundTone,
		Composition:     summary.Composition,
		PropsLevel:      summary.PropsLevel,
		AudienceHint:    summary.AudienceHint,
		CustomSceneHint: summary.CustomSceneHint,
	}
}
