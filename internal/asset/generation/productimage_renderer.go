package generation

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"strings"

	"task-processor/internal/asset"
	"task-processor/internal/productimage"
)

type productImageDeferredRenderer struct {
	renderer              productimage.SceneRenderer
	publisher             productimage.AssetPublisher
	cleanupTemporaryFiles bool
}

func NewProductImageDeferredRenderer(renderer productimage.SceneRenderer) DeferredRenderer {
	return NewProductImageDeferredRendererWithPublisher(renderer, nil)
}

func NewProductImageDeferredRendererWithPublisher(renderer productimage.SceneRenderer, publisher productimage.AssetPublisher) DeferredRenderer {
	return NewProductImageDeferredRendererWithPublisherAndCleanup(renderer, publisher, true)
}

// NewProductImageDeferredRendererWithPublisherAndCleanup wires the lifecycle
// policy into the renderer so cleanup cannot bypass the configured service
// behavior.
func NewProductImageDeferredRendererWithPublisherAndCleanup(renderer productimage.SceneRenderer, publisher productimage.AssetPublisher, cleanupTemporaryFiles bool) DeferredRenderer {
	if renderer == nil {
		return nil
	}
	return &productImageDeferredRenderer{
		renderer:              renderer,
		publisher:             publisher,
		cleanupTemporaryFiles: cleanupTemporaryFiles,
	}
}

func (r *productImageDeferredRenderer) Render(ctx context.Context, req DeferredRenderRequest) (*asset.AssetRecord, error) {
	if r == nil || r.renderer == nil {
		return nil, fmt.Errorf("scene renderer is not configured")
	}

	inputAsset := toProductImageAsset(req.BaseAsset)
	if inputAsset.Metadata == nil {
		inputAsset.Metadata = map[string]string{}
	}
	promotePublishedPath(inputAsset.Metadata)
	clearPublicationMetadata(inputAsset.Metadata)
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
	if r.publisher != nil {
		published, err := r.publish(ctx, req, selected)
		if err != nil {
			return nil, fmt.Errorf("publish deferred scene asset: %w", err)
		}
		selected = published
		if r.cleanupTemporaryFiles {
			productimage.CleanupTemporaryAsset(&selected)
		}
	}
	if err := requirePublicAssetURL(selected.URL); err != nil {
		return nil, err
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
	if value := sourceProvenanceURL(req.BaseAsset); value != "" {
		record.Metadata["source_url"] = value
	}
	return record, nil
}

func sourceProvenanceURL(record asset.AssetRecord) string {
	if value := strings.TrimSpace(record.Metadata["source_url"]); value != "" {
		return value
	}
	return strings.TrimSpace(record.URL)
}

func (r *productImageDeferredRenderer) publish(ctx context.Context, req DeferredRenderRequest, selected productimage.ImageAsset) (productimage.ImageAsset, error) {
	result := &productimage.ImageProcessResult{}
	switch selected.Type {
	case productimage.AssetTypeMainImage:
		result.MainImage = &selected
	case productimage.AssetTypeWhiteBgImage:
		result.WhiteBgImage = &selected
	case productimage.AssetTypeSubjectCutout:
		result.SubjectCutout = &selected
	default:
		result.GalleryImages = []productimage.ImageAsset{selected}
	}
	if err := r.publisher.Publish(ctx, &productimage.ImageProcessRequest{
		Text:           req.TaskID,
		TargetPlatform: req.Task.Platform,
	}, result); err != nil {
		return productimage.ImageAsset{}, err
	}
	switch selected.Type {
	case productimage.AssetTypeMainImage:
		if result.MainImage != nil {
			return *result.MainImage, nil
		}
	case productimage.AssetTypeWhiteBgImage:
		if result.WhiteBgImage != nil {
			return *result.WhiteBgImage, nil
		}
	case productimage.AssetTypeSubjectCutout:
		if result.SubjectCutout != nil {
			return *result.SubjectCutout, nil
		}
	default:
		if len(result.GalleryImages) > 0 {
			return result.GalleryImages[0], nil
		}
	}
	return selected, nil
}

func clearPublicationMetadata(metadata map[string]string) {
	for _, key := range []string{
		"published_url",
		"uploaded_url",
		"published_path",
		"published_key",
		"published_provider",
		"published_size_bytes",
	} {
		delete(metadata, key)
	}
}

func promotePublishedPath(metadata map[string]string) {
	if metadata == nil {
		return
	}
	publishedPath := strings.TrimSpace(metadata["published_path"])
	if publishedPath == "" {
		return
	}
	if _, err := os.Stat(publishedPath); err == nil {
		metadata["local_path"] = publishedPath
	}
}

func requirePublicAssetURL(value string) error {
	parsed, err := url.Parse(strings.TrimSpace(value))
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("deferred scene asset must have a public http(s) URL, got %q", value)
	}
	return nil
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
