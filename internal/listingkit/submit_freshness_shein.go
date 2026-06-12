package listingkit

import (
	"context"
	"fmt"
	"strings"
	"time"

	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
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
