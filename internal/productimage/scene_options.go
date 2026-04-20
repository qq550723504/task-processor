package productimage

import "strings"

type SceneGenerationOptions struct {
	SceneCategory   string `json:"scene_category,omitempty"`
	SceneStyle      string `json:"scene_style,omitempty"`
	BackgroundTone  string `json:"background_tone,omitempty"`
	Composition     string `json:"composition,omitempty"`
	PropsLevel      string `json:"props_level,omitempty"`
	AudienceHint    string `json:"audience_hint,omitempty"`
	CustomSceneHint string `json:"custom_scene_hint,omitempty"`
}

func (o *SceneGenerationOptions) Clone() *SceneGenerationOptions {
	if o == nil {
		return nil
	}
	cloned := *o
	return &cloned
}

func (o *SceneGenerationOptions) IsEmpty() bool {
	return o == nil || strings.TrimSpace(o.SceneCategory) == "" &&
		strings.TrimSpace(o.SceneStyle) == "" &&
		strings.TrimSpace(o.BackgroundTone) == "" &&
		strings.TrimSpace(o.Composition) == "" &&
		strings.TrimSpace(o.PropsLevel) == "" &&
		strings.TrimSpace(o.AudienceHint) == "" &&
		strings.TrimSpace(o.CustomSceneHint) == ""
}

func DefaultSceneGenerationOptionsForMarketplace(marketplace string) *SceneGenerationOptions {
	return resolveScenePreset(marketplace, "").Options
}

func MergeSceneGenerationOptions(base, override *SceneGenerationOptions) *SceneGenerationOptions {
	if base == nil && override == nil {
		return nil
	}
	if base == nil {
		return override.Clone()
	}

	merged := base.Clone()
	if override == nil {
		return merged
	}
	if value := strings.TrimSpace(override.SceneCategory); value != "" {
		merged.SceneCategory = value
	}
	if value := strings.TrimSpace(override.SceneStyle); value != "" {
		merged.SceneStyle = value
	}
	if value := strings.TrimSpace(override.BackgroundTone); value != "" {
		merged.BackgroundTone = value
	}
	if value := strings.TrimSpace(override.Composition); value != "" {
		merged.Composition = value
	}
	if value := strings.TrimSpace(override.PropsLevel); value != "" {
		merged.PropsLevel = value
	}
	if value := strings.TrimSpace(override.AudienceHint); value != "" {
		merged.AudienceHint = value
	}
	if value := strings.TrimSpace(override.CustomSceneHint); value != "" {
		merged.CustomSceneHint = value
	}
	return merged
}
