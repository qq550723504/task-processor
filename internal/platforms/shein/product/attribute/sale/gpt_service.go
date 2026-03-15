// Package modules 提供SHEIN平台的销售属性GPT处理功能
package sale

import (
	"task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// callGPTAPI 调用GPT API
// 参数:
//   - ctx: 任务上下文
//   - request: 生成请求
//
// 返回值:
//   - ResultSaleAttribute: 销售属性结果
func (h *SaleAttributeHandler) callGPTAPI(ctx *model.TaskContext, request *model.GenerationRequest) model.ResultSaleAttribute {
	// 检查变体数量，决定是否需要分批处理
	const maxVariantsPerBatch = 20
	variantCount := len(request.VariationData)

	if variantCount > maxVariantsPerBatch {
		logrus.Infof("🔄 变体数量(%d)超过单批限制(%d)，将分批处理", variantCount, maxVariantsPerBatch)
		batchProcessor := NewSaleAttributeBatchProcessor(h)
		return batchProcessor.ProcessInBatches(ctx, request, maxVariantsPerBatch)
	}

	// 单批处理
	singleProcessor := NewSaleAttributeSingleProcessor(h)
	return singleProcessor.ProcessSingleBatch(ctx, request)
}
