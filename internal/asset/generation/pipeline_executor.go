package generation

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/asset"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog"
	"task-processor/internal/productimage"
)

func (s *service) executeWhiteBackground(ctx context.Context, req Request, idx int, item assetrecipe.AssetRecipe) (asset.AssetRecord, bool) {
	if s.whiteBackgroundRenderer == nil {
		return asset.AssetRecord{}, false
	}
	base, ok := preferredBaseRecord(req.Inventory, asset.KindMainImage, asset.KindSourceImage, asset.KindCleanImage)
	if !ok {
		return asset.AssetRecord{}, false
	}
	productAsset := toProductImageAsset(base)
	rendered, err := s.whiteBackgroundRenderer.Render(ctx, productAsset, buildProductContext(req.Product))
	if err != nil || rendered == nil {
		return asset.AssetRecord{}, false
	}
	return buildPipelineRecord(req.TaskID, idx, item, base, rendered, "white_bg"), true
}

func (s *service) executeSubjectCutout(ctx context.Context, req Request, idx int, item assetrecipe.AssetRecipe) (asset.AssetRecord, bool) {
	if s.subjectExtractor == nil {
		return asset.AssetRecord{}, false
	}
	base, ok := preferredBaseRecord(req.Inventory, asset.KindSourceImage, asset.KindMainImage, asset.KindCleanImage)
	if !ok {
		return asset.AssetRecord{}, false
	}
	imageURL := readableSourceURL(base)
	if strings.TrimSpace(imageURL) == "" {
		return asset.AssetRecord{}, false
	}
	extracted, err := s.subjectExtractor.Extract(ctx, imageURL, buildProductContext(req.Product))
	if err != nil || extracted == nil {
		return asset.AssetRecord{}, false
	}
	return buildPipelineRecord(req.TaskID, idx, item, base, extracted, "cutout"), true
}

func toProductImageAsset(record asset.AssetRecord) *productimage.ImageAsset {
	metadata := cloneMetadataMap(record.Metadata)
	return &productimage.ImageAsset{
		URL:        record.URL,
		Type:       assetKindToProductImageType(record.Kind),
		SourceURL:  readableSourceURL(record),
		Operations: append([]string(nil), record.Operations...),
		Width:      record.Width,
		Height:     record.Height,
		Metadata:   metadata,
	}
}

func buildProductContext(product *catalog.Product) *productimage.ProductContext {
	if product == nil {
		return nil
	}
	attrs := map[string]string{}
	for _, item := range product.Attributes {
		name := strings.TrimSpace(item.Name)
		value := strings.TrimSpace(item.Value)
		if name == "" || value == "" {
			continue
		}
		attrs[name] = value
	}
	productType := ""
	if len(product.CategoryPath) > 0 {
		productType = strings.TrimSpace(product.CategoryPath[len(product.CategoryPath)-1])
	}
	return &productimage.ProductContext{
		Title:       strings.TrimSpace(product.Title),
		ProductType: productType,
		Attributes:  attrs,
	}
}

func buildPipelineRecord(taskID string, idx int, item assetrecipe.AssetRecipe, base asset.AssetRecord, rendered *productimage.ImageAsset, role string) asset.AssetRecord {
	metadata := cloneMetadataMap(rendered.Metadata)
	if metadata == nil {
		metadata = map[string]string{}
	}
	metadata["execution_mode"] = "pipeline_backed"
	metadata["source_kind"] = string(base.Kind)
	if rendered.SourceURL != "" {
		metadata["source_url"] = rendered.SourceURL
	}
	return asset.AssetRecord{
		ID:         fmt.Sprintf("generated-%s-%d", strings.ReplaceAll(string(item.AssetKind), "_image", ""), idx+1),
		TaskID:     taskID,
		Kind:       item.AssetKind,
		Origin:     asset.OriginGenerated,
		Role:       role,
		URL:        rendered.URL,
		Generator:  "productimage_pipeline",
		RecipeID:   item.ID,
		Version:    &asset.AssetVersion{Number: 1, Label: "generated"},
		Lineage:    &asset.AssetLineage{ParentAssetIDs: []string{base.ID}, SourceAssetIDs: []string{base.ID}, Step: "productimage_pipeline"},
		Operations: append([]string(nil), rendered.Operations...),
		Labels:     []string{role},
		Width:      rendered.Width,
		Height:     rendered.Height,
		Metadata:   metadata,
	}
}

func readableSourceURL(record asset.AssetRecord) string {
	if value := strings.TrimSpace(record.Metadata["source_url"]); value != "" {
		return value
	}
	return strings.TrimSpace(record.URL)
}

func assetKindToProductImageType(kind asset.Kind) productimage.AssetType {
	switch kind {
	case asset.KindWhiteBgImage:
		return productimage.AssetTypeWhiteBgImage
	case asset.KindSubjectCutout:
		return productimage.AssetTypeSubjectCutout
	case asset.KindGalleryImage:
		return productimage.AssetTypeGalleryImage
	case asset.KindSourceImage:
		return productimage.AssetTypeSourceImage
	default:
		return productimage.AssetTypeMainImage
	}
}

func cloneMetadataMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
