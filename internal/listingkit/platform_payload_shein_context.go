package listingkit

import (
	"task-processor/internal/asset"
	sheinpub "task-processor/internal/publishing/shein"
)

type sheinPlatformPayloadContext struct {
	pkg           *sheinpub.Package
	assetBundle   *asset.Bundle
	previewBase   platformVisualPreviewPayloadBase
	exportBase    platformVisualExportBase
	renderPreview *PlatformAssetRenderPreviews
}

func buildSheinPlatformPayloadContext(
	result *ListingKitResult,
	platformPreviews []PlatformAssetRenderPreviews,
) (*sheinPlatformPayloadContext, bool) {
	if result == nil || result.Shein == nil {
		return nil, false
	}
	sheinpub.NormalizePackageSemanticFields(result.Shein)
	context := buildPlatformPayloadResultContext(result, platformPreviews)
	return &sheinPlatformPayloadContext{
		pkg:           result.Shein,
		assetBundle:   context.assetBundle,
		previewBase:   context.previewVisualBase("shein", result.Shein.ImageBundle),
		exportBase:    context.exportVisualBase("shein", result.Shein.ImageBundle),
		renderPreview: context.previewRenderPreviews("shein"),
	}, true
}
