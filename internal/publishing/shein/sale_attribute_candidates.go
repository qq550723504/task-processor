package shein

import (
	"strconv"
	"strings"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

type saleAttributeCandidate struct {
	SourceName     string
	Values         []string
	SampleValue    string
	TemplateName   string
	AttributeID    int
	TemplateOrder  int
	SKCScope       bool
	Required       bool
	Important      bool
	SourceOrder    int
	DistinctCount  int
	ValueFitCount  int
	ValueFitTotal  int
	PrimaryScore   int
	SecondaryScore int
	Match          ResolvedAttribute
}

func buildSaleAttributeCandidates(dimensions []SourceVariantDimension, attributes []sheinattribute.AttributeInfo) []saleAttributeCandidate {
	if len(dimensions) == 0 || len(attributes) == 0 {
		return nil
	}
	index := newTemplateIndex(attributes)
	candidates := make([]saleAttributeCandidate, 0, len(dimensions))
	seen := make(map[string]struct{}, len(dimensions))
	sourceDimensionNames := sourceSaleDimensionNames(dimensions)
	for i, dimension := range dimensions {
		candidates = append(candidates, buildSaleAttributeCandidatesForDimension(index, attributes, dimension, i, seen, sourceDimensionNames)...)
	}
	return candidates
}

func buildSaleAttributeCandidatesForDimension(
	index *templateIndex,
	attributes []sheinattribute.AttributeInfo,
	dimension SourceVariantDimension,
	sourceOrder int,
	seen map[string]struct{},
	sourceDimensionNames []string,
) []saleAttributeCandidate {
	if index == nil {
		return nil
	}
	result := make([]saleAttributeCandidate, 0, len(attributes))

	if match := index.Match(dimension.Name, dimension.SampleValue); match.AttributeID > 0 {
		fitCount, fitTotal := countTemplateValueFits(index, match.Name, dimension.Values)
		if candidate, ok := newSaleAttributeCandidate(dimension, sourceOrder, templateAttributeOrder(attributes, match.AttributeID), match, fitCount, fitTotal); ok {
			appendSaleAttributeCandidate(&result, seen, candidate)
		}
	}

	for order, attr := range attributes {
		if isTechnicalSaleSourceDimension(dimension.Name) {
			continue
		}
		if len(attr.AttributeValueInfoList) == 0 {
			fitTotal := len(uniqueNormalizedValues(dimension.Values))
			if fitTotal == 0 || !canUseCustomSaleAttributeCandidate(dimension, attr, sourceDimensionNames) {
				continue
			}
			match := buildTemplateAttributeMatch(attr, dimension.SampleValue)
			match.MatchedBy = "custom_attribute_value_candidate"
			candidate, ok := newSaleAttributeCandidate(dimension, sourceOrder, order, match, fitTotal, fitTotal)
			if !ok {
				continue
			}
			appendSaleAttributeCandidate(&result, seen, candidate)
			continue
		}
		fitCount, fitTotal := countTemplateValueFitsForAttribute(attr, dimension.Values)
		if fitCount > 0 && !canUseTemplateValueSaleAttributeCandidate(dimension, attr, sourceDimensionNames) {
			continue
		}
		if fitCount == 0 {
			if shouldAllowZeroFitCustomSaleCandidate(dimension, attr, order) &&
				len(uniqueNormalizedValues(dimension.Values)) > 0 &&
				canUseCustomSaleAttributeCandidate(dimension, attr, sourceDimensionNames) {
				match := buildTemplateAttributeMatch(attr, dimension.SampleValue)
				match.MatchedBy = "custom_attribute_value_candidate"
				candidate, ok := newSaleAttributeCandidate(dimension, sourceOrder, order, match, len(uniqueNormalizedValues(dimension.Values)), len(uniqueNormalizedValues(dimension.Values)))
				if !ok {
					continue
				}
				appendSaleAttributeCandidate(&result, seen, candidate)
			}
			continue
		}
		match := buildTemplateAttributeMatch(attr, dimension.SampleValue)
		candidate, ok := newSaleAttributeCandidate(dimension, sourceOrder, order, match, fitCount, fitTotal)
		if !ok {
			continue
		}
		appendSaleAttributeCandidate(&result, seen, candidate)
	}

	return result
}

func shouldAllowZeroFitCustomSaleCandidate(dimension SourceVariantDimension, attr sheinattribute.AttributeInfo, order int) bool {
	if order == 0 {
		return true
	}
	return isGenericSecondaryName(dimension.Name) || isGenericSecondaryName(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
}

func sourceSaleDimensionNames(dimensions []SourceVariantDimension) []string {
	names := make([]string, 0, len(dimensions))
	for _, dimension := range dimensions {
		if name := strings.TrimSpace(dimension.Name); name != "" {
			names = append(names, name)
		}
	}
	return names
}

func canUseCustomSaleAttributeCandidate(dimension SourceVariantDimension, attr sheinattribute.AttributeInfo, sourceDimensionNames []string) bool {
	if isTechnicalSaleSourceDimension(dimension.Name) {
		return false
	}
	if sourceDimensionMatchesSaleTemplate(dimension.Name, attr) {
		return true
	}
	return isAIStyleSourceDimension(dimension.Name) && !hasSourceDimensionForSaleTemplate(sourceDimensionNames, attr)
}

func canUseTemplateValueSaleAttributeCandidate(dimension SourceVariantDimension, attr sheinattribute.AttributeInfo, sourceDimensionNames []string) bool {
	if isTechnicalSaleSourceDimension(dimension.Name) {
		return false
	}
	if sourceDimensionMatchesSaleTemplate(dimension.Name, attr) {
		return true
	}
	return isAIStyleSourceDimension(dimension.Name) && !hasSourceDimensionForSaleTemplate(sourceDimensionNames, attr)
}

func sourceDimensionMatchesSaleTemplate(name string, attr sheinattribute.AttributeInfo) bool {
	return matchesAnyName(name, collectAttributeNames(attr))
}

func hasSourceDimensionForSaleTemplate(names []string, attr sheinattribute.AttributeInfo) bool {
	for _, name := range names {
		if sourceDimensionMatchesSaleTemplate(name, attr) {
			return true
		}
	}
	return false
}

func isAIStyleSourceDimension(name string) bool {
	return normalizeText(name) == "ai style"
}

func isTechnicalSaleSourceDimension(name string) bool {
	normalized := normalizeText(name)
	if normalized == "" {
		return true
	}
	for _, token := range strings.Fields(normalized) {
		switch token {
		case "sku", "id", "code":
			return true
		}
	}
	return false
}

func appendSaleAttributeCandidate(
	candidates *[]saleAttributeCandidate,
	seen map[string]struct{},
	candidate saleAttributeCandidate,
) bool {
	key := normalizeText(candidate.SourceName) + "#" + strconv.Itoa(candidate.AttributeID)
	if _, ok := seen[key]; ok {
		return false
	}
	seen[key] = struct{}{}
	*candidates = append(*candidates, candidate)
	return true
}

func newSaleAttributeCandidate(
	dimension SourceVariantDimension,
	sourceOrder int,
	templateOrder int,
	match ResolvedAttribute,
	valueFitCount int,
	valueFitTotal int,
) (saleAttributeCandidate, bool) {
	if match.AttributeID == 0 {
		return saleAttributeCandidate{}, false
	}
	candidate := saleAttributeCandidate{
		SourceName:    dimension.Name,
		Values:        append([]string(nil), dimension.Values...),
		SampleValue:   dimension.SampleValue,
		TemplateName:  match.Name,
		AttributeID:   match.AttributeID,
		TemplateOrder: templateOrder,
		SKCScope:      match.SKCScope,
		Required:      match.Required,
		Important:     match.Important,
		SourceOrder:   sourceOrder,
		DistinctCount: dimension.DistinctCount,
		ValueFitCount: valueFitCount,
		ValueFitTotal: valueFitTotal,
		Match:         match,
	}
	candidate.PrimaryScore = primaryPriority(candidate)
	candidate.SecondaryScore = secondaryPriority(candidate)
	return candidate, true
}

func templateAttributeOrder(attributes []sheinattribute.AttributeInfo, attributeID int) int {
	for i, attr := range attributes {
		if attr.AttributeID == attributeID {
			return i
		}
	}
	return len(attributes)
}

func buildTemplateAttributeMatch(attr sheinattribute.AttributeInfo, sampleValue string) ResolvedAttribute {
	return ResolvedAttribute{
		Name:                firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		Value:               strings.TrimSpace(sampleValue),
		AttributeID:         attr.AttributeID,
		AttributeExtraValue: strings.TrimSpace(sampleValue),
		MatchedBy:           "attribute_value_fit",
		Required:            isTemplateRequired(attr),
		Important:           isTemplateImportant(attr),
		SKCScope:            attr.SKCScope != nil && *attr.SKCScope,
	}
}

func countTemplateValueFitsForAttribute(attr sheinattribute.AttributeInfo, values []string) (int, int) {
	normalizedValues := uniqueNormalizedValues(values)
	if len(normalizedValues) == 0 {
		return 0, 0
	}
	fitCount := 0
	for _, value := range normalizedValues {
		if _, ok := matchSaleAttributeValueExact(attr, value, ""); ok {
			fitCount++
			continue
		}
		if _, ok := matchSaleAttributeValueNormalized(attr, value, ""); ok {
			fitCount++
		}
	}
	return fitCount, len(normalizedValues)
}
