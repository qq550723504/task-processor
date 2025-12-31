// Package handlers 提供TEMU平台的AI SKU映射批处理功能
package handlers

import (
	"fmt"

	"task-processor/internal/model"
	temucontext "task-processor/internal/platforms/temu/context"
	"task-processor/internal/platforms/temu/types"
)

// generateAISkuMappingInBatches 分批生成AI SKU映射
func (vp *SkuVariantProcessor) generateAISkuMappingInBatches(temuCtx *temucontext.TemuTaskContext, variants []*model.Product, batchSize int) (*types.AISkuMappingResponse, error) {

	totalBatches := (len(variants) + batchSize - 1) / batchSize
	vp.logger.Infof("🔨 开始分批处理: 总变体数=%d, 批次大小=%d, 总批次=%d", len(variants), batchSize, totalBatches)

	var allSkus []types.AIGeneratedSku
	var selectedSpecDimensions []string // 记录第一批选择的规格维度

	for batchIndex := 0; batchIndex < totalBatches; batchIndex++ {
		start := batchIndex * batchSize
		end := start + batchSize
		if end > len(variants) {
			end = len(variants)
		}

		batchVariants := variants[start:end]
		vp.logger.Infof("🔨 处理批次 %d/%d: 变体[%d-%d]", batchIndex+1, totalBatches, start, end-1)

		// 处理当前批次 - 使用强类型上下文
		batchResponse, err := vp.generateAISkuMappingSingleBatch(temuCtx, batchVariants)
		if err != nil {
			vp.logger.Errorf("❌ 批次 %d/%d 处理失败: %v", batchIndex+1, totalBatches, err)
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
			vp.logger.Infof("📋 第一批选择的规格维度: %v", selectedSpecDimensions)
		}

		allSkus = append(allSkus, batchResponse.SkuList...)
		vp.logger.Infof("✅ 批次 %d/%d 完成，生成%d个SKU", batchIndex+1, totalBatches, len(batchResponse.SkuList))
	}

	vp.logger.Infof("✅ 所有批次处理完成，共生成%d个SKU", len(allSkus))

	// 合并后的结果需要再次统一规格维度
	mergedResponse := &types.AISkuMappingResponse{
		SkuList: allSkus,
	}

	// 重新启用规格维度统一器，处理混合属性和批次间的不一致问题
	// 1. 统一规格维度（解决不同批次选择不同维度的问题）
	unifier := NewSpecDimensionUnifier()
	if err := unifier.UnifySpecDimensions(mergedResponse); err != nil {
		vp.logger.Errorf("❌ 规格维度统一失败: %v", err)
		return nil, fmt.Errorf("规格维度统一失败: %w", err)
	}

	// 2. 执行规格数量限制
	vp.logger.Info("🔧 对合并结果执行规格数量限制...")
	vp.enforceSpecCountLimit(mergedResponse)

	return mergedResponse, nil
}
