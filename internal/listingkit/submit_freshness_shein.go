package listingkit

import (
	"context"
	"fmt"

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
	addCheck := func(check sheinworkspace.ReadinessCheckSpec) { checks = append(checks, check) }

	if err := s.validateSheinOnlineAuthPreflight(ctx, task); err != nil {
		addCheck(buildSheinFreshnessAuthFailureCheck(err))
		return buildSheinSubmitFreshnessReadiness(pkg, checks), nil
	}
	addCheck(buildSheinFreshnessAuthSuccessCheck())

	currentCanonical := sheinFreshnessCanonicalProduct(task)
	if !s.canRunSheinTemplateFreshnessChecks(task, currentCanonical) {
		return buildSheinSubmitFreshnessReadiness(pkg, checks), nil
	}

	state, preflightCheck := s.prepareSheinFreshnessValidationContext(ctx, task, pkg)
	if preflightCheck != nil {
		addCheck(*preflightCheck)
		return buildSheinSubmitFreshnessReadiness(pkg, checks), nil
	}
	if state == nil || state.request == nil {
		return buildSheinSubmitFreshnessReadiness(pkg, checks), nil
	}

	categoryCheck, categoryReady := s.validateSheinFreshnessCategory(state.request.Context, task, pkg, state)
	addCheck(categoryCheck)
	if !categoryReady {
		return buildSheinSubmitFreshnessReadiness(pkg, checks), nil
	}

	addCheck(s.validateSheinFreshnessAttributes(state.request.Context, task, pkg, state))

	saleCheck, err := s.validateSheinFreshnessSaleAttributes(state.request.Context, task, pkg, state)
	if err != nil {
		return nil, err
	}
	addCheck(saleCheck)

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
		resolveSheinStoreCatalog(s) != nil &&
		resolveSheinAPIClientFactory(s) != nil
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
