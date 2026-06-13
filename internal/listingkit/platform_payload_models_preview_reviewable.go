package listingkit

import common "task-processor/internal/publishing/common"

type TemuPreviewPayload struct {
	Headline       string                       `json:"headline,omitempty"`
	NeedsReview    bool                         `json:"needs_review"`
	ReviewNotes    []string                     `json:"review_notes,omitempty"`
	ImageBundle    *common.PublishImageBundle   `json:"image_bundle,omitempty"`
	RenderPreviews *PlatformAssetRenderPreviews `json:"render_previews,omitempty"`
	ScenePresets   []PlatformScenePresetSummary `json:"scene_presets,omitempty"`
	Package        *TemuPackage                 `json:"package,omitempty"`
}

type WalmartPreviewPayload struct {
	Headline       string                       `json:"headline,omitempty"`
	NeedsReview    bool                         `json:"needs_review"`
	ReviewNotes    []string                     `json:"review_notes,omitempty"`
	ImageBundle    *common.PublishImageBundle   `json:"image_bundle,omitempty"`
	RenderPreviews *PlatformAssetRenderPreviews `json:"render_previews,omitempty"`
	ScenePresets   []PlatformScenePresetSummary `json:"scene_presets,omitempty"`
	Package        *WalmartPackage              `json:"package,omitempty"`
}
