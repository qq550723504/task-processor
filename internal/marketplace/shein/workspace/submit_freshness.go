package workspace

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
)

func EvaluateCategoryFreshness(current *sheinpub.Package, info *sheincategory.CategoryInfo) (bool, string) {
	current = sheinpub.NormalizePackageSemanticFields(current)
	if current == nil {
		return true, ""
	}
	if info == nil {
		return false, "当前类目模板在线校验失败，需重新刷新类目结果后再提交"
	}
	if current.CategoryID <= 0 {
		return false, "当前类目结果缺少 category_id，需重新刷新类目结果后再提交"
	}
	if info.CategoryID != current.CategoryID {
		return false, fmt.Sprintf(
			"当前类目模板已发生变化：原 category_id=%d，当前在线查询结果为 category_id=%d",
			current.CategoryID,
			info.CategoryID,
		)
	}
	currentProductTypeID := 0
	if current.ProductTypeID != nil {
		currentProductTypeID = *current.ProductTypeID
	}
	if currentProductTypeID <= 0 {
		return false, "当前类目结果缺少 product_type_id，需重新刷新类目结果后再提交"
	}
	if info.ProductTypeID != currentProductTypeID {
		return false, fmt.Sprintf(
			"当前类目模板已发生变化：原 category_id=%d/product_type_id=%d，当前在线查询结果为 category_id=%d/product_type_id=%d",
			current.CategoryID,
			currentProductTypeID,
			info.CategoryID,
			info.ProductTypeID,
		)
	}
	return true, "当前类目结果仍然可用于当前提交"
}

func EvaluateAttributeFreshness(current *sheinpub.Package, templates *sheinattribute.AttributeTemplateInfo) (bool, string) {
	current = sheinpub.NormalizePackageSemanticFields(current)
	if current == nil {
		return true, ""
	}
	if templates == nil || len(templates.Data) == 0 {
		return false, "当前普通属性模板在线校验失败，需重新刷新属性模板后再提交"
	}

	templateContext, ok := buildAttributeFreshnessTemplateContext(current, templates)
	if !ok {
		return false, "当前普通属性模板为空，需重新刷新属性模板后再提交"
	}

	return evaluateResolvedAttributeFreshness(current, templateContext)
}

func EvaluateSaleAttributeFreshness(current *sheinpub.Package, templates *sheinattribute.AttributeTemplateInfo) (bool, string) {
	ok, message, _ := EvaluateSaleAttributeFreshnessWithCustomValidation(current, templates, nil)
	return ok, message
}

func EvaluateSaleAttributeFreshnessWithCustomValidation(
	current *sheinpub.Package,
	templates *sheinattribute.AttributeTemplateInfo,
	api sheinpub.AttributeAPI,
) (bool, string, bool) {
	current = sheinpub.NormalizePackageSemanticFields(current)
	if current == nil {
		return true, "", false
	}
	if templates == nil || len(templates.Data) == 0 {
		return false, "当前销售属性模板在线校验失败，需重新刷新销售属性后再提交", false
	}
	currentResolution := current.SaleAttributeResolution
	if currentResolution == nil {
		return true, "", false
	}

	templateContext, ok := buildSaleAttributeFreshnessTemplateContext(templates)
	if !ok {
		return false, "当前销售属性模板为空，需重新刷新销售属性后再提交", false
	}

	return evaluateSaleAttributeFreshnessResolution(current, currentResolution, templateContext, api)
}

type attributeFreshnessTemplateContext struct {
	attributes     []sheinattribute.AttributeInfo
	attributeIndex map[int]sheinattribute.AttributeInfo
	resolvedByID   map[int]sheinpub.ResolvedAttribute
}

type attributeFreshnessIssueState struct {
	invalid      []string
	invalidItems []sheinpub.ResolvedAttribute
	missing      []string
}

func buildAttributeFreshnessTemplateContext(
	current *sheinpub.Package,
	templates *sheinattribute.AttributeTemplateInfo,
) (attributeFreshnessTemplateContext, bool) {
	attributes := filterFreshnessDisplayAttributes(templates.Data[0].AttributeInfos)
	if len(attributes) == 0 {
		return attributeFreshnessTemplateContext{}, false
	}

	attributeIndex := make(map[int]sheinattribute.AttributeInfo, len(attributes))
	for _, attr := range attributes {
		attributeIndex[attr.AttributeID] = attr
	}

	resolvedByID := make(map[int]sheinpub.ResolvedAttribute, len(current.ResolvedAttributes))
	for _, item := range current.ResolvedAttributes {
		if item.AttributeID > 0 {
			resolvedByID[item.AttributeID] = item
		}
	}

	return attributeFreshnessTemplateContext{
		attributes:     attributes,
		attributeIndex: attributeIndex,
		resolvedByID:   resolvedByID,
	}, true
}

func evaluateResolvedAttributeFreshness(
	current *sheinpub.Package,
	templateContext attributeFreshnessTemplateContext,
) (bool, string) {
	issueState := evaluateAttributeFreshnessIssueState(current, templateContext)
	return buildAttributeFreshnessOutcome(issueState, templateContext)
}

func evaluateAttributeFreshnessIssueState(
	current *sheinpub.Package,
	templateContext attributeFreshnessTemplateContext,
) attributeFreshnessIssueState {
	invalid := make([]string, 0)
	invalidItems := make([]sheinpub.ResolvedAttribute, 0)
	for _, item := range current.ResolvedAttributes {
		if item.AttributeID <= 0 {
			continue
		}
		if resolvedAttributeStillLegal(item, templateContext.attributeIndex) {
			continue
		}
		invalid = append(invalid, formatResolvedAttributeDiffItem(item))
		invalidItems = append(invalidItems, item)
	}

	missingRequired := make([]string, 0)
	for _, attr := range templateContext.attributes {
		if !isTemplateRequired(attr) {
			continue
		}
		if !dependencyIsActive(attr, templateContext.resolvedByID) {
			continue
		}
		if _, ok := templateContext.resolvedByID[attr.AttributeID]; ok {
			continue
		}
		missingRequired = append(missingRequired, formatFreshnessAttributeName(attr))
	}

	return attributeFreshnessIssueState{
		invalid:      invalid,
		invalidItems: invalidItems,
		missing:      missingRequired,
	}
}

func buildAttributeFreshnessOutcome(
	issueState attributeFreshnessIssueState,
	templateContext attributeFreshnessTemplateContext,
) (bool, string) {
	if len(issueState.invalid) > 0 || len(issueState.missing) > 0 {
		parts := []string{"当前普通属性模板已变化，现有 resolved attributes 中有内容已不再满足当前提交要求"}
		if len(issueState.invalid) > 0 {
			sort.Strings(issueState.invalid)
			parts = append(parts, "当前模板已失效的属性值: "+strings.Join(issueState.invalid, "; "))
			if drift := buildResolvedAttributeTemplateDriftDetails(issueState.invalidItems, templateContext.attributeIndex); drift != "" {
				parts = append(parts, "同属性在线模板差异: "+drift)
			}
		}
		if len(issueState.missing) > 0 {
			sort.Strings(issueState.missing)
			parts = append(parts, "当前模板新增或恢复生效的必填属性: "+strings.Join(issueState.missing, "; "))
		}
		return false, strings.Join(parts, "；")
	}
	return true, "当前普通属性模板中的已选值仍然合法"
}

func buildResolvedAttributeTemplateDriftDetails(
	invalidItems []sheinpub.ResolvedAttribute,
	attributeIndex map[int]sheinattribute.AttributeInfo,
) string {
	if len(invalidItems) == 0 || len(attributeIndex) == 0 {
		return ""
	}

	currentOnly := append([]sheinpub.ResolvedAttribute(nil), invalidItems...)
	freshCandidates := make([]sheinpub.ResolvedAttribute, 0)
	seen := make(map[string]struct{})
	for _, item := range invalidItems {
		attr, ok := attributeIndex[item.AttributeID]
		if !ok {
			continue
		}
		for _, candidate := range buildResolvedAttributeTemplateCandidates(attr) {
			key := formatResolvedAttributeDiffItem(candidate)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			freshCandidates = append(freshCandidates, candidate)
		}
	}
	if len(currentOnly) == 0 && len(freshCandidates) == 0 {
		return ""
	}

	leftOnly, rightOnly := diffResolvedAttributes(currentOnly, freshCandidates)
	parts := make([]string, 0, 2)
	if len(leftOnly) > 0 {
		parts = append(parts, "当前任务独有: "+strings.Join(leftOnly, "; "))
	}
	if len(rightOnly) > 0 {
		parts = append(parts, "在线模板独有: "+strings.Join(rightOnly, "; "))
	}
	return strings.Join(parts, "；")
}

func diffResolvedAttributes(current []sheinpub.ResolvedAttribute, fresh []sheinpub.ResolvedAttribute) ([]string, []string) {
	currentCounts := make(map[string]int, len(current))
	for _, item := range current {
		currentCounts[formatResolvedAttributeDiffItem(item)]++
	}
	freshCounts := make(map[string]int, len(fresh))
	for _, item := range fresh {
		freshCounts[formatResolvedAttributeDiffItem(item)]++
	}

	currentOnly := make([]string, 0)
	freshOnly := make([]string, 0)
	for key, count := range currentCounts {
		diff := count - freshCounts[key]
		for i := 0; i < diff; i++ {
			currentOnly = append(currentOnly, key)
		}
	}
	for key, count := range freshCounts {
		diff := count - currentCounts[key]
		for i := 0; i < diff; i++ {
			freshOnly = append(freshOnly, key)
		}
	}
	sort.Strings(currentOnly)
	sort.Strings(freshOnly)
	return currentOnly, freshOnly
}

func buildResolvedAttributeTemplateCandidates(attr sheinattribute.AttributeInfo) []sheinpub.ResolvedAttribute {
	if attr.AttributeID <= 0 || len(attr.AttributeValueInfoList) == 0 {
		return nil
	}

	name := strings.TrimSpace(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
	candidates := make([]sheinpub.ResolvedAttribute, 0, len(attr.AttributeValueInfoList))
	for _, option := range attr.AttributeValueInfoList {
		if option.AttributeValueID <= 0 {
			continue
		}
		valueID := option.AttributeValueID
		candidates = append(candidates, sheinpub.ResolvedAttribute{
			Name:             name,
			Value:            strings.TrimSpace(firstNonEmpty(option.AttributeValueEn, option.AttributeValue)),
			AttributeID:      attr.AttributeID,
			AttributeValueID: &valueID,
		})
	}
	return candidates
}

func formatResolvedAttributeDiffItem(item sheinpub.ResolvedAttribute) string {
	valueID := 0
	if item.AttributeValueID != nil {
		valueID = *item.AttributeValueID
	}
	extraValue := strings.TrimSpace(item.AttributeExtraValue)
	if extraValue == "" {
		return fmt.Sprintf(
			"%s=%s (attribute_id=%d, attribute_value_id=%d)",
			strings.TrimSpace(item.Name),
			strings.TrimSpace(item.Value),
			item.AttributeID,
			valueID,
		)
	}
	return fmt.Sprintf(
		"%s=%s (attribute_id=%d, attribute_value_id=%d, extra=%s)",
		strings.TrimSpace(item.Name),
		strings.TrimSpace(item.Value),
		item.AttributeID,
		valueID,
		extraValue,
	)
}

func resolvedAttributeStillLegal(
	item sheinpub.ResolvedAttribute,
	attributeIndex map[int]sheinattribute.AttributeInfo,
) bool {
	attr, ok := attributeIndex[item.AttributeID]
	if !ok {
		return false
	}
	if item.AttributeValueID != nil && *item.AttributeValueID > 0 {
		for _, option := range attr.AttributeValueInfoList {
			if option.AttributeValueID == *item.AttributeValueID {
				return true
			}
		}
		return false
	}
	if strings.TrimSpace(item.AttributeExtraValue) != "" {
		return true
	}
	return len(attr.AttributeValueInfoList) == 0
}

func dependencyIsActive(attr sheinattribute.AttributeInfo, resolvedByID map[int]sheinpub.ResolvedAttribute) bool {
	if attr.CascadeAttributeID <= 0 {
		return true
	}
	parent, ok := resolvedByID[attr.CascadeAttributeID]
	if !ok || parent.AttributeID <= 0 {
		return false
	}
	if conditionalOtherAttribute(attr, parent) {
		return false
	}
	allowed := parseCascadeValueIDs(attr.CascadeAttributeValueIDList)
	if len(allowed) == 0 {
		return true
	}
	if parent.AttributeValueID == nil || *parent.AttributeValueID <= 0 {
		return false
	}
	_, ok = allowed[*parent.AttributeValueID]
	return ok
}

func conditionalOtherAttribute(attr sheinattribute.AttributeInfo, parent sheinpub.ResolvedAttribute) bool {
	name := normalizeText(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
	if name == "" || !strings.HasPrefix(name, "other ") {
		return false
	}
	if values := parseCascadeValueIDs(attr.CascadeAttributeValueIDList); len(values) > 0 {
		return false
	}
	return parent.AttributeValueID != nil && *parent.AttributeValueID > 0
}

func formatFreshnessAttributeName(attr sheinattribute.AttributeInfo) string {
	return strings.TrimSpace(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
}

func filterFreshnessDisplayAttributes(attributes []sheinattribute.AttributeInfo) []sheinattribute.AttributeInfo {
	if len(attributes) == 0 {
		return nil
	}
	filtered := make([]sheinattribute.AttributeInfo, 0, len(attributes))
	for _, attr := range attributes {
		if attr.AttributeType == 1 || (attr.SKCScope != nil && *attr.SKCScope) {
			continue
		}
		filtered = append(filtered, attr)
	}
	return filtered
}

type saleAttributeFreshnessTemplateContext struct {
	byID     map[int]sheinpub.SaleAttributeTemplateOption
	attrByID map[int]sheinattribute.AttributeInfo
}

type saleAttributeFreshnessInvalidState struct {
	invalidSKC []string
	invalidSKU []string
	changed    bool
}

func buildSaleAttributeFreshnessTemplateContext(templates *sheinattribute.AttributeTemplateInfo) (saleAttributeFreshnessTemplateContext, bool) {
	saleOptions := buildFreshnessSaleTemplateOptions(templates)
	if len(saleOptions) == 0 {
		return saleAttributeFreshnessTemplateContext{}, false
	}

	byID := make(map[int]sheinpub.SaleAttributeTemplateOption, len(saleOptions))
	for _, option := range saleOptions {
		byID[option.AttributeID] = option
	}

	return saleAttributeFreshnessTemplateContext{
		byID:     byID,
		attrByID: flattenAttributeTemplatesByID(templates),
	}, true
}

func evaluateSaleAttributeFreshnessResolution(
	current *sheinpub.Package,
	currentResolution *sheinpub.SaleAttributeResolution,
	templateContext saleAttributeFreshnessTemplateContext,
	api sheinpub.AttributeAPI,
) (bool, string, bool) {
	baseIssues := make([]string, 0)

	if currentResolution.PrimaryAttributeID > 0 {
		if _, ok := templateContext.byID[currentResolution.PrimaryAttributeID]; !ok {
			baseIssues = append(baseIssues, fmt.Sprintf("主规格 attribute_id=%d 已不在当前销售属性模板中", currentResolution.PrimaryAttributeID))
		}
	}
	if currentResolution.SecondaryAttributeID > 0 {
		if _, ok := templateContext.byID[currentResolution.SecondaryAttributeID]; !ok {
			baseIssues = append(baseIssues, fmt.Sprintf("副规格 attribute_id=%d 已不在当前销售属性模板中", currentResolution.SecondaryAttributeID))
		}
	}

	invalidState := evaluateSaleAttributeFreshnessInvalidState(current, currentResolution, templateContext, api)
	return buildSaleAttributeFreshnessResolutionOutcome(baseIssues, invalidState)
}

func evaluateSaleAttributeFreshnessInvalidState(
	current *sheinpub.Package,
	currentResolution *sheinpub.SaleAttributeResolution,
	templateContext saleAttributeFreshnessTemplateContext,
	api sheinpub.AttributeAPI,
) saleAttributeFreshnessInvalidState {
	customRelationIDs := freshnessCustomAttributeValueIDs(currentResolution.CustomAttributeRelation)
	invalidSKC := collectInvalidSaleAttributes(currentResolution.SKCAttributes, templateContext.byID, customRelationIDs)
	invalidSKU := collectInvalidSaleAttributes(currentResolution.SKUAttributes, templateContext.byID, customRelationIDs)
	changed := false
	if len(invalidSKC) > 0 || len(invalidSKU) > 0 {
		repaired := repairFreshnessSaleAttributes(current, templateContext.attrByID, api)
		if repaired {
			changed = true
			currentResolution = current.SaleAttributeResolution
			customRelationIDs = freshnessCustomAttributeValueIDs(currentResolution.CustomAttributeRelation)
			invalidSKC = collectInvalidSaleAttributes(currentResolution.SKCAttributes, templateContext.byID, customRelationIDs)
			invalidSKU = collectInvalidSaleAttributes(currentResolution.SKUAttributes, templateContext.byID, customRelationIDs)
		}
	}

	return saleAttributeFreshnessInvalidState{
		invalidSKC: invalidSKC,
		invalidSKU: invalidSKU,
		changed:    changed,
	}
}

func buildSaleAttributeFreshnessResolutionOutcome(
	baseIssues []string,
	invalidState saleAttributeFreshnessInvalidState,
) (bool, string, bool) {
	issues := append([]string(nil), baseIssues...)
	if len(invalidState.invalidSKC) > 0 {
		sort.Strings(invalidState.invalidSKC)
		issues = append(issues, "当前模板已失效的 SKC 销售属性值: "+strings.Join(invalidState.invalidSKC, "; "))
	}
	if len(invalidState.invalidSKU) > 0 {
		sort.Strings(invalidState.invalidSKU)
		issues = append(issues, "当前模板已失效的 SKU 销售属性值: "+strings.Join(invalidState.invalidSKU, "; "))
	}

	if len(issues) > 0 {
		return false, "当前销售属性模板已变化，现有销售属性中有内容已不再满足当前提交要求；" + strings.Join(issues, "；"), invalidState.changed
	}
	if invalidState.changed {
		return true, "当前销售属性模板中的已选值仍然合法，失效值已通过 SHEIN 自定义销售属性校验自动修正", true
	}
	return true, "当前销售属性模板中的已选值仍然合法", false
}

func buildFreshnessSaleTemplateOptions(info *sheinattribute.AttributeTemplateInfo) []sheinpub.SaleAttributeTemplateOption {
	if info == nil || len(info.Data) == 0 {
		return nil
	}
	template := info.Data[0]
	attributes := orderFreshnessSaleScopeAttributes(filterFreshnessSaleScopeAttributes(template.AttributeInfos), template.AttributeID)
	options := make([]sheinpub.SaleAttributeTemplateOption, 0, len(attributes))
	for _, attribute := range attributes {
		option := sheinpub.SaleAttributeTemplateOption{
			AttributeID: attribute.AttributeID,
			Name:        attribute.AttributeName,
			NameEn:      attribute.AttributeNameEn,
			Required:    attribute.AttributeIsShow == 1,
			Important:   attribute.AttributeLabel == 1,
		}
		if attribute.SKCScope != nil {
			option.SKCScope = *attribute.SKCScope
		}
		for _, value := range attribute.AttributeValueInfoList {
			if value.AttributeValueID <= 0 {
				continue
			}
			option.AttributeValueList = append(option.AttributeValueList, sheinpub.AttributeValueCandidate{
				AttributeValueID: value.AttributeValueID,
				Value:            value.AttributeValue,
				ValueEn:          value.AttributeValueEn,
			})
		}
		options = append(options, option)
	}
	return options
}

func filterFreshnessSaleScopeAttributes(attributes []sheinattribute.AttributeInfo) []sheinattribute.AttributeInfo {
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
	return result
}

func orderFreshnessSaleScopeAttributes(attributes []sheinattribute.AttributeInfo, orderedIDs []int) []sheinattribute.AttributeInfo {
	if len(attributes) == 0 {
		return nil
	}
	if len(orderedIDs) == 0 {
		ordered := append([]sheinattribute.AttributeInfo(nil), attributes...)
		sort.SliceStable(ordered, func(i, j int) bool {
			left := ordered[i].AttributeLabel == 1
			right := ordered[j].AttributeLabel == 1
			if left != right {
				return left
			}
			return false
		})
		return ordered
	}
	byID := make(map[int]sheinattribute.AttributeInfo, len(attributes))
	for _, attr := range attributes {
		byID[attr.AttributeID] = attr
	}
	ordered := make([]sheinattribute.AttributeInfo, 0, len(attributes))
	seen := make(map[int]struct{}, len(attributes))
	for _, id := range orderedIDs {
		attr, ok := byID[id]
		if !ok {
			continue
		}
		ordered = append(ordered, attr)
		seen[id] = struct{}{}
	}
	for _, attr := range attributes {
		if _, ok := seen[attr.AttributeID]; ok {
			continue
		}
		ordered = append(ordered, attr)
	}
	sort.SliceStable(ordered, func(i, j int) bool {
		left := ordered[i].AttributeLabel == 1
		right := ordered[j].AttributeLabel == 1
		if left != right {
			return left
		}
		return false
	})
	return ordered
}

func collectInvalidSaleAttributes(
	items []sheinpub.ResolvedSaleAttribute,
	options map[int]sheinpub.SaleAttributeTemplateOption,
	customRelationIDs map[int]struct{},
) []string {
	invalid := make([]string, 0)
	for _, item := range items {
		option, ok := options[item.AttributeID]
		if !ok {
			invalid = append(invalid, formatResolvedSaleAttributeDiffItem(item))
			continue
		}
		if item.AttributeValueID != nil && *item.AttributeValueID > 0 {
			if _, ok := customRelationIDs[*item.AttributeValueID]; ok {
				continue
			}
			found := false
			for _, candidate := range option.AttributeValueList {
				if candidate.AttributeValueID == *item.AttributeValueID {
					found = true
					break
				}
			}
			if !found {
				invalid = append(invalid, formatResolvedSaleAttributeDiffItem(item))
			}
		}
	}
	return invalid
}

func freshnessCustomAttributeValueIDs(relations []sheinattribute.CustomAttributeRelation) map[int]struct{} {
	if len(relations) == 0 {
		return nil
	}
	result := make(map[int]struct{}, len(relations))
	for _, relation := range relations {
		if relation.AttributeValueID <= 0 {
			continue
		}
		result[int(relation.AttributeValueID)] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func formatResolvedSaleAttributeDiffItem(item sheinpub.ResolvedSaleAttribute) string {
	valueID := 0
	if item.AttributeValueID != nil {
		valueID = *item.AttributeValueID
	}
	return fmt.Sprintf(
		"%s=%s (scope=%s, attribute_id=%d, attribute_value_id=%d)",
		strings.TrimSpace(item.Name),
		strings.TrimSpace(item.Value),
		strings.TrimSpace(item.Scope),
		item.AttributeID,
		valueID,
	)
}

func repairFreshnessSaleAttributes(
	current *sheinpub.Package,
	attrByID map[int]sheinattribute.AttributeInfo,
	api sheinpub.AttributeAPI,
) bool {
	current = sheinpub.NormalizePackageSemanticFields(current)
	if current == nil || current.SaleAttributeResolution == nil || len(attrByID) == 0 || api == nil || current.CategoryID <= 0 {
		return false
	}

	spuName := strings.TrimSpace(current.SpuName)
	if spuName == "" {
		spuName = strings.TrimSpace(current.ProductNameEn)
	}

	changed := false
	relations := make([]sheinattribute.CustomAttributeRelation, 0)
	for index, item := range current.SaleAttributeResolution.SKCAttributes {
		repaired, itemRelations, ok := tryRepairFreshnessSaleAttribute(item, attrByID, current.CategoryID, spuName, api)
		if !ok {
			continue
		}
		current.SaleAttributeResolution.SKCAttributes[index] = repaired
		relations = append(relations, itemRelations...)
		changed = true
	}
	for index, item := range current.SaleAttributeResolution.SKUAttributes {
		repaired, itemRelations, ok := tryRepairFreshnessSaleAttribute(item, attrByID, current.CategoryID, spuName, api)
		if !ok {
			continue
		}
		current.SaleAttributeResolution.SKUAttributes[index] = repaired
		relations = append(relations, itemRelations...)
		changed = true
	}
	if !changed {
		return false
	}
	if len(relations) > 0 {
		current.SaleAttributeResolution.CustomAttributeRelation = dedupeCustomAttributeRelations(append(
			append([]sheinattribute.CustomAttributeRelation(nil), current.SaleAttributeResolution.CustomAttributeRelation...),
			relations...,
		))
	}
	sheinpub.ApplySaleAttributeResolution(current, current.SaleAttributeResolution)
	return true
}

func tryRepairFreshnessSaleAttribute(
	item sheinpub.ResolvedSaleAttribute,
	attrByID map[int]sheinattribute.AttributeInfo,
	categoryID int,
	spuName string,
	api sheinpub.AttributeAPI,
) (sheinpub.ResolvedSaleAttribute, []sheinattribute.CustomAttributeRelation, bool) {
	if item.AttributeID <= 0 || strings.TrimSpace(item.Value) == "" {
		return sheinpub.ResolvedSaleAttribute{}, nil, false
	}
	attr, ok := attrByID[item.AttributeID]
	if !ok {
		return sheinpub.ResolvedSaleAttribute{}, nil, false
	}
	repaired, relations, _, matched := sheinpub.ResolveSingleSaleAttributeValue(
		attr,
		firstNonEmpty(strings.TrimSpace(item.Name), firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)),
		item.Value,
		strings.TrimSpace(item.Scope),
		api,
		categoryID,
		spuName,
	)
	if !matched || repaired.AttributeValueID == nil || *repaired.AttributeValueID <= 0 {
		return sheinpub.ResolvedSaleAttribute{}, nil, false
	}
	return repaired, relations, true
}

func flattenAttributeTemplatesByID(info *sheinattribute.AttributeTemplateInfo) map[int]sheinattribute.AttributeInfo {
	if info == nil {
		return nil
	}
	result := make(map[int]sheinattribute.AttributeInfo)
	for _, template := range info.Data {
		for _, attr := range template.AttributeInfos {
			if attr.AttributeID <= 0 {
				continue
			}
			result[attr.AttributeID] = attr
		}
	}
	return result
}

func isTemplateRequired(attribute sheinattribute.AttributeInfo) bool {
	switch {
	case len(attribute.AttributeRemarkList) > 0:
		return true
	case attribute.AttributeLabel == 1:
		return true
	case attribute.AttributeStatus == 3:
		return true
	default:
		return false
	}
}

func parseCascadeValueIDs(raw *string) map[int]struct{} {
	value := strings.TrimSpace(firstNonEmptyValue(raw))
	if value == "" {
		return nil
	}
	out := make(map[int]struct{})
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parsed, err := strconv.Atoi(part)
		if err != nil || parsed <= 0 {
			continue
		}
		out[parsed] = struct{}{}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func dedupeCustomAttributeRelations(relations []sheinattribute.CustomAttributeRelation) []sheinattribute.CustomAttributeRelation {
	if len(relations) == 0 {
		return nil
	}
	out := make([]sheinattribute.CustomAttributeRelation, 0, len(relations))
	seen := make(map[string]struct{}, len(relations))
	for _, relation := range relations {
		key := fmt.Sprintf("%d|%d", relation.PreAttributeValueID, relation.AttributeValueID)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, relation)
	}
	return out
}

func firstNonEmptyValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
