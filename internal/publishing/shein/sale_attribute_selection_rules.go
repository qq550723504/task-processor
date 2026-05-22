package shein

import (
	"sort"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

func selectTemplateOrderedCandidates(candidates []saleAttributeCandidate, attributes []sheinattribute.AttributeInfo) (*saleAttributeCandidate, *saleAttributeCandidate) {
	first := firstSaleAttributeTemplate(attributes)
	if first == nil {
		return nil, nil
	}
	primary := bestCandidateForAttribute(candidates, first.AttributeID, nil)
	if primary == nil {
		return nil, nil
	}
	for _, attr := range attributes[1:] {
		if secondary := bestCandidateForAttribute(candidates, attr.AttributeID, primary); secondary != nil {
			return primary, secondary
		}
	}
	return primary, nil
}

func resolvePrimarySecondaryCandidates(candidates []saleAttributeCandidate, attributes []sheinattribute.AttributeInfo) (*saleAttributeCandidate, *saleAttributeCandidate) {
	if primary, secondary := selectTemplateOrderedCandidates(candidates, attributes); primary != nil {
		return primary, secondary
	}
	if hasMarkedPrimarySaleTemplate(attributes) {
		return nil, nil
	}
	return selectPrimarySecondaryCandidates(candidates)
}

func bestCandidateForAttribute(candidates []saleAttributeCandidate, attributeID int, primary *saleAttributeCandidate) *saleAttributeCandidate {
	var best *saleAttributeCandidate
	for i := range candidates {
		candidate := &candidates[i]
		if candidate.AttributeID != attributeID {
			continue
		}
		if candidate.ValueFitCount == 0 && (primary == nil || !canUseFallbackSecondaryCandidate(*candidate)) {
			continue
		}
		if primary != nil && (candidate.AttributeID == primary.AttributeID || normalizeText(candidate.SourceName) == normalizeText(primary.SourceName)) {
			continue
		}
		if isBetterTemplateAttributeCandidate(candidate, best) {
			best = candidate
		}
	}
	return best
}

func isBetterTemplateAttributeCandidate(candidate, best *saleAttributeCandidate) bool {
	if best == nil {
		return true
	}
	if candidate.ValueFitCount != best.ValueFitCount {
		return candidate.ValueFitCount > best.ValueFitCount
	}
	return candidate.DistinctCount > best.DistinctCount
}

func isWeakPrimarySaleAttributeCandidate(candidate saleAttributeCandidate) bool {
	if isAIStyleSourceDimension(candidate.SourceName) {
		return true
	}
	return candidate.ValueFitCount == 0
}

func canUseFallbackPrimaryCandidate(candidate saleAttributeCandidate) bool {
	if candidate.ValueFitCount > 0 {
		return true
	}
	if isAIStyleSourceDimension(candidate.SourceName) {
		return false
	}
	return isPromotedPrimarySaleCandidate(candidate)
}

func selectPrimarySecondaryCandidates(candidates []saleAttributeCandidate) (*saleAttributeCandidate, *saleAttributeCandidate) {
	if len(candidates) == 0 {
		return nil, nil
	}
	sorted := append([]saleAttributeCandidate(nil), candidates...)
	sortPrimaryCandidatePool(sorted)
	primary := pickPrimaryCandidate(sorted)
	if primary == nil {
		return nil, nil
	}
	secondaryPool := buildSecondaryCandidatePool(sorted[1:], primary)
	if len(secondaryPool) == 0 {
		return primary, nil
	}
	sortSecondaryCandidatePool(secondaryPool)
	if secondaryPool[0].SecondaryScore == 0 && !canUseFallbackSecondaryCandidate(secondaryPool[0]) {
		return primary, nil
	}
	secondary := secondaryPool[0]
	return primary, &secondary
}

func sortPrimaryCandidatePool(candidates []saleAttributeCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if canUseFallbackPrimaryCandidate(a) != canUseFallbackPrimaryCandidate(b) {
			return canUseFallbackPrimaryCandidate(a)
		}
		if a.ValueFitCount == 0 || b.ValueFitCount == 0 {
			if a.ValueFitCount != b.ValueFitCount {
				return a.ValueFitCount > b.ValueFitCount
			}
		}
		if isPromotedPrimarySaleCandidate(a) != isPromotedPrimarySaleCandidate(b) {
			return isPromotedPrimarySaleCandidate(a)
		}
		if a.TemplateOrder != b.TemplateOrder {
			return a.TemplateOrder < b.TemplateOrder
		}
		if saleCandidateMatchQuality(a) != saleCandidateMatchQuality(b) {
			return saleCandidateMatchQuality(a) > saleCandidateMatchQuality(b)
		}
		if a.PrimaryScore != b.PrimaryScore {
			return a.PrimaryScore > b.PrimaryScore
		}
		if a.DistinctCount != b.DistinctCount {
			return a.DistinctCount > b.DistinctCount
		}
		return a.SourceOrder < b.SourceOrder
	})
}

func pickPrimaryCandidate(candidates []saleAttributeCandidate) *saleAttributeCandidate {
	for i := range candidates {
		if candidates[i].ValueFitCount == 0 && !canUseFallbackPrimaryCandidate(candidates[i]) {
			continue
		}
		return &candidates[i]
	}
	return nil
}

func buildSecondaryCandidatePool(candidates []saleAttributeCandidate, primary *saleAttributeCandidate) []saleAttributeCandidate {
	secondaryPool := make([]saleAttributeCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ValueFitCount == 0 && !canUseFallbackSecondaryCandidate(candidate) {
			continue
		}
		if candidate.SourceName == primary.SourceName || candidate.AttributeID == primary.AttributeID {
			continue
		}
		secondaryPool = append(secondaryPool, candidate)
	}
	return secondaryPool
}

func sortSecondaryCandidatePool(candidates []saleAttributeCandidate) {
	sort.SliceStable(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if a.SecondaryScore != b.SecondaryScore {
			return a.SecondaryScore > b.SecondaryScore
		}
		if a.DistinctCount != b.DistinctCount {
			return a.DistinctCount > b.DistinctCount
		}
		return a.SourceOrder < b.SourceOrder
	})
}

func saleCandidateMatchQuality(candidate saleAttributeCandidate) int {
	if candidate.Match.MatchedBy == "custom_attribute_value_candidate" {
		if isAIStyleSourceDimension(candidate.SourceName) {
			return 1
		}
		return 2
	}
	return 3
}

func isPromotedPrimarySaleCandidate(candidate saleAttributeCandidate) bool {
	if isAIStyleSourceDimension(candidate.SourceName) {
		return false
	}
	if isColorSaleCandidate(candidate) && candidate.Important {
		return true
	}
	return candidate.Required && candidate.DistinctCount > 1
}

func isColorSaleCandidate(candidate saleAttributeCandidate) bool {
	return matchesAnyName(candidate.SourceName, []string{"color", "colour", "颜色", "颜色分类"}) ||
		matchesAnyName(candidate.TemplateName, []string{"color", "colour", "颜色", "颜色分类"})
}

func primaryPriority(candidate saleAttributeCandidate) int {
	score := 0
	if candidate.Required {
		score += 8
	}
	if candidate.SKCScope {
		score += 6
	}
	if candidate.DistinctCount > 1 {
		score += 4
	}
	score += candidate.ValueFitCount * 3
	return score
}

func secondaryPriority(candidate saleAttributeCandidate) int {
	score := 0
	if candidate.DistinctCount > 1 {
		score += 6
	}
	if !candidate.SKCScope {
		score += 2
	}
	if isGenericSecondaryName(candidate.TemplateName) {
		score += 2
	}
	score += candidate.ValueFitCount * 3
	return score
}

func selectCandidatesBySourceDimensions(candidates []saleAttributeCandidate, primaryName, secondaryName string) (*saleAttributeCandidate, *saleAttributeCandidate) {
	if len(candidates) == 0 {
		return nil, nil
	}
	primaryCandidate, secondaryCandidate := findSourceDimensionCandidates(candidates, primaryName, secondaryName)
	if primaryCandidate != nil && secondaryCandidate != nil && normalizeText(primaryCandidate.SourceName) == normalizeText(secondaryCandidate.SourceName) {
		secondaryCandidate = nil
	}
	return primaryCandidate, secondaryCandidate
}

func findSourceDimensionCandidates(candidates []saleAttributeCandidate, primaryName, secondaryName string) (*saleAttributeCandidate, *saleAttributeCandidate) {
	var primaryCandidate *saleAttributeCandidate
	var secondaryCandidate *saleAttributeCandidate
	for i := range candidates {
		candidate := &candidates[i]
		if candidate.ValueFitCount == 0 && !canUseFallbackSecondaryCandidate(*candidate) {
			continue
		}
		switch {
		case primaryCandidate == nil && normalizeText(candidate.SourceName) == normalizeText(primaryName):
			primaryCandidate = candidate
		case secondaryCandidate == nil && normalizeText(candidate.SourceName) == normalizeText(secondaryName):
			secondaryCandidate = candidate
		}
	}
	return primaryCandidate, secondaryCandidate
}

func isGenericSecondaryName(name string) bool {
	switch normalizeText(name) {
	case "size", "capacity":
		return true
	}
	return false
}

func canUseFallbackSecondaryCandidate(candidate saleAttributeCandidate) bool {
	if candidate.ValueFitCount > 0 {
		return true
	}
	if candidate.DistinctCount <= 1 {
		return false
	}
	if isAIStyleSourceDimension(candidate.SourceName) {
		return false
	}
	return isGenericSecondaryName(candidate.TemplateName) || isGenericSecondaryName(candidate.SourceName)
}
