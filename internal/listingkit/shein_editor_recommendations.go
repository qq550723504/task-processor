package listingkit

type SheinEditorRecommendationMeta struct {
	Source      string   `json:"source,omitempty"`
	Confidence  string   `json:"confidence,omitempty"`
	Reason      string   `json:"reason,omitempty"`
	ReviewNotes []string `json:"review_notes,omitempty"`
}

type SheinEditorAttributeSuggestion struct {
	Name             string `json:"name,omitempty"`
	Value            string `json:"value,omitempty"`
	AttributeID      int    `json:"attribute_id,omitempty"`
	AttributeValueID *int   `json:"attribute_value_id,omitempty"`
	Source           string `json:"source,omitempty"`
	Confidence       string `json:"confidence,omitempty"`
	Reason           string `json:"reason,omitempty"`
}

type SheinEditorSaleCandidateSuggestion struct {
	Name           string   `json:"name,omitempty"`
	AttributeID    int      `json:"attribute_id,omitempty"`
	SelectedScope  string   `json:"selected_scope,omitempty"`
	SampleValue    string   `json:"sample_value,omitempty"`
	PrimaryScore   int      `json:"primary_score,omitempty"`
	SecondaryScore int      `json:"secondary_score,omitempty"`
	Source         string   `json:"source,omitempty"`
	Confidence     string   `json:"confidence,omitempty"`
	Reason         string   `json:"reason,omitempty"`
	Reasons        []string `json:"reasons,omitempty"`
}

func buildSheinCategoryRecommendationMeta(pkg *SheinPackage) *SheinEditorRecommendationMeta {
	if pkg == nil || pkg.CategoryResolution == nil {
		return nil
	}
	return &SheinEditorRecommendationMeta{
		Source:      pkg.CategoryResolution.Source,
		Confidence:  sheinCategoryConfidence(pkg),
		Reason:      sheinCategoryReason(pkg),
		ReviewNotes: append([]string(nil), pkg.CategoryResolution.ReviewNotes...),
	}
}

func buildSheinAttributeRecommendationMeta(pkg *SheinPackage) *SheinEditorRecommendationMeta {
	if pkg == nil || pkg.AttributeResolution == nil {
		return nil
	}
	return &SheinEditorRecommendationMeta{
		Source:      pkg.AttributeResolution.Source,
		Confidence:  sheinAttributeConfidence(pkg),
		Reason:      sheinAttributeReason(pkg),
		ReviewNotes: append([]string(nil), pkg.AttributeResolution.ReviewNotes...),
	}
}

func buildSheinSaleRecommendationMeta(pkg *SheinPackage) *SheinEditorRecommendationMeta {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return nil
	}
	return &SheinEditorRecommendationMeta{
		Source:      pkg.SaleAttributeResolution.Source,
		Confidence:  sheinSaleConfidence(pkg),
		Reason:      sheinSaleReason(pkg),
		ReviewNotes: append([]string(nil), pkg.SaleAttributeResolution.ReviewNotes...),
	}
}

func buildSheinAttributeSuggestions(pkg *SheinPackage) []SheinEditorAttributeSuggestion {
	if pkg == nil {
		return nil
	}
	source := ""
	if pkg.AttributeResolution != nil {
		source = pkg.AttributeResolution.Source
	}
	suggestions := make([]SheinEditorAttributeSuggestion, 0, len(pkg.ResolvedAttributes))
	for _, attr := range pkg.ResolvedAttributes {
		suggestions = append(suggestions, SheinEditorAttributeSuggestion{
			Name:             attr.Name,
			Value:            attr.Value,
			AttributeID:      attr.AttributeID,
			AttributeValueID: attr.AttributeValueID,
			Source:           firstNonEmpty(attr.MatchedBy, source),
			Confidence:       sheinAttributeSuggestionConfidence(attr),
			Reason:           sheinAttributeSuggestionReason(attr),
		})
	}
	return suggestions
}

func buildSheinSaleCandidateSuggestions(pkg *SheinPackage) []SheinEditorSaleCandidateSuggestion {
	if pkg == nil || pkg.SaleAttributeResolution == nil || len(pkg.SaleAttributeResolution.Candidates) == 0 {
		return nil
	}
	suggestions := make([]SheinEditorSaleCandidateSuggestion, 0, len(pkg.SaleAttributeResolution.Candidates))
	for _, candidate := range pkg.SaleAttributeResolution.Candidates {
		suggestions = append(suggestions, SheinEditorSaleCandidateSuggestion{
			Name:           candidate.Name,
			AttributeID:    candidate.AttributeID,
			SelectedScope:  candidate.SelectedScope,
			SampleValue:    candidate.SampleValue,
			PrimaryScore:   candidate.PrimaryScore,
			SecondaryScore: candidate.SecondaryScore,
			Source:         firstNonEmpty(pkg.SaleAttributeResolution.Source, "sale_attribute_templates"),
			Confidence:     sheinSaleCandidateConfidence(candidate),
			Reason:         sheinSaleCandidateReason(candidate),
			Reasons:        append([]string(nil), candidate.Reasons...),
		})
	}
	return suggestions
}

func sheinCategoryConfidence(pkg *SheinPackage) string {
	if pkg == nil || pkg.CategoryResolution == nil {
		return "low"
	}
	if isSheinCategoryResolved(pkg) && pkg.CategoryResolution.Source == "suggest_category" {
		return "high"
	}
	if isSheinCategoryResolved(pkg) && pkg.CategoryResolution.Source == "target_category_hint" {
		return "medium"
	}
	if pkg.CategoryResolution.Status == "partial" {
		return "low"
	}
	return "low"
}

func sheinAttributeConfidence(pkg *SheinPackage) string {
	if pkg == nil || pkg.AttributeResolution == nil {
		return "low"
	}
	switch pkg.AttributeResolution.Status {
	case "resolved":
		return "high"
	case "partial":
		return "medium"
	default:
		return "low"
	}
}

func sheinSaleConfidence(pkg *SheinPackage) string {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return "low"
	}
	switch pkg.SaleAttributeResolution.Status {
	case "resolved":
		return "high"
	case "partial":
		return "medium"
	default:
		return "low"
	}
}

func sheinCategoryReason(pkg *SheinPackage) string {
	if pkg == nil || pkg.CategoryResolution == nil {
		return ""
	}
	switch pkg.CategoryResolution.Source {
	case "suggest_category":
		return "根据类目搜索结果和类目详情回填 category_id 与路径"
	case "target_category_hint":
		return "根据 target_category_hint 直接命中类目，再补全类目层级"
	default:
		return "根据当前商品标题、类目路径和提示信息推断类目"
	}
}

func sheinAttributeReason(pkg *SheinPackage) string {
	if pkg == nil || pkg.AttributeResolution == nil {
		return ""
	}
	if pkg.AttributeResolution.TemplateCount > 0 {
		return "根据当前类目的属性模板，把商品属性映射到真实 attribute_id / attribute_value_id"
	}
	return "根据当前商品属性尝试匹配 SHEIN 属性模板"
}

func sheinSaleReason(pkg *SheinPackage) string {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return ""
	}
	return "根据销售属性模板和 SKC/SKU 变体差异，推断主副销售属性"
}

func sheinAttributeSuggestionConfidence(attr SheinResolvedAttribute) string {
	if attr.AttributeID <= 0 {
		return "low"
	}
	if attr.AttributeValueID != nil {
		return "high"
	}
	return "medium"
}

func sheinAttributeSuggestionReason(attr SheinResolvedAttribute) string {
	if attr.AttributeID <= 0 {
		return "尚未命中真实 attribute_id，仍需人工确认"
	}
	if attr.AttributeValueID != nil {
		return "已命中属性模板和枚举值，可直接作为默认推荐"
	}
	return "已命中属性模板，但值仍需人工确认或走自定义值"
}

func sheinSaleCandidateConfidence(candidate SheinSaleAttributeCandidateInfo) string {
	if candidate.SelectedScope != "" && candidate.PrimaryScore >= 8 {
		return "high"
	}
	if candidate.SelectedScope != "" || candidate.PrimaryScore > 0 || candidate.SecondaryScore > 0 {
		return "medium"
	}
	return "low"
}

func sheinSaleCandidateReason(candidate SheinSaleAttributeCandidateInfo) string {
	if candidate.SelectedScope == "skc" {
		return "当前候选被选为主销售属性，因为它更符合模板约束和 SKC 差异"
	}
	if candidate.SelectedScope == "sku" {
		return "当前候选被选为次销售属性，因为它更符合 SKU 层差异"
	}
	return "当前候选保留为备选规格，供人工确认"
}
