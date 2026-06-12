package listingkit

import (
	"context"
	"strings"
	"time"

	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
	sheinattribute "task-processor/internal/shein/api/attribute"
)

type sheinFreshnessValidationContext struct {
	request            *sheinpub.BuildRequest
	freshPkg           *SheinPackage
	attributeTemplates *sheinattribute.AttributeTemplateInfo
	saleAPI            sheinpub.AttributeAPI
}

func buildSheinFreshnessAuthFailureCheck(err error) sheinworkspace.ReadinessCheckSpec {
	return sheinworkspace.ReadinessCheckSpec{
		Key:             sheinFreshnessAuthKey,
		Label:           "SHEIN 在线登录态",
		OK:              false,
		Message:         "SHEIN 提交店铺当前不可用，请先刷新登录态后再提交：" + strings.TrimSpace(err.Error()),
		FieldPaths:      []string{"shein.store_resolution", "shein.review_notes"},
		SuggestedAction: "重新登录 SHEIN 店铺",
	}
}

func buildSheinFreshnessAuthSuccessCheck() sheinworkspace.ReadinessCheckSpec {
	return sheinworkspace.ReadinessCheckSpec{
		Key:             sheinFreshnessAuthKey,
		Label:           "SHEIN 在线登录态",
		OK:              true,
		Message:         "SHEIN 提交店铺当前可用",
		FieldPaths:      []string{"shein.store_resolution"},
		SuggestedAction: "重新登录 SHEIN 店铺",
	}
}

func buildSheinFreshnessCategoryCheck(ok bool, message string) sheinworkspace.ReadinessCheckSpec {
	return sheinworkspace.ReadinessCheckSpec{
		Key:             sheinFreshnessCategoryKey,
		Label:           "类目模板新鲜度",
		OK:              ok,
		Message:         message,
		FieldPaths:      []string{"shein.category_id", "shein.category_id_list", "shein.product_type_id"},
		SuggestedAction: "刷新类目模板",
	}
}

func buildSheinFreshnessAttributeCheck(ok bool, message string) sheinworkspace.ReadinessCheckSpec {
	return sheinworkspace.ReadinessCheckSpec{
		Key:             sheinFreshnessAttributeKey,
		Label:           "普通属性模板新鲜度",
		OK:              ok,
		Message:         message,
		FieldPaths:      []string{"shein.resolved_attributes", "shein.attribute_resolution"},
		SuggestedAction: "刷新属性模板",
	}
}

func buildSheinFreshnessSaleAttributeCheck(ok bool, message string) sheinworkspace.ReadinessCheckSpec {
	return sheinworkspace.ReadinessCheckSpec{
		Key:             sheinFreshnessSaleAttributeKey,
		Label:           "销售属性模板新鲜度",
		OK:              ok,
		Message:         message,
		FieldPaths:      []string{"shein.sale_attribute_resolution", "shein.request_draft.skc_list"},
		SuggestedAction: "刷新销售属性",
	}
}

func (s *service) prepareSheinFreshnessValidationContext(ctx context.Context, task *Task, pkg *SheinPackage) (*sheinFreshnessValidationContext, *sheinworkspace.ReadinessCheckSpec) {
	req := buildSheinPublishRequestForTask(task, task.Request)
	if req == nil {
		return nil, nil
	}
	if storeID, err := s.resolveSheinStoreID(ctx, task); err == nil && storeID > 0 {
		req.SheinStoreID = storeID
	}

	freshPkg, err := cloneSheinPackageForFreshness(pkg)
	if err != nil {
		check := buildSheinFreshnessCategoryCheck(false, "SHEIN 在线模板预检失败，当前无法构建 freshness 校验上下文")
		return nil, &check
	}

	return &sheinFreshnessValidationContext{
		request:  req,
		freshPkg: freshPkg,
	}, nil
}

func (s *service) validateSheinFreshnessCategory(ctx context.Context, task *Task, pkg *SheinPackage, state *sheinFreshnessValidationContext) (sheinworkspace.ReadinessCheckSpec, bool) {
	categoryInfo, categoryInfoErr := s.loadSheinCategoryInfoForFreshness(ctx, task, pkg.CategoryID)
	categoryReady, categoryMessage := evaluateSheinCategoryFreshness(pkg, categoryInfo)
	if categoryInfoErr != nil {
		categoryReady = false
		categoryMessage = "当前类目模板在线校验失败，需重新刷新类目结果后再提交：" + strings.TrimSpace(categoryInfoErr.Error())
	}
	return buildSheinFreshnessCategoryCheck(categoryReady, categoryMessage), categoryReady
}

func (s *service) validateSheinFreshnessAttributes(ctx context.Context, task *Task, pkg *SheinPackage, state *sheinFreshnessValidationContext) sheinworkspace.ReadinessCheckSpec {
	attributeTemplates, attributeTemplateErr := s.loadSheinAttributeTemplatesForFreshness(ctx, task, state.freshPkg.CategoryID)
	state.attributeTemplates = attributeTemplates

	attributeReady, attributeMessage := evaluateSheinAttributeFreshness(pkg, attributeTemplates)
	if attributeTemplateErr != nil {
		attributeReady = false
		attributeMessage = "当前普通属性模板在线校验失败，需重新刷新属性模板后再提交：" + strings.TrimSpace(attributeTemplateErr.Error())
		return buildSheinFreshnessAttributeCheck(attributeReady, attributeMessage)
	}

	state.saleAPI, _ = s.buildSheinAttributeAPI(ctx, task)
	return buildSheinFreshnessAttributeCheck(attributeReady, attributeMessage)
}

func (s *service) validateSheinFreshnessSaleAttributes(ctx context.Context, task *Task, pkg *SheinPackage, state *sheinFreshnessValidationContext) (sheinworkspace.ReadinessCheckSpec, error) {
	_ = ctx
	_ = task

	saleReady, saleMessage, saleChanged := evaluateSheinSaleAttributeFreshnessWithCustomValidation(pkg, state.attributeTemplates, state.saleAPI)
	if saleChanged && task.Result != nil && s.repo != nil {
		task.Result.UpdatedAt = time.Now()
		if err := s.repo.SaveTaskResult(ctx, task.ID, task.Result); err != nil {
			return sheinworkspace.ReadinessCheckSpec{}, err
		}
	}
	return buildSheinFreshnessSaleAttributeCheck(saleReady, saleMessage), nil
}
