package listingkit

import (
	"strings"

	listingsubmission "task-processor/internal/listingkit/submission"
	sheinworkspace "task-processor/internal/listingkit/workspace/shein"
	sheinpub "task-processor/internal/publishing/shein"
	sheinproduct "task-processor/internal/shein/api/product"
)

type sheinSubmitReadinessSummaryShape struct {
	blockingLabel       string
	warningLabel        string
	prependFirstBlocker bool
}

type SheinReadinessReason struct {
	Code     string `json:"code,omitempty"`
	Category string `json:"category,omitempty"`
	Summary  string `json:"summary,omitempty"`
}

type SheinRepairHint struct {
	Action        string                        `json:"action,omitempty"`
	Priority      string                        `json:"priority,omitempty"`
	Target        string                        `json:"target,omitempty"`
	EditorSection string                        `json:"editor_section,omitempty"`
	EditorFocus   []string                      `json:"editor_focus,omitempty"`
	RevisionPath  string                        `json:"revision_path,omitempty"`
	Description   string                        `json:"description,omitempty"`
	FieldPaths    []string                      `json:"field_paths,omitempty"`
	Patch         *SheinRepairPatchPayload      `json:"patch,omitempty"`
	Skeleton      *SheinEditorRevisionSkeleton  `json:"skeleton,omitempty"`
	Revision      *ApplyRevisionRequest         `json:"revision,omitempty"`
	Validation    *SheinRepairValidationPreview `json:"validation,omitempty"`
}

type sheinReadinessGuidance struct {
	reason      *SheinReadinessReason
	repairHints []SheinRepairHint
}

func buildSheinSubmitReadiness(pkg *SheinPackage) *SheinSubmitReadiness {
	return buildSheinSubmitReadinessWithPodForAction(pkg, nil, "publish")
}

func buildSheinSubmitReadinessForAction(pkg *SheinPackage, action string) *SheinSubmitReadiness {
	return buildSheinSubmitReadinessWithPodForAction(pkg, nil, action)
}

func buildSheinSubmitReadinessWithPod(pkg *SheinPackage, pod *PodExecutionSummary) *SheinSubmitReadiness {
	return buildSheinSubmitReadinessWithPodForAction(pkg, pod, "publish")
}

func buildSheinSubmitReadinessWithPodForAction(pkg *SheinPackage, pod *PodExecutionSummary, action string) *SheinSubmitReadiness {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return nil
	}
	pod = normalizePodExecutionSummary(clonePodExecutionSummary(pod))
	action = strings.ToLower(strings.TrimSpace(action))
	if action == "" {
		action = "publish"
	}

	checks := make([]sheinworkspace.ReadinessCheckSpec, 0, 8)
	validation := ValidateSheinPackageAgainstTemplates(pkg)

	addCheck := func(key, label string, ok bool, message string, fieldPaths []string, suggestedAction string, warningOnly bool) {
		checks = append(checks, sheinworkspace.ReadinessCheckSpec{
			Key:             key,
			Label:           label,
			OK:              ok,
			Message:         message,
			FieldPaths:      append([]string(nil), fieldPaths...),
			SuggestedAction: suggestedAction,
			WarningOnly:     warningOnly,
		})
	}

	addCheck(
		sheinCookieUnavailableIssueCode,
		"SHEIN 店铺登录",
		!sheinCookieUnavailable(pkg),
		"SHEIN 店铺 cookie 不可用。标准商品和 SDS 图片仍可继续使用，但 SHEIN 平台在线类目、属性解析和提交已在平台阶段受阻，请先重新登录店铺后再继续。",
		[]string{"shein.review_notes", "shein.category_resolution.review_notes", "shein.attribute_resolution.review_notes", "shein.sale_attribute_resolution.review_notes"},
		"重新登录 SHEIN 店铺",
		false,
	)

	if pod != nil && pod.DependencyMode != podDependencyModeDisabled {
		podBlocked := action != "save_draft" && podSubmissionBlocked(pod)
		podMessage := podReadinessMessage(pod)
		if action == "save_draft" && pod.Status != podStatusSucceeded {
			podMessage = firstNonEmptyString(podMessage, "POD 平台处理尚未完成；当前允许先保存草稿，正式发布前仍需确认平台结果")
		}
		addCheck(
			"pod_platform",
			"POD 平台处理",
			!podBlocked && (action == "save_draft" || (pod.Status != podStatusFailedDegraded && pod.Status != podStatusBypassed)),
			firstNonEmptyString(podMessage, "POD 平台处理状态尚未满足发布要求"),
			[]string{"pod_execution"},
			"处理 POD 平台结果",
			action == "save_draft" || !podBlocked,
		)
	}

	addCheck(
		"category",
		"类目骨架",
		validation.categoryReady,
		validation.categoryMessage,
		[]string{"shein.category_id", "shein.category_id_list", "shein.product_type_id", "shein.sale_attribute_resolution.category_review_reason"},
		"确认类目",
		false,
	)

	addCheck(
		"category_review",
		"类目复核",
		validation.categoryReviewReady,
		"当前类目仍被建议复核，提交前必须先确认 SHEIN 类目是否匹配",
		[]string{"shein.category_resolution.suggested_category", "shein.sale_attribute_resolution.category_review_reason"},
		"复核类目",
		false,
	)

	addCheck(
		"attributes",
		"普通属性",
		validation.attributeReady,
		validation.attributeMessage,
		[]string{"shein.resolved_attributes", "shein.request_draft.resolved_attributes"},
		"确认属性",
		false,
	)

	addCheck(
		"attribute_review",
		"属性复核",
		validation.attributeReady,
		"普通属性仍有模板必填项未确认，提交前必须补齐或人工确认",
		[]string{"shein.attribute_resolution.pending_attributes", "shein.attribute_resolution.review_notes"},
		"复核属性",
		false,
	)

	addCheck(
		"sale_attributes",
		"销售属性",
		validation.saleAttributeReady,
		validation.saleAttributeMessage,
		[]string{"shein.sale_attribute_resolution", "shein.request_draft.skc_list"},
		"确认规格",
		false,
	)

	requestDraftReady := pkg.DraftPayload != nil
	addCheck(
		"request_draft",
		"请求草稿",
		requestDraftReady,
		"request_draft 尚未生成，当前无法作为提交草稿继续流转",
		[]string{"shein.request_draft"},
		"重新生成预览草稿",
		false,
	)

	previewProductReady := pkg.PreviewPayload != nil
	addCheck(
		"preview_product",
		"预览载荷",
		previewProductReady,
		"preview_product 尚未生成，当前还不能进入提交前预览校验",
		[]string{"shein.preview_product"},
		"重建预览载荷",
		false,
	)

	imageReady := sheinHasSubmitImage(pkg)
	addCheck(
		"images",
		"主图资产",
		imageReady,
		"主图还没有准备好，提交前至少需要一张可用主图",
		[]string{"shein.images.main_image", "shein.request_draft.image_info.main_image"},
		"补充主图",
		false,
	)

	finalImagesReady, finalImagesMessage := sheinFinalImagesReadyForAction(pkg, action)
	addCheck(
		"final_images",
		"最终图片",
		finalImagesReady,
		finalImagesMessage,
		[]string{"shein.final_draft.main_image_url", "shein.final_draft.image_role_overrides", "shein.preview_product.image_info"},
		"确认图片",
		false,
	)

	coverageMessage, coverageBlocked := sheinVariantImageCoverageStatus(pkg)
	addCheck(
		"variant_image_coverage",
		"变体图片覆盖",
		!coverageBlocked,
		firstNonEmptyString(coverageMessage, "变体图片覆盖不完整，请为每个颜色规格补齐独立商品图后再提交"),
		[]string{"shein.metadata.variant_image_coverage_status", "shein.metadata.variant_image_coverage_message", "shein.request_draft.skc_list"},
		"确认图片",
		false,
	)

	skcCount := len(pkg.SkcList)
	if skcCount == 0 && pkg.DraftPayload != nil {
		skcCount = len(pkg.DraftPayload.SKCList)
	}
	addCheck(
		"variants",
		"规格结构",
		skcCount > 0 && sheinHasAnySKU(pkg),
		"当前还没有完整的 SKC/SKU 结构，提交前需要至少一个 SKC 和一个 SKU",
		[]string{"shein.skc_list", "shein.request_draft.skc_list"},
		"补充规格",
		false,
	)

	pricingReady := pkg.Pricing == nil || sheinPricingReady(pkg)
	addCheck(
		"pricing",
		"价格确认",
		pricingReady,
		"SKU 价格尚未全部生成或确认，提交前需要完成价格规则预览和人工覆盖确认",
		[]string{"shein.pricing", "shein.request_draft.skc_list.sku_list.base_price"},
		"确认价格",
		false,
	)

	finalDraftReady := pkg.FinalSubmissionDraft == nil || pkg.FinalSubmissionDraft.Confirmed
	if action == "save_draft" {
		finalDraftReady = true
	}
	addCheck(
		"final_review",
		"最终确认",
		finalDraftReady,
		func() string {
			if action == "save_draft" {
				return "保存草稿允许跳过最终确认；正式发布前仍需在最终确认页核对图片、价格、属性和 SKU"
			}
			return "提交前必须在最终确认页核对图片、价格、属性和 SKU 后确认"
		}(),
		[]string{"shein.final_draft", "shein.final_review"},
		"最终确认",
		false,
	)

	manualNotes := filterManualSheinReviewNotes(pkg.ReviewNotes)
	addCheck(
		"manual_notes",
		"人工备注",
		len(manualNotes) == 0,
		"仍有人工备注未处理，建议在提交前再次确认",
		[]string{"shein.review_notes"},
		"处理备注",
		true,
	)
	sourceFactsReady, sourceFactsMessage := sheinSourceFactsReady(pkg)
	addCheck(
		"source_facts",
		"来源事实",
		sourceFactsReady,
		sourceFactsMessage,
		[]string{"shein.metadata.source_fact_review_required", "shein.metadata.source_fact_review_fields"},
		"复核来源事实",
		false,
	)
	checks = appendSheinBuildValidationChecks(checks, validation)

	readiness := sheinworkspace.BuildSubmitReadiness(
		checks,
		buildSheinSubmitReadinessGuidanceResolver(pkg),
		"当前仍有关键字段未完成，SHEIN 资料包还不能直接进入提交态",
		"SHEIN 资料包已经基本可提交，但仍建议先处理人工备注",
		"SHEIN 资料包已具备提交前所需的关键骨架",
	)
	if readiness == nil {
		return nil
	}
	return shapeSheinSubmitReadinessSummary(readiness, sheinSubmitReadinessSummaryShape{
		blockingLabel: "待补关键项：",
		warningLabel:  "待确认项：",
	})
}

func buildSheinSubmitReadinessGuidanceResolver(
	pkg *SheinPackage,
) func(spec sheinworkspace.ReadinessCheckSpec) sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint] {
	return func(spec sheinworkspace.ReadinessCheckSpec) sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint] {
		guidance := buildSheinReadinessGuidance(pkg, spec.Key, spec.FieldPaths, spec.SuggestedAction, spec.WarningOnly)
		return sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint]{
			Reason:      cloneSheinReadinessReason(guidance.reason),
			RepairHints: cloneSheinRepairHints(guidance.repairHints),
		}
	}
}

func buildSheinReadinessReason(spec *sheinworkspace.ReadinessReasonSpec) *SheinReadinessReason {
	if spec == nil {
		return nil
	}
	return &SheinReadinessReason{
		Code:     spec.Code,
		Category: spec.Category,
		Summary:  spec.Summary,
	}
}

func buildSheinReadinessPatchPayload(pkg *SheinPackage, key string) *SheinRepairPatchPayload {
	switch key {
	case "category", "category_review":
		return &SheinRepairPatchPayload{
			CategoryResolution: buildSheinCategoryResolutionPatch(pkg),
		}
	case "attributes", "attribute_review":
		return &SheinRepairPatchPayload{
			AttributeResolution: buildSheinAttributeResolutionPatch(pkg),
		}
	case "sale_attributes", "variants":
		return &SheinRepairPatchPayload{
			SaleAttributeResolution: buildSheinSaleAttributeResolutionPatch(pkg),
			SKCPatches:              buildSheinEditorSKCPatches(pkg),
		}
	case "images":
		return &SheinRepairPatchPayload{
			Images: clonePlatformImageSetForEditor(pkg.Images),
		}
	case "manual_notes":
		return &SheinRepairPatchPayload{
			ReviewNotes: append([]string(nil), pkg.ReviewNotes...),
		}
	default:
		return nil
	}
}

func buildSheinReadinessRepairHint(pkg *SheinPackage, action string, fieldPaths []string, hint sheinworkspace.ReadinessHintSpec, patch *SheinRepairPatchPayload) SheinRepairHint {
	artifacts := buildSheinRepairArtifacts(pkg, action, hint.EditorSection, patch)
	return SheinRepairHint{
		Action:        action,
		Priority:      hint.Priority,
		Target:        hint.Target,
		EditorSection: hint.EditorSection,
		EditorFocus:   append([]string(nil), hint.EditorFocus...),
		RevisionPath:  hint.RevisionPath,
		Description:   hint.Description,
		FieldPaths:    append([]string(nil), fieldPaths...),
		Patch:         artifacts.patch,
		Skeleton:      artifacts.skeleton,
		Revision:      artifacts.request,
		Validation:    artifacts.validation,
	}
}

func buildSheinReadinessGuidance(pkg *SheinPackage, key string, fieldPaths []string, suggestedAction string, warningOnly bool) sheinReadinessGuidance {
	spec := sheinworkspace.BuildReadinessGuidanceSpec(key, warningOnly)
	if spec == nil || spec.Reason == nil {
		return sheinReadinessGuidance{}
	}

	guidance := sheinReadinessGuidance{
		reason: buildSheinReadinessReason(spec.Reason),
	}
	patch := buildSheinReadinessPatchPayload(pkg, key)
	for _, hint := range spec.Hints {
		guidance.repairHints = append(guidance.repairHints, buildSheinReadinessRepairHint(
			pkg,
			suggestedAction,
			fieldPaths,
			hint,
			patch,
		))
	}
	return guidance
}

func cloneSheinReadinessReason(reason *SheinReadinessReason) *SheinReadinessReason {
	if reason == nil {
		return nil
	}
	cloned := *reason
	return &cloned
}

func cloneSheinRepairHints(items []SheinRepairHint) []SheinRepairHint {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]SheinRepairHint, 0, len(items))
	for _, item := range items {
		artifacts := cloneSheinRepairArtifacts(item.Patch, item.Skeleton, item.Revision, item.Validation)
		cloned = append(cloned, SheinRepairHint{
			Action:        item.Action,
			Priority:      item.Priority,
			Target:        item.Target,
			EditorSection: item.EditorSection,
			EditorFocus:   append([]string(nil), item.EditorFocus...),
			RevisionPath:  item.RevisionPath,
			Description:   item.Description,
			FieldPaths:    append([]string(nil), item.FieldPaths...),
			Patch:         artifacts.patch,
			Skeleton:      artifacts.skeleton,
			Revision:      artifacts.request,
			Validation:    artifacts.validation,
		})
	}
	return cloned
}

func sheinHasAnySKU(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	for _, skc := range pkg.SkcList {
		if len(skc.SKUs) > 0 {
			return true
		}
	}
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			if len(skc.SKUList) > 0 {
				return true
			}
		}
	}
	return false
}

func sheinSourceFactsReady(pkg *SheinPackage) (bool, string) {
	if pkg == nil {
		return listingsubmission.SourceFactsReady(nil)
	}
	return listingsubmission.SourceFactsReady(pkg.Metadata)
}

func sheinFinalImagesReady(pkg *SheinPackage) (bool, string) {
	return sheinFinalImagesReadyForAction(pkg, "publish")
}

func sheinFinalImagesReadyForAction(pkg *SheinPackage, action string) (bool, string) {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.FinalSubmissionDraft == nil {
		return true, "旧任务未启用最终图片确认，按兼容路径处理"
	}
	action = strings.ToLower(strings.TrimSpace(action))
	main := strings.TrimSpace(pkg.FinalSubmissionDraft.MainImageURL)
	if main == "" && pkg.DraftPayload != nil && pkg.DraftPayload.ImageInfo != nil {
		main = strings.TrimSpace(pkg.DraftPayload.ImageInfo.MainImage)
	}
	if main == "" {
		return false, "最终确认页还没有设置主图"
	}
	if !sheinHasFinalGalleryImage(pkg) {
		return false, "最终图库为空，提交前至少需要一张图库图片"
	}
	if action == "save_draft" {
		return true, "草稿保存图片已具备主图和图库；色块图、SKC 图和尺寸图会在正式发布前严格校验"
	}
	if !sheinHasSKCImage(pkg) {
		return false, "缺少 SKC/色块图，提交前需要为每个颜色规格准备可提交图片"
	}
	if !sheinHasSwatchRole(pkg) {
		return false, "缺少色块图标记，请在 SHEIN data images 中标记一张色块图"
	}
	return true, "最终图片已具备主图、图库和可用的色块/SKC 图；尺寸图未选择时不阻断提交"
}

func sheinHasFinalGalleryImage(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil || pkg.DraftPayload.ImageInfo == nil {
		return false
	}
	return len(uniqueNonEmptyStrings(append([]string{pkg.DraftPayload.ImageInfo.MainImage}, pkg.DraftPayload.ImageInfo.Gallery...))) > 0
}

func sheinHasSKCImage(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	if pkg.DraftPayload != nil {
		for _, skc := range pkg.DraftPayload.SKCList {
			if sheinImageDraftHasImage(skc.ImageInfo) {
				return true
			}
		}
	}
	if pkg.PreviewPayload != nil {
		for _, skc := range pkg.PreviewPayload.SKCList {
			if sheinProductImageInfoHasImage(&skc.ImageInfo) {
				return true
			}
		}
	}
	if sheinHasSingleSKC(pkg) && sheinHasFinalMainImage(pkg) {
		return true
	}
	return false
}

func sheinHasSwatchRole(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.FinalSubmissionDraft == nil {
		return true
	}
	for _, role := range pkg.FinalSubmissionDraft.ImageRoleOverrides {
		switch strings.ToLower(strings.TrimSpace(role)) {
		case "swatch", "skc":
			return true
		}
	}
	// The submit payload normalizer derives a SHEIN color block image from the
	// first SKC image when no explicit swatch role is selected. Readiness should
	// match the actual submit path instead of forcing a redundant UI role.
	return sheinHasSKCImage(pkg)
}

func sheinHasSingleSKC(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	count := 0
	if pkg.DraftPayload != nil && len(pkg.DraftPayload.SKCList) > 0 {
		count = len(pkg.DraftPayload.SKCList)
	} else if len(pkg.SkcList) > 0 {
		count = len(pkg.SkcList)
	} else if pkg.PreviewPayload != nil && len(pkg.PreviewPayload.SKCList) > 0 {
		count = len(pkg.PreviewPayload.SKCList)
	}
	return count == 1
}

func sheinHasFinalMainImage(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	if pkg.FinalSubmissionDraft != nil && strings.TrimSpace(pkg.FinalSubmissionDraft.MainImageURL) != "" {
		return true
	}
	if pkg.DraftPayload != nil && pkg.DraftPayload.ImageInfo != nil && strings.TrimSpace(pkg.DraftPayload.ImageInfo.MainImage) != "" {
		return true
	}
	if pkg.Images != nil && strings.TrimSpace(pkg.Images.MainImage) != "" {
		return true
	}
	return false
}

func sheinPricingReady(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil || pkg.DraftPayload == nil {
		return false
	}
	hasSKU := false
	for _, skc := range pkg.DraftPayload.SKCList {
		for _, sku := range skc.SKUList {
			hasSKU = true
			if parseMoney(sku.BasePrice) <= 0 {
				return false
			}
			if len(sku.SitePriceList) == 0 {
				return false
			}
			for _, sitePrice := range sku.SitePriceList {
				if parseMoney(sitePrice.BasePrice) <= 0 {
					return false
				}
			}
		}
	}
	return hasSKU
}

func sheinHasSubmitImage(pkg *SheinPackage) bool {
	pkg = sheinpub.NormalizePackageSemanticFields(pkg)
	if pkg == nil {
		return false
	}
	if pkg.Images != nil && firstNonEmpty(pkg.Images.MainImage, pkg.Images.WhiteBgImage) != "" {
		return true
	}
	if pkg.DraftPayload != nil {
		if sheinImageDraftHasImage(pkg.DraftPayload.ImageInfo) {
			return true
		}
		for _, skc := range pkg.DraftPayload.SKCList {
			if sheinImageDraftHasImage(skc.ImageInfo) {
				return true
			}
			for _, sku := range skc.SKUList {
				if strings.TrimSpace(sku.MainImage) != "" {
					return true
				}
			}
		}
	}
	if pkg.PreviewPayload != nil {
		if sheinProductImageInfoHasImage(pkg.PreviewPayload.ImageInfo) {
			return true
		}
		for _, skc := range pkg.PreviewPayload.SKCList {
			if sheinProductImageInfoHasImage(&skc.ImageInfo) {
				return true
			}
			for _, sku := range skc.SKUS {
				if sheinProductImageInfoHasImage(sku.ImageInfo) {
					return true
				}
			}
		}
	}
	return false
}

func sheinImageDraftHasImage(info *SheinImageDraft) bool {
	if info == nil {
		return false
	}
	if firstNonEmpty(info.MainImage, info.WhiteBg) != "" {
		return true
	}
	for _, image := range append(append([]string(nil), info.Gallery...), info.Source...) {
		if strings.TrimSpace(image) != "" {
			return true
		}
	}
	return false
}

func sheinProductImageInfoHasImage(info *sheinproduct.ImageInfo) bool {
	if info == nil {
		return false
	}
	for _, image := range info.ImageInfoList {
		if strings.TrimSpace(image.ImageURL) != "" {
			return true
		}
	}
	return false
}

func shapeSheinSubmitReadinessSummary(
	readiness *SheinSubmitReadiness,
	shape sheinSubmitReadinessSummaryShape,
) *SheinSubmitReadiness {
	if readiness == nil {
		return nil
	}
	if len(readiness.BlockingItems) > 0 {
		if shape.prependFirstBlocker {
			if message := strings.TrimSpace(readiness.BlockingItems[0].Message); message != "" {
				readiness.Summary = append([]string{message}, readiness.Summary...)
			}
		}
		if label := strings.TrimSpace(shape.blockingLabel); label != "" {
			readiness.Summary = append(readiness.Summary, label+sheinworkspace.JoinReadinessLabels(readiness.BlockingItems, "、"))
		}
	}
	if len(readiness.WarningItems) > 0 {
		if label := strings.TrimSpace(shape.warningLabel); label != "" {
			readiness.Summary = append(readiness.Summary, label+sheinworkspace.JoinReadinessLabels(readiness.WarningItems, "、"))
		}
	}
	readiness.Summary = uniqueStrings(readiness.Summary)
	return readiness
}
