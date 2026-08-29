package domain

import (
	"strings"
)

type SceneGenerationOptions struct {
	SceneCategory     string   `json:"scene_category,omitempty"`
	SceneStyle        string   `json:"scene_style,omitempty"`
	BackgroundTone    string   `json:"background_tone,omitempty"`
	Composition       string   `json:"composition,omitempty"`
	PropsLevel        string   `json:"props_level,omitempty"`
	AudienceHint      string   `json:"audience_hint,omitempty"`
	CustomSceneHint   string   `json:"custom_scene_hint,omitempty"`
	SlotRole          string   `json:"slot_role,omitempty"`
	SlotBrief         string   `json:"slot_brief,omitempty"`
	StyleReferenceIDs []string `json:"style_reference_ids,omitempty"`
}

func (o *SceneGenerationOptions) Clone() *SceneGenerationOptions {
	if o == nil {
		return nil
	}
	cloned := *o
	cloned.StyleReferenceIDs = append([]string(nil), o.StyleReferenceIDs...)
	return &cloned
}

func (o *SceneGenerationOptions) IsEmpty() bool {
	return o == nil || strings.TrimSpace(o.SceneCategory) == "" &&
		strings.TrimSpace(o.SceneStyle) == "" &&
		strings.TrimSpace(o.BackgroundTone) == "" &&
		strings.TrimSpace(o.Composition) == "" &&
		strings.TrimSpace(o.PropsLevel) == "" &&
		strings.TrimSpace(o.AudienceHint) == "" &&
		strings.TrimSpace(o.CustomSceneHint) == "" &&
		strings.TrimSpace(o.SlotRole) == "" &&
		strings.TrimSpace(o.SlotBrief) == "" &&
		len(normalizedStyleReferenceIDs(o.StyleReferenceIDs)) == 0
}

func normalizedStyleReferenceIDs(ids []string) []string {
	seen := make(map[string]struct{}, len(ids))
	normalized := make([]string, 0, len(ids))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		normalized = append(normalized, id)
	}
	return normalized
}
