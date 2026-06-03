package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
	sheinclient "task-processor/internal/shein/client"
	sheinworkspace "task-processor/internal/workspace/shein"
)

const (
	sheinFreshnessAuthKey          = "shein_online_auth"
	sheinFreshnessCategoryKey      = "shein_category_template_freshness"
	sheinFreshnessAttributeKey     = "shein_attribute_template_freshness"
	sheinFreshnessSaleAttributeKey = "shein_sale_attribute_freshness"
)

func (s *service) validateSheinPublishFreshness(ctx context.Context, task *Task, pkg *SheinPackage, action string) (*SheinSubmitReadiness, error) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if s == nil || task == nil || pkg == nil {
		return nil, nil
	}

	checks := make([]sheinworkspace.ReadinessCheckSpec, 0, 4)
	addCheck := func(key, label string, ok bool, message string, fieldPaths []string, suggestedAction string) {
		checks = append(checks, sheinworkspace.ReadinessCheckSpec{
			Key:             key,
			Label:           label,
			OK:              ok,
			Message:         message,
			FieldPaths:      append([]string(nil), fieldPaths...),
			SuggestedAction: suggestedAction,
		})
	}

	if err := s.validateSheinOnlineAuthPreflight(ctx, task); err != nil {
		addCheck(
			sheinFreshnessAuthKey,
			"SHEIN 在线登录态",
			false,
			"SHEIN 提交店铺当前不可用，请先刷新登录态后再提交："+strings.TrimSpace(err.Error()),
			[]string{"shein.store_resolution", "shein.review_notes"},
			"重新登录 SHEIN 店铺",
		)
		return buildSheinSubmitFreshnessReadiness(pkg, checks), nil
	}
	addCheck(
		sheinFreshnessAuthKey,
		"SHEIN 在线登录态",
		true,
		"SHEIN 提交店铺当前可用",
		[]string{"shein.store_resolution"},
		"重新登录 SHEIN 店铺",
	)

	currentCanonical := sheinFreshnessCanonicalProduct(task)
	if !s.canRunSheinTemplateFreshnessChecks(task, currentCanonical) {
		return buildSheinSubmitFreshnessReadiness(pkg, checks), nil
	}

	req := buildSheinPublishRequestForTask(task, task.Request)
	if req == nil {
		return buildSheinSubmitFreshnessReadiness(pkg, checks), nil
	}
	if storeID, err := s.resolveSheinStoreID(ctx, task); err == nil && storeID > 0 {
		req.SheinStoreID = storeID
	}

	freshPkg, err := cloneSheinPackageForFreshness(pkg)
	if err != nil {
		addCheck(
			sheinFreshnessCategoryKey,
			"类目模板新鲜度",
			false,
			"SHEIN 在线模板预检失败，当前无法构建 freshness 校验上下文",
			[]string{"shein.category_id", "shein.category_id_list", "shein.product_type_id"},
			"刷新类目模板",
		)
		return buildSheinSubmitFreshnessReadiness(pkg, checks), nil
	}

	categoryInfo, categoryInfoErr := s.loadSheinCategoryInfoForFreshness(req.Context, task, pkg.CategoryID)
	categoryReady, categoryMessage := evaluateSheinCategoryFreshness(pkg, categoryInfo)
	if categoryInfoErr != nil {
		categoryReady = false
		categoryMessage = "当前类目模板在线校验失败，需重新刷新类目结果后再提交：" + strings.TrimSpace(categoryInfoErr.Error())
	}
	addCheck(
		sheinFreshnessCategoryKey,
		"类目模板新鲜度",
		categoryReady,
		categoryMessage,
		[]string{"shein.category_id", "shein.category_id_list", "shein.product_type_id"},
		"刷新类目模板",
	)
	if !categoryReady {
		return buildSheinSubmitFreshnessReadiness(pkg, checks), nil
	}

	attributeTemplates, attributeTemplateErr := s.loadSheinAttributeTemplatesForFreshness(req.Context, task, freshPkg.CategoryID)
	attributeReady, attributeMessage := evaluateSheinAttributeFreshness(pkg, attributeTemplates)
	if attributeTemplateErr != nil {
		attributeReady = false
		attributeMessage = "当前普通属性模板在线校验失败，需重新刷新属性模板后再提交：" + strings.TrimSpace(attributeTemplateErr.Error())
	}
	addCheck(
		sheinFreshnessAttributeKey,
		"普通属性模板新鲜度",
		attributeReady,
		attributeMessage,
		[]string{"shein.resolved_attributes", "shein.attribute_resolution"},
		"刷新属性模板",
	)

	var saleAPI sheinpub.AttributeAPI
	if attributeTemplateErr == nil {
		saleAPI, _ = s.buildSheinAttributeAPI(req.Context, task)
	}
	saleReady, saleMessage, saleChanged := evaluateSheinSaleAttributeFreshnessWithCustomValidation(pkg, attributeTemplates, saleAPI)
	if saleChanged && task.Result != nil && s.repo != nil {
		task.Result.UpdatedAt = time.Now()
		if err := s.repo.SaveTaskResult(ctx, task.ID, task.Result); err != nil {
			return nil, err
		}
	}
	addCheck(
		sheinFreshnessSaleAttributeKey,
		"销售属性模板新鲜度",
		saleReady,
		saleMessage,
		[]string{"shein.sale_attribute_resolution", "shein.request_draft.skc_list"},
		"刷新销售属性",
	)

	return buildSheinSubmitFreshnessReadiness(pkg, checks), nil
}

func (s *service) validateSheinOnlineAuthPreflight(ctx context.Context, task *Task) error {
	if s == nil || task == nil {
		return nil
	}
	_, err := s.buildSheinSubmitProductAPI(ctx, task)
	return err
}

func (s *service) canRunSheinTemplateFreshnessChecks(task *Task, currentCanonical *canonical.Product) bool {
	return s != nil &&
		task != nil &&
		currentCanonical != nil &&
		s.sheinStoreCatalog != nil &&
		s.sheinAPIClientFactory != nil
}

func buildSheinSubmitFreshnessReadiness(pkg *SheinPackage, checks []sheinworkspace.ReadinessCheckSpec) *SheinSubmitReadiness {
	if len(checks) == 0 {
		return nil
	}
	readiness := sheinworkspace.BuildSubmitReadiness(
		checks,
		buildSheinSubmitReadinessGuidanceResolver(pkg),
		"当前 SHEIN 在线模板或店铺状态已变化，提交前需要先刷新在线解析结果",
		"SHEIN 在线模板可用，但仍建议再次确认最新平台状态",
		"SHEIN 在线模板与登录态仍可用于当前提交",
	)
	if readiness == nil {
		return nil
	}
	return shapeSheinSubmitReadinessSummary(readiness, sheinSubmitReadinessSummaryShape{
		blockingLabel:       "在线阻断项：",
		prependFirstBlocker: true,
	})
}

func evaluateSheinCategoryFreshness(current *SheinPackage, info *sheincategory.CategoryInfo) (bool, string) {
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

func evaluateSheinAttributeFreshness(current *SheinPackage, templates *sheinattribute.AttributeTemplateInfo) (bool, string) {
	current = sheinpub.NormalizePackageSemanticFields(current)
	if current == nil {
		return true, ""
	}
	if templates == nil || len(templates.Data) == 0 {
		return false, "当前普通属性模板在线校验失败，需重新刷新属性模板后再提交"
	}

	templateContext, ok := buildSheinAttributeFreshnessTemplateContext(current, templates)
	if !ok {
		return false, "当前普通属性模板为空，需重新刷新属性模板后再提交"
	}

	return evaluateSheinResolvedAttributeFreshness(current, templateContext)
}

func evaluateSheinSaleAttributeFreshness(current *SheinPackage, templates *sheinattribute.AttributeTemplateInfo) (bool, string) {
	ok, message, _ := evaluateSheinSaleAttributeFreshnessWithCustomValidation(current, templates, nil)
	return ok, message
}

func evaluateSheinSaleAttributeFreshnessWithCustomValidation(
	current *SheinPackage,
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

	templateContext, ok := buildSheinSaleAttributeFreshnessTemplateContext(templates)
	if !ok {
		return false, "当前销售属性模板为空，需重新刷新销售属性后再提交", false
	}

	return evaluateSheinSaleAttributeFreshnessResolution(current, currentResolution, templateContext, api)
}

func repairSheinFreshnessSaleAttributes(
	current *SheinPackage,
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

func joinReviewNotes(notes []string) string {
	return strings.Join(uniqueNonEmptyStrings(notes), "；")
}

func sheinFreshnessCanonicalProduct(task *Task) *canonical.Product {
	if task == nil || task.Result == nil {
		return nil
	}
	if task.Result.CanonicalProduct != nil {
		return task.Result.CanonicalProduct
	}
	if task.Result.StandardProductSnapshot != nil {
		return task.Result.StandardProductSnapshot.CanonicalProduct
	}
	return nil
}

func sheinFreshnessImageAssets(task *Task) *productimage.ImageProcessResult {
	if task == nil || task.Result == nil {
		return nil
	}
	if task.Result.ImageAssets != nil {
		return task.Result.ImageAssets
	}
	if task.Result.StandardProductSnapshot != nil {
		return task.Result.StandardProductSnapshot.ImageAssets
	}
	return nil
}

func cloneSheinPackageForFreshness(pkg *SheinPackage) (*SheinPackage, error) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil, nil
	}
	raw, err := json.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	var cloned SheinPackage
	if err := json.Unmarshal(raw, &cloned); err != nil {
		return nil, err
	}
	return sheinpub.NormalizePackageSemanticFields(&cloned), nil
}

func sameResolvedAttributeSet(left []sheinpub.ResolvedAttribute, right []sheinpub.ResolvedAttribute) bool {
	return sameNormalizedStringSet(normalizeResolvedAttributes(left), normalizeResolvedAttributes(right))
}

func sameResolvedSaleAttributeSet(left []sheinpub.ResolvedSaleAttribute, right []sheinpub.ResolvedSaleAttribute) bool {
	return sameNormalizedStringSet(normalizeResolvedSaleAttributes(left), normalizeResolvedSaleAttributes(right))
}

func sameNormalizedStringSet(left []string, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	sort.Strings(left)
	sort.Strings(right)
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}

func normalizeResolvedAttributes(items []sheinpub.ResolvedAttribute) []string {
	if len(items) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		valueID := 0
		if item.AttributeValueID != nil {
			valueID = *item.AttributeValueID
		}
		normalized = append(normalized, fmt.Sprintf(
			"%d|%d|%s|%s",
			item.AttributeID,
			valueID,
			strings.ToLower(strings.TrimSpace(item.Value)),
			strings.ToLower(strings.TrimSpace(item.AttributeExtraValue)),
		))
	}
	return normalized
}

func buildResolvedAttributeFreshnessDriftMessage(current []sheinpub.ResolvedAttribute, fresh []sheinpub.ResolvedAttribute) string {
	currentOnly, freshOnly := diffResolvedAttributes(current, fresh)
	parts := []string{"当前普通属性模板已变化，现有 resolved attributes 与在线模板结果不一致"}
	if len(currentOnly) > 0 {
		parts = append(parts, "当前任务独有: "+strings.Join(currentOnly, "; "))
	}
	if len(freshOnly) > 0 {
		parts = append(parts, "在线模板独有: "+strings.Join(freshOnly, "; "))
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

func (s *service) loadSheinAttributeTemplatesForFreshness(
	ctx context.Context,
	task *Task,
	categoryID int,
) (*sheinattribute.AttributeTemplateInfo, error) {
	if categoryID <= 0 {
		return nil, fmt.Errorf("missing SHEIN category_id")
	}
	api, err := s.buildSheinAttributeAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	return api.GetAttributeTemplates(categoryID)
}

func (s *service) loadSheinCategoryInfoForFreshness(
	ctx context.Context,
	task *Task,
	categoryID int,
) (*sheincategory.CategoryInfo, error) {
	if categoryID <= 0 {
		return nil, fmt.Errorf("missing SHEIN category_id")
	}
	api, err := s.buildSheinCategoryAPI(ctx, task)
	if err != nil {
		return nil, err
	}
	return api.GetCategory(categoryID)
}

func sheinResolvedAttributeStillLegal(
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

func sheinpubDependencyIsActive(attr sheinattribute.AttributeInfo, resolvedByID map[int]sheinpub.ResolvedAttribute) bool {
	if attr.CascadeAttributeID <= 0 {
		return true
	}
	parent, ok := resolvedByID[attr.CascadeAttributeID]
	if !ok || parent.AttributeID <= 0 {
		return false
	}
	if sheinConditionalOtherAttribute(attr, parent) {
		return false
	}
	allowed := sheinParseCascadeValueIDs(attr.CascadeAttributeValueIDList)
	if len(allowed) == 0 {
		return true
	}
	if parent.AttributeValueID == nil || *parent.AttributeValueID <= 0 {
		return false
	}
	_, ok = allowed[*parent.AttributeValueID]
	return ok
}

func sheinConditionalOtherAttribute(attr sheinattribute.AttributeInfo, parent sheinpub.ResolvedAttribute) bool {
	name := normalizeSheinText(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
	if name == "" || !strings.HasPrefix(name, "other ") {
		return false
	}
	if values := sheinParseCascadeValueIDs(attr.CascadeAttributeValueIDList); len(values) > 0 {
		return false
	}
	return parent.AttributeValueID != nil && *parent.AttributeValueID > 0
}

func sheinParseCascadeValueIDs(raw *string) map[int]struct{} {
	if raw == nil {
		return nil
	}
	text := strings.TrimSpace(*raw)
	if text == "" {
		return nil
	}
	fields := strings.FieldsFunc(text, func(r rune) bool {
		return r == ',' || r == ';' || r == '|' || r == ' ' || r == '\n' || r == '\t'
	})
	if len(fields) == 0 {
		return nil
	}
	result := make(map[int]struct{}, len(fields))
	for _, field := range fields {
		id, err := strconv.Atoi(strings.TrimSpace(field))
		if err != nil || id <= 0 {
			continue
		}
		result[id] = struct{}{}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func formatSheinFreshnessAttributeName(attr sheinattribute.AttributeInfo) string {
	return strings.TrimSpace(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName))
}

func filterSheinFreshnessDisplayAttributes(attributes []sheinattribute.AttributeInfo) []sheinattribute.AttributeInfo {
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

func buildSheinFreshnessSaleTemplateOptions(info *sheinattribute.AttributeTemplateInfo) []sheinpub.SaleAttributeTemplateOption {
	if info == nil || len(info.Data) == 0 {
		return nil
	}
	template := info.Data[0]
	attributes := orderSheinFreshnessSaleScopeAttributes(filterSheinFreshnessSaleScopeAttributes(template.AttributeInfos), template.AttributeID)
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

func filterSheinFreshnessSaleScopeAttributes(attributes []sheinattribute.AttributeInfo) []sheinattribute.AttributeInfo {
	result := make([]sheinattribute.AttributeInfo, 0, len(attributes))
	for _, attr := range attributes {
		if attr.AttributeType == 1 || (attr.SKCScope != nil && *attr.SKCScope) {
			result = append(result, attr)
			continue
		}
		switch normalizeSheinText(firstNonEmpty(attr.AttributeNameEn, attr.AttributeName)) {
		case "color", "colour", "size", "style", "pattern", "capacity", "type", "model", "set", "颜色", "颜色分类", "尺码", "尺寸", "规格", "容量", "款式", "类型", "型号", "套装":
			result = append(result, attr)
		}
	}
	return result
}

func orderSheinFreshnessSaleScopeAttributes(attributes []sheinattribute.AttributeInfo, orderedIDs []int) []sheinattribute.AttributeInfo {
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

func sheinFreshnessCustomAttributeValueIDs(relations []sheinattribute.CustomAttributeRelation) map[int]struct{} {
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

func normalizeResolvedSaleAttributes(items []sheinpub.ResolvedSaleAttribute) []string {
	if len(items) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(items))
	for _, item := range items {
		valueID := 0
		if item.AttributeValueID != nil {
			valueID = *item.AttributeValueID
		}
		normalized = append(normalized, fmt.Sprintf(
			"%s|%d|%d|%s",
			strings.ToLower(strings.TrimSpace(item.Scope)),
			item.AttributeID,
			valueID,
			strings.ToLower(strings.TrimSpace(item.Value)),
		))
	}
	return normalized
}

type sheinFreshnessRuntimeClientFactory struct {
	svc  *service
	task *Task
}

func (f sheinFreshnessRuntimeClientFactory) NewAPIClient(ctx context.Context, storeID int64) *sheinclient.APIClient {
	if f.svc == nil || f.task == nil {
		return nil
	}
	client, resolvedStoreID, err := f.svc.newSheinAPIClient(ctx, f.task)
	if err != nil {
		return nil
	}
	if storeID > 0 && resolvedStoreID > 0 && resolvedStoreID != storeID {
		return nil
	}
	return client
}
