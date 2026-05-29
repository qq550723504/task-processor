package listingkit

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"task-processor/internal/catalog/canonical"
	"task-processor/internal/productimage"
	sheinpub "task-processor/internal/publishing/shein"
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

	runtimeFactory := sheinFreshnessRuntimeClientFactory{svc: s, task: task}
	categoryResolver := sheinpub.NewRuntimeCategoryResolver(runtimeFactory, s.sheinContentOptimizer)
	attributeResolver := sheinpub.NewRuntimeAttributeResolver(runtimeFactory, s.sheinContentOptimizer)
	saleAttributeResolver := sheinpub.NewRuntimeSaleAttributeResolver(runtimeFactory, s.sheinContentOptimizer, s.sheinResolutionCacheStore)

	categoryReq := *req
	if pkg.CategoryID > 0 {
		categoryReq.TargetCategoryHint = strconv.Itoa(pkg.CategoryID)
	}
	freshCategory := categoryResolver.Resolve(&categoryReq, currentCanonical, freshPkg)
	categoryReady, categoryMessage := evaluateSheinCategoryFreshness(pkg, freshCategory)
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

	freshPkg.CategoryResolution = freshCategory
	sheinpub.ApplyCategoryResolution(freshPkg, freshCategory)
	sheinpub.RefreshDerivedState(
		req,
		currentCanonical,
		sheinFreshnessImageAssets(task),
		freshPkg,
		nil,
		attributeResolver,
		saleAttributeResolver,
		s.sheinPricingPolicy,
	)

	attributeReady, attributeMessage := evaluateSheinAttributeFreshness(pkg, freshPkg.AttributeResolution)
	addCheck(
		sheinFreshnessAttributeKey,
		"普通属性模板新鲜度",
		attributeReady,
		attributeMessage,
		[]string{"shein.resolved_attributes", "shein.attribute_resolution"},
		"刷新属性模板",
	)

	saleReady, saleMessage := evaluateSheinSaleAttributeFreshness(pkg, freshPkg.SaleAttributeResolution)
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
		func(spec sheinworkspace.ReadinessCheckSpec) sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint] {
			guidance := buildSheinReadinessGuidance(pkg, spec.Key, spec.FieldPaths, spec.SuggestedAction, spec.WarningOnly)
			return sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint]{
				Reason:      cloneSheinReadinessReason(guidance.reason),
				RepairHints: cloneSheinRepairHints(guidance.repairHints),
			}
		},
		"当前 SHEIN 在线模板或店铺状态已变化，提交前需要先刷新在线解析结果",
		"SHEIN 在线模板可用，但仍建议再次确认最新平台状态",
		"SHEIN 在线模板与登录态仍可用于当前提交",
	)
	if readiness == nil {
		return nil
	}
	if len(readiness.BlockingItems) > 0 {
		if message := strings.TrimSpace(readiness.BlockingItems[0].Message); message != "" {
			readiness.Summary = append([]string{message}, readiness.Summary...)
		}
		readiness.Summary = append(readiness.Summary, "在线阻断项："+sheinworkspace.JoinReadinessLabels(readiness.BlockingItems, "、"))
	}
	readiness.Summary = uniqueStrings(readiness.Summary)
	return readiness
}

func evaluateSheinCategoryFreshness(current *SheinPackage, fresh *sheinpub.CategoryResolution) (bool, string) {
	current = sheinpub.NormalizePackageSemanticFields(current)
	if current == nil {
		return true, ""
	}
	if fresh == nil {
		return false, "当前类目模板在线校验失败，需重新刷新类目结果后再提交"
	}
	if fresh.Status != "resolved" {
		return false, firstNonEmptyString(joinReviewNotes(fresh.ReviewNotes), "当前类目模板在线校验未通过，需重新刷新类目结果后再提交")
	}
	currentProductTypeID := 0
	if current.ProductTypeID != nil {
		currentProductTypeID = *current.ProductTypeID
	}
	if fresh.CategoryID != current.CategoryID || fresh.ProductTypeID != currentProductTypeID {
		return false, fmt.Sprintf(
			"当前类目模板已发生变化：原 category_id=%d/product_type_id=%d，当前在线结果为 category_id=%d/product_type_id=%d",
			current.CategoryID,
			currentProductTypeID,
			fresh.CategoryID,
			fresh.ProductTypeID,
		)
	}
	return true, "当前类目模板仍与在线结果一致"
}

func evaluateSheinAttributeFreshness(current *SheinPackage, fresh *sheinpub.AttributeResolution) (bool, string) {
	current = sheinpub.NormalizePackageSemanticFields(current)
	if current == nil {
		return true, ""
	}
	if fresh == nil {
		return false, "当前普通属性模板在线校验失败，需重新刷新属性模板后再提交"
	}
	if fresh.Status != "resolved" || fresh.UnresolvedCount > 0 || len(fresh.PendingAttributes) > 0 {
		return false, firstNonEmptyString(joinReviewNotes(fresh.ReviewNotes), "当前普通属性模板已变化，需重新刷新属性模板后再提交")
	}
	if !sameResolvedAttributeSet(current.ResolvedAttributes, fresh.ResolvedAttributes) {
		return false, buildResolvedAttributeFreshnessDriftMessage(current.ResolvedAttributes, fresh.ResolvedAttributes)
	}
	return true, "当前普通属性模板仍与在线结果一致"
}

func evaluateSheinSaleAttributeFreshness(current *SheinPackage, fresh *sheinpub.SaleAttributeResolution) (bool, string) {
	current = sheinpub.NormalizePackageSemanticFields(current)
	if current == nil {
		return true, ""
	}
	if fresh == nil {
		return false, "当前销售属性模板在线校验失败，需重新刷新销售属性后再提交"
	}
	currentResolution := current.SaleAttributeResolution
	if currentResolution == nil {
		return true, ""
	}
	if fresh.Status != "resolved" {
		return false, firstNonEmptyString(joinReviewNotes(fresh.ReviewNotes), "当前销售属性模板已变化，需重新刷新销售属性后再提交")
	}
	if fresh.PrimaryAttributeID != currentResolution.PrimaryAttributeID || fresh.SecondaryAttributeID != currentResolution.SecondaryAttributeID {
		return false, "当前销售属性模板已变化，主副规格映射与在线模板结果不一致"
	}
	if !sameResolvedSaleAttributeSet(currentResolution.SKCAttributes, fresh.SKCAttributes) ||
		!sameResolvedSaleAttributeSet(currentResolution.SKUAttributes, fresh.SKUAttributes) {
		return false, "当前销售属性模板已变化，sale attribute/value 映射与在线模板结果不一致"
	}
	return true, "当前销售属性模板仍与在线结果一致"
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
