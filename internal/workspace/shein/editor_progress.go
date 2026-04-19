package shein

import sheinpub "task-processor/internal/publishing/shein"

func BuildEditorProgress(pkg *sheinpub.Package, checklistTotal int) *EditorProgress {
	if pkg == nil {
		return nil
	}

	sections := []EditorProgressSection{
		buildBasicProgressSection(pkg),
		buildCategoryProgressSection(pkg),
		buildAttributeProgressSection(pkg),
		buildSaleProgressSection(pkg),
	}

	progress := &EditorProgress{Sections: sections}
	for _, section := range sections {
		progress.Completed += section.Completed
		progress.Total += section.Total
		progress.Unresolved += section.Unresolved
	}
	if checklistTotal > 0 {
		progress.Total = maxInt(progress.Total, checklistTotal)
	}
	return progress
}

func buildBasicProgressSection(pkg *sheinpub.Package) EditorProgressSection {
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
	return progressSection("basics", "基础信息", completed, total)
}

func buildCategoryProgressSection(pkg *sheinpub.Package) EditorProgressSection {
	total := 3
	completed := 0
	if pkg != nil && len(pkg.CategoryPath) > 0 {
		completed++
	}
	if pkg != nil && pkg.CategoryID > 0 {
		completed++
	}
	if pkg != nil && pkg.ProductTypeID != nil && *pkg.ProductTypeID > 0 && IsCategoryResolved(pkg) {
		completed++
	}
	return progressSection("category", "类目", completed, total)
}

func buildAttributeProgressSection(pkg *sheinpub.Package) EditorProgressSection {
	total := 2
	completed := 0
	if pkg != nil && len(pkg.ProductAttributes) > 0 {
		completed++
	}
	if pkg != nil && IsAttributeResolved(pkg) {
		completed++
	}
	return progressSection("attributes", "普通属性", completed, total)
}

func buildSaleProgressSection(pkg *sheinpub.Package) EditorProgressSection {
	total := 2
	completed := 0
	if pkg != nil && pkg.RequestDraft != nil && len(pkg.RequestDraft.SKCList) > 0 {
		completed++
	}
	if pkg != nil && IsSaleAttributeResolved(pkg) {
		completed++
	}
	return progressSection("sale_attributes", "规格", completed, total)
}

func progressSection(key, label string, completed, total int) EditorProgressSection {
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
	return EditorProgressSection{
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
