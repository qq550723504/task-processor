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
	SKCScope       bool
	Required       bool
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
	for i, dimension := range dimensions {
		candidates = append(candidates, buildSaleAttributeCandidatesForDimension(index, attributes, dimension, i, seen)...)
	}
	return candidates
}

func buildSaleAttributeCandidatesForDimension(
	index *templateIndex,
	attributes []sheinattribute.AttributeInfo,
	dimension SourceVariantDimension,
	sourceOrder int,
	seen map[string]struct{},
) []saleAttributeCandidate {
	if index == nil {
		return nil
	}
	result := make([]saleAttributeCandidate, 0, len(attributes))

	if match := index.Match(dimension.Name, dimension.SampleValue); match.AttributeID > 0 {
		fitCount, fitTotal := countTemplateValueFits(index, match.Name, dimension.Values)
		if candidate, ok := newSaleAttributeCandidate(dimension, sourceOrder, match, fitCount, fitTotal); ok {
			if appendSaleAttributeCandidate(&result, seen, candidate) {
				// Keep the name-matched candidate even when fit is zero so resolver can explain why it was rejected.
			}
		}
	}

	for _, attr := range attributes {
		fitCount, fitTotal := countTemplateValueFitsForAttribute(attr, dimension.Values)
		if fitCount == 0 {
			continue
		}
		match := buildTemplateAttributeMatch(attr, dimension.SampleValue)
		candidate, ok := newSaleAttributeCandidate(dimension, sourceOrder, match, fitCount, fitTotal)
		if !ok {
			continue
		}
		appendSaleAttributeCandidate(&result, seen, candidate)
	}

	return result
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
		SKCScope:      match.SKCScope,
		Required:      match.Required,
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

func buildTemplateAttributeMatch(attr sheinattribute.AttributeInfo, sampleValue string) ResolvedAttribute {
	return ResolvedAttribute{
		Name:                firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		Value:               strings.TrimSpace(sampleValue),
		AttributeID:         attr.AttributeID,
		AttributeExtraValue: strings.TrimSpace(sampleValue),
		MatchedBy:           "attribute_value_fit",
		Required:            isTemplateRequired(attr),
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
