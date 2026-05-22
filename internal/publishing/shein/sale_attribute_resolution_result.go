package shein

import (
	"sort"
	"strconv"

	openaiclient "task-processor/internal/infra/clients/openai"
)

func applySelectedCandidate(
	index *templateIndex,
	candidate *saleAttributeCandidate,
	scope string,
	api AttributeAPI,
	categoryID int,
	spuName string,
	llm openaiclient.ChatCompleter,
	resolution *SaleAttributeResolution,
) {
	if candidate == nil || candidate.Match.AttributeID <= 0 || resolution == nil {
		return
	}
	resolved := toResolvedSaleAttribute(candidate.Match, scope)
	switch scope {
	case "skc":
		resolution.PrimaryAttributeID = resolved.AttributeID
		resolution.PrimarySourceDimension = candidate.SourceName
		resolution.SKCAttributes = append(resolution.SKCAttributes, resolved)
		assignments, relations, notes, summary := buildValueAssignments(
			candidate.Values,
			candidate.SourceName,
			candidate.TemplateName,
			"skc",
			index,
			api,
			categoryID,
			spuName,
			llm,
		)
		resolution.skcValueAssignments = assignments
		resolution.SKCValueAssignments = cloneResolvedSaleAttributeMap(assignments)
		resolution.SKCAttributes = resolvedSaleAttributesForPublicView(resolution.SKCAttributes, assignments)
		resolution.CustomAttributeRelation = dedupeCustomAttributeRelations(append(resolution.CustomAttributeRelation, relations...))
		resolution.ReviewNotes = dedupeStrings(append(resolution.ReviewNotes, notes...))
		applySaleAttributeValueSummary(resolution, summary)
	case "sku":
		resolution.SecondaryAttributeID = resolved.AttributeID
		resolution.SecondarySourceDimension = candidate.SourceName
		resolution.SKUAttributes = append(resolution.SKUAttributes, resolved)
		assignments, relations, notes, summary := buildValueAssignments(
			candidate.Values,
			candidate.SourceName,
			candidate.TemplateName,
			"sku",
			index,
			api,
			categoryID,
			spuName,
			llm,
		)
		resolution.skuValueAssignments = assignments
		resolution.SKUValueAssignments = cloneResolvedSaleAttributeMap(assignments)
		resolution.SKUAttributes = resolvedSaleAttributesForPublicView(resolution.SKUAttributes, assignments)
		resolution.CustomAttributeRelation = dedupeCustomAttributeRelations(append(resolution.CustomAttributeRelation, relations...))
		resolution.ReviewNotes = dedupeStrings(append(resolution.ReviewNotes, notes...))
		applySaleAttributeValueSummary(resolution, summary)
	}
}

func resolvedSaleAttributesForPublicView(current []ResolvedSaleAttribute, assignments map[string]ResolvedSaleAttribute) []ResolvedSaleAttribute {
	if len(current) == 0 || len(assignments) == 0 {
		return current
	}
	for _, assignment := range assignments {
		if !saleAttributeHasResolvedValue(assignment) {
			continue
		}
		next := append([]ResolvedSaleAttribute(nil), current...)
		next[0] = assignment
		return next
	}
	return current
}

func buildSaleAttributeCandidateInfos(candidates []saleAttributeCandidate, primary, secondary *saleAttributeCandidate) []SaleAttributeCandidateInfo {
	if len(candidates) == 0 {
		return nil
	}
	result := make([]SaleAttributeCandidateInfo, 0, len(candidates))
	for _, candidate := range candidates {
		info := SaleAttributeCandidateInfo{
			SourceDimension: candidate.SourceName,
			Name:            candidate.TemplateName,
			AttributeID:     candidate.AttributeID,
			SKCScope:        candidate.SKCScope,
			Required:        candidate.Required,
			Important:       candidate.Important,
			SKCDistinct:     candidate.DistinctCount,
			SKUDistinct:     candidate.DistinctCount,
			TotalDistinct:   candidate.DistinctCount,
			PrimaryScore:    candidate.PrimaryScore,
			SecondaryScore:  candidate.SecondaryScore,
			SampleValue:     candidate.SampleValue,
			Reasons:         explainSaleAttributeCandidate(candidate),
		}
		switch {
		case primary != nil && candidate.SourceName == primary.SourceName && candidate.AttributeID == primary.AttributeID:
			info.SelectedScope = "skc"
		case secondary != nil && candidate.SourceName == secondary.SourceName && candidate.AttributeID == secondary.AttributeID:
			info.SelectedScope = "sku"
		}
		result = append(result, info)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].SelectedScope != result[j].SelectedScope {
			if result[i].SelectedScope == "" || result[j].SelectedScope == "" {
				return result[i].SelectedScope != ""
			}
			return selectedSaleScopeRank(result[i].SelectedScope) > selectedSaleScopeRank(result[j].SelectedScope)
		}
		if result[i].SelectedScope != "" && result[j].SelectedScope != "" {
			return selectedSaleScopeRank(result[i].SelectedScope) > selectedSaleScopeRank(result[j].SelectedScope)
		}
		if result[i].PrimaryScore != result[j].PrimaryScore {
			return result[i].PrimaryScore > result[j].PrimaryScore
		}
		if result[i].SecondaryScore != result[j].SecondaryScore {
			return result[i].SecondaryScore > result[j].SecondaryScore
		}
		return result[i].AttributeID < result[j].AttributeID
	})
	return result
}

func selectedSaleScopeRank(scope string) int {
	switch scope {
	case "skc":
		return 2
	case "sku":
		return 1
	default:
		return 0
	}
}

func explainSaleAttributeCandidate(candidate saleAttributeCandidate) []string {
	reasons := make([]string, 0, 4)
	reasons = append(reasons, "源维度为 "+candidate.SourceName)
	if candidate.Required {
		reasons = append(reasons, "模板标记为必填销售属性")
	}
	if candidate.Important {
		reasons = append(reasons, "模板标记为重要销售属性")
	}
	if candidate.SKCScope {
		reasons = append(reasons, "模板标记为 SKC scope")
	}
	if candidate.DistinctCount > 1 {
		reasons = append(reasons, "源维度存在多值差异")
	}
	if candidate.ValueFitTotal > 0 {
		reasons = append(reasons, buildValueFitSummary(candidate.ValueFitCount, candidate.ValueFitTotal))
	}
	return reasons
}

func buildSelectionSummary(primary, secondary *saleAttributeCandidate) []string {
	var summary []string
	if primary != nil {
		summary = append(summary, "主销售属性使用源维度 "+primary.SourceName+" 映射到 "+primary.TemplateName)
	}
	if secondary != nil {
		summary = append(summary, "次销售属性使用源维度 "+secondary.SourceName+" 映射到 "+secondary.TemplateName)
	}
	return summary
}

type saleAttributeCoverage struct {
	expected int
	resolved int
	complete bool
}

func resolvedSaleAttributeCoverage(candidate *saleAttributeCandidate, assignments map[string]ResolvedSaleAttribute) saleAttributeCoverage {
	if candidate == nil {
		return saleAttributeCoverage{complete: true}
	}
	expected := len(uniqueNormalizedValues(candidate.Values))
	resolved := len(assignments)
	return saleAttributeCoverage{
		expected: expected,
		resolved: resolved,
		complete: expected == 0 || resolved >= expected,
	}
}

func buildValueFitSummary(fitCount, total int) string {
	if total <= 0 {
		return "源值拟合度未知"
	}
	return "模板值拟合度 " + strconv.Itoa(fitCount) + "/" + strconv.Itoa(total)
}

func buildNoFitCandidateReviewNotes(candidates []saleAttributeCandidate) []string {
	notes := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.ValueFitTotal == 0 || candidate.ValueFitCount > 0 {
			continue
		}
		note := `源维度 "` + candidate.SourceName + `" 的值集合与模板属性 "` + candidate.TemplateName +
			`" 无有效拟合（` + buildValueFitSummary(candidate.ValueFitCount, candidate.ValueFitTotal) +
			`），当前不自动映射该销售属性`
		notes = append(notes, note)
	}
	return notes
}

func toResolvedSaleAttribute(match ResolvedAttribute, scope string) ResolvedSaleAttribute {
	return ResolvedSaleAttribute{
		Scope:            scope,
		Name:             match.Name,
		Value:            match.Value,
		AttributeID:      match.AttributeID,
		AttributeValueID: match.AttributeValueID,
		MatchedBy:        match.MatchedBy,
	}
}
