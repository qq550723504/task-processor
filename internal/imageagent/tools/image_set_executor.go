package tools

import (
	"context"
	"strings"
	"task-processor/internal/imageagent"
	productimage "task-processor/internal/product/image"
)

func (e *ProductImageSlotExecutor) resolveImageSetInput(input imageagent.SlotExecutionInput) (resolvedSlotInput, error) {
	if e.legacyV2 || input.RunID == "" || input.TenantID == "" || input.UserID == "" || input.PlanRevision <= 0 || input.Attempt <= 0 || input.IdempotencyKey == "" || input.Slot.IdempotencyKey == "" {
		return resolvedSlotInput{}, imageagent.ErrValidation
	}
	if err := imageagent.ValidateImageSetExecution(input); err != nil {
		return resolvedSlotInput{}, err
	}
	slot := cloneSlot(input.Slot)
	sources := make([]productimage.Asset, len(slot.SourceAssetIDs))
	for i, id := range slot.SourceAssetIDs {
		source, err := authorizedProductAsset(input.AssetCatalog, id, imageagent.AuthorizedAssetSource)
		if err != nil {
			return resolvedSlotInput{}, err
		}
		observed := slot.Recipe.References[i]
		if source.Width > 0 && (source.Width != observed.Width || source.Height != observed.Height) {
			return resolvedSlotInput{}, imageagent.ErrRevisionConflict
		}
		source.Width = observed.Width
		source.Height = observed.Height
		source.MediaType = observed.MediaType
		sources[i] = source
	}
	references := make([][]byte, len(input.SourceReferences))
	for i, data := range input.SourceReferences {
		references[i] = append([]byte(nil), data...)
	}
	product := productimage.ProductContext{ProductKey: strings.TrimSpace(input.ProductContext.ProductID), Title: strings.TrimSpace(input.ProductContext.Title), ProductType: strings.TrimSpace(input.ProductContext.ProductType), Attributes: cloneStringMap(input.ProductContext.Attributes)}
	if product.ProductKey == "" || product.ProductKey != input.ImageSet.Source.ProductID {
		return resolvedSlotInput{}, imageagent.ErrRevisionConflict
	}
	return resolvedSlotInput{slot: slot, source: sources[0], sources: sources, styles: sources[1:], product: product, sourceAssetID: sources[0].SourceAssetID, references: references}, nil
}

func (e *ProductImageSlotExecutor) generateSourceSet(ctx context.Context, input resolvedSlotInput, quoted *quotedSlotExecution) ([]productimage.Candidate, imageagent.SlotUsageReceipt, error) {
	if e.dependencies.SourceEditor == nil || quoted == nil {
		return nil, imageagent.SlotUsageReceipt{}, providerError(imageagent.ProviderRejectedBeforeEffect, imageagent.ErrValidation)
	}
	if err := imageagent.ValidateImageSourceBytes(input.slot.Recipe, input.references, productimage.MaxInlineArtifactBytes); err != nil {
		return nil, imageagent.SlotUsageReceipt{}, providerError(imageagent.ProviderRejectedBeforeEffect, err)
	}
	candidate, err := e.dependencies.SourceEditor.EditSources(ctx, productimage.SourceEditRequest{Sources: input.sources, ReferenceBytes: input.references, Product: input.product, Prompt: input.slot.Recipe.Prompt, PromptVersion: input.slot.Recipe.PromptVersion, Authorization: capabilityAuthorization(quoted, productimage.SourceEditOperation)})
	if err != nil {
		return nil, imageagent.SlotUsageReceipt{}, providerError(imageagent.ProviderDispatchedUnknown, err)
	}
	candidates := []productimage.Candidate{candidate}
	return candidates, receiptForQuote(quoted, 1, candidates), nil
}
