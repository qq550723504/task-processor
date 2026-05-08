package generation

import "strings"

type ScenePresetSummary struct {
	PromptKey       string
	DefaultsSource  string
	SceneCategory   string
	SceneStyle      string
	BackgroundTone  string
	Composition     string
	PropsLevel      string
	AudienceHint    string
	CustomSceneHint string
}

func ScenePresetSummaryFromMetadata(metadata map[string]string) *ScenePresetSummary {
	if len(metadata) == 0 {
		return nil
	}
	summary := &ScenePresetSummary{
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
	if !HasScenePresetSummary(summary) {
		return nil
	}
	return summary
}

func HasScenePresetSummary(summary *ScenePresetSummary) bool {
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
