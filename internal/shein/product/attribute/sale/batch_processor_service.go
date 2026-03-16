// Package sale 提供SHEIN平台的销售属性批处理功能
package sale

import (
	"task-processor/internal/shein"

	"github.com/sirupsen/logrus"
)

// SaleAttributeBatchProcessor 销售属性批处理器，负责处理大量变体的分批处理
type SaleAttributeBatchProcessor struct {
	handler *SaleAttributeHandler
}

// NewSaleAttributeBatchProcessor 创建新的销售属性批处理器
// 参数:
//   - handler: 销售属性处理器实例
//
// 返回值:
//   - *SaleAttributeBatchProcessor: 批处理器实例
func NewSaleAttributeBatchProcessor(handler *SaleAttributeHandler) *SaleAttributeBatchProcessor {
	return &SaleAttributeBatchProcessor{
		handler: handler,
	}
}

// ProcessInBatches 分批调用GPT API
// 参数:
//   - ctx: 任务上下文
//   - request: 生成请求
//   - batchSize: 批次大小
//
// 返回值:
//   - ResultSaleAttribute: 销售属性结果
func (p *SaleAttributeBatchProcessor) ProcessInBatches(ctx *shein.TaskContext, request *shein.GenerationRequest, batchSize int) shein.ResultSaleAttribute {
	variationData := request.VariationData
	productsData := request.ProductsData
	totalBatches := (len(variationData) + batchSize - 1) / batchSize

	logrus.Infof("📦 开始分批处理: 总变体数=%d, 产品数据数=%d, 批次大小=%d, 总批次=%d",
		len(variationData), len(productsData), batchSize, totalBatches)

	var allVariants []shein.Variant
	var allSaleAttributes []shein.ResultAttribute

	for batchIndex := 0; batchIndex < totalBatches; batchIndex++ {
		start := batchIndex * batchSize
		end := start + batchSize
		if end > len(variationData) {
			end = len(variationData)
		}

		// 安全地切片ProductsData，确保不越界
		var batchProductsData []shein.ProductVariantData
		if start < len(productsData) {
			productsEnd := end
			if productsEnd > len(productsData) {
				productsEnd = len(productsData)
			}
			batchProductsData = productsData[start:productsEnd]
		} else {
			// 如果start已经超出ProductsData范围，使用空切片
			batchProductsData = []shein.ProductVariantData{}
		}

		// 创建当前批次的请求
		batchRequest := &shein.GenerationRequest{
			ProductsData:             batchProductsData,
			VariationData:            variationData[start:end],
			VariationAttributeValues: request.VariationAttributeValues,
			SaleAttributesData:       request.SaleAttributesData,
			AttributeMappings:        request.AttributeMappings,
			RequiredVariantCount:     end - start,
		}

		logrus.Infof("📦 处理批次 %d/%d: 变体[%d-%d], 产品数据[%d-%d]",
			batchIndex+1, totalBatches, start, end-1, start, len(batchProductsData)-1+start)

		// 处理当前批次
		singleProcessor := NewSaleAttributeSingleProcessor(p.handler)
		batchResult := singleProcessor.ProcessSingleBatch(ctx, batchRequest)

		// 合并变体数据
		allVariants = append(allVariants, batchResult.Variants...)

		// 合并销售属性数据（避免重复）
		for _, saleAttr := range batchResult.SaleAttributes {
			// 检查是否已存在相同AttrID的属性
			exists := false
			for _, existing := range allSaleAttributes {
				if existing.AttrID == saleAttr.AttrID {
					exists = true
					break
				}
			}
			if !exists {
				allSaleAttributes = append(allSaleAttributes, saleAttr)
			}
		}

		logrus.Infof("✅ 批次 %d/%d 完成，生成%d个变体，%d个销售属性",
			batchIndex+1, totalBatches, len(batchResult.Variants), len(batchResult.SaleAttributes))
	}

	logrus.Infof("✅ 所有批次处理完成，共生成%d个变体，%d个销售属性", len(allVariants), len(allSaleAttributes))

	return shein.ResultSaleAttribute{
		SaleAttributes: allSaleAttributes,
		Variants:       allVariants,
	}
}
