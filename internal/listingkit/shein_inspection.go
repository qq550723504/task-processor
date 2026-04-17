package listingkit

func buildSheinInspection(pkg *SheinPackage) *SheinInspection {
	if pkg == nil {
		return nil
	}

	sections := []SheinInspectionSection{
		buildSheinCategoryInspectionSection(pkg),
		buildSheinAttributeInspectionSection(pkg),
		buildSheinSaleAttributeInspectionSection(pkg),
	}

	summary := uniqueStrings(append([]string(nil), pkg.ReviewNotes...))
	needsReview := len(summary) > 0
	for _, section := range sections {
		if section.Status != "resolved" {
			needsReview = true
		}
	}

	return &SheinInspection{
		NeedsReview: needsReview,
		Summary:     summary,
		Sections:    sections,
	}
}

func buildSheinCategoryInspectionSection(pkg *SheinPackage) SheinInspectionSection {
	section := SheinInspectionSection{
		Key:   "category",
		Title: "类目解析",
	}
	if pkg == nil || pkg.CategoryResolution == nil {
		section.Status = "missing"
		section.Summary = "尚未生成类目解析结果"
		section.ActionItems = []string{"确认 SHEIN 目标类目并补充 target_category_hint"}
		section.Actions = buildSheinCategoryActions(pkg)
		return section
	}

	section.Status = firstNonEmpty(pkg.CategoryResolution.Status, "unresolved")
	section.Summary = firstNonEmpty(joinCategoryPath(pkg.CategoryPath), "未命中类目路径")
	if pkg.CategoryID > 0 {
		section.Highlights = append(section.Highlights, "category_id 已解析: "+formatInt(pkg.CategoryID))
	}
	if pkg.ProductTypeID != nil {
		section.Highlights = append(section.Highlights, "product_type_id 已解析: "+formatInt(*pkg.ProductTypeID))
	}
	section.ActionItems = append(section.ActionItems, pkg.CategoryResolution.ReviewNotes...)
	section.Actions = buildSheinCategoryActions(pkg)
	return section
}

func buildSheinAttributeInspectionSection(pkg *SheinPackage) SheinInspectionSection {
	section := SheinInspectionSection{
		Key:   "attributes",
		Title: "普通属性映射",
	}
	if pkg == nil || pkg.AttributeResolution == nil {
		section.Status = "missing"
		section.Summary = "尚未生成属性模板映射结果"
		section.ActionItems = []string{"检查类目是否已命中，并确认 shein_store_id 可用"}
		section.Actions = buildSheinAttributeActions(pkg)
		return section
	}

	section.Status = firstNonEmpty(pkg.AttributeResolution.Status, "unresolved")
	section.Summary = "已解析 " + formatInt(pkg.AttributeResolution.ResolvedCount) + " 个属性"
	if pkg.AttributeResolution.UnresolvedCount > 0 {
		section.Highlights = append(section.Highlights, "未解析属性数: "+formatInt(pkg.AttributeResolution.UnresolvedCount))
	}
	for _, attr := range pkg.ResolvedAttributes {
		if attr.AttributeID > 0 {
			section.Highlights = append(section.Highlights, attr.Name+" -> "+formatInt(attr.AttributeID))
		}
	}
	section.ActionItems = append(section.ActionItems, pkg.AttributeResolution.ReviewNotes...)
	section.Actions = buildSheinAttributeActions(pkg)
	return section
}

func buildSheinSaleAttributeInspectionSection(pkg *SheinPackage) SheinInspectionSection {
	section := SheinInspectionSection{
		Key:   "sale_attributes",
		Title: "销售属性选择",
	}
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		section.Status = "missing"
		section.Summary = "尚未生成销售属性解析结果"
		section.ActionItems = []string{"确认当前类目销售属性模板是否可用"}
		section.Actions = buildSheinSaleAttributeActions(pkg)
		return section
	}

	section.Status = firstNonEmpty(pkg.SaleAttributeResolution.Status, "unresolved")
	section.Summary = firstNonEmpty(joinStrings(pkg.SaleAttributeResolution.SelectionSummary, "；"), "尚未选出主副销售属性")
	if pkg.SaleAttributeResolution.PrimaryAttributeID > 0 {
		section.Highlights = append(section.Highlights, "主销售属性 ID: "+formatInt(pkg.SaleAttributeResolution.PrimaryAttributeID))
	}
	if pkg.SaleAttributeResolution.SecondaryAttributeID > 0 {
		section.Highlights = append(section.Highlights, "次销售属性 ID: "+formatInt(pkg.SaleAttributeResolution.SecondaryAttributeID))
	}
	for _, candidate := range pkg.SaleAttributeResolution.Candidates {
		if candidate.SelectedScope != "" {
			section.Highlights = append(section.Highlights, candidate.Name+" 选为 "+candidate.SelectedScope)
		}
	}
	section.ActionItems = append(section.ActionItems, pkg.SaleAttributeResolution.ReviewNotes...)
	section.Actions = buildSheinSaleAttributeActions(pkg)
	return section
}

func joinCategoryPath(path []string) string {
	return joinStrings(path, " > ")
}

func joinStrings(items []string, sep string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	default:
		out := items[0]
		for _, item := range items[1:] {
			out += sep + item
		}
		return out
	}
}

func formatInt(v int) string {
	return itoa(v)
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	var digits [20]byte
	i := len(digits)
	for v > 0 {
		i--
		digits[i] = byte('0' + v%10)
		v /= 10
	}
	return sign + string(digits[i:])
}
