package shein

import (
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"

	"task-processor/internal/catalog/canonical"
	openaiclient "task-processor/internal/infra/clients/openai"
	common "task-processor/internal/publishing/common"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type saleAttributeResolver struct {
	api AttributeAPI
	llm openaiclient.ChatCompleter
}

func NewSaleAttributeResolver(api AttributeAPI, llm openaiclient.ChatCompleter) SaleAttributeResolver {
	return &saleAttributeResolver{api: api, llm: llm}
}

func (r *saleAttributeResolver) Resolve(req *BuildRequest, canonical *canonical.Product, pkg *Package) *SaleAttributeResolution {
	resolution := &SaleAttributeResolution{Status: "unresolved", Source: "fallback", CategoryID: categoryID(pkg)}
	log := sheinLogger("shein/sale_attribute")
	sourceDimensions := buildSourceVariantDimensions(canonical, common.BuildVariants(canonical))
	resolution.SourceDimensions = append([]SourceVariantDimension(nil), sourceDimensions...)
	if sourceSelection := selectSourceDimensions(sourceDimensions, r.llm); sourceSelection != nil {
		resolution.PrimarySourceDimension = sourceSelection.PrimarySourceDimension
		resolution.SecondarySourceDimension = sourceSelection.SecondarySourceDimension
		resolution.SelectionSummary = append(resolution.SelectionSummary, sourceSelection.Reasons...)
		if r.llm != nil {
			resolution.Source = "llm_source_dimensions"
		} else {
			resolution.Source = "source_dimensions_fallback"
		}
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
	log.WithFields(logrus.Fields{
		"category_id":         resolution.CategoryID,
		"source_dimensions":   len(sourceDimensions),
		"primary_dimension":   resolution.PrimarySourceDimension,
		"secondary_dimension": resolution.SecondarySourceDimension,
	}).Info("loading SHEIN sale attribute templates")
	templates, err := r.api.GetAttributeTemplates(resolution.CategoryID)
	if err != nil {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 销售属性模板加载失败: "+err.Error())
		log.WithError(err).WithField("category_id", resolution.CategoryID).Warn("failed to load SHEIN sale attribute templates")
		return resolution
	}
	if templates == nil || len(templates.Data) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 销售属性模板为空")
		log.WithField("category_id", resolution.CategoryID).Warn("SHEIN sale attribute templates are empty")
		return resolution
	}
	saleAttributes := filterSaleScopeAttributes(templates.Data[0].AttributeInfos)
	index := newTemplateIndex(saleAttributes)
	log.WithFields(logrus.Fields{
		"category_id":          resolution.CategoryID,
		"template_groups":      len(templates.Data),
		"sale_attribute_count": len(saleAttributes),
	}).Info("loaded SHEIN sale attribute templates")
	if len(index.attributes) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "当前类目未识别到可用的销售属性模板")
		return resolution
	}
	if len(sourceDimensions) == 0 {
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "缺少源销售属性维度，当前无法构建 SHEIN SKC/SKU 映射")
		return resolution
	}

	candidates := buildSaleAttributeCandidates(sourceDimensions, saleAttributes)
	var primaryCandidate, secondaryCandidate *saleAttributeCandidate
	primaryCandidate, secondaryCandidate = selectPrimarySecondaryCandidates(candidates)
	blockPromptDerivedAIStyleFallback := false
	if shouldSelectSaleAttributeMappingWithLLM(primaryCandidate, sourceDimensions) {
		mappingDimensions := saleAttributeMappingSourceDimensions(sourceDimensions)
		if selection, err := selectSaleAttributeMappingWithLLM(r.llm, mappingDimensions, saleAttributes); err == nil && selection != nil {
			llmPrimary, llmSecondary, augmentedCandidates := matchSelectedCandidates(candidates, selection, sourceDimensions, saleAttributes)
			if shouldUseLLMSaleAttributeMapping(primaryCandidate, llmPrimary) {
				candidates = augmentedCandidates
				primaryCandidate, secondaryCandidate = llmPrimary, llmSecondary
				resolution.Source = "llm_sale_attribute_mapping"
				resolution.ReviewNotes = append(resolution.ReviewNotes, selection.Reasons...)
			}
		}
	}
	if shouldBlockPromptDerivedAIStylePrimary(primaryCandidate, sourceDimensions) && resolution.Source != "llm_sale_attribute_mapping" {
		blockPromptDerivedAIStyleFallback = true
		primaryCandidate = nil
		secondaryCandidate = nil
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN Style Type 不能使用用户设计提示词作为销售属性来源，需使用 SDS 稳定销售维度重新映射")
		if surrogatePrimary, surrogateSecondary, augmentedCandidates, ok := selectStableSaleAttributeSurrogate(candidates, sourceDimensions, saleAttributes); ok {
			candidates = augmentedCandidates
			primaryCandidate = surrogatePrimary
			secondaryCandidate = surrogateSecondary
			blockPromptDerivedAIStyleFallback = false
			resolution.Source = "sds_stable_sale_attribute_surrogate"
			resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 主销售属性使用 SDS 颜色维度作为稳定款式替代来源")
		}
	}
	if primaryCandidate == nil {
		if !blockPromptDerivedAIStyleFallback {
			primaryCandidate, secondaryCandidate = selectCandidatesBySourceDimensions(
				candidates,
				resolution.PrimarySourceDimension,
				resolution.SecondarySourceDimension,
			)
		}
	}
	if primaryCandidate == nil && len(candidates) > 0 {
		resolution.ReviewNotes = append(resolution.ReviewNotes, buildNoFitCandidateReviewNotes(candidates)...)
		resolution.ReviewNotes = append(resolution.ReviewNotes, buildCategoryTemplateGapReviewNotes(candidates, saleAttributes)...)
		resolution.RecommendCategoryReview, resolution.CategoryReviewReason = buildCategoryTemplateGapSummary(candidates, saleAttributes)
	}
	if primaryCandidate != nil {
		resolution.PrimarySourceDimension = primaryCandidate.SourceName
	}
	if secondaryCandidate != nil {
		resolution.SecondarySourceDimension = secondaryCandidate.SourceName
	} else if secondaryCandidate == nil && primaryCandidate != nil && normalizeText(resolution.SecondarySourceDimension) == normalizeText(primaryCandidate.SourceName) {
		resolution.SecondarySourceDimension = ""
	}

	resolution.Candidates = buildSaleAttributeCandidateInfos(candidates, primaryCandidate, secondaryCandidate)
	spuName := ""
	if pkg != nil {
		spuName = firstNonEmpty(pkg.SpuName, pkg.ProductNameEn)
	}
	applySelectedCandidate(index, primaryCandidate, "skc", r.api, resolution.CategoryID, spuName, r.llm, resolution)
	applySelectedCandidate(index, secondaryCandidate, "sku", r.api, resolution.CategoryID, spuName, r.llm, resolution)
	resolution.SelectionSummary = append(resolution.SelectionSummary, buildSelectionSummary(primaryCandidate, secondaryCandidate)...)
	if primaryCandidate != nil && resolution.Source != "llm_sale_attribute_mapping" {
		resolution.Source = "sale_attribute_templates"
	}
	primaryCoverage := resolvedSaleAttributeCoverage(primaryCandidate, resolution.skcValueAssignments)
	secondaryCoverage := resolvedSaleAttributeCoverage(secondaryCandidate, resolution.skuValueAssignments)
	switch {
	case resolution.PrimaryAttributeID > 0 &&
		primaryCoverage.complete &&
		((resolution.SecondaryAttributeID > 0 && secondaryCoverage.complete) || secondaryCandidate == nil):
		resolution.Status = "resolved"
	case resolution.PrimaryAttributeID > 0 || resolution.SecondaryAttributeID > 0:
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "SHEIN 销售属性已部分解析，仍建议人工确认变体规格")
	default:
		resolution.Status = "partial"
		resolution.ReviewNotes = append(resolution.ReviewNotes, "未命中可用的 SHEIN 销售属性映射")
	}
	if !resolution.RecommendCategoryReview {
		if recommend, reason := buildCategoryFamilyConflictSummary(canonical, pkg); recommend {
			resolution.RecommendCategoryReview = true
			resolution.CategoryReviewReason = reason
			resolution.ReviewNotes = append(resolution.ReviewNotes, buildCategoryFamilyConflictReviewNotes(canonical, pkg)...)
		}
	}
	log.WithFields(logrus.Fields{
		"category_id":            resolution.CategoryID,
		"status":                 resolution.Status,
		"primary_attribute_id":   resolution.PrimaryAttributeID,
		"secondary_attribute_id": resolution.SecondaryAttributeID,
		"candidate_count":        len(resolution.Candidates),
	}).Info("resolved SHEIN sale attributes")
	return resolution
}

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

func filterSaleScopeAttributes(attributes []sheinattribute.AttributeInfo) []sheinattribute.AttributeInfo {
	result := make([]sheinattribute.AttributeInfo, 0, len(attributes))
	for _, attr := range attributes {
		if attr.AttributeType == 1 || (attr.SKCScope != nil && *attr.SKCScope) {
			result = append(result, attr)
			continue
		}
		switch normalizeText(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)) {
		case "color", "colour", "size", "style", "pattern", "capacity", "type", "model", "set", "颜色", "颜色分类", "尺码", "尺寸", "规格", "容量", "款式", "类型", "型号", "套装":
			result = append(result, attr)
		}
	}
	if hasPrimarySaleAttribute(result) {
		sort.SliceStable(result, func(i, j int) bool {
			left := isPrimarySaleTemplateAttribute(result[i])
			right := isPrimarySaleTemplateAttribute(result[j])
			if left != right {
				return left
			}
			return false
		})
	}
	return result
}

func hasPrimarySaleAttribute(attributes []sheinattribute.AttributeInfo) bool {
	for _, attr := range attributes {
		if isPrimarySaleTemplateAttribute(attr) {
			return true
		}
	}
	return false
}

func isPrimarySaleTemplateAttribute(attr sheinattribute.AttributeInfo) bool {
	return attr.AttributeLabel == 1 || (attr.SKCScope != nil && *attr.SKCScope)
}

func matchSelectedCandidates(
	candidates []saleAttributeCandidate,
	selection *saleAttributeMappingSelection,
	dimensions []SourceVariantDimension,
	attributes []sheinattribute.AttributeInfo,
) (*saleAttributeCandidate, *saleAttributeCandidate, []saleAttributeCandidate) {
	if selection == nil {
		return nil, nil, candidates
	}
	lookup := func(sourceName string, attributeID int) *saleAttributeCandidate {
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
	primary := lookup(selection.PrimarySourceDimension, selection.PrimaryAttributeID)
	secondary := lookup(selection.SecondarySourceDimension, selection.SecondaryAttributeID)
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
	var dimension *SourceVariantDimension
	sourceOrder := 0
	for i := range dimensions {
		if normalizeText(dimensions[i].Name) != normalizeText(sourceName) {
			continue
		}
		dimension = &dimensions[i]
		sourceOrder = i
		break
	}
	if dimension == nil || isTechnicalSaleSourceDimension(dimension.Name) {
		return saleAttributeCandidate{}, false
	}
	var attr *sheinattribute.AttributeInfo
	templateOrder := 0
	for i := range attributes {
		if attributes[i].AttributeID != attributeID {
			continue
		}
		attr = &attributes[i]
		templateOrder = i
		break
	}
	if attr == nil {
		return saleAttributeCandidate{}, false
	}
	match := buildTemplateAttributeMatch(*attr, dimension.SampleValue)
	match.MatchedBy = "llm_sale_attribute_mapping"
	distinct := len(uniqueNormalizedValues(dimension.Values))
	return newSaleAttributeCandidate(*dimension, sourceOrder, templateOrder, match, distinct, distinct)
}

func shouldSelectSaleAttributeMappingWithLLM(primary *saleAttributeCandidate, dimensions []SourceVariantDimension) bool {
	if len(dimensions) == 0 {
		return false
	}
	if primary == nil {
		return true
	}
	return isWeakPrimarySaleAttributeCandidate(*primary) && hasAlternativePrimarySaleSourceDimension(dimensions, primary.SourceName)
}

func shouldUseLLMSaleAttributeMapping(current, selected *saleAttributeCandidate) bool {
	if selected == nil {
		return false
	}
	if current == nil {
		return true
	}
	if normalizeText(current.SourceName) == normalizeText(selected.SourceName) && current.AttributeID == selected.AttributeID {
		return false
	}
	return isWeakPrimarySaleAttributeCandidate(*current)
}

func isWeakPrimarySaleAttributeCandidate(candidate saleAttributeCandidate) bool {
	if isAIStyleSourceDimension(candidate.SourceName) {
		return true
	}
	return candidate.ValueFitCount == 0
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

func selectStableSaleAttributeSurrogate(
	candidates []saleAttributeCandidate,
	dimensions []SourceVariantDimension,
	attributes []sheinattribute.AttributeInfo,
) (*saleAttributeCandidate, *saleAttributeCandidate, []saleAttributeCandidate, bool) {
	source, ok := selectStableStyleSurrogateDimension(dimensions)
	if !ok {
		return nil, nil, candidates, false
	}
	primaryAttr, ok := selectPrimarySaleTemplateForSurrogate(attributes)
	if !ok {
		return nil, nil, candidates, false
	}
	primary, ok := buildLLMSelectedSaleAttributeCandidate(source.Name, primaryAttr.AttributeID, dimensions, attributes)
	if !ok {
		return nil, nil, candidates, false
	}
	candidates = append(candidates, primary)
	primaryPtr := &candidates[len(candidates)-1]

	var secondaryPtr *saleAttributeCandidate
	if secondary, ok := buildStableSecondarySaleAttributeCandidate(source.Name, dimensions, attributes); ok {
		candidates = append(candidates, secondary)
		secondaryPtr = &candidates[len(candidates)-1]
	}
	return primaryPtr, secondaryPtr, candidates, true
}

func selectStableStyleSurrogateDimension(dimensions []SourceVariantDimension) (SourceVariantDimension, bool) {
	for _, dimension := range dimensions {
		if isColorSourceDimension(dimension.Name) && len(uniqueNormalizedValues(dimension.Values)) > 0 {
			return dimension, true
		}
	}
	for _, dimension := range dimensions {
		if isAIStyleSourceDimension(dimension.Name) || isTechnicalSaleSourceDimension(dimension.Name) || isGenericSecondaryName(dimension.Name) {
			continue
		}
		if len(uniqueNormalizedValues(dimension.Values)) > 0 {
			return dimension, true
		}
	}
	return SourceVariantDimension{}, false
}

func selectPrimarySaleTemplateForSurrogate(attributes []sheinattribute.AttributeInfo) (sheinattribute.AttributeInfo, bool) {
	for _, attr := range attributes {
		if isPrimarySaleTemplateAttribute(attr) {
			return attr, true
		}
	}
	return sheinattribute.AttributeInfo{}, false
}

func buildStableSecondarySaleAttributeCandidate(
	primarySourceName string,
	dimensions []SourceVariantDimension,
	attributes []sheinattribute.AttributeInfo,
) (saleAttributeCandidate, bool) {
	for _, dimension := range dimensions {
		if normalizeText(dimension.Name) == normalizeText(primarySourceName) || !isGenericSecondaryName(dimension.Name) {
			continue
		}
		for _, attr := range attributes {
			if sourceDimensionMatchesSaleTemplate(dimension.Name, attr) {
				return buildLLMSelectedSaleAttributeCandidate(dimension.Name, attr.AttributeID, dimensions, attributes)
			}
		}
	}
	return saleAttributeCandidate{}, false
}

func isColorSourceDimension(name string) bool {
	switch normalizeText(name) {
	case "color", "colour", "颜色", "顏色":
		return true
	default:
		return false
	}
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

func selectPrimarySecondaryCandidates(candidates []saleAttributeCandidate) (*saleAttributeCandidate, *saleAttributeCandidate) {
	if len(candidates) == 0 {
		return nil, nil
	}
	sorted := append([]saleAttributeCandidate(nil), candidates...)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if a.ValueFitCount == 0 || b.ValueFitCount == 0 {
			return a.ValueFitCount > b.ValueFitCount
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
	var primary *saleAttributeCandidate
	for i := range sorted {
		if sorted[i].ValueFitCount == 0 {
			continue
		}
		primary = &sorted[i]
		break
	}
	if primary == nil {
		return nil, nil
	}

	secondaryPool := make([]saleAttributeCandidate, 0, len(sorted))
	for _, candidate := range sorted[1:] {
		if candidate.ValueFitCount == 0 && !canUseFallbackSecondaryCandidate(candidate) {
			continue
		}
		if candidate.SourceName == primary.SourceName || candidate.AttributeID == primary.AttributeID {
			continue
		}
		secondaryPool = append(secondaryPool, candidate)
	}
	if len(secondaryPool) == 0 {
		return primary, nil
	}
	sort.SliceStable(secondaryPool, func(i, j int) bool {
		a, b := secondaryPool[i], secondaryPool[j]
		if a.SecondaryScore != b.SecondaryScore {
			return a.SecondaryScore > b.SecondaryScore
		}
		if a.DistinctCount != b.DistinctCount {
			return a.DistinctCount > b.DistinctCount
		}
		return a.SourceOrder < b.SourceOrder
	})
	if secondaryPool[0].SecondaryScore == 0 && !canUseFallbackSecondaryCandidate(secondaryPool[0]) {
		return primary, nil
	}
	secondary := secondaryPool[0]
	return primary, &secondary
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

func selectCandidatesBySourceDimensions(candidates []saleAttributeCandidate, primaryName, secondaryName string) (*saleAttributeCandidate, *saleAttributeCandidate) {
	if len(candidates) == 0 {
		return nil, nil
	}

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
	if primaryCandidate != nil && secondaryCandidate != nil && normalizeText(primaryCandidate.SourceName) == normalizeText(secondaryCandidate.SourceName) {
		secondaryCandidate = nil
	}
	return primaryCandidate, secondaryCandidate
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

func countTemplateValueFits(index *templateIndex, templateName string, values []string) (int, int) {
	if index == nil || strings.TrimSpace(templateName) == "" {
		return 0, 0
	}
	attr := index.FindAttribute(templateName)
	if attr == nil {
		return 0, 0
	}
	return countTemplateValueFitsForAttribute(*attr, values)
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
		if semantic := inferSourceValueSemantic(candidate.Values); semantic != "" {
			note += "；这些值更像" + semantic
		}
		notes = append(notes, note)
	}
	return notes
}

func inferSourceValueSemantic(values []string) string {
	if len(values) == 0 {
		return ""
	}
	type score struct {
		label string
		hits  int
	}
	scores := []score{
		{label: "套装/组合款", hits: countSemanticMatches(values, "套装", "套组", "组合", "桌椅", "套")},
		{label: "款式/型号", hits: countSemanticMatches(values, "款", "型", "型号", "高椅", "矮椅", "折叠桌", "月亮椅")},
		{label: "规格/尺寸", hits: countSemanticMatches(values, "cm", "mm", "ml", "l", "kg", "g", "x", "*")},
	}
	best := score{}
	for _, current := range scores {
		if current.hits > best.hits {
			best = current
		}
	}
	if best.hits == 0 {
		return ""
	}
	return best.label
}

func countSemanticMatches(values []string, keywords ...string) int {
	count := 0
	for _, value := range values {
		normalized := normalizeText(value)
		for _, keyword := range keywords {
			if strings.Contains(normalized, normalizeText(keyword)) {
				count++
				break
			}
		}
	}
	return count
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

func uniqueNormalizedValues(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	result := make([]string, 0, len(values))
	seen := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		normalized := normalizeText(trimmed)
		if slices.Contains(seen, normalized) {
			continue
		}
		seen = append(seen, normalized)
		result = append(result, trimmed)
	}
	return result
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
