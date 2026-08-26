package tools

import (
	"context"
	"fmt"
	"strings"

	"task-processor/internal/imageagent"
	productimage "task-processor/internal/productimage"
)

// Dependencies are the ProductImage capabilities and explicitly authorized
// inputs available to a single image-agent slot execution.
type Dependencies struct {
	SubjectExtractor            productimage.SubjectExtractor
	WhiteBackgroundRenderer     productimage.WhiteBackgroundRenderer
	SceneRenderer               productimage.SceneRenderer
	ProductContext              *productimage.ProductContext
	SourceAssets                map[string]productimage.ImageAsset
	AuthorizedStyleReferenceIDs map[string]struct{}
}

// ProductImageSlotExecutor adapts ProductImage capabilities to the
// imageagent one-slot execution contract.
type ProductImageSlotExecutor struct {
	dependencies Dependencies
}

func NewProductImageSlotExecutor(dependencies Dependencies) *ProductImageSlotExecutor {
	return &ProductImageSlotExecutor{dependencies: dependencies}
}

func (e *ProductImageSlotExecutor) ExecuteSlot(ctx context.Context, input imageagent.SlotExecutionInput) (imageagent.SlotExecutionResult, error) {
	slot, sourceAssetID, source, err := e.validateAndResolve(input)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}

	productContext := productContextForSlot(e.dependencies.ProductContext, slot, e.authorizedStyleReferences(slot.StyleReferenceIDs))
	var assets []productimage.ImageAsset
	switch slot.Role {
	case imageagent.SlotRoleMain:
		assets, err = e.executeMain(ctx, source, productContext)
	case imageagent.SlotRoleScene, imageagent.SlotRoleDetail, imageagent.SlotRoleSellingPoint:
		assets, err = e.executeScene(ctx, source, productContext)
	case imageagent.SlotRoleSize:
		if source.Width <= 0 || source.Height <= 0 {
			return imageagent.SlotExecutionResult{}, fmt.Errorf("size slot %q requires reliable dimensions", slot.ID)
		}
		assets, err = e.executeScene(ctx, source, productContext)
	default:
		return imageagent.SlotExecutionResult{}, fmt.Errorf("slot %q has unsupported role %q", slot.ID, slot.Role)
	}
	if err != nil {
		return imageagent.SlotExecutionResult{}, fmt.Errorf("execute slot %q: %w", slot.ID, err)
	}

	candidates, err := generatedCandidates(slot.ID, sourceAssetID, source, assets)
	if err != nil {
		return imageagent.SlotExecutionResult{}, err
	}
	return imageagent.SlotExecutionResult{SlotID: slot.ID, Attempt: input.Attempt, Candidates: candidates}, nil
}

func (e *ProductImageSlotExecutor) validateAndResolve(input imageagent.SlotExecutionInput) (imageagent.Slot, string, productimage.ImageAsset, error) {
	slot := input.Slot
	slot.ID = strings.TrimSpace(slot.ID)
	slot.Brief = strings.TrimSpace(slot.Brief)
	if slot.ID == "" {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("slot ID is required")
	}
	if input.Attempt <= 0 {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("slot %q requires a positive attempt", slot.ID)
	}
	for i := range slot.SourceAssetIDs {
		slot.SourceAssetIDs[i] = strings.TrimSpace(slot.SourceAssetIDs[i])
	}
	var sourceAssetID string
	for _, id := range slot.SourceAssetIDs {
		if id != "" {
			sourceAssetID = id
			break
		}
	}
	if sourceAssetID == "" {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("slot %q requires a source asset", slot.ID)
	}
	source, ok := e.dependencies.SourceAssets[sourceAssetID]
	if !ok {
		source = productimage.ImageAsset{URL: sourceAssetID, SourceURL: sourceAssetID, Type: productimage.AssetTypeSourceImage}
	}
	source.URL = strings.TrimSpace(source.URL)
	source.SourceURL = strings.TrimSpace(source.SourceURL)
	if source.URL == "" {
		return imageagent.Slot{}, "", productimage.ImageAsset{}, fmt.Errorf("source asset %q has no readable URL", sourceAssetID)
	}
	if source.SourceURL == "" {
		source.SourceURL = source.URL
	}
	return slot, sourceAssetID, source, nil
}

func (e *ProductImageSlotExecutor) executeMain(ctx context.Context, source productimage.ImageAsset, productContext *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	if e.dependencies.SubjectExtractor == nil {
		return nil, fmt.Errorf("subject extractor is required for main slot")
	}
	if e.dependencies.WhiteBackgroundRenderer == nil {
		return nil, fmt.Errorf("white background renderer is required for main slot")
	}
	subject, err := e.dependencies.SubjectExtractor.Extract(ctx, source.URL, productContext)
	if err != nil {
		return nil, fmt.Errorf("extract subject: %w", err)
	}
	if subject == nil || strings.TrimSpace(subject.URL) == "" {
		return nil, fmt.Errorf("subject extractor returned no generated asset")
	}
	main, err := e.dependencies.WhiteBackgroundRenderer.Render(ctx, subject, productContext)
	if err != nil {
		return nil, fmt.Errorf("render white background: %w", err)
	}
	if main == nil {
		return nil, fmt.Errorf("white background renderer returned no generated asset")
	}
	return []productimage.ImageAsset{*main}, nil
}

func (e *ProductImageSlotExecutor) executeScene(ctx context.Context, source productimage.ImageAsset, productContext *productimage.ProductContext) ([]productimage.ImageAsset, error) {
	if e.dependencies.SceneRenderer == nil {
		return nil, fmt.Errorf("scene renderer is required")
	}
	return e.dependencies.SceneRenderer.Render(ctx, &source, productContext)
}

func (e *ProductImageSlotExecutor) authorizedStyleReferences(ids []string) []string {
	if len(e.dependencies.AuthorizedStyleReferenceIDs) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(ids))
	authorized := make([]string, 0, len(ids))
	for _, rawID := range ids {
		id := strings.TrimSpace(rawID)
		if id == "" {
			continue
		}
		if _, allowed := e.dependencies.AuthorizedStyleReferenceIDs[id]; !allowed {
			continue
		}
		if _, duplicate := seen[id]; duplicate {
			continue
		}
		seen[id] = struct{}{}
		authorized = append(authorized, id)
	}
	return authorized
}

func productContextForSlot(base *productimage.ProductContext, slot imageagent.Slot, styleReferenceIDs []string) *productimage.ProductContext {
	context := cloneProductContext(base)
	return productimage.ApplySceneOptionsToProductContext(context, &productimage.ImageProcessRequest{Scene: &productimage.SceneGenerationOptions{
		SlotRole: slotRoleName(slot.Role), SlotBrief: slot.Brief, StyleReferenceIDs: styleReferenceIDs,
	}})
}

func cloneProductContext(base *productimage.ProductContext) *productimage.ProductContext {
	if base == nil {
		return &productimage.ProductContext{}
	}
	cloned := *base
	if base.Attributes != nil {
		cloned.Attributes = make(map[string]string, len(base.Attributes))
		for key, value := range base.Attributes {
			cloned.Attributes[key] = value
		}
	}
	return &cloned
}

func slotRoleName(role imageagent.SlotRole) string {
	return string(role)
}

func generatedCandidates(slotID, sourceAssetID string, source productimage.ImageAsset, assets []productimage.ImageAsset) ([]imageagent.AssetCandidate, error) {
	candidates := make([]imageagent.AssetCandidate, 0, len(assets))
	for _, asset := range assets {
		url := strings.TrimSpace(asset.URL)
		if !isGeneratedAsset(url, source, asset) {
			continue
		}
		candidates = append(candidates, imageagent.AssetCandidate{
			AssetID:       fmt.Sprintf("%s-candidate-%d", slotID, len(candidates)+1),
			URL:           url,
			SourceAssetID: sourceAssetID,
			Metadata:      cloneMetadata(asset.Metadata),
		})
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("slot %q provider returned no generated candidates", slotID)
	}
	return candidates, nil
}

func isGeneratedAsset(url string, source productimage.ImageAsset, asset productimage.ImageAsset) bool {
	if url == "" || url == strings.TrimSpace(source.URL) || url == strings.TrimSpace(source.SourceURL) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(asset.Metadata["scene_mode"]), "local_canvas") ||
		strings.EqualFold(strings.TrimSpace(asset.Metadata["background_mode"]), "white_canvas") {
		return false
	}
	for _, operation := range asset.Operations {
		if strings.EqualFold(strings.TrimSpace(operation), "render_white_bg_placeholder") {
			return false
		}
	}
	return true
}

func cloneMetadata(metadata map[string]string) map[string]string {
	if metadata == nil {
		return nil
	}
	cloned := make(map[string]string, len(metadata))
	for key, value := range metadata {
		cloned[key] = value
	}
	return cloned
}

var _ imageagent.SlotExecutor = (*ProductImageSlotExecutor)(nil)
