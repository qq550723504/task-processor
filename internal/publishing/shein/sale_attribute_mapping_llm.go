package shein

import (
	"strings"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

func matchSelectedCandidates(
	candidates []saleAttributeCandidate,
	selection *saleAttributeMappingSelection,
	dimensions []SourceVariantDimension,
	attributes []sheinattribute.AttributeInfo,
) (*saleAttributeCandidate, *saleAttributeCandidate, []saleAttributeCandidate) {
	if selection == nil {
		return nil, nil, candidates
	}
	primary := findMappedCandidate(candidates, selection.PrimarySourceDimension, selection.PrimaryAttributeID)
	secondary := findMappedCandidate(candidates, selection.SecondarySourceDimension, selection.SecondaryAttributeID)
	if primary == nil {
		if candidate, ok := buildLLMSelectedSaleAttributeCandidate(selection.PrimarySourceDimension, selection.PrimaryAttributeID, dimensions, attributes); ok {
			candidates = append(candidates, candidate)
			primary = &candidates[len(candidates)-1]
		}
	}
	if secondary == nil {
		if candidate, ok := buildLLMSelectedSaleAttributeCandidate(selection.SecondarySourceDimension, selection.SecondaryAttributeID, dimensions, attributes); ok {
			candidates = append(candidates, candidate)
			secondary = &candidates[len(candidates)-1]
		}
	}
	if primary != nil && secondary != nil && primary.SourceName == secondary.SourceName {
		secondary = nil
	}
	return primary, secondary, candidates
}

func findMappedCandidate(candidates []saleAttributeCandidate, sourceName string, attributeID int) *saleAttributeCandidate {
	sourceName = normalizeText(sourceName)
	for i := range candidates {
		candidate := &candidates[i]
		if candidate.ValueFitCount == 0 {
			continue
		}
		if sourceName != "" && normalizeText(candidate.SourceName) != sourceName {
			continue
		}
		if attributeID > 0 && candidate.AttributeID != attributeID {
			continue
		}
		return candidate
	}
	return nil
}

func buildLLMSelectedSaleAttributeCandidate(
	sourceName string,
	attributeID int,
	dimensions []SourceVariantDimension,
	attributes []sheinattribute.AttributeInfo,
) (saleAttributeCandidate, bool) {
	sourceName = strings.TrimSpace(sourceName)
	if sourceName == "" || attributeID <= 0 {
		return saleAttributeCandidate{}, false
	}
	dimension, sourceOrder, ok := findSourceDimensionForCandidate(dimensions, sourceName)
	if !ok {
		return saleAttributeCandidate{}, false
	}
	attr, templateOrder, ok := findTemplateAttribute(attributes, attributeID)
	if !ok {
		return saleAttributeCandidate{}, false
	}
	match := buildTemplateAttributeMatch(*attr, dimension.SampleValue)
	match.MatchedBy = "llm_sale_attribute_mapping"
	distinct := len(uniqueNormalizedValues(dimension.Values))
	return newSaleAttributeCandidate(*dimension, sourceOrder, templateOrder, match, distinct, distinct)
}

func findSourceDimensionForCandidate(dimensions []SourceVariantDimension, sourceName string) (*SourceVariantDimension, int, bool) {
	for i := range dimensions {
		if normalizeText(dimensions[i].Name) != normalizeText(sourceName) {
			continue
		}
		if isTechnicalSaleSourceDimension(dimensions[i].Name) {
			return nil, 0, false
		}
		return &dimensions[i], i, true
	}
	return nil, 0, false
}

func findTemplateAttribute(attributes []sheinattribute.AttributeInfo, attributeID int) (*sheinattribute.AttributeInfo, int, bool) {
	for i := range attributes {
		if attributes[i].AttributeID != attributeID {
			continue
		}
		return &attributes[i], i, true
	}
	return nil, 0, false
}

func buildPrimaryLabelCandidateFromSourceSelection(sourceName string, dimensions []SourceVariantDimension, attributes []sheinattribute.AttributeInfo) (saleAttributeCandidate, bool) {
	first := firstSaleAttributeTemplate(attributes)
	if first == nil || !isPrimarySaleTemplateAttribute(*first) {
		return saleAttributeCandidate{}, false
	}
	sourceName = strings.TrimSpace(sourceName)
	if sourceName == "" || isAIStyleSourceDimension(sourceName) || isTechnicalSaleSourceDimension(sourceName) {
		return saleAttributeCandidate{}, false
	}
	if isGenericSecondaryName(sourceName) {
		return saleAttributeCandidate{}, false
	}
	return buildLLMSelectedSaleAttributeCandidate(sourceName, first.AttributeID, dimensions, attributes)
}

func shouldSelectSaleAttributeMappingWithLLM(primary *saleAttributeCandidate, dimensions []SourceVariantDimension, attributes []sheinattribute.AttributeInfo) bool {
	if len(dimensions) == 0 {
		return false
	}
	if primary == nil {
		return true
	}
	if selectedCandidateMissesPrimarySaleTemplate(primary, attributes) && hasAlternativePrimarySaleSourceDimension(dimensions, "") {
		return true
	}
	return isWeakPrimarySaleAttributeCandidate(*primary) && hasAlternativePrimarySaleSourceDimension(dimensions, primary.SourceName)
}

func shouldUseLLMSaleAttributeMapping(current, selected *saleAttributeCandidate, attributes []sheinattribute.AttributeInfo) bool {
	if selected == nil {
		return false
	}
	if current == nil {
		return true
	}
	if normalizeText(current.SourceName) == normalizeText(selected.SourceName) && current.AttributeID == selected.AttributeID {
		return false
	}
	if selectedCandidateMatchesFirstSaleTemplate(selected, attributes) && selectedCandidateMissesPrimarySaleTemplate(current, attributes) {
		return true
	}
	if selected.SKCScope || selected.Required || selected.Important {
		return true
	}
	return isWeakPrimarySaleAttributeCandidate(*current)
}

func selectedCandidateMissesPrimarySaleTemplate(primary *saleAttributeCandidate, attributes []sheinattribute.AttributeInfo) bool {
	if primary == nil || len(attributes) == 0 {
		return false
	}
	first := firstSaleAttributeTemplate(attributes)
	return first != nil && primary.AttributeID != first.AttributeID
}

func selectedCandidateMatchesFirstSaleTemplate(primary *saleAttributeCandidate, attributes []sheinattribute.AttributeInfo) bool {
	if primary == nil || len(attributes) == 0 {
		return false
	}
	first := firstSaleAttributeTemplate(attributes)
	return first != nil && primary.AttributeID == first.AttributeID
}

func shouldBlockPromptDerivedAIStylePrimary(candidate *saleAttributeCandidate, dimensions []SourceVariantDimension) bool {
	if candidate == nil || !isAIStyleSourceDimension(candidate.SourceName) {
		return false
	}
	if !shouldExtractSaleAttributeSourceValue(candidate.SourceName, candidate.SampleValue) {
		return false
	}
	return hasAlternativePrimarySaleSourceDimension(dimensions, candidate.SourceName)
}

func saleAttributeMappingSourceDimensions(dimensions []SourceVariantDimension) []SourceVariantDimension {
	if len(dimensions) == 0 || !hasAlternativePrimarySaleSourceDimension(dimensions, "ai_style") {
		return dimensions
	}
	filtered := make([]SourceVariantDimension, 0, len(dimensions))
	for _, dimension := range dimensions {
		if isAIStyleSourceDimension(dimension.Name) && sourceDimensionRequiresExternalSaleAttributeExtraction(dimension) {
			continue
		}
		filtered = append(filtered, dimension)
	}
	if len(filtered) == 0 {
		return dimensions
	}
	return filtered
}

func hasAlternativePrimarySaleSourceDimension(dimensions []SourceVariantDimension, currentName string) bool {
	for _, dimension := range dimensions {
		if normalizeText(dimension.Name) == normalizeText(currentName) {
			continue
		}
		if isTechnicalSaleSourceDimension(dimension.Name) || isAIStyleSourceDimension(dimension.Name) || isGenericSecondaryName(dimension.Name) {
			continue
		}
		if len(uniqueNormalizedValues(dimension.Values)) == 0 {
			continue
		}
		return true
	}
	return false
}
