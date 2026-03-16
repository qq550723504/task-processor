// Package property 提供属性提取策略实现
package property

import (
	"fmt"

	"task-processor/internal/domain/model"

	"github.com/sirupsen/logrus"
)

// AttributeExtractor 属性提取器接口
type AttributeExtractor interface {
	Extract(variant *model.Product, amazonProduct *model.Product, index int) (map[string]any, bool)
	GetName() string
}

// AttributeExtractionResult 属性提取结果
type AttributeExtractionResult struct {
	Attributes map[string]any
	Success    bool
	Method     string
	Details    string
}

// AttributeExtractionManager 属性提取管理器
type AttributeExtractionManager struct {
	extractors []AttributeExtractor
	logger     *logrus.Entry
}

// NewAttributeExtractionManager 创建属性提取管理器
func NewAttributeExtractionManager(logger *logrus.Entry) *AttributeExtractionManager {
	return &AttributeExtractionManager{
		extractors: []AttributeExtractor{
			NewParentVariationExtractor(logger),
			NewSelfVariationExtractor(logger),
			NewIndexBasedExtractor(logger),
		},
		logger: logger,
	}
}

// ExtractAttributes 提取变体属性，按优先级尝试多种策略
func (m *AttributeExtractionManager) ExtractAttributes(
	variant *model.Product,
	amazonProduct *model.Product,
	index int,
) AttributeExtractionResult {

	for _, extractor := range m.extractors {
		attributes, success := extractor.Extract(variant, amazonProduct, index)
		if success && len(attributes) > 0 {
			return AttributeExtractionResult{
				Attributes: attributes,
				Success:    true,
				Method:     extractor.GetName(),
				Details:    fmt.Sprintf("成功提取%d个属性", len(attributes)),
			}
		}
	}

	return AttributeExtractionResult{
		Attributes: make(map[string]any),
		Success:    false,
		Method:     "none",
		Details:    "所有提取策略均失败",
	}
}

// ParentVariationExtractor 父产品变体提取器
type ParentVariationExtractor struct {
	logger *logrus.Entry
}

// NewParentVariationExtractor 创建父产品变体提取器
func NewParentVariationExtractor(logger *logrus.Entry) *ParentVariationExtractor {
	return &ParentVariationExtractor{logger: logger}
}

// Extract 从父产品的Variations中通过ASIN精确匹配提取属性
func (e *ParentVariationExtractor) Extract(variant *model.Product, amazonProduct *model.Product, index int) (map[string]any, bool) {
	if amazonProduct == nil || len(amazonProduct.Variations) == 0 {
		return nil, false
	}

	e.logger.Infof("🔍 [父产品变体提取] 变体[%d]在父产品Variations中查找ASIN匹配: ASIN=%s", index, variant.Asin)

	for j, variation := range amazonProduct.Variations {
		if variation.Asin == variant.Asin {
			if len(variation.Attributes) > 0 {
				e.logger.Infof("✅ [父产品变体提取] 变体[%d]从父产品Variations[%d]精确匹配: ASIN=%s, Attributes=%+v",
					index, j, variant.Asin, variation.Attributes)
				return variation.Attributes, true
			}
			e.logger.Warnf("⚠️ [父产品变体提取] 变体[%d]从父产品Variations[%d]找到ASIN但Attributes为空: ASIN=%s",
				index, j, variant.Asin)
			return nil, false
		}
	}

	return nil, false
}

// GetName 获取提取器名称
func (e *ParentVariationExtractor) GetName() string {
	return "父产品变体提取"
}

// SelfVariationExtractor 自身变体提取器
type SelfVariationExtractor struct {
	logger *logrus.Entry
}

// NewSelfVariationExtractor 创建自身变体提取器
func NewSelfVariationExtractor(logger *logrus.Entry) *SelfVariationExtractor {
	return &SelfVariationExtractor{logger: logger}
}

// Extract 从变体自身的Variations中提取属性
func (e *SelfVariationExtractor) Extract(variant *model.Product, amazonProduct *model.Product, index int) (map[string]any, bool) {
	if len(variant.Variations) == 0 {
		return nil, false
	}

	e.logger.Infof("🔍 [自身变体提取] 变体[%d]从自身Variations中查找: ASIN=%s", index, variant.Asin)

	for _, variation := range variant.Variations {
		if variation.Asin == variant.Asin {
			if len(variation.Attributes) > 0 {
				e.logger.Infof("✅ [自身变体提取] 变体[%d]从自身Variations匹配: ASIN=%s, Attributes=%+v",
					index, variant.Asin, variation.Attributes)
				return variation.Attributes, true
			}
			return nil, false
		}
	}

	return nil, false
}

// GetName 获取提取器名称
func (e *SelfVariationExtractor) GetName() string {
	return "自身变体提取"
}

// IndexBasedExtractor 索引位置提取器
type IndexBasedExtractor struct {
	logger *logrus.Entry
}

// NewIndexBasedExtractor 创建索引位置提取器
func NewIndexBasedExtractor(logger *logrus.Entry) *IndexBasedExtractor {
	return &IndexBasedExtractor{logger: logger}
}

// Extract 按索引位置匹配，但必须验证ASIN一致性
func (e *IndexBasedExtractor) Extract(variant *model.Product, amazonProduct *model.Product, index int) (map[string]any, bool) {
	if amazonProduct == nil || index >= len(amazonProduct.Variations) {
		return nil, false
	}

	variation := amazonProduct.Variations[index]
	if variation.Asin == variant.Asin && len(variation.Attributes) > 0 {
		e.logger.Infof("✅ [索引位置提取] 变体[%d]通过索引匹配且ASIN验证通过: ASIN=%s, Attributes=%+v",
			index, variant.Asin, variation.Attributes)
		return variation.Attributes, true
	}

	if variation.Asin != variant.Asin {
		e.logger.Warnf("⚠️ [索引位置提取] 变体[%d]索引匹配失败，ASIN不一致: 期望=%s, 实际=%s",
			index, variant.Asin, variation.Asin)
	}

	return nil, false
}

// GetName 获取提取器名称
func (e *IndexBasedExtractor) GetName() string {
	return "索引位置提取"
}
