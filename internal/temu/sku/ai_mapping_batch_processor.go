package sku

import (
	"fmt"

	"task-processor/internal/model"
	temucontext "task-processor/internal/temu/context"
)

// generateAISkuMappingInBatches splits large variant sets into smaller AI requests
// and merges the generated SKU mappings back into one response.
func (vp *SkuVariantProcessor) generateAISkuMappingInBatches(
	temuCtx *temucontext.TemuTaskContext,
	variants []*model.Product,
	batchSize int,
) (*temucontext.AISkuMappingResponse, error) {
	input, totalBatches, err := vp.prepareAIMappingBatches(variants, batchSize)
	if err != nil {
		return nil, err
	}

	mergedResponse, err := vp.processAllAIMappingBatches(temuCtx, input, totalBatches)
	if err != nil {
		return nil, err
	}

	if err := vp.finalizeMergedAIMapping(mergedResponse); err != nil {
		return nil, err
	}

	return mergedResponse, nil
}

func (vp *SkuVariantProcessor) prepareAIMappingBatches(
	variants []*model.Product,
	batchSize int,
) (*AIBatchInput, int, error) {
	input, err := buildAIBatchInput(variants, batchSize)
	if err != nil {
		return nil, 0, err
	}

	totalBatches := input.TotalBatches()
	vp.logger.Infof(
		"start generating AI SKU mapping in batches: variants=%d, batch_size=%d, batches=%d",
		len(input.Variants), input.BatchSize, totalBatches,
	)

	return input, totalBatches, nil
}

func (vp *SkuVariantProcessor) processAllAIMappingBatches(
	temuCtx *temucontext.TemuTaskContext,
	input *AIBatchInput,
	totalBatches int,
) (*temucontext.AISkuMappingResponse, error) {
	mergedResponse := temucontext.NewEmptyAISkuMappingResponse()
	for batchIndex := 0; batchIndex < totalBatches; batchIndex++ {
		if err := vp.processAndMergeAIMappingBatch(temuCtx, input, mergedResponse, batchIndex, totalBatches); err != nil {
			return nil, err
		}
	}

	return mergedResponse, nil
}

func (vp *SkuVariantProcessor) finalizeMergedAIMapping(aiMapping *temucontext.AISkuMappingResponse) error {
	vp.logger.Infof("all AI mapping batches completed: generated_skus=%d", aiMapping.SkuCount())
	return vp.normalizeMergedAIMapping(aiMapping)
}

func (vp *SkuVariantProcessor) normalizeMergedAIMapping(aiMapping *temucontext.AISkuMappingResponse) error {
	if err := vp.unifyAIMappingSpecDimensions(aiMapping); err != nil {
		vp.logger.Errorf("failed to unify spec dimensions after batch merge: %v", err)
		return fmt.Errorf("unify spec dimensions after batch merge: %w", err)
	}

	vp.logger.Info("enforcing spec count limit on merged AI mapping result")
	vp.enforceSpecCountLimit(aiMapping)

	return nil
}

func (vp *SkuVariantProcessor) processAndMergeAIMappingBatch(
	temuCtx *temucontext.TemuTaskContext,
	input *AIBatchInput,
	mergedResponse *temucontext.AISkuMappingResponse,
	batchIndex int,
	totalBatches int,
) error {
	batchResponse, err := vp.processAIMappingBatch(temuCtx, input, batchIndex, totalBatches)
	if err != nil {
		return err
	}

	vp.handleProcessedBatchResponse(mergedResponse, batchResponse, batchIndex, totalBatches)
	return nil
}

func (vp *SkuVariantProcessor) processAIMappingBatch(
	temuCtx *temucontext.TemuTaskContext,
	input *AIBatchInput,
	batchIndex int,
	totalBatches int,
) (*temucontext.AISkuMappingResponse, error) {
	batchVariants, err := vp.prepareAIMappingBatch(input, batchIndex, totalBatches)
	if err != nil {
		return nil, err
	}

	return vp.executeAIMappingBatch(temuCtx, batchVariants, batchIndex, totalBatches)
}

func (vp *SkuVariantProcessor) prepareAIMappingBatch(
	input *AIBatchInput,
	batchIndex int,
	totalBatches int,
) ([]*model.Product, error) {
	batchVariants, start, end, ok := input.BatchVariants(batchIndex)
	if !ok {
		return nil, fmt.Errorf("invalid ai batch index: %d", batchIndex)
	}

	vp.logger.Infof(
		"processing AI mapping batch %d/%d: variants[%d-%d]",
		batchIndex+1, totalBatches, start, end-1,
	)

	return batchVariants, nil
}

func (vp *SkuVariantProcessor) executeAIMappingBatch(
	temuCtx *temucontext.TemuTaskContext,
	batchVariants []*model.Product,
	batchIndex int,
	totalBatches int,
) (*temucontext.AISkuMappingResponse, error) {
	batchResponse, err := vp.GenerateAISkuMappingSingleBatch(temuCtx, batchVariants)
	if err != nil {
		vp.logger.Errorf("AI mapping batch %d/%d failed: %v", batchIndex+1, totalBatches, err)
		return nil, fmt.Errorf("AI mapping batch %d failed: %w", batchIndex+1, err)
	}

	return batchResponse, nil
}

func (vp *SkuVariantProcessor) logFirstBatchSpecDimensions(batchResponse *temucontext.AISkuMappingResponse) {
	selectedSpecDimensions := batchResponse.FirstSpecDimensions()
	if len(selectedSpecDimensions) > 0 {
		vp.logger.Infof("selected spec dimensions from first batch: %v", selectedSpecDimensions)
	}
}

func (vp *SkuVariantProcessor) handleProcessedBatchResponse(
	mergedResponse *temucontext.AISkuMappingResponse,
	batchResponse *temucontext.AISkuMappingResponse,
	batchIndex int,
	totalBatches int,
) {
	if batchIndex == 0 {
		vp.logFirstBatchSpecDimensions(batchResponse)
	}

	vp.appendBatchResponse(mergedResponse, batchResponse, batchIndex, totalBatches)
}

func (vp *SkuVariantProcessor) appendBatchResponse(
	mergedResponse *temucontext.AISkuMappingResponse,
	batchResponse *temucontext.AISkuMappingResponse,
	batchIndex int,
	totalBatches int,
) {
	mergedResponse.AppendResponse(batchResponse)
	vp.logger.Infof(
		"AI mapping batch %d/%d completed: generated_skus=%d",
		batchIndex+1, totalBatches, batchResponse.SkuCount(),
	)
}
