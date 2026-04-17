package listingkit

type SheinSubmitReadiness struct {
	Ready         bool                  `json:"ready"`
	Status        string                `json:"status,omitempty"`
	Summary       []string              `json:"summary,omitempty"`
	BlockingItems []SheinReadinessItem  `json:"blocking_items,omitempty"`
	WarningItems  []SheinReadinessItem  `json:"warning_items,omitempty"`
	Checks        []SheinReadinessCheck `json:"checks,omitempty"`
}

type SheinReadinessItem struct {
	Key             string                `json:"key,omitempty"`
	Label           string                `json:"label,omitempty"`
	Message         string                `json:"message,omitempty"`
	FieldPaths      []string              `json:"field_paths,omitempty"`
	SuggestedAction string                `json:"suggested_action,omitempty"`
	Reason          *SheinReadinessReason `json:"reason,omitempty"`
	RepairHints     []SheinRepairHint     `json:"repair_hints,omitempty"`
}

type SheinReadinessCheck struct {
	Key             string                `json:"key,omitempty"`
	Label           string                `json:"label,omitempty"`
	Status          string                `json:"status,omitempty"`
	Message         string                `json:"message,omitempty"`
	FieldPaths      []string              `json:"field_paths,omitempty"`
	SuggestedAction string                `json:"suggested_action,omitempty"`
	Reason          *SheinReadinessReason `json:"reason,omitempty"`
	RepairHints     []SheinRepairHint     `json:"repair_hints,omitempty"`
}

func buildSheinSubmitReadiness(pkg *SheinPackage) *SheinSubmitReadiness {
	if pkg == nil {
		return nil
	}

	readiness := &SheinSubmitReadiness{}
	var blockers []SheinReadinessItem
	var warnings []SheinReadinessItem
	var checks []SheinReadinessCheck

	addCheck := func(key, label string, ok bool, message string, fieldPaths []string, suggestedAction string, warningOnly bool) {
		guidance := buildSheinReadinessGuidance(pkg, key, fieldPaths, suggestedAction, warningOnly)
		status := "ready"
		if !ok && warningOnly {
			status = "warning"
		}
		if !ok && !warningOnly {
			status = "blocking"
		}
		checks = append(checks, SheinReadinessCheck{
			Key:             key,
			Label:           label,
			Status:          status,
			Message:         message,
			FieldPaths:      append([]string(nil), fieldPaths...),
			SuggestedAction: suggestedAction,
			Reason:          cloneSheinReadinessReason(guidance.reason),
			RepairHints:     cloneSheinRepairHints(guidance.repairHints),
		})
		if ok {
			return
		}
		item := SheinReadinessItem{
			Key:             key,
			Label:           label,
			Message:         message,
			FieldPaths:      append([]string(nil), fieldPaths...),
			SuggestedAction: suggestedAction,
			Reason:          cloneSheinReadinessReason(guidance.reason),
			RepairHints:     cloneSheinRepairHints(guidance.repairHints),
		}
		if warningOnly {
			warnings = append(warnings, item)
			return
		}
		blockers = append(blockers, item)
	}

	categoryReady := isSheinCategoryResolved(pkg) && pkg.CategoryID > 0 && pkg.ProductTypeID != nil && *pkg.ProductTypeID > 0
	addCheck(
		"category",
		"类目骨架",
		categoryReady,
		"类目、类目层级和 product_type_id 需要确认后才能进入提交态",
		[]string{"shein.category_id", "shein.category_id_list", "shein.product_type_id"},
		"确认类目",
		false,
	)

	attributeReady := isSheinAttributeResolved(pkg) && len(pkg.ResolvedAttributes) > 0
	addCheck(
		"attributes",
		"普通属性",
		attributeReady,
		"普通属性还没有全部映射到真实 attribute_id / attribute_value_id",
		[]string{"shein.resolved_attributes", "shein.request_draft.resolved_attributes"},
		"确认属性",
		false,
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

	imageReady := pkg.Images != nil && firstNonEmpty(pkg.Images.MainImage, pkg.Images.WhiteBgImage) != ""
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

	readiness.Checks = checks
	readiness.BlockingItems = blockers
	readiness.WarningItems = warnings
	readiness.Ready = len(blockers) == 0
	switch {
	case len(blockers) > 0:
		readiness.Status = "blocked"
		readiness.Summary = append(readiness.Summary, "当前仍有关键字段未完成，SHEIN 资料包还不能直接进入提交态")
	case len(warnings) > 0:
		readiness.Status = "ready_with_warnings"
		readiness.Summary = append(readiness.Summary, "SHEIN 资料包已经基本可提交，但仍建议先处理人工备注")
	default:
		readiness.Status = "ready"
		readiness.Summary = append(readiness.Summary, "SHEIN 资料包已具备提交前所需的关键骨架")
	}
	if len(blockers) > 0 {
		readiness.Summary = append(readiness.Summary, "待补关键项："+joinReadinessLabels(blockers))
	}
	if len(warnings) > 0 {
		readiness.Summary = append(readiness.Summary, "待确认项："+joinReadinessLabels(warnings))
	}
	readiness.Summary = uniqueStrings(readiness.Summary)
	return readiness
}

func sheinHasAnySKU(pkg *SheinPackage) bool {
	if pkg == nil {
		return false
	}
	for _, skc := range pkg.SkcList {
		if len(skc.SKUs) > 0 {
			return true
		}
	}
	if pkg.RequestDraft != nil {
		for _, skc := range pkg.RequestDraft.SKCList {
			if len(skc.SKUList) > 0 {
				return true
			}
		}
	}
	return false
}

func joinReadinessLabels(items []SheinReadinessItem) string {
	if len(items) == 0 {
		return ""
	}
	labels := make([]string, 0, len(items))
	for _, item := range items {
		if item.Label == "" {
			continue
		}
		labels = append(labels, item.Label)
	}
	return joinStrings(labels, "、")
}
