package variant

import (
	"strings"
	"task-processor/internal/platforms/shein/api/attribute"
	"task-processor/internal/platforms/shein/model"

	"github.com/sirupsen/logrus"
)

// VariantMatcher 变体匹配器
type VariantMatcher struct {
	exactMatcher     *VariantExactMatcher
	compositeMatcher *VariantCompositeMatcher
	fuzzyMatcher     *VariantFuzzyMatcher
	utils            *VariantMatcherUtils
}

// NewVariantMatcher 创建新的变体匹配器
func NewVariantMatcher() *VariantMatcher {
	utils := NewVariantMatcherUtils()
	return &VariantMatcher{
		exactMatcher:     NewVariantExactMatcher(),
		compositeMatcher: NewVariantCompositeMatcher(utils),
		fuzzyMatcher:     NewVariantFuzzyMatcher(utils),
		utils:            utils,
	}
}

// FindMatchingVariants 查找匹配的变体
func (m *VariantMatcher) FindMatchingVariants(ctx *model.TaskContext, variants []model.Variant, attrID int, targetValue string) []model.Variant {
	logrus.Infof("开始变体匹配流程")

	targetValueTrimmed := strings.TrimSpace(targetValue)
	attrName := m.getAttributeName(attrID, ctx.AttributeTemplates)
	attrNameAlternatives := m.getAttributeNameAlternatives(attrID, ctx.AttributeTemplates)

	attrNames := append([]string{attrName}, attrNameAlternatives...)
	attrNames = m.utils.RemoveDuplicates(attrNames)
	targetValueNorm := strings.ToLower(targetValueTrimmed)

	// 执行多阶段匹配
	matched := m.performMultiStageMatching(variants, attrNames, targetValueNorm, targetValue)

	logrus.Infof("变体匹配完成，匹配到 %d 个变体", len(matched))
	return matched
}

// performMultiStageMatching 执行多阶段匹配
func (m *VariantMatcher) performMultiStageMatching(variants []model.Variant, attrNames []string, targetValueNorm, targetValue string) []model.Variant {
	// 阶段1：精确匹配
	exactMatches := m.exactMatcher.FindExactMatches(variants, attrNames, targetValueNorm)
	if len(exactMatches) > 0 && m.isMatchCountReasonable(exactMatches, targetValue) {
		return exactMatches
	}

	// 阶段2：组合值匹配
	compositeMatches := m.compositeMatcher.FindCompositeMatches(variants, attrNames, targetValueNorm, targetValue)
	if len(compositeMatches) > 0 && m.isMatchCountReasonable(compositeMatches, targetValue) {
		return compositeMatches
	}

	// 阶段3：模糊匹配
	fuzzyMatches := m.fuzzyMatcher.FindFuzzyMatches(variants, attrNames, targetValueNorm, targetValue)
	if len(fuzzyMatches) > 0 {
		return fuzzyMatches
	}

	return []model.Variant{}
}

// getAttributeName 获取属性名称
func (m *VariantMatcher) getAttributeName(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) string {
	if attrInfo := m.findAttributeInfoByID(attrID, attributeTemplates); attrInfo != nil {
		if attrInfo.AttributeName != "" {
			return attrInfo.AttributeName
		}
		if attrInfo.AttributeNameEn != "" {
			return attrInfo.AttributeNameEn
		}
	}
	return ""
}

// getAttributeNameAlternatives 获取属性名的替代形式
func (m *VariantMatcher) getAttributeNameAlternatives(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) []string {
	alternatives := make([]string, 0)

	if attrInfo := m.findAttributeInfoByID(attrID, attributeTemplates); attrInfo != nil {
		if attrInfo.AttributeName != "" {
			alternatives = append(alternatives, attrInfo.AttributeName)
		}
		if attrInfo.AttributeNameEn != "" && attrInfo.AttributeNameEn != attrInfo.AttributeName {
			alternatives = append(alternatives, attrInfo.AttributeNameEn)
		}
	}

	return alternatives
}

// findAttributeInfoByID 根据属性ID查找属性信息
func (m *VariantMatcher) findAttributeInfoByID(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) *attribute.AttributeInfo {
	if attributeTemplates == nil || len(attributeTemplates.Data) == 0 {
		return nil
	}

	for _, data := range attributeTemplates.Data {
		for i := range data.AttributeInfos {
			if data.AttributeInfos[i].AttributeID == attrID {
				return &data.AttributeInfos[i]
			}
		}
	}
	return nil
}

// isMatchCountReasonable 验证匹配结果的数量是否合理
func (m *VariantMatcher) isMatchCountReasonable(matches []model.Variant, targetValue string) bool {
	if len(matches) == 0 {
		return false
	}

	if len(matches) <= 5 {
		return true
	}

	logrus.Warnf("匹配数量异常: 属性值 '%s' 匹配到 %d 个变体", targetValue, len(matches))
	return false
}
