package shein

import (
	"strings"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

type sourceValueSemanticKind struct {
	label       string
	templateCue []string
}

func buildCategoryTemplateGapSummary(candidates []saleAttributeCandidate, attributes []sheinattribute.AttributeInfo) (bool, string) {
	if len(candidates) == 0 || len(attributes) == 0 {
		return false, ""
	}
	for _, candidate := range candidates {
		if candidate.ValueFitTotal == 0 || candidate.ValueFitCount > 0 {
			continue
		}
		semantic := detectSourceValueSemanticKind(candidate.Values)
		if semantic == nil {
			continue
		}
		if templateSupportsSemantic(attributes, semantic.templateCue...) {
			continue
		}
		return true, "当前类目销售属性模板未提供可承接" + semantic.label + "的销售属性字段"
	}
	return false, ""
}

func buildCategoryTemplateGapReviewNotes(candidates []saleAttributeCandidate, attributes []sheinattribute.AttributeInfo) []string {
	if recommend, reason := buildCategoryTemplateGapSummary(candidates, attributes); recommend {
		return []string{
			reason + "，建议优先人工确认 SHEIN 类目是否正确，再决定是否继续映射该销售属性",
		}
	}
	return nil
}

func detectSourceValueSemanticKind(values []string) *sourceValueSemanticKind {
	semantic := inferSourceValueSemantic(values)
	switch semantic {
	case "套装/组合款":
		return &sourceValueSemanticKind{
			label:       semantic,
			templateCue: []string{"set", "套装", "组合", "quantity", "件数", "type", "类型", "style", "款式"},
		}
	case "款式/型号":
		return &sourceValueSemanticKind{
			label:       semantic,
			templateCue: []string{"style", "款式", "type", "类型", "model", "型号"},
		}
	case "规格/尺寸":
		return &sourceValueSemanticKind{
			label:       semantic,
			templateCue: []string{"size", "尺码", "尺寸", "dimension", "规格", "capacity", "容量"},
		}
	default:
		return nil
	}
}

func templateSupportsSemantic(attributes []sheinattribute.AttributeInfo, cues ...string) bool {
	if len(attributes) == 0 || len(cues) == 0 {
		return false
	}
	for _, attr := range attributes {
		names := collectAttributeNames(attr)
		for _, name := range names {
			normalizedName := normalizeText(name)
			for _, cue := range cues {
				if strings.Contains(normalizedName, normalizeText(cue)) {
					return true
				}
			}
		}
	}
	return false
}
