package handlers

import (
	"fmt"

	"task-processor/internal/common/amazon/model"
	"task-processor/internal/common/pipeline"
)

// generateAISkuMapping 使用AI生成SKU映射
func (sb *SkuBuilder) generateAISkuMapping(ctx *pipeline.TaskContext, variants []*model.Product) (*AISkuMappingResponse, error) {
	if sb.aiClient == nil {
		return nil, fmt.Errorf("AI客户端未初始化")
	}

	// 检查变体数量限制（超过100个变体无法处理，不应重试）
	if len(variants) > 100 {
		sb.logger.Errorf("❌ 变体数量超过限制: %d > 100，系统无法处理如此多的变体", len(variants))
		sb.logger.Error("❌ 此错误不应重试，请检查产品数据或联系技术支持")
		return nil, fmt.Errorf("变体数量超过限制: %d > 100，系统无法处理", len(variants))
	}

	// 根据token限制决定是否需要分批处理
	// Gemini 2.0 Flash输出限制约8000 tokens，每个SKU约300 tokens
	// 安全起见，每批最多处理20个变体
	const maxVariantsPerBatch = 20

	if len(variants) > maxVariantsPerBatch {
		sb.logger.Infof("🔄 变体数量(%d)超过单批限制(%d)，将分批处理", len(variants), maxVariantsPerBatch)
		return sb.generateAISkuMappingInBatches(ctx, variants, maxVariantsPerBatch)
	}

	// 单批处理
	return sb.generateAISkuMappingSingleBatch(ctx, variants)
}

// generateAISkuMappingInBatches 分批生成AI SKU映射
func (sb *SkuBuilder) generateAISkuMappingInBatches(ctx *pipeline.TaskContext, variants []*model.Product, batchSize int) (*AISkuMappingResponse, error) {
	totalBatches := (len(variants) + batchSize - 1) / batchSize
	sb.logger.Infof("📦 开始分批处理: 总变体数=%d, 批次大小=%d, 总批次=%d", len(variants), batchSize, totalBatches)

	var allSkus []AIGeneratedSku
	var selectedSpecDimensions []string // 记录第一批选择的规格维度

	for batchIndex := 0; batchIndex < totalBatches; batchIndex++ {
		start := batchIndex * batchSize
		end := start + batchSize
		if end > len(variants) {
			end = len(variants)
		}

		batchVariants := variants[start:end]
		sb.logger.Infof("📦 处理批次 %d/%d: 变体[%d-%d]", batchIndex+1, totalBatches, start, end-1)

		// 处理当前批次
		batchResponse, err := sb.generateAISkuMappingSingleBatch(ctx, batchVariants)
		if err != nil {
			sb.logger.Errorf("❌ 批次 %d/%d 处理失败: %v", batchIndex+1, totalBatches, err)
			return nil, fmt.Errorf("批次 %d 处理失败: %w", batchIndex+1, err)
		}

		// 第一批：记录选择的规格维度
		if batchIndex == 0 && len(batchResponse.SkuList) > 0 {
			specDimensions := make(map[string]bool)
			for _, spec := range batchResponse.SkuList[0].Spec {
				specDimensions[spec.ParentSpecID] = true
			}
			for parentSpecID := range specDimensions {
				selectedSpecDimensions = append(selectedSpecDimensions, parentSpecID)
			}
			sb.logger.Infof("📌 第一批选择的规格维度: %v", selectedSpecDimensions)
		}

		allSkus = append(allSkus, batchResponse.SkuList...)
		sb.logger.Infof("✅ 批次 %d/%d 完成，生成%d个SKU", batchIndex+1, totalBatches, len(batchResponse.SkuList))
	}

	sb.logger.Infof("✅ 所有批次处理完成，共生成%d个SKU", len(allSkus))

	// 合并后的结果需要再次统一规格维度
	// 因为不同批次可能选择了不同的规格维度
	mergedResponse := &AISkuMappingResponse{
		SkuList: allSkus,
	}

	sb.logger.Info("🔄 对合并结果执行规格维度统一...")
	sb.enforceSpecCountLimit(mergedResponse)

	return mergedResponse, nil
}
