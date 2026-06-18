package variant

import (
	"fmt"
	"strings"
	"task-processor/internal/core/logger"
	"task-processor/internal/shein"
	"task-processor/internal/shein/api/attribute"
	sheinattr "task-processor/internal/shein/product/attribute"
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
func (m *VariantMatcher) FindMatchingVariants(ctx *shein.TaskContext, variants []sheinattr.Variant, attrID int, targetValue string) []sheinattr.Variant {
	logger.GetGlobalLogger("shein/product").Infof("开始变体匹配流程")

	attrNames, targetValueNorm, targetValueTrimmed := m.prepareMatchInputs(ctx, attrID, targetValue)

	// 执行多阶段匹配
	matched := m.performMultiStageMatching(variants, attrNames, targetValueNorm, targetValueTrimmed)

	logger.GetGlobalLogger("shein/product").Infof("变体匹配完成，匹配到 %d 个变体", len(matched))
	return matched
}

// FindUniqueMatchesForValues 按候选属性值顺序为变体做唯一分配，保证一个变体只归属到一个值。
func (m *VariantMatcher) FindUniqueMatchesForValues(ctx *shein.TaskContext, variants []sheinattr.Variant, attrID int, targetValues []string) map[string][]sheinattr.Variant {
	assignments := make(map[string][]sheinattr.Variant, len(targetValues))
	if len(targetValues) == 0 || len(variants) == 0 {
		return assignments
	}

	type targetSpec struct {
		original string
		trimmed  string
		norm     string
	}

	attrNames, _, _ := m.prepareMatchInputs(ctx, attrID, "")
	attrLabel := m.describeAttribute(attrID, ctx.AttributeTemplates)
	usedTargetNorms := make(map[string]bool)
	targets := make([]targetSpec, 0, len(targetValues))
	for _, targetValue := range targetValues {
		trimmed := strings.TrimSpace(targetValue)
		norm := strings.ToLower(trimmed)
		if norm == "" || usedTargetNorms[norm] {
			continue
		}
		usedTargetNorms[norm] = true
		targets = append(targets, targetSpec{
			original: targetValue,
			trimmed:  trimmed,
			norm:     norm,
		})
		assignments[targetValue] = nil
	}

	remaining := append([]sheinattr.Variant(nil), variants...)
	matchPhases := []struct {
		name string
		fn   func([]sheinattr.Variant, []string, string, string) []sheinattr.Variant
	}{
		{
			name: "exact",
			fn: func(candidates []sheinattr.Variant, names []string, targetNorm, _ string) []sheinattr.Variant {
				return m.exactMatcher.FindExactMatches(candidates, names, targetNorm)
			},
		},
		{
			name: "composite",
			fn:   m.compositeMatcher.FindCompositeMatches,
		},
		{
			name: "fuzzy",
			fn:   m.fuzzyMatcher.FindFuzzyMatches,
		},
	}

	for _, phase := range matchPhases {
		if len(remaining) == 0 {
			break
		}

		usedInPhase := make(map[string]bool)
		for _, target := range targets {
			matches := phase.fn(remaining, attrNames, target.norm, target.trimmed)
			if len(matches) == 0 {
				continue
			}

			for _, matched := range matches {
				if usedInPhase[matched.ASIN] {
					logger.GetGlobalLogger("shein/product").Infof(
						"unique variant assignment skip: attribute=%s phase=%s asin=%s target=%q reason=already_assigned_in_phase",
						attrLabel,
						phase.name,
						matched.ASIN,
						target.original,
					)
					continue
				}
				assignments[target.original] = append(assignments[target.original], matched)
				usedInPhase[matched.ASIN] = true
				logger.GetGlobalLogger("shein/product").Infof(
					"unique variant assignment: attribute=%s phase=%s asin=%s target=%q matched_value=%q",
					attrLabel,
					phase.name,
					matched.ASIN,
					target.original,
					extractVariantAttributeValue(matched, attrNames),
				)
			}
		}

		if len(usedInPhase) == 0 {
			continue
		}

		nextRemaining := make([]sheinattr.Variant, 0, len(remaining)-len(usedInPhase))
		for _, variant := range remaining {
			if !usedInPhase[variant.ASIN] {
				nextRemaining = append(nextRemaining, variant)
			}
		}
		logger.GetGlobalLogger("shein/product").Infof("unique variant assignment phase=%s consumed=%d remaining=%d", phase.name, len(usedInPhase), len(nextRemaining))
		remaining = nextRemaining
	}

	for _, target := range targets {
		logger.GetGlobalLogger("shein/product").Infof(
			"unique variant assignment summary: attribute=%s target=%q assigned=%d",
			attrLabel,
			target.original,
			len(assignments[target.original]),
		)
	}
	if len(remaining) > 0 {
		for _, unmatched := range remaining {
			logger.GetGlobalLogger("shein/product").Warnf(
				"unique variant assignment unmatched: attribute=%s asin=%s value=%q",
				attrLabel,
				unmatched.ASIN,
				extractVariantAttributeValue(unmatched, attrNames),
			)
		}
	}

	return assignments
}

// performMultiStageMatching 执行多阶段匹配
func (m *VariantMatcher) performMultiStageMatching(variants []sheinattr.Variant, attrNames []string, targetValueNorm, targetValue string) []sheinattr.Variant {
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

	return []sheinattr.Variant{}
}

func (m *VariantMatcher) prepareMatchInputs(ctx *shein.TaskContext, attrID int, targetValue string) ([]string, string, string) {
	targetValueTrimmed := strings.TrimSpace(targetValue)
	attrName := m.getAttributeName(attrID, ctx.AttributeTemplates)
	attrNameAlternatives := m.getAttributeNameAlternatives(attrID, ctx.AttributeTemplates)

	attrNames := append([]string{attrName}, attrNameAlternatives...)
	attrNames = m.utils.RemoveDuplicates(attrNames)
	targetValueNorm := strings.ToLower(targetValueTrimmed)
	return attrNames, targetValueNorm, targetValueTrimmed
}

func (m *VariantMatcher) describeAttribute(attrID int, attributeTemplates *attribute.AttributeTemplateInfo) string {
	if attrInfo := m.findAttributeInfoByID(attrID, attributeTemplates); attrInfo != nil {
		if attrInfo.AttributeNameEn != "" && attrInfo.AttributeName != "" {
			return attrInfo.AttributeNameEn + "/" + attrInfo.AttributeName
		}
		if attrInfo.AttributeNameEn != "" {
			return attrInfo.AttributeNameEn
		}
		if attrInfo.AttributeName != "" {
			return attrInfo.AttributeName
		}
	}
	return fmt.Sprintf("attr_%d", attrID)
}

func extractVariantAttributeValue(variant sheinattr.Variant, attrNames []string) string {
	for _, attrName := range attrNames {
		for key, value := range variant.Attributes {
			if strings.EqualFold(key, attrName) {
				return value
			}
		}
	}
	return ""
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
func (m *VariantMatcher) isMatchCountReasonable(matches []sheinattr.Variant, targetValue string) bool {
	if len(matches) == 0 {
		return false
	}

	if len(matches) <= 5 {
		return true
	}

	logger.GetGlobalLogger("shein/product").Warnf("匹配数量异常: 属性值 '%s' 匹配到 %d 个变体", targetValue, len(matches))
	return false
}
