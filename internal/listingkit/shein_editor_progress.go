package listingkit

type SheinEditorProgress struct {
	Completed  int                          `json:"completed"`
	Total      int                          `json:"total"`
	Unresolved int                          `json:"unresolved"`
	Sections   []SheinEditorProgressSection `json:"sections,omitempty"`
}

type SheinEditorProgressSection struct {
	Key        string `json:"key,omitempty"`
	Label      string `json:"label,omitempty"`
	Completed  int    `json:"completed"`
	Total      int    `json:"total"`
	Unresolved int    `json:"unresolved"`
	Status     string `json:"status,omitempty"`
}

func buildSheinEditorProgress(pkg *SheinPackage, checklist *SheinSubmitChecklist) *SheinEditorProgress {
	if pkg == nil {
		return nil
	}

	sections := []SheinEditorProgressSection{
		buildSheinBasicProgressSection(pkg),
		buildSheinCategoryProgressSection(pkg),
		buildSheinAttributeProgressSection(pkg),
		buildSheinSaleProgressSection(pkg),
	}

	progress := &SheinEditorProgress{
		Sections: sections,
	}
	for _, section := range sections {
		progress.Completed += section.Completed
		progress.Total += section.Total
		progress.Unresolved += section.Unresolved
	}

	if checklist != nil {
		// Keep section totals grounded in current submit expectations when available.
		progress.Total = maxInt(progress.Total, len(checklist.Required)+len(checklist.Recommended)+len(checklist.Optional))
	}
	return progress
}

func buildSheinBasicProgressSection(pkg *SheinPackage) SheinEditorProgressSection {
	total := 4
	completed := 0
	if pkg != nil && pkg.SpuName != "" {
		completed++
	}
	if pkg != nil && pkg.BrandName != "" {
		completed++
	}
	if pkg != nil && pkg.Description != "" {
		completed++
	}
	if pkg != nil && pkg.Images != nil && firstNonEmpty(pkg.Images.MainImage, pkg.Images.WhiteBgImage) != "" {
		completed++
	}
	return sheinProgressSection("basics", "基础信息", completed, total)
}

func buildSheinCategoryProgressSection(pkg *SheinPackage) SheinEditorProgressSection {
	total := 3
	completed := 0
	if pkg != nil && len(pkg.CategoryPath) > 0 {
		completed++
	}
	if pkg != nil && pkg.CategoryID > 0 {
		completed++
	}
	if pkg != nil && pkg.ProductTypeID != nil && *pkg.ProductTypeID > 0 && isSheinCategoryResolved(pkg) {
		completed++
	}
	return sheinProgressSection("category", "类目", completed, total)
}

func buildSheinAttributeProgressSection(pkg *SheinPackage) SheinEditorProgressSection {
	total := 2
	completed := 0
	if pkg != nil && len(pkg.ProductAttributes) > 0 {
		completed++
	}
	if pkg != nil && isSheinAttributeResolved(pkg) {
		completed++
	}
	return sheinProgressSection("attributes", "普通属性", completed, total)
}

func buildSheinSaleProgressSection(pkg *SheinPackage) SheinEditorProgressSection {
	total := 2
	completed := 0
	if pkg != nil && pkg.RequestDraft != nil && len(pkg.RequestDraft.SKCList) > 0 {
		completed++
	}
	if pkg != nil && isSheinSaleAttributeResolved(pkg) {
		completed++
	}
	return sheinProgressSection("sale_attributes", "规格", completed, total)
}

func sheinProgressSection(key, label string, completed, total int) SheinEditorProgressSection {
	unresolved := total - completed
	status := "not_started"
	switch {
	case completed == 0:
		status = "not_started"
	case unresolved == 0:
		status = "completed"
	default:
		status = "in_progress"
	}
	return SheinEditorProgressSection{
		Key:        key,
		Label:      label,
		Completed:  completed,
		Total:      total,
		Unresolved: unresolved,
		Status:     status,
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
