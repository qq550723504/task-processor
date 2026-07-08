// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/mxschmitt/playwright-go"
)

// SpecificationExtractor 规格提取器 - 协调各个专门的提取器
type SpecificationExtractor struct {
	attributeExtractor     *AttributeExtractor
	variantValuesExtractor *VariantValuesExtractor
	detailExtractor        *DetailExtractor
	packInfoExtractor      *PackInfoExtractor
	variantExtractor       *VariantExtractor
}

// NewSpecificationExtractor 创建规格提取器
func NewSpecificationExtractor() *SpecificationExtractor {
	return &SpecificationExtractor{
		attributeExtractor:     NewAttributeExtractor(),
		variantValuesExtractor: NewVariantValuesExtractor(),
		detailExtractor:        NewDetailExtractor(),
		packInfoExtractor:      NewPackInfoExtractor(),
		variantExtractor:       NewVariantExtractor(),
	}
}

// Extract 提取商品规格信息 - 协调各个提取器
func (se *SpecificationExtractor) Extract(page playwright.Page, product *model.Product1688) error {
	logger.GetGlobalLogger("crawler/alibaba1688").Debug("开始提取商品规格信息")

	// 1. 提取商品属性
	if err := se.attributeExtractor.Extract(page, product); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取商品属性失败: %v", err)
	}

	// 2. 提取销售规格
	if err := se.variantValuesExtractor.Extract(page, product); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取销售规格失败: %v", err)
	}

	// 3. 提取商品详情
	if err := se.detailExtractor.Extract(page, product); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取商品详情失败: %v", err)
	}

	// 4. 提取包装信息
	if err := se.packInfoExtractor.Extract(page, product); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取包装信息失败: %v", err)
	}

	// 5. 提取变体数据
	if err := se.variantExtractor.Extract(page, product); err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取变体数据失败: %v", err)
	}

	return nil
}
