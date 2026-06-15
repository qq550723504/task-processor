package listingkit

import (
	"strings"

	"task-processor/internal/asset"
	common "task-processor/internal/publishing/common"
)

func buildPlatformAssetRenderPreviewGroup(platform string, previewByAssetID map[string]AssetRenderPreview, assetURLByID map[string]string, assetBundle *asset.Bundle, bundle *common.PublishImageBundle) (PlatformAssetRenderPreviews, bool) {
	if bundle == nil {
		return PlatformAssetRenderPreviews{}, false
	}
	group := PlatformAssetRenderPreviews{Platform: platform}
	group.Main = buildAssetRenderPreviewSlot("main", bundle.Main, previewByAssetID, assetURLByID, assetBundle)
	group.Gallery = buildAssetRenderPreviewSlots("gallery", bundle.Gallery, previewByAssetID, assetURLByID, assetBundle)
	group.Auxiliary = buildAssetRenderPreviewSlots("auxiliary", bundle.Auxiliary, previewByAssetID, assetURLByID, assetBundle)
	if group.Main == nil && len(group.Gallery) == 0 && len(group.Auxiliary) == 0 {
		return PlatformAssetRenderPreviews{}, false
	}
	group.Summary = buildPlatformAssetRenderPreviewSummary(group)
	return group, true
}

func buildAssetRenderPreviewSlots(defaultSlot string, slots []common.BundleSlot, previewByAssetID map[string]AssetRenderPreview, assetURLByID map[string]string, assetBundle *asset.Bundle) []AssetRenderPreviewSlot {
	if len(slots) == 0 {
		return nil
	}
	out := make([]AssetRenderPreviewSlot, 0, len(slots))
	for _, slot := range slots {
		built := buildAssetRenderPreviewSlot(defaultSlot, &slot, previewByAssetID, assetURLByID, assetBundle)
		if built != nil {
			out = append(out, *built)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func buildAssetRenderPreviewSlot(defaultSlot string, slot *common.BundleSlot, previewByAssetID map[string]AssetRenderPreview, assetURLByID map[string]string, assetBundle *asset.Bundle) *AssetRenderPreviewSlot {
	if slot == nil || slot.AssetID == "" {
		return nil
	}
	preview, hasPreview := previewByAssetID[slot.AssetID]
	assetURL := ""
	if assetURLByID != nil {
		assetURL = assetURLByID[slot.AssetID]
		if assetURL == "" {
			assetURL = assetURLByID[strings.TrimSpace(slot.URL)]
		}
	}
	resolvedPublishedURL := publishedAssetURLForBundleSlot(slot, assetURLByID, assetBundle)
	if resolvedPublishedURL != "" && shouldReplaceAssetURL(assetURL, resolvedPublishedURL) {
		assetURL = resolvedPublishedURL
	}
	if !hasPreview && assetURL == "" {
		return nil
	}
	slotKey := slot.Key
	if slotKey == "" {
		slotKey = defaultSlot
	}
	return &AssetRenderPreviewSlot{
		Slot:                slotKey,
		Purpose:             slot.Purpose,
		AssetID:             slot.AssetID,
		AssetURL:            assetURL,
		AssetRevision:       preview.AssetRevision,
		PreviewRevision:     preview.PreviewRevision,
		TaskRevision:        preview.TaskRevision,
		Kind:                slot.Kind,
		Role:                preview.Role,
		StateLabel:          slot.StateLabel,
		RetryHint:           slot.RetryHint,
		TemplateLabel:       firstNonEmpty(slot.TemplateLabel, preview.TemplateLabel),
		RenderProfile:       preview.RenderProfile,
		PreviewFormat:       preview.PreviewFormat,
		PreviewSVG:          preview.PreviewSVG,
		SourceKind:          preview.SourceKind,
		GenerationMode:      preview.GenerationMode,
		VisualMode:          preview.VisualMode,
		LayoutEngine:        preview.LayoutEngine,
		RenderOutputVersion: preview.RenderOutputVersion,
		DrawOutputVersion:   preview.DrawOutputVersion,
		DrawPreviewVersion:  preview.DrawPreviewVersion,
		LayerTypes:          append([]string(nil), preview.LayerTypes...),
		Regions:             append([]string(nil), preview.Regions...),
		StyleTokens:         append([]string(nil), preview.StyleTokens...),
	}
}

func imageBundleFromAmazon(pkg *AmazonPackage) *common.PublishImageBundle {
	if pkg == nil {
		return nil
	}
	return pkg.ImageBundle
}

func imageBundleFromShein(pkg *SheinPackage) *common.PublishImageBundle {
	if pkg == nil {
		return nil
	}
	return pkg.ImageBundle
}

func imageBundleFromTemu(pkg *TemuPackage) *common.PublishImageBundle {
	if pkg == nil {
		return nil
	}
	return pkg.ImageBundle
}

func imageBundleFromWalmart(pkg *WalmartPackage) *common.PublishImageBundle {
	if pkg == nil {
		return nil
	}
	return pkg.ImageBundle
}
