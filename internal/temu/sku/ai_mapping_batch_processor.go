package sku

import (
	"fmt"

	"task-processor/internal/model"
	temucontext "task-processor/internal/temu/context"
	"task-processor/internal/temu/spec"
)

// generateAISkuMappingInBatches splits large variant sets into smaller AI requests
// and merges the generated SKU mappings back into one response.
func (vp *SkuVariantProcessor) generateAISkuMappingInBatches(
	temuCtx *temucontext.TemuTaskContext,
	variants []*model.Product,
	batchSize int,
) (*temucontext.AISkuMappingResponse, error) {
	input, err := buildAIBatchInput(variants, batchSize)
	if err != nil {
		return nil, err
	}

	totalBatches := input.TotalBatches()
	vp.logger.Infof(
		"start generating AI SKU mapping in batches: variants=%d, batch_size=%d, batches=%d",
		len(input.Variants), input.BatchSize, totalBatches,
	)

	mergedResponse := &temucontext.AISkuMappingResponse{}
	var selectedSpecDimensions []string

	for batchIndex := 0; batchIndex < totalBatches; batchIndex++ {
		batchVariants, start, end, ok := input.BatchVariants(batchIndex)
		if !ok {
			return nil, fmt.Errorf("invalid ai batch index: %d", batchIndex)
		}
		vp.logger.Infof(
			"processing AI mapping batch %d/%d: variants[%d-%d]",
			batchIndex+1, totalBatches, start, end-1,
		)

		batchResponse, err := vp.GenerateAISkuMappingSingleBatch(temuCtx, batchVariants)
		if err != nil {
			vp.logger.Errorf("AI mapping batch %d/%d failed: %v", batchIndex+1, totalBatches, err)
			return nil, fmt.Errorf("AI mapping batch %d failed: %w", batchIndex+1, err)
		}

		if batchIndex == 0 {
			selectedSpecDimensions = batchResponse.FirstSpecDimensions()
			if len(selectedSpecDimensions) > 0 {
				vp.logger.Infof("selected spec dimensions from first batch: %v", selectedSpecDimensions)
			}
		}

		mergedResponse.AppendResponse(batchResponse)
		vp.logger.Infof(
			"AI mapping batch %d/%d completed: generated_skus=%d",
			batchIndex+1, totalBatches, batchResponse.SkuCount(),
		)
	}

	vp.logger.Infof("all AI mapping batches completed: generated_skus=%d", mergedResponse.SkuCount())

	unifier := spec.NewSpecDimensionUnifier()
	if err := unifier.UnifySpecDimensions(mergedResponse); err != nil {
		vp.logger.Errorf("failed to unify spec dimensions after batch merge: %v", err)
		return nil, fmt.Errorf("unify spec dimensions after batch merge: %w", err)
	}

	vp.logger.Info("enforcing spec count limit on merged AI mapping result")
	vp.enforceSpecCountLimit(mergedResponse)

	return mergedResponse, nil
}
