package listingkit

import (
	"strings"

	sheinproduct "task-processor/internal/shein/api/product"
	sheinworkspace "task-processor/internal/workspace/shein"
)

func buildSheinSubmitReadiness(pkg *SheinPackage) *SheinSubmitReadiness {
	if pkg == nil {
		return nil
	}

	checks := make([]sheinworkspace.ReadinessCheckSpec, 0, 8)

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

	categoryReady := isSheinCategoryResolved(pkg) &&
		pkg.CategoryID > 0 &&
		pkg.ProductTypeID != nil &&
		*pkg.ProductTypeID > 0
	addCheck(
		"category",
		"类目骨架",
		categoryReady,
		"类目、类目层级和 product_type_id 需要确认；如当前类目被建议复核，也不能直接进入提交态",
		[]string{"shein.category_id", "shein.category_id_list", "shein.product_type_id", "shein.sale_attribute_resolution.category_review_reason"},
		"确认类目",
		false,
	)

	addCheck(
		"category_review",
		"类目复核",
		!categoryReady || !sheinCategoryReviewPending(pkg),
		"当前类目可用，但系统建议复核候选类目；MVP 流程允许继续，但提交前建议确认",
		[]string{"shein.category_resolution.suggested_category", "shein.sale_attribute_resolution.category_review_reason"},
		"复核类目",
		true,
	)

	attributeReady := pkg.AttributeResolution != nil && len(pkg.ResolvedAttributes) > 0
	addCheck(
		"attributes",
		"普通属性",
		attributeReady,
		"普通属性还没有全部映射到真实 attribute_id / attribute_value_id",
		[]string{"shein.resolved_attributes", "shein.request_draft.resolved_attributes"},
		"确认属性",
		false,
	)

	addCheck(
		"attribute_review",
		"属性复核",
		!attributeReady || isSheinAttributeResolved(pkg),
		"普通属性已部分映射，但仍有模板必填项未能自动推断；MVP 流程允许继续，提交前建议确认",
		[]string{"shein.attribute_resolution.pending_attributes", "shein.attribute_resolution.review_notes"},
		"复核属性",
		true,
	)

	saleAttributeReady := isSheinSaleAttributeResolved(pkg)
	addCheck(
		"sale_attributes",
		"销售属性",
		saleAttributeReady,
		"销售属性主副规格还没有稳定映射到真实 sale attribute",
		[]string{"shein.sale_attribute_resolution", "shein.request_draft.skc_list"},
		"确认规格",
		false,
	)

	requestDraftReady := pkg.RequestDraft != nil
	addCheck(
		"request_draft",
		"请求草稿",
		requestDraftReady,
		"request_draft 尚未生成，当前无法作为提交草稿继续流转",
		[]string{"shein.request_draft"},
		"重新生成预览草稿",
		false,
	)

	previewProductReady := pkg.PreviewProduct != nil
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

	skcCount := len(pkg.SkcList)
	if skcCount == 0 && pkg.RequestDraft != nil {
		skcCount = len(pkg.RequestDraft.SKCList)
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

	readiness := sheinworkspace.BuildSubmitReadiness(
		checks,
		func(spec sheinworkspace.ReadinessCheckSpec) sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint] {
			guidance := buildSheinReadinessGuidance(pkg, spec.Key, spec.FieldPaths, spec.SuggestedAction, spec.WarningOnly)
			return sheinworkspace.Guidance[SheinReadinessReason, SheinRepairHint]{
				Reason:      cloneSheinReadinessReason(guidance.reason),
				RepairHints: cloneSheinRepairHints(guidance.repairHints),
			}
		},
		"当前仍有关键字段未完成，SHEIN 资料包还不能直接进入提交态",
		"SHEIN 资料包已经基本可提交，但仍建议先处理人工备注",
		"SHEIN 资料包已具备提交前所需的关键骨架",
	)
	if readiness == nil {
		return nil
	}
	if len(readiness.BlockingItems) > 0 {
		readiness.Summary = append(readiness.Summary, "待补关键项："+joinReadinessLabels(readiness.BlockingItems))
	}
	if len(readiness.WarningItems) > 0 {
		readiness.Summary = append(readiness.Summary, "待确认项："+joinReadinessLabels(readiness.WarningItems))
	}
	readiness.Summary = uniqueStrings(readiness.Summary)
	return readiness
}

func sheinHasSubmitImage(pkg *SheinPackage) bool {
	if pkg == nil {
		return false
	}
	if pkg.Images != nil && firstNonEmpty(pkg.Images.MainImage, pkg.Images.WhiteBgImage) != "" {
		return true
	}
	if pkg.RequestDraft != nil {
		if sheinImageDraftHasImage(pkg.RequestDraft.ImageInfo) {
			return true
		}
		for _, skc := range pkg.RequestDraft.SKCList {
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
	if pkg.PreviewProduct != nil {
		if sheinProductImageInfoHasImage(pkg.PreviewProduct.ImageInfo) {
			return true
		}
		for _, skc := range pkg.PreviewProduct.SKCList {
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
