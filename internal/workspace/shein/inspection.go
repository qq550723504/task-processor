package shein

import (
	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func BuildInspection(pkg *sheinpub.Package) *sheinpub.Inspection {
	if pkg == nil {
		return nil
	}

	sections := []sheinpub.InspectionSection{
		buildCategoryInspectionSection(pkg),
		buildAttributeInspectionSection(pkg),
		buildSaleAttributeInspectionSection(pkg),
	}

	summary := uniqueStrings(append([]string(nil), pkg.ReviewNotes...))
	needsReview := len(summary) > 0
	for _, section := range sections {
		if section.Status != "resolved" {
			needsReview = true
		}
	}

	return &sheinpub.Inspection{
		NeedsReview: needsReview,
		Summary:     summary,
		Sections:    sections,
	}
}

func buildCategoryInspectionSection(pkg *sheinpub.Package) sheinpub.InspectionSection {
	section := sheinpub.InspectionSection{Key: "category", Title: "类目解析"}
	if pkg == nil || pkg.CategoryResolution == nil {
		section.Status = "missing"
		section.Summary = "尚未生成类目解析结果"
		section.ActionItems = []string{"确认 SHEIN 目标类目并补充 target_category_hint"}
		section.Actions = buildCategoryActions(pkg)
		return section
	}

	section.Status = firstNonEmpty(pkg.CategoryResolution.Status, "unresolved")
	section.Summary = joinStrings(pkg.CategoryPath, " > ")
	if section.Summary == "" {
		section.Summary = "未命中类目路径"
	}
	if pkg.CategoryID > 0 {
		section.Highlights = append(section.Highlights, "category_id 已解析: "+formatInt(pkg.CategoryID))
	}
	if pkg.ProductTypeID != nil {
		section.Highlights = append(section.Highlights, "product_type_id 已解析: "+formatInt(*pkg.ProductTypeID))
	}
	section.ActionItems = append(section.ActionItems, pkg.CategoryResolution.ReviewNotes...)
	section.Actions = buildCategoryActions(pkg)
	return section
}

func buildAttributeInspectionSection(pkg *sheinpub.Package) sheinpub.InspectionSection {
	section := sheinpub.InspectionSection{Key: "attributes", Title: "普通属性映射"}
	if pkg == nil || pkg.AttributeResolution == nil {
		section.Status = "missing"
		section.Summary = "尚未生成属性模板映射结果"
		section.ActionItems = []string{"检查类目是否已命中，并确认 shein_store_id 可用"}
		section.Actions = buildAttributeActions(pkg)
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
	section.Actions = buildAttributeActions(pkg)
	return section
}

func buildSaleAttributeInspectionSection(pkg *sheinpub.Package) sheinpub.InspectionSection {
	section := sheinpub.InspectionSection{Key: "sale_attributes", Title: "销售属性选择"}
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		section.Status = "missing"
		section.Summary = "尚未生成销售属性解析结果"
		section.ActionItems = []string{"确认当前类目销售属性模板是否可用"}
		section.Actions = buildSaleAttributeActions(pkg)
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
	section.Actions = buildSaleAttributeActions(pkg)
	return section
}

func buildCategoryActions(pkg *sheinpub.Package) []sheinpub.InspectionAction {
	if pkg == nil || IsCategoryResolved(pkg) {
		return nil
	}
	payload := buildCategoryPayload(pkg)
	return []sheinpub.InspectionAction{{
		Key:         "resolve_category",
		Label:       "确认类目",
		Target:      "revision.category_resolution",
		ActionType:  "patch",
		Description: "补充或修正 SHEIN category_id、category_id_list 和 product_type_id。",
		Payload:     buildCategoryPayloadMap(payload),
		Category:    payload,
	}}
}

func buildAttributeActions(pkg *sheinpub.Package) []sheinpub.InspectionAction {
	if pkg == nil || IsAttributeResolved(pkg) {
		return nil
	}
	payload := buildAttributePayload(pkg)
	return []sheinpub.InspectionAction{{
		Key:         "resolve_attributes",
		Label:       "确认属性",
		Target:      "revision.attribute_resolution",
		ActionType:  "patch",
		Description: "补充 attribute_id、attribute_value_id 或修正属性映射结果。",
		Payload:     buildAttributePayloadMap(payload),
		Attributes:  payload,
	}}
}

func buildSaleAttributeActions(pkg *sheinpub.Package) []sheinpub.InspectionAction {
	if pkg == nil || IsSaleAttributeResolved(pkg) {
		return nil
	}
	payload := buildSaleAttributePayload(pkg)
	return []sheinpub.InspectionAction{{
		Key:         "resolve_sale_attributes",
		Label:       "确认规格",
		Target:      "revision.sale_attribute_resolution",
		ActionType:  "patch",
		Description: "修正主副销售属性以及对应的 SKC/SKU 规格映射。",
		Payload:     buildSaleAttributePayloadMap(payload),
		Sale:        payload,
	}}
}

func buildCategoryPayload(pkg *sheinpub.Package) *sheinpub.InspectionCategoryPayload {
	if pkg == nil {
		return nil
	}
	payload := &sheinpub.InspectionCategoryPayload{
		Platform:       "shein",
		Target:         "category_resolution",
		CategoryName:   pkg.CategoryName,
		CategoryPath:   append([]string(nil), pkg.CategoryPath...),
		CategoryID:     pkg.CategoryID,
		CategoryIDList: append([]int(nil), pkg.CategoryIDList...),
		TopCategoryID:  pkg.TopCategoryID,
	}
	if pkg.ProductTypeID != nil {
		productTypeID := *pkg.ProductTypeID
		payload.ProductTypeID = &productTypeID
	}
	if pkg.CategoryResolution != nil {
		payload.Status = pkg.CategoryResolution.Status
		payload.Source = pkg.CategoryResolution.Source
		payload.ReviewNotes = append([]string(nil), pkg.CategoryResolution.ReviewNotes...)
	}
	return payload
}

func buildCategoryPayloadMap(payload *sheinpub.InspectionCategoryPayload) map[string]any {
	if payload == nil {
		return nil
	}
	out := map[string]any{"platform": "shein", "target": "category_resolution"}
	if payload.Status != "" {
		out["status"] = payload.Status
	}
	if payload.Source != "" {
		out["source"] = payload.Source
	}
	if payload.CategoryName != "" {
		out["category_name"] = payload.CategoryName
	}
	if len(payload.CategoryPath) > 0 {
		out["category_path"] = append([]string(nil), payload.CategoryPath...)
	}
	if payload.CategoryID > 0 {
		out["category_id"] = payload.CategoryID
	}
	if len(payload.CategoryIDList) > 0 {
		out["category_id_list"] = append([]int(nil), payload.CategoryIDList...)
	}
	if payload.ProductTypeID != nil {
		out["product_type_id"] = *payload.ProductTypeID
	}
	if payload.TopCategoryID > 0 {
		out["top_category_id"] = payload.TopCategoryID
	}
	if len(payload.ReviewNotes) > 0 {
		out["review_notes"] = append([]string(nil), payload.ReviewNotes...)
	}
	return out
}

func buildAttributePayload(pkg *sheinpub.Package) *sheinpub.InspectionAttributePayload {
	if pkg == nil {
		return nil
	}
	payload := &sheinpub.InspectionAttributePayload{
		Platform:           "shein",
		Target:             "attribute_resolution",
		ProductAttributes:  append([]common.Attribute(nil), pkg.ProductAttributes...),
		ResolvedAttributes: append([]sheinpub.ResolvedAttribute(nil), pkg.ResolvedAttributes...),
		PendingAttributes:  buildPendingAttributes(pkg),
	}
	if pkg.AttributeResolution != nil {
		payload.Status = pkg.AttributeResolution.Status
		payload.Source = pkg.AttributeResolution.Source
		payload.TemplateCount = pkg.AttributeResolution.TemplateCount
		payload.ResolvedCount = pkg.AttributeResolution.ResolvedCount
		payload.UnresolvedCount = pkg.AttributeResolution.UnresolvedCount
		payload.ReviewNotes = append([]string(nil), pkg.AttributeResolution.ReviewNotes...)
	}
	return payload
}

func buildAttributePayloadMap(payload *sheinpub.InspectionAttributePayload) map[string]any {
	if payload == nil {
		return nil
	}
	out := map[string]any{"platform": "shein", "target": "attribute_resolution"}
	if payload.Status != "" {
		out["status"] = payload.Status
	}
	if payload.Source != "" {
		out["source"] = payload.Source
	}
	if payload.TemplateCount > 0 {
		out["template_count"] = payload.TemplateCount
	}
	if payload.ResolvedCount > 0 {
		out["resolved_count"] = payload.ResolvedCount
	}
	out["unresolved_count"] = payload.UnresolvedCount
	if len(payload.ProductAttributes) > 0 {
		out["product_attributes"] = append([]common.Attribute(nil), payload.ProductAttributes...)
	}
	if len(payload.ResolvedAttributes) > 0 {
		out["resolved_attributes"] = append([]sheinpub.ResolvedAttribute(nil), payload.ResolvedAttributes...)
	}
	if len(payload.PendingAttributes) > 0 {
		out["pending_attributes"] = append([]common.Attribute(nil), payload.PendingAttributes...)
	}
	if len(payload.ReviewNotes) > 0 {
		out["review_notes"] = append([]string(nil), payload.ReviewNotes...)
	}
	return out
}

func buildSaleAttributePayload(pkg *sheinpub.Package) *sheinpub.InspectionSaleAttributePayload {
	if pkg == nil {
		return nil
	}
	payload := &sheinpub.InspectionSaleAttributePayload{
		Platform:   "shein",
		Target:     "sale_attribute_resolution",
		SKCPatches: buildSKCPatchSuggestions(pkg),
	}
	if pkg.SaleAttributeResolution != nil {
		payload.Status = pkg.SaleAttributeResolution.Status
		payload.Source = pkg.SaleAttributeResolution.Source
		payload.PrimaryAttributeID = pkg.SaleAttributeResolution.PrimaryAttributeID
		payload.SecondaryAttributeID = pkg.SaleAttributeResolution.SecondaryAttributeID
		payload.SelectionSummary = append([]string(nil), pkg.SaleAttributeResolution.SelectionSummary...)
		payload.SKCAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), pkg.SaleAttributeResolution.SKCAttributes...)
		payload.SKUAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), pkg.SaleAttributeResolution.SKUAttributes...)
		payload.CandidateCount = len(pkg.SaleAttributeResolution.Candidates)
		payload.Candidates = append([]sheinpub.SaleAttributeCandidateInfo(nil), pkg.SaleAttributeResolution.Candidates...)
		payload.ReviewNotes = append([]string(nil), pkg.SaleAttributeResolution.ReviewNotes...)
	}
	return payload
}

func buildSaleAttributePayloadMap(payload *sheinpub.InspectionSaleAttributePayload) map[string]any {
	if payload == nil {
		return nil
	}
	out := map[string]any{"platform": "shein", "target": "sale_attribute_resolution"}
	if payload.Status != "" {
		out["status"] = payload.Status
	}
	if payload.Source != "" {
		out["source"] = payload.Source
	}
	out["primary_attribute_id"] = payload.PrimaryAttributeID
	out["secondary_attribute_id"] = payload.SecondaryAttributeID
	if len(payload.SelectionSummary) > 0 {
		out["selection_summary"] = append([]string(nil), payload.SelectionSummary...)
	}
	if len(payload.SKCAttributes) > 0 {
		out["skc_attributes"] = append([]sheinpub.ResolvedSaleAttribute(nil), payload.SKCAttributes...)
	}
	if len(payload.SKUAttributes) > 0 {
		out["sku_attributes"] = append([]sheinpub.ResolvedSaleAttribute(nil), payload.SKUAttributes...)
	}
	if payload.CandidateCount > 0 {
		out["candidate_count"] = payload.CandidateCount
	}
	if len(payload.Candidates) > 0 {
		out["candidates"] = append([]sheinpub.SaleAttributeCandidateInfo(nil), payload.Candidates...)
	}
	if len(payload.SKCPatches) > 0 {
		out["skc_patches"] = append([]sheinpub.InspectionSKCPatchPayload(nil), payload.SKCPatches...)
	}
	if len(payload.ReviewNotes) > 0 {
		out["review_notes"] = append([]string(nil), payload.ReviewNotes...)
	}
	return out
}

func buildPendingAttributes(pkg *sheinpub.Package) []common.Attribute {
	if pkg == nil || len(pkg.ProductAttributes) == 0 {
		return nil
	}
	resolvedNames := map[string]struct{}{}
	for _, attr := range pkg.ResolvedAttributes {
		name := normalizeText(attr.Name)
		if name != "" {
			resolvedNames[name] = struct{}{}
		}
	}
	pending := make([]common.Attribute, 0, len(pkg.ProductAttributes))
	for _, attr := range pkg.ProductAttributes {
		name := normalizeText(attr.Name)
		if _, ok := resolvedNames[name]; ok {
			continue
		}
		pending = append(pending, attr)
	}
	return pending
}

func buildSKCPatchSuggestions(pkg *sheinpub.Package) []sheinpub.InspectionSKCPatchPayload {
	if pkg == nil || pkg.RequestDraft == nil || len(pkg.RequestDraft.SKCList) == 0 {
		return nil
	}
	patches := make([]sheinpub.InspectionSKCPatchPayload, 0, len(pkg.RequestDraft.SKCList))
	for _, skc := range pkg.RequestDraft.SKCList {
		entry := sheinpub.InspectionSKCPatchPayload{
			SupplierCode: skc.SupplierCode,
			SkcName:      skc.SkcName,
			SaleName:     skc.SaleName,
		}
		if skc.SaleAttribute != nil {
			attr := *skc.SaleAttribute
			entry.SaleAttribute = &attr
		}
		if skc.ImageInfo != nil {
			entry.MainImageURL = skc.ImageInfo.MainImage
		}
		if len(skc.SKUList) > 0 {
			entry.SKUPatches = buildSKUPatchSuggestions(skc.SKUList)
		}
		patches = append(patches, entry)
	}
	return patches
}

func buildSKUPatchSuggestions(items []sheinpub.SKUDraft) []sheinpub.InspectionSKUPatchPayload {
	if len(items) == 0 {
		return nil
	}
	patches := make([]sheinpub.InspectionSKUPatchPayload, 0, len(items))
	for _, sku := range items {
		entry := sheinpub.InspectionSKUPatchPayload{
			SupplierSKU: sku.SupplierSKU,
			Attributes:  cloneMap(sku.Attributes),
			BasePrice:   sku.BasePrice,
			CostPrice:   sku.CostPrice,
			Currency:    sku.Currency,
			StockCount:  sku.StockCount,
			MainImage:   sku.MainImage,
			Barcode:     sku.Barcode,
		}
		if len(sku.SaleAttributes) > 0 {
			entry.SaleAttributes = append([]sheinpub.ResolvedSaleAttribute(nil), sku.SaleAttributes...)
		}
		if len(sku.SitePriceList) > 0 {
			entry.SitePriceList = append([]sheinpub.SitePrice(nil), sku.SitePriceList...)
		}
		if len(sku.StockInfoList) > 0 {
			entry.StockInfoList = append([]sheinpub.StockInfo(nil), sku.StockInfoList...)
		}
		patches = append(patches, entry)
	}
	return patches
}

func cloneMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]string, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
}
