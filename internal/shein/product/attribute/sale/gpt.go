package sale

import (
	"task-processor/internal/core/logger"
	sheinattr "task-processor/internal/shein/product/attribute"
)

func (h *SaleAttributeHandler) callGPTAPI(input *SaleAttributeInput, request *sheinattr.GenerationRequest) sheinattr.ResultSaleAttribute {
	const maxVariantsPerBatch = 20
	variantCount := len(request.VariationData)

	if variantCount > maxVariantsPerBatch {
		logger.GetGlobalLogger("shein/product").Infof("processing sale attributes in batches: variants=%d batch_size=%d", variantCount, maxVariantsPerBatch)
		batchProcessor := NewSaleAttributeBatchProcessor(h)
		return batchProcessor.ProcessInBatches(input, request, maxVariantsPerBatch)
	}

	singleProcessor := NewSaleAttributeSingleProcessor(h)
	return singleProcessor.ProcessSingleBatch(input, request)
}
