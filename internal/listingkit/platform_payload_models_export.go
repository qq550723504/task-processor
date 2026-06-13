package listingkit

import (
	"task-processor/internal/amazonlisting"
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type PlatformScenePresetSummary struct {
	Slot        string                        `json:"slot,omitempty"`
	Purpose     string                        `json:"purpose,omitempty"`
	AssetID     string                        `json:"asset_id,omitempty"`
	ScenePreset *GenerationScenePresetSummary `json:"scene_preset,omitempty"`
}

type AmazonExportPayload struct {
	Draft          *amazonlisting.AmazonListingDraft `json:"draft,omitempty"`
	ImageBundle    *common.PublishImageBundle        `json:"image_bundle,omitempty"`
	RenderPreviews *PlatformAssetRenderPreviews      `json:"render_previews,omitempty"`
	ScenePresets   []PlatformScenePresetSummary      `json:"scene_presets,omitempty"`
}

type SheinExportPayload struct {
	Inspection     *sheinpub.Inspection         `json:"inspection,omitempty"`
	ImageBundle    *common.PublishImageBundle   `json:"image_bundle,omitempty"`
	RenderPreviews *PlatformAssetRenderPreviews `json:"render_previews,omitempty"`
	ScenePresets   []PlatformScenePresetSummary `json:"scene_presets,omitempty"`
	// Deprecated: kept only for export JSON compatibility. New business code should use DraftPayload.
	RequestDraft *sheinpub.RequestDraft `json:"request_draft,omitempty"`
	// DraftPayload is the canonical SHEIN draft payload exposed to internal export builders.
	DraftPayload *sheinpub.RequestDraft `json:"draft_payload,omitempty"`
	// Deprecated: kept only for export JSON compatibility. New business code should use PreviewPayload.
	PreviewProduct *sheinproduct.Product `json:"preview_product,omitempty"`
	// PreviewPayload is the canonical SHEIN preview payload exposed to internal export builders.
	PreviewPayload *sheinproduct.Product `json:"preview_payload,omitempty"`
	ReviewNotes    []string              `json:"review_notes,omitempty"`
}

type TemuExportPayload struct {
	ImageBundle    *common.PublishImageBundle   `json:"image_bundle,omitempty"`
	RenderPreviews *PlatformAssetRenderPreviews `json:"render_previews,omitempty"`
	ScenePresets   []PlatformScenePresetSummary `json:"scene_presets,omitempty"`
	Package        *TemuPackage                 `json:"package,omitempty"`
}

type WalmartExportPayload struct {
	ImageBundle    *common.PublishImageBundle   `json:"image_bundle,omitempty"`
	RenderPreviews *PlatformAssetRenderPreviews `json:"render_previews,omitempty"`
	ScenePresets   []PlatformScenePresetSummary `json:"scene_presets,omitempty"`
	Package        *WalmartPackage              `json:"package,omitempty"`
}
