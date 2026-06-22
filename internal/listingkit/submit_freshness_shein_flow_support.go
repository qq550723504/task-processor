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
		check := sheinworkspace.BuildFreshnessCategoryCheck(false, "SHEIN 在线模板预检失败，当前无法构建 freshness 校验上下文")
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
	return sheinworkspace.BuildFreshnessCategoryCheck(categoryReady, categoryMessage), categoryReady
}

func (s *service) validateSheinFreshnessAttributes(ctx context.Context, task *Task, pkg *SheinPackage, state *sheinFreshnessValidationContext) sheinworkspace.ReadinessCheckSpec {
	attributeTemplates, attributeTemplateErr := s.loadSheinAttributeTemplatesForFreshness(ctx, task, state.freshPkg.CategoryID)
	state.attributeTemplates = attributeTemplates

	attributeReady, attributeMessage := evaluateSheinAttributeFreshness(pkg, attributeTemplates)
	if attributeTemplateErr != nil {
		attributeReady = false
		attributeMessage = "当前普通属性模板在线校验失败，需重新刷新属性模板后再提交：" + strings.TrimSpace(attributeTemplateErr.Error())
		return sheinworkspace.BuildFreshnessAttributeCheck(attributeReady, attributeMessage)
	}

	state.saleAPI, _ = s.buildSheinAttributeAPI(ctx, task)
	return sheinworkspace.BuildFreshnessAttributeCheck(attributeReady, attributeMessage)
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
	return sheinworkspace.BuildFreshnessSaleAttributeCheck(saleReady, saleMessage), nil
}
