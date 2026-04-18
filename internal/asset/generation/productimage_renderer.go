package generation

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/productimage"
)

type productImageDeferredRenderer struct {
	renderer productimage.SceneRenderer
}

func NewProductImageDeferredRenderer(renderer productimage.SceneRenderer) DeferredRenderer {
	if renderer == nil {
		return nil
	}
	return &productImageDeferredRenderer{renderer: renderer}
}

func (r *productImageDeferredRenderer) Render(ctx context.Context, req DeferredRenderRequest) (*asset.AssetRecord, error) {
	if r == nil || r.renderer == nil {
		return nil, fmt.Errorf("scene renderer is not configured")
	}

	inputAsset := toProductImageAsset(req.BaseAsset)
	if inputAsset.Metadata == nil {
		inputAsset.Metadata = map[string]string{}
	}
	if value := strings.TrimSpace(req.Task.RenderProfile); value != "" {
		inputAsset.Metadata["render_profile"] = value
	}
	if value := strings.TrimSpace(req.Task.TemplateLabel); value != "" {
		inputAsset.Metadata["template_label"] = value
	}
	if value := strings.TrimSpace(req.Task.Slot); value != "" {
		inputAsset.Metadata["bundle_slot"] = value
	}
	if value := strings.TrimSpace(req.Task.Purpose); value != "" {
		inputAsset.Metadata["purpose"] = value
	}
	productimage.ApplyScenePresetMetadata(inputAsset.Metadata, req.Task.RenderProfile)

	rendered, err := r.renderer.Render(ctx, inputAsset, buildProductContext(req.Product))
	if err != nil {
		return nil, err
	}
	selected, ok := firstRenderableSceneAsset(rendered)
	if !ok {
		return nil, fmt.Errorf("scene renderer returned no assets")
	}

	record := &asset.AssetRecord{
		ID:         fmt.Sprintf("rendered-%s-%s", strings.ReplaceAll(string(req.Task.AssetKind), "_", "-"), req.BaseAsset.ID),
		TaskID:     req.TaskID,
		Kind:       req.Task.AssetKind,
		Origin:     asset.OriginGenerated,
		Role:       req.Task.Purpose,
		URL:        selected.URL,
		Generator:  "productimage_scene_renderer",
		RecipeID:   req.Task.RecipeID,
		Version:    &asset.AssetVersion{Number: 1, Label: "generated"},
		Lineage:    &asset.AssetLineage{ParentAssetIDs: []string{req.BaseAsset.ID}, SourceAssetIDs: []string{req.BaseAsset.ID}, Step: "productimage_scene_renderer"},
		Operations: append([]string(nil), selected.Operations...),
		Labels:     []string{req.Task.Purpose},
		Width:      selected.Width,
		Height:     selected.Height,
		Metadata:   cloneMetadataMap(selected.Metadata),
	}
	if record.Metadata == nil {
		record.Metadata = map[string]string{}
	}
	productimage.ApplyScenePresetMetadata(record.Metadata, req.Task.RenderProfile)
	productimage.ApplySellingPointContentPlanMetadata(record.Metadata, req.Task.RenderProfile, buildProductContext(req.Product))
	productimage.ApplySellingPointFillInputMetadata(record.Metadata, req.Task.RenderProfile, buildProductContext(req.Product))
	productimage.ApplySellingPointRenderBlocksMetadata(record.Metadata, req.Task.RenderProfile, buildProductContext(req.Product))
	productimage.ApplySellingPointRenderPlanMetadata(record.Metadata, req.Task.RenderProfile, buildProductContext(req.Product))
	productimage.ApplySellingPointRenderOutputMetadata(record.Metadata, req.Task.RenderProfile, buildProductContext(req.Product))
	productimage.ApplySellingPointDrawOutputMetadata(record.Metadata, req.Task.RenderProfile, buildProductContext(req.Product))
	productimage.ApplySellingPointDrawPreviewMetadata(record.Metadata, req.Task.RenderProfile, buildProductContext(req.Product))
	record.Metadata["execution_mode"] = ExecutionModeRendererBacked
	record.Metadata["source_kind"] = string(req.BaseAsset.Kind)
	if value := strings.TrimSpace(req.Task.RenderProfile); value != "" {
		record.Metadata["render_profile"] = value
	}
	if value := strings.TrimSpace(req.Task.TemplateLabel); value != "" {
		record.Metadata["template_label"] = value
	}
	if value := strings.TrimSpace(req.Task.Slot); value != "" {
		record.Metadata["bundle_slot"] = value
		record.Metadata["slot"] = value
	}
	if value := strings.TrimSpace(req.Task.Purpose); value != "" {
		record.Metadata["purpose"] = value
	}
	if value := strings.TrimSpace(selected.SourceURL); value != "" {
		record.Metadata["source_url"] = value
	} else if value := readableSourceURL(req.BaseAsset); value != "" {
		record.Metadata["source_url"] = value
	}
	return record, nil
}

func firstRenderableSceneAsset(items []productimage.ImageAsset) (productimage.ImageAsset, bool) {
	for _, item := range items {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		return item, true
	}
	return productimage.ImageAsset{}, false
}
