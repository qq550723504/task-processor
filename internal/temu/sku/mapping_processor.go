// Package sku 提供TEMU平台的SKU映射处理功能
package sku

import (
	"fmt"
	"task-processor/internal/model"
	temucontext "task-processor/internal/temu/context"

	"github.com/sirupsen/logrus"
)

// SkuMappingProcessor SKU映射处理器
type SkuMappingProcessor struct {
	logger      *logrus.Entry
	specHandler *SkuSpecHandler
}

// NewSkuMappingProcessor 创建新的SKU映射处理器
func NewSkuMappingProcessor(logger *logrus.Entry, specHandler *SkuSpecHandler) *SkuMappingProcessor {
	return &SkuMappingProcessor{
		logger:      logger,
		specHandler: specHandler,
	}
}

// FixMappingCountMismatch 修复映射数量不匹配问题
func (mp *SkuMappingProcessor) FixMappingCountMismatch(aiMapping *temucontext.AISkuMappingResponse, variants []*model.Product) error {
	// 如果AI映射数量少于变体数量，尝试补充缺失的映射
	if aiMapping.SkuCount() < len(variants) {
		mp.logger.Infof("尝试为缺失的%d个变体补充默认映射", len(variants)-aiMapping.SkuCount())
		mp.supplementMissingMappings(aiMapping, variants)
		mp.logger.Infof("✅ 成功补充缺失映射，当前映射数量: %d", aiMapping.SkuCount())
	} else {
		// AI映射数量多于变体数量，尝试去重或移除多余的映射
		diff := aiMapping.SkuCount() - len(variants)
		mp.logger.Warnf("⚠️ AI映射数量多于变体数量，差异: %d个", diff)

		// 如果差异在可接受范围内（≤2个），尝试智能处理
		if diff <= 2 {
			mp.logger.Infof("差异在可接受范围内，尝试去重和修复...")
			if err := mp.removeDuplicateOrExcessMappings(aiMapping, variants); err != nil {
				return fmt.Errorf("移除多余映射失败: %w", err)
			}
			mp.logger.Infof("✅ 成功处理多余映射，当前映射数量: %d", aiMapping.SkuCount())
		} else {
			// 差异过大，无法处理
			return fmt.Errorf("AI映射数量(%d)远多于变体数量(%d)，差异过大(%d)，无法处理",
				aiMapping.SkuCount(), len(variants), diff)
		}
	}

	return nil
}

// removeDuplicateOrExcessMappings 移除重复或多余的AI映射
func (mp *SkuMappingProcessor) removeDuplicateOrExcessMappings(aiMapping *temucontext.AISkuMappingResponse, variants []*model.Product) error {
	// 创建变体ASIN集合
	validAsins := make(map[string]bool)
	for _, variant := range variants {
		validAsins[variant.Asin] = true
	}

	duplicateAsins, invalidAsins := mp.analyzeInvalidAndDuplicateAsins(aiMapping, validAsins)
	filteredSkus := mp.collectFilteredMappings(aiMapping, duplicateAsins, invalidAsins)

	// 如果过滤后数量仍然不匹配，移除多余的映射（保留前N个）
	if len(filteredSkus) > len(variants) {
		excess := len(filteredSkus) - len(variants)
		mp.logger.Warnf("⚠️ 过滤后仍有%d个多余映射，将移除末尾的映射", excess)
		filteredSkus = filteredSkus[:len(variants)]
	}

	// 更新映射列表
	removedCount := aiMapping.SkuCount() - len(filteredSkus)
	aiMapping.ReplaceSKUs(filteredSkus)
	mp.logger.Infof("✅ 移除了%d个多余/重复的映射，剩余%d个映射", removedCount, len(filteredSkus))

	// 验证最终数量
	if aiMapping.SkuCount() != len(variants) {
		return fmt.Errorf("处理后映射数量(%d)仍与变体数量(%d)不匹配", aiMapping.SkuCount(), len(variants))
	}

	return nil
}

func (mp *SkuMappingProcessor) analyzeInvalidAndDuplicateAsins(aiMapping *temucontext.AISkuMappingResponse, validAsins map[string]bool) (map[string]bool, map[string]bool) {
	asinCount := make(map[string]int)
	aiMapping.ForEachSKU(func(sku *temucontext.AIGeneratedSku) {
		asinCount[sku.Asin]++
	})

	duplicateAsins := make(map[string]bool)
	for asin, count := range asinCount {
		if count > 1 {
			duplicateAsins[asin] = true
			mp.logger.Warnf("Detected duplicate ASIN: %s (%d occurrences)", asin, count)
		}
	}

	invalidAsins := make(map[string]bool)
	aiMapping.ForEachSKU(func(sku *temucontext.AIGeneratedSku) {
		if !validAsins[sku.Asin] {
			invalidAsins[sku.Asin] = true
			mp.logger.Warnf("Detected invalid ASIN: %s (not in variant list)", sku.Asin)
		}
	})

	return duplicateAsins, invalidAsins
}

func (mp *SkuMappingProcessor) collectFilteredMappings(aiMapping *temucontext.AISkuMappingResponse, duplicateAsins, invalidAsins map[string]bool) []temucontext.AIGeneratedSku {
	var filteredSkus []temucontext.AIGeneratedSku
	seenAsins := make(map[string]bool)

	aiMapping.ForEachSKU(func(sku *temucontext.AIGeneratedSku) {
		if invalidAsins[sku.Asin] {
			mp.logger.Infof("Removing invalid mapping: ASIN=%s", sku.Asin)
			return
		}

		if duplicateAsins[sku.Asin] && seenAsins[sku.Asin] {
			mp.logger.Infof("Removing duplicate mapping: ASIN=%s", sku.Asin)
			return
		}

		filteredSkus = append(filteredSkus, *sku)
		seenAsins[sku.Asin] = true
	})

	return filteredSkus
}

// supplementMissingMappings 为缺失的变体补充默认映射
func (mp *SkuMappingProcessor) supplementMissingMappings(aiMapping *temucontext.AISkuMappingResponse, variants []*model.Product) {
	// 创建已映射的ASIN集合
	mappedAsins := make(map[string]bool)
	aiMapping.ForEachSKU(func(sku *temucontext.AIGeneratedSku) {
		mappedAsins[sku.Asin] = true
	})

	// 分析已有映射的spec模式，用于推断缺失映射的spec
	specTemplate := mp.analyzeSpecPattern(aiMapping)

	// 为未映射的变体创建默认映射
	missingCount := 0
	for _, variant := range variants {
		if !mappedAsins[variant.Asin] {
			missingCount++
			mp.logger.Infof("为变体 %s 创建补充映射 (第%d个缺失)", variant.Asin, missingCount)

			// 创建默认SKU映射，尝试使用spec模板
			defaultSku := temucontext.AIGeneratedSku{
				UniqueID:          variant.Asin,
				Asin:              variant.Asin,
				Spec:              specTemplate, // 使用从已有映射推断的spec模板
				Weight:            "",
				Length:            "",
				Width:             "",
				Height:            "",
				VariantAttributes: make(map[string]string),
			}

			aiMapping.AppendSKU(defaultSku)
			mp.logger.Infof("✅ 已为变体 %s 添加补充映射 (使用spec模板: %d个规格)", variant.Asin, len(specTemplate))
		}
	}

	if missingCount > 0 {
		mp.logger.Warnf("⚠️ 补充了%d个缺失的映射，这些映射使用了推断的spec模板", missingCount)
		mp.logger.Warn("⚠️ 建议检查AI映射生成逻辑，确保为所有变体生成正确的映射")
	}
}

// analyzeSpecPattern 分析已有映射的spec模式，返回一个spec模板
func (mp *SkuMappingProcessor) analyzeSpecPattern(aiMapping *temucontext.AISkuMappingResponse) []temucontext.SpecInfo {
	if aiMapping.SkuCount() == 0 {
		return []temucontext.SpecInfo{}
	}

	// 统计每个spec_id出现的频率
	specFrequency := make(map[string]int)
	specExamples := make(map[string]temucontext.SpecInfo)

	aiMapping.ForEachSKU(func(sku *temucontext.AIGeneratedSku) {
		for _, spec := range sku.Spec {
			specFrequency[spec.SpecID]++
			if _, exists := specExamples[spec.SpecID]; !exists {
				specExamples[spec.SpecID] = temucontext.SpecInfo{
					SpecID:         spec.SpecID,
					SpecName:       spec.SpecName,
					ParentSpecID:   spec.ParentSpecID,
					ParentSpecName: spec.ParentSpecName,
					ParentID:       "",
				}
			}
		}
	})

	// 选择出现频率最高的spec作为模板
	var template []temucontext.SpecInfo
	for specID, spec := range specExamples {
		if specFrequency[specID] > aiMapping.SkuCount()/2 {
			template = append(template, spec)
		}
	}

	if len(template) > 0 {
		mp.logger.Infof("从已有映射中推断出spec模板: %d个规格", len(template))
	} else {
		mp.logger.Warn("无法从已有映射中推断spec模板，将使用空spec")
	}

	return template
}
