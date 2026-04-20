package listingkit

import (
	"strings"

	"task-processor/internal/asset"
)

type GenerationScenePresetSummary struct {
	PromptKey         string `json:"prompt_key,omitempty"`
	DefaultsSource    string `json:"defaults_source,omitempty"`
	SceneCategory     string `json:"scene_category,omitempty"`
	SceneStyle        string `json:"scene_style,omitempty"`
	BackgroundTone    string `json:"background_tone,omitempty"`
	Composition       string `json:"composition,omitempty"`
	PropsLevel        string `json:"props_level,omitempty"`
	AudienceHint      string `json:"audience_hint,omitempty"`
	CustomSceneHint   string `json:"custom_scene_hint,omitempty"`
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
	if len(metadata) == 0 {
		return nil
	}
	summary := &GenerationScenePresetSummary{
		PromptKey:       strings.TrimSpace(metadata["prompt_key"]),
		DefaultsSource:  strings.TrimSpace(metadata["scene_defaults_source"]),
		SceneCategory:   strings.TrimSpace(metadata["scene_category"]),
		SceneStyle:      strings.TrimSpace(metadata["scene_style"]),
		BackgroundTone:  strings.TrimSpace(metadata["background_tone"]),
		Composition:     strings.TrimSpace(metadata["composition"]),
		PropsLevel:      strings.TrimSpace(metadata["props_level"]),
		AudienceHint:    strings.TrimSpace(metadata["audience_hint"]),
		CustomSceneHint: strings.TrimSpace(metadata["custom_scene_hint"]),
	}
	if !hasGenerationScenePresetSummary(summary) {
		return nil
	}
	return summary
}

func hasGenerationScenePresetSummary(summary *GenerationScenePresetSummary) bool {
	if summary == nil {
		return false
	}
	if strings.HasPrefix(summary.PromptKey, "productimage.scene.") {
		return true
	}
	return summary.DefaultsSource != "" ||
		summary.SceneCategory != "" ||
		summary.SceneStyle != "" ||
		summary.BackgroundTone != "" ||
		summary.Composition != "" ||
		summary.PropsLevel != "" ||
		summary.AudienceHint != "" ||
		summary.CustomSceneHint != ""
}

func cloneGenerationScenePresetSummary(summary *GenerationScenePresetSummary) *GenerationScenePresetSummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	return &cloned
}
