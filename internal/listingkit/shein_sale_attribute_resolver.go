package listingkit

import (
	"slices"
	"sort"
	"strings"

	"task-processor/internal/productenrich"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type sheinSaleAttributeResolver struct {
	api SheinAttributeAPI
}

func NewSheinSaleAttributeResolver(api SheinAttributeAPI) SheinSaleAttributeResolver {
	return &sheinSaleAttributeResolver{api: api}
}

func (r *sheinSaleAttributeResolver) Resolve(req *GenerateRequest, canonical *productenrich.CanonicalProduct, pkg *SheinPackage) *SheinSaleAttributeResolution {
	resolution := &SheinSaleAttributeResolution{
		Status:     "unresolved",
		Source:     "fallback",
		CategoryID: sheinCategoryID(pkg),
	}
	if resolution.CategoryID == 0 {
		resolution.ReviewNotes = append(resolution.ReviewNotes, "缺少 SHEIN category_id，无法解析销售属性")
		return resolution
	}
	if r.api == nil {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "缺少 SHEIN AttributeAPI，当前无法加载销售属性模板")
		return resolution
	}

	templates, err := r.api.GetAttributeTemplates(resolution.CategoryID)
	if err != nil {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 销售属性模板加载失败: "+err.Error())
		return resolution
	}
	if templates == nil || len(templates.Data) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 销售属性模板为空")
		return resolution
	}

	saleAttributes := filterSaleScopeAttributes(templates.Data[0].AttributeInfos)
	index := newSheinTemplateIndex(saleAttributes)
	if len(index.attributes) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "当前类目未识别到可用的销售属性模板")
		return resolution
	}

	candidates := buildSheinSaleAttributeCandidates(pkg, saleAttributes)
	primaryCandidate, secondaryCandidate := selectSheinPrimarySecondaryCandidates(candidates)
	resolution.Candidates = buildSheinSaleAttributeCandidateInfos(candidates, primaryCandidate, secondaryCandidate)
	if primaryCandidate != nil {
		match := index.Match(primaryCandidate.Name, primaryCandidate.SampleValue)
		if match.AttributeID > 0 {
			resolved := toSheinResolvedSaleAttribute(match, "skc")
			resolution.PrimaryAttributeID = resolved.AttributeID
			resolution.SKCAttributes = append(resolution.SKCAttributes, resolved)
		}
	}

	if secondaryCandidate != nil && secondaryCandidate.AttributeID != resolution.PrimaryAttributeID {
		match := index.Match(secondaryCandidate.Name, secondaryCandidate.SampleValue)
		if match.AttributeID > 0 && match.AttributeID != resolution.PrimaryAttributeID {
			resolved := toSheinResolvedSaleAttribute(match, "sku")
			resolution.SecondaryAttributeID = resolved.AttributeID
			resolution.SKUAttributes = append(resolution.SKUAttributes, resolved)
		}
	}
	resolution.SelectionSummary = buildSheinSaleAttributeSelectionSummary(primaryCandidate, secondaryCandidate)

	resolution.Source = "sale_attribute_templates"
	switch {
	case resolution.PrimaryAttributeID > 0 && (resolution.SecondaryAttributeID > 0 || secondaryCandidate == nil):
		resolution.Status = "resolved"
	case resolution.PrimaryAttributeID > 0 || resolution.SecondaryAttributeID > 0:
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 销售属性已部分解析，仍建议人工确认变体规格")
	default:
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "未命中可用的 SHEIN 销售属性映射")
	}
	return resolution
}

func filterSaleScopeAttributes(attributes []sheinattribute.AttributeInfo) []sheinattribute.AttributeInfo {
	result := make([]sheinattribute.AttributeInfo, 0, len(attributes))
	for _, attr := range attributes {
		if attr.AttributeType == 1 {
			result = append(result, attr)
			continue
		}
		if attr.SKCScope != nil && *attr.SKCScope {
			result = append(result, attr)
			continue
		}
		name := normalizeSheinText(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
		switch name {
		case "color", "colour", "size", "style", "pattern", "capacity":
			result = append(result, attr)
		}
	}
	return result
}

type sheinSaleAttributeCandidate struct {
	Name          string
	SampleValue   string
	AttributeID   int
	SKCScope      bool
	Required      bool
	SKCDistinct   int
	SKUDistinct   int
	TotalDistinct int
}

func buildSheinSaleAttributeCandidates(pkg *SheinPackage, attributes []sheinattribute.AttributeInfo) []sheinSaleAttributeCandidate {
	if pkg == nil || len(pkg.SkcList) == 0 || len(attributes) == 0 {
		return nil
	}
	candidates := make([]sheinSaleAttributeCandidate, 0, len(attributes))
	for _, attr := range attributes {
		names := collectSheinAttributeNames(attr)
		skcValues := collectMatchingValuesFromSKC(pkg.SkcList, names)
		skuValues := collectMatchingValuesFromSKU(pkg.SkcList, names)
		allValues := append(append([]string(nil), skcValues...), skuValues...)
		allValues = uniqueNormalizedValues(allValues)
		if len(allValues) == 0 {
			continue
		}
		sample := firstNonEmpty(firstValue(skcValues), firstValue(skuValues))
		candidates = append(candidates, sheinSaleAttributeCandidate{
			Name:          firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
			SampleValue:   sample,
			AttributeID:   attr.AttributeID,
			SKCScope:      attr.SKCScope != nil && *attr.SKCScope,
			Required:      isSheinTemplateRequired(attr),
			SKCDistinct:   len(uniqueNormalizedValues(skcValues)),
			SKUDistinct:   len(uniqueNormalizedValues(skuValues)),
			TotalDistinct: len(allValues),
		})
	}
	return candidates
}

func selectSheinPrimarySecondaryCandidates(candidates []sheinSaleAttributeCandidate) (*sheinSaleAttributeCandidate, *sheinSaleAttributeCandidate) {
	if len(candidates) == 0 {
		return nil, nil
	}
	sorted := append([]sheinSaleAttributeCandidate(nil), candidates...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if primaryPriority(a) != primaryPriority(b) {
			return primaryPriority(a) > primaryPriority(b)
		}
		if a.SKCDistinct != b.SKCDistinct {
			return a.SKCDistinct > b.SKCDistinct
		}
		if a.TotalDistinct != b.TotalDistinct {
			return a.TotalDistinct > b.TotalDistinct
		}
		return a.AttributeID < b.AttributeID
	})
	primary := &sorted[0]

	secondaryPool := make([]sheinSaleAttributeCandidate, 0, len(sorted))
	for _, candidate := range sorted[1:] {
		if candidate.AttributeID != primary.AttributeID {
			secondaryPool = append(secondaryPool, candidate)
		}
	}
	if len(secondaryPool) == 0 {
		return primary, nil
	}
	sort.SliceStable(secondaryPool, func(i, j int) bool {
		a, b := secondaryPool[i], secondaryPool[j]
		if secondaryPriority(a) != secondaryPriority(b) {
			return secondaryPriority(a) > secondaryPriority(b)
		}
		if a.SKUDistinct != b.SKUDistinct {
			return a.SKUDistinct > b.SKUDistinct
		}
		if a.TotalDistinct != b.TotalDistinct {
			return a.TotalDistinct > b.TotalDistinct
		}
		return a.AttributeID < b.AttributeID
	})
	if secondaryPriority(secondaryPool[0]) == 0 {
		return primary, nil
	}
	secondary := secondaryPool[0]
	return primary, &secondary
}

func primaryPriority(candidate sheinSaleAttributeCandidate) int {
	score := 0
	if candidate.Required {
		score += 8
	}
	if candidate.SKCScope {
		score += 6
	}
	if candidate.SKCDistinct > 1 {
		score += 4
	}
	if isSheinGenericSecondaryName(candidate.Name) {
		score -= 2
	}
	return score
}

func secondaryPriority(candidate sheinSaleAttributeCandidate) int {
	score := 0
	if candidate.SKUDistinct > 1 {
		score += 6
	}
	if !candidate.SKCScope {
		score += 2
	}
	if candidate.TotalDistinct > 1 {
		score += 2
	}
	return score
}

func buildSheinSaleAttributeCandidateInfos(candidates []sheinSaleAttributeCandidate, primary, secondary *sheinSaleAttributeCandidate) []SheinSaleAttributeCandidateInfo {
	if len(candidates) == 0 {
		return nil
	}
	result := make([]SheinSaleAttributeCandidateInfo, 0, len(candidates))
	for _, candidate := range candidates {
		info := SheinSaleAttributeCandidateInfo{
			Name:           candidate.Name,
			AttributeID:    candidate.AttributeID,
			SKCScope:       candidate.SKCScope,
			Required:       candidate.Required,
			SKCDistinct:    candidate.SKCDistinct,
			SKUDistinct:    candidate.SKUDistinct,
			TotalDistinct:  candidate.TotalDistinct,
			PrimaryScore:   primaryPriority(candidate),
			SecondaryScore: secondaryPriority(candidate),
			SampleValue:    candidate.SampleValue,
			Reasons:        explainSheinSaleAttributeCandidate(candidate),
		}
		switch {
		case primary != nil && candidate.AttributeID == primary.AttributeID:
			info.SelectedScope = "skc"
		case secondary != nil && candidate.AttributeID == secondary.AttributeID:
			info.SelectedScope = "sku"
		}
		result = append(result, info)
	}
	sort.SliceStable(result, func(i, j int) bool {
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

func explainSheinSaleAttributeCandidate(candidate sheinSaleAttributeCandidate) []string {
	reasons := make([]string, 0, 4)
	if candidate.Required {
		reasons = append(reasons, "模板标记为必填销售属性")
	}
	if candidate.SKCScope {
		reasons = append(reasons, "模板标记为 SKC scope")
	}
	if candidate.SKCDistinct > 1 {
		reasons = append(reasons, "在 SKC 层存在多值差异")
	}
	if candidate.SKUDistinct > 1 {
		reasons = append(reasons, "在 SKU 层存在多值差异")
	}
	if len(reasons) == 0 {
		reasons = append(reasons, "仅作为弱候选保留")
	}
	return reasons
}

func buildSheinSaleAttributeSelectionSummary(primary, secondary *sheinSaleAttributeCandidate) []string {
	var summary []string
	if primary != nil {
		summary = append(summary, "主销售属性按模板和变体差异选择为 "+primary.Name)
	}
	if secondary != nil {
		summary = append(summary, "次销售属性按剩余候选和 SKU 差异选择为 "+secondary.Name)
	}
	return summary
}

func collectSheinAttributeNames(attr sheinattribute.AttributeInfo) []string {
	names := []string{attr.AttributeName, attr.AttributeNameEn}
	for _, extra := range sheinAttributeAliases(normalizeSheinText(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))) {
		names = append(names, extra)
	}
	return names
}

func collectMatchingValuesFromSKC(skcs []SheinSKCPackage, names []string) []string {
	var values []string
	for _, skc := range skcs {
		for attrKey, value := range skc.Attributes {
			if matchesAnySheinName(attrKey, names) && strings.TrimSpace(value) != "" {
				values = append(values, value)
			}
		}
	}
	return values
}

func collectMatchingValuesFromSKU(skcs []SheinSKCPackage, names []string) []string {
	var values []string
	for _, skc := range skcs {
		for _, sku := range skc.SKUs {
			for attrKey, value := range sku.Attributes {
				if matchesAnySheinName(attrKey, names) && strings.TrimSpace(value) != "" {
					values = append(values, value)
				}
			}
		}
	}
	return values
}

func matchesAnySheinName(attrKey string, names []string) bool {
	attrNorm := normalizeSheinText(attrKey)
	for _, name := range names {
		if normalizeSheinText(name) == attrNorm && attrNorm != "" {
			return true
		}
	}
	return false
}

func uniqueNormalizedValues(values []string) []string {
	seen := make([]string, 0, len(values))
	for _, value := range values {
		norm := normalizeSheinText(value)
		if norm == "" || slices.Contains(seen, norm) {
			continue
		}
		seen = append(seen, norm)
	}
	return seen
}

func firstValue(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func isSheinGenericSecondaryName(name string) bool {
	switch normalizeSheinText(name) {
	case "size", "capacity", "packsize", "length":
		return true
	default:
		return false
	}
}

func toSheinResolvedSaleAttribute(attr SheinResolvedAttribute, scope string) SheinResolvedSaleAttribute {
	return SheinResolvedSaleAttribute{
		Scope:            scope,
		Name:             attr.Name,
		Value:            attr.Value,
		AttributeID:      attr.AttributeID,
		AttributeValueID: attr.AttributeValueID,
		MatchedBy:        attr.MatchedBy,
	}
}
