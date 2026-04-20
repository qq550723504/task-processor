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
