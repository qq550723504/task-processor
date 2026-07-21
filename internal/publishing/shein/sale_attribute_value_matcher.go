package shein

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	sheinattribute "task-processor/internal/shein/api/attribute"
)

var (
	saleAttributeLeadingScalePattern = regexp.MustCompile(`(?i)\b(eur|eu|us|uk)\s*([0-9])`)
	saleAttributeNoisePattern        = regexp.MustCompile(`(?i)\b(eur|eu|us|uk|size)\b`)
	saleAttributeParenContentPattern = regexp.MustCompile(`[(（][^)）]*[)）]`)
)

func buildValueAssignments(
	values []string,
	sourceDimension string,
	templateName string,
	scope string,
	index *templateIndex,
	api AttributeAPI,
	categoryID int,
	spuName string,
	llm TextGenerator,
) (map[string]ResolvedSaleAttribute, []sheinattribute.CustomAttributeRelation, []string, saleAttributeValueSummary) {
	return buildValueAssignmentsWithDeniedStore(values, sourceDimension, templateName, scope, index, api, categoryID, spuName, "", nil, llm)
}

func buildValueAssignmentsWithDeniedStore(
	values []string,
	sourceDimension string,
	templateName string,
	scope string,
	index *templateIndex,
	api AttributeAPI,
	categoryID int,
	spuName string,
	storeID string,
	deniedStore ResolutionCacheStore,
	llm TextGenerator,
) (map[string]ResolvedSaleAttribute, []sheinattribute.CustomAttributeRelation, []string, saleAttributeValueSummary) {
	return buildValueAssignmentsWithPreparationPolicy(values, sourceDimension, templateName, scope, index, api, categoryID, spuName, storeID, deniedStore, llm, false)
}

func buildValueAssignmentsPreservingSourceValues(
	values []string,
	sourceDimension string,
	templateName string,
	scope string,
	index *templateIndex,
	api AttributeAPI,
	categoryID int,
	spuName string,
) (map[string]ResolvedSaleAttribute, []sheinattribute.CustomAttributeRelation, []string, saleAttributeValueSummary) {
	return buildValueAssignmentsWithPreparationPolicy(values, sourceDimension, templateName, scope, index, api, categoryID, spuName, "", nil, nil, true)
}

func buildValueAssignmentsWithPreparationPolicy(
	values []string,
	sourceDimension string,
	templateName string,
	scope string,
	index *templateIndex,
	api AttributeAPI,
	categoryID int,
	spuName string,
	storeID string,
	deniedStore ResolutionCacheStore,
	llm TextGenerator,
	preserveSourceValues bool,
) (map[string]ResolvedSaleAttribute, []sheinattribute.CustomAttributeRelation, []string, saleAttributeValueSummary) {
	if len(values) == 0 || strings.TrimSpace(templateName) == "" || index == nil {
		return nil, nil, nil, saleAttributeValueSummary{}
	}
	attr := index.FindAttribute(templateName)
	if attr == nil {
		return nil, nil, []string{fmt.Sprintf("SHEIN 销售属性模板 %q 不存在，无法映射源维度 %q", templateName, sourceDimension)}, saleAttributeValueSummary{}
	}

	assignments := make(map[string]ResolvedSaleAttribute, len(values))
	var relations []sheinattribute.CustomAttributeRelation
	pendingNotes := make(map[string][]string, len(values))
	unresolved := make([]string, 0, len(values))
	preparedValues := make(map[string]saleAttributeValuePreparation, len(values))
	summary := saleAttributeValueSummary{}
	var notes []string
	for _, value := range uniqueNormalizedValues(values) {
		prepared := prepareSaleAttributeSourceValue(attr, sourceDimension, value, spuName, llm)
		if preserveSourceValues {
			prepared = saleAttributeValuePreparation{
				Original:           value,
				Effective:          value,
				SanitizationSource: "manual_direct",
			}
		}
		preparedValues[normalizeText(value)] = prepared
		mergeSaleAttributeValueSummary(&summary, prepared)
		if prepared.NeedsManualReview {
			notes = append(notes, buildBlockedSaleAttributeValueNote(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), sourceDimension, prepared.Original))
			continue
		}
		effectiveValue := firstNonEmpty(prepared.Effective, value)
		resolved, matchNotes := matchSaleAttributeValueDeterministic(attr, sourceDimension, effectiveValue, scope)
		if resolved.AttributeID <= 0 || resolved.AttributeValueID == nil {
			if len(matchNotes) > 0 {
				pendingNotes[normalizeText(value)] = append([]string(nil), matchNotes...)
			}
			unresolved = append(unresolved, value)
			continue
		}
		notes = append(notes, matchNotes...)
		assignments[normalizeText(value)] = resolved
	}
	if len(unresolved) > 0 && llm != nil {
		llmInput := make([]string, 0, len(unresolved))
		for _, value := range unresolved {
			prepared := preparedValues[normalizeText(value)]
			llmInput = append(llmInput, firstNonEmpty(prepared.Effective, value))
		}
		llmAssignments, llmNotes := matchSaleAttributeValuesWithLLM(*attr, sourceDimension, llmInput, scope, llm)
		notes = append(notes, llmNotes...)
		for _, value := range unresolved {
			prepared := preparedValues[normalizeText(value)]
			effectiveKey := normalizeText(firstNonEmpty(prepared.Effective, value))
			resolved, ok := llmAssignments[effectiveKey]
			if !ok {
				continue
			}
			assignments[normalizeText(value)] = resolved
			delete(pendingNotes, normalizeText(value))
		}
	}
	if len(unresolved) > 0 {
		stillUnresolved := make([]string, 0, len(unresolved))
		originalByEffective := make(map[string][]string, len(unresolved))
		for _, value := range unresolved {
			if _, ok := assignments[normalizeText(value)]; ok {
				continue
			}
			prepared := preparedValues[normalizeText(value)]
			effective := firstNonEmpty(prepared.Effective, value)
			stillUnresolved = append(stillUnresolved, effective)
			effectiveKey := normalizeText(effective)
			originalByEffective[effectiveKey] = append(originalByEffective[effectiveKey], normalizeText(value))
		}
		customAssignments, customRelations, customNotes := resolveCustomSaleAttributeValues(*attr, sourceDimension, stillUnresolved, scope, api, categoryID, spuName, storeID, deniedStore, preserveSourceValues)
		notes = append(notes, customNotes...)
		relations = append(relations, customRelations...)
		for key, resolved := range customAssignments {
			originalKeys := originalByEffective[key]
			if len(originalKeys) == 0 {
				assignments[key] = resolved
				delete(pendingNotes, key)
				continue
			}
			for _, originalKey := range originalKeys {
				assignments[originalKey] = resolved
				delete(pendingNotes, originalKey)
			}
		}
	}
	for _, matchNotes := range pendingNotes {
		notes = append(notes, matchNotes...)
	}
	if len(assignments) == 0 {
		if hasBlockedSaleAttributeValue(summary) {
			notes = append(notes, buildSaleAttributeManualReviewNote(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), sourceDimension))
		}
		return nil, dedupeCustomAttributeRelations(relations), dedupeStrings(notes), summary
	}
	if hasBlockedSaleAttributeValue(summary) {
		notes = append(notes, buildSaleAttributeManualReviewNote(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), sourceDimension))
	}
	return assignments, dedupeCustomAttributeRelations(relations), dedupeStrings(notes), summary
}

func ResolveSingleSaleAttributeValue(
	attr sheinattribute.AttributeInfo,
	sourceDimension string,
	sourceValue string,
	scope string,
	api AttributeAPI,
	categoryID int,
	spuName string,
) (ResolvedSaleAttribute, []sheinattribute.CustomAttributeRelation, []string, bool) {
	return resolveSingleSaleAttributeValue(attr, sourceDimension, sourceValue, scope, api, categoryID, spuName, false)
}

// ResolveSingleManualSaleAttributeValue resolves a user-entered value without
// applying prompt or canvas-size cleanup before the online SHEIN validation.
func ResolveSingleManualSaleAttributeValue(
	attr sheinattribute.AttributeInfo,
	sourceDimension string,
	sourceValue string,
	scope string,
	api AttributeAPI,
	categoryID int,
	spuName string,
) (ResolvedSaleAttribute, []sheinattribute.CustomAttributeRelation, []string, bool) {
	return resolveSingleSaleAttributeValue(attr, sourceDimension, sourceValue, scope, api, categoryID, spuName, true)
}

func resolveSingleSaleAttributeValue(
	attr sheinattribute.AttributeInfo,
	sourceDimension string,
	sourceValue string,
	scope string,
	api AttributeAPI,
	categoryID int,
	spuName string,
	preserveSourceValue bool,
) (ResolvedSaleAttribute, []sheinattribute.CustomAttributeRelation, []string, bool) {
	index := newTemplateIndex([]sheinattribute.AttributeInfo{attr})
	var assignments map[string]ResolvedSaleAttribute
	var relations []sheinattribute.CustomAttributeRelation
	var notes []string
	if preserveSourceValue {
		assignments, relations, notes, _ = buildValueAssignmentsPreservingSourceValues(
			[]string{sourceValue}, sourceDimension, firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), scope, index, api, categoryID, spuName,
		)
	} else {
		assignments, relations, notes, _ = buildValueAssignments(
			[]string{sourceValue}, sourceDimension, firstNonEmpty(attr.AttributeNameEn, attr.AttributeName), scope, index, api, categoryID, spuName, nil,
		)
	}
	if len(assignments) == 0 {
		return ResolvedSaleAttribute{}, relations, notes, false
	}
	resolved, ok := assignments[normalizeText(sourceValue)]
	if ok {
		return resolved, relations, notes, true
	}
	for _, value := range assignments {
		return value, relations, notes, true
	}
	return ResolvedSaleAttribute{}, relations, notes, false
}

func matchSaleAttributeValueDeterministic(
	attr *sheinattribute.AttributeInfo,
	sourceDimension string,
	sourceValue string,
	scope string,
) (ResolvedSaleAttribute, []string) {
	sourceValue = strings.TrimSpace(sourceValue)
	if attr == nil || sourceValue == "" {
		return ResolvedSaleAttribute{}, nil
	}
	if resolved, ok := matchSaleAttributeValueExact(*attr, sourceValue, scope); ok {
		return resolved, nil
	}

	return ResolvedSaleAttribute{}, []string{
		fmt.Sprintf(
			"SHEIN 销售属性值未匹配: 源维度 %q 的值 %q 无法映射到模板属性 %q",
			sourceDimension,
			sourceValue,
			firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		),
	}
}

func matchSaleAttributeValueExact(attr sheinattribute.AttributeInfo, sourceValue string, scope string) (ResolvedSaleAttribute, bool) {
	sourceValue = strings.TrimSpace(sourceValue)
	if sourceValue == "" {
		return ResolvedSaleAttribute{}, false
	}
	for _, option := range attr.AttributeValueInfoList {
		candidates := []string{
			firstNonEmpty(option.AttributeValueEn, option.AttributeValue),
			option.AttributeValue,
			option.AttributeValueEn,
		}
		for _, candidate := range candidates {
			if normalizeText(candidate) != normalizeText(sourceValue) {
				continue
			}
			return buildResolvedSaleAttribute(attr, option, sourceValue, scope, "attribute_value"), true
		}
	}
	return ResolvedSaleAttribute{}, false
}

func matchSaleAttributeValueNormalized(attr sheinattribute.AttributeInfo, sourceValue string, scope string) (ResolvedSaleAttribute, bool) {
	normalizedSource := comparableAttributeValueForms(sourceValue)
	if len(normalizedSource) == 0 {
		return ResolvedSaleAttribute{}, false
	}
	sourceSet := make(map[string]struct{}, len(normalizedSource))
	for _, value := range normalizedSource {
		sourceSet[value] = struct{}{}
	}
	for _, option := range attr.AttributeValueInfoList {
		candidates := []string{
			firstNonEmpty(option.AttributeValueEn, option.AttributeValue),
			option.AttributeValue,
			option.AttributeValueEn,
		}
		for _, candidate := range candidates {
			for _, comparable := range comparableAttributeValueForms(candidate) {
				if _, ok := sourceSet[comparable]; !ok {
					continue
				}
				return buildResolvedSaleAttribute(attr, option, sourceValue, scope, "attribute_value_normalized"), true
			}
		}
	}
	if option, ok := matchSaleAttributeValueByComparableScore(attr, sourceValue); ok {
		return buildResolvedSaleAttribute(attr, option, sourceValue, scope, "attribute_value_comparable"), true
	}
	return ResolvedSaleAttribute{}, false
}

func matchSaleAttributeValueByComparableScore(attr sheinattribute.AttributeInfo, sourceValue string) (sheinattribute.AttributeValue, bool) {
	sourceForms := comparableAttributeValueForms(sourceValue)
	if len(sourceForms) == 0 || len(attr.AttributeValueInfoList) == 0 {
		return sheinattribute.AttributeValue{}, false
	}

	type candidateScore struct {
		option sheinattribute.AttributeValue
		score  int
	}

	scores := make([]candidateScore, 0, len(attr.AttributeValueInfoList))
	for _, option := range attr.AttributeValueInfoList {
		score := comparableOptionScore(sourceForms, option)
		if score <= 0 {
			continue
		}
		scores = append(scores, candidateScore{option: option, score: score})
	}
	if len(scores) == 0 {
		return sheinattribute.AttributeValue{}, false
	}
	sort.SliceStable(scores, func(i, j int) bool {
		if scores[i].score != scores[j].score {
			return scores[i].score > scores[j].score
		}
		return scores[i].option.AttributeValueID < scores[j].option.AttributeValueID
	})
	if scores[0].score < 3 {
		return sheinattribute.AttributeValue{}, false
	}
	if len(scores) > 1 && scores[1].score == scores[0].score {
		return sheinattribute.AttributeValue{}, false
	}
	return scores[0].option, true
}

func normalizeSaleAttributeValue(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}
	value = strings.NewReplacer(
		"，", ",",
		"（", "(",
		"）", ")",
		"_", " ",
		"-", " ",
		"/", " ",
	).Replace(value)
	value = saleAttributeLeadingScalePattern.ReplaceAllString(value, `$2`)
	value = saleAttributeNoisePattern.ReplaceAllString(value, " ")
	value = trimSaleAttributeCodePrefix(value)
	value = strings.Join(strings.Fields(value), " ")
	return value
}

func comparableAttributeValueForms(value string) []string {
	rawSegments := comparableAttributeSegments(value)
	forms := []string{
		normalizeSaleAttributeValue(value),
		normalizeText(value),
		stripComparableSizeAnnotations(value),
	}
	for _, segment := range rawSegments {
		forms = append(forms, normalizeSaleAttributeValue(segment), normalizeText(segment), stripComparableSizeAnnotations(segment))
	}

	result := make([]string, 0, len(forms))
	seen := make(map[string]struct{}, len(forms))
	for _, form := range forms {
		form = strings.TrimSpace(form)
		if form == "" {
			continue
		}
		if _, ok := seen[form]; ok {
			continue
		}
		seen[form] = struct{}{}
		result = append(result, form)
	}
	return result
}

func comparableOptionScore(sourceForms []string, option sheinattribute.AttributeValue) int {
	best := 0
	for _, source := range sourceForms {
		for _, candidate := range comparableAttributeValueForms(firstNonEmpty(option.AttributeValueEn, option.AttributeValue)) {
			score := compareComparableForm(source, candidate)
			if score > best {
				best = score
			}
		}
		for _, candidate := range comparableAttributeValueForms(option.AttributeValue) {
			score := compareComparableForm(source, candidate)
			if score > best {
				best = score
			}
		}
		for _, candidate := range comparableAttributeValueForms(option.AttributeValueEn) {
			score := compareComparableForm(source, candidate)
			if score > best {
				best = score
			}
		}
	}
	return best
}

func stripComparableSizeAnnotations(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	value = saleAttributeParenContentPattern.ReplaceAllString(value, " ")
	value = strings.NewReplacer(
		"cm", "cm",
		"厘米", "cm",
		"*", "x",
		"×", "x",
		"x", "x",
		" ", "",
	).Replace(strings.ToLower(value))
	return strings.TrimSpace(value)
}

func compareComparableForm(source, candidate string) int {
	source = strings.TrimSpace(source)
	candidate = strings.TrimSpace(candidate)
	if source == "" || candidate == "" {
		return 0
	}
	if source == candidate {
		return 10
	}
	if strings.Contains(source, candidate) || strings.Contains(candidate, source) {
		return 6
	}
	sourceTokens := strings.Fields(source)
	candidateTokens := strings.Fields(candidate)
	if len(sourceTokens) == 0 || len(candidateTokens) == 0 {
		return 0
	}
	overlap := 0
	for _, left := range sourceTokens {
		for _, right := range candidateTokens {
			if left == right {
				overlap++
			}
		}
	}
	return overlap
}

func buildResolvedSaleAttribute(
	attr sheinattribute.AttributeInfo,
	option sheinattribute.AttributeValue,
	sourceValue string,
	scope string,
	matchedBy string,
) ResolvedSaleAttribute {
	valueID := option.AttributeValueID
	return ResolvedSaleAttribute{
		Scope:            scope,
		Name:             firstNonEmpty(attr.AttributeNameEn, attr.AttributeName),
		Value:            sourceValue,
		AttributeID:      attr.AttributeID,
		AttributeValueID: &valueID,
		MatchedBy:        matchedBy,
	}
}

func trimSaleAttributeCodePrefix(value string) string {
	for i, r := range value {
		if r > 127 {
			prefix := strings.TrimSpace(value[:i])
			if prefix == "" {
				return value
			}
			if isLikelySaleAttributeCodePrefix(prefix) {
				return value[i:]
			}
			return value
		}
	}
	return value
}

func isLikelySaleAttributeCodePrefix(prefix string) bool {
	if prefix == "" {
		return false
	}
	hasLetterOrDigit := false
	for _, r := range prefix {
		switch {
		case r >= 'a' && r <= 'z':
			hasLetterOrDigit = true
		case r >= '0' && r <= '9':
			hasLetterOrDigit = true
		case strings.ContainsRune(" -_./", r):
		default:
			return false
		}
	}
	return hasLetterOrDigit
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		key := strings.TrimSpace(value)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}
