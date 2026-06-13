package listingkit

import (
	"task-processor/internal/amazonlisting"
	common "task-processor/internal/publishing/common"
)

type AmazonPreviewPayload struct {
	Title          string                            `json:"title,omitempty"`
	Brand          string                            `json:"brand,omitempty"`
	ProductType    string                            `json:"product_type,omitempty"`
	ImageBundle    *common.PublishImageBundle        `json:"image_bundle,omitempty"`
	RenderPreviews *PlatformAssetRenderPreviews      `json:"render_previews,omitempty"`
	ScenePresets   []PlatformScenePresetSummary      `json:"scene_presets,omitempty"`
	Draft          *amazonlisting.AmazonListingDraft `json:"draft,omitempty"`
}
