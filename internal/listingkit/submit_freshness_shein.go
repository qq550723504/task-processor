package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
	sheincategory "task-processor/internal/shein/api/category"
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
	_, err := s.taskSubmissionExecutionOrDefault().buildSheinSubmitProductAPI(ctx, task)
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

type sheinFreshnessRuntimeClientFactory struct {
	svc  *service
	task *Task
}

func (f sheinFreshnessRuntimeClientFactory) NewAPIClient(ctx context.Context, storeID int64) *SheinRuntimeAPIClient {
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
