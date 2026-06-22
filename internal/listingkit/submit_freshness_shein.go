package listingkit

import (
	"context"

	"task-processor/internal/catalog/canonical"
	sheinworkspace "task-processor/internal/marketplace/shein/workspace"
	sheinpub "task-processor/internal/publishing/shein"
)

const (
	sheinFreshnessAuthKey          = sheinworkspace.FreshnessAuthKey
	sheinFreshnessCategoryKey      = sheinworkspace.FreshnessCategoryKey
	sheinFreshnessAttributeKey     = sheinworkspace.FreshnessAttributeKey
	sheinFreshnessSaleAttributeKey = sheinworkspace.FreshnessSaleAttributeKey
)

func (s *service) validateSheinPublishFreshness(ctx context.Context, task *Task, pkg *SheinPackage, action string) (*SheinSubmitReadiness, error) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if s == nil || task == nil || pkg == nil {
		return nil, nil
	}

	checks := make([]sheinworkspace.ReadinessCheckSpec, 0, 4)
	addCheck := func(check sheinworkspace.ReadinessCheckSpec) { checks = append(checks, check) }

	if err := s.validateSheinOnlineAuthPreflight(ctx, task); err != nil {
		addCheck(sheinworkspace.BuildFreshnessAuthFailureCheck(err))
		return buildSheinSubmitFreshnessReadiness(pkg, checks), nil
	}
	addCheck(sheinworkspace.BuildFreshnessAuthSuccessCheck())

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
