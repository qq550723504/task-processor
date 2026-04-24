package shein

import (
	"strings"

	"task-processor/internal/productenrich"
	common "task-processor/internal/publishing/common"
	sheincategoryselector "task-processor/internal/shein/category"
)

func buildCategoryQuery(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) string {
	return buildRichCategoryQuery(req, canonical, pkg, true)
}

func buildCategorySuggestionQuery(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) string {
	for _, candidate := range []string{
		firstNonGenericTitle(canonicalTitle(canonical), packageTitle(pkg)),
		categoryLeafSeed(canonical, pkg),
		normalizeCategoryQueryPart(reqText(req)),
		buildCompactCategoryAttributeSeed(canonical, pkg),
	} {
		if candidate != "" {
			return candidate
		}
	}
	return ""
}

func buildCategorySuggestInput(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package) sheincategoryselector.CoreItemInput {
	input := sheincategoryselector.CoreItemInput{
		Title:        firstNonGenericTitle(canonicalTitle(canonical), packageTitle(pkg), reqText(req)),
		ProductType:  extractCategoryProductType(canonical, pkg),
		CategoryPath: append([]string(nil), categoryPathFromCanonical(canonical)...),
		Attributes:   extractCategorySuggestAttributes(canonical, pkg),
	}
	if len(input.CategoryPath) == 0 {
		input.CategoryPath = append([]string(nil), categoryPathFromPackage(pkg)...)
	}
	return input
}

func buildRichCategoryQuery(req *BuildRequest, canonical *productenrich.CanonicalProduct, pkg *Package, includeHint bool) string {
	strongSignals := make([]string, 0, 12)
	contextSignals := make([]string, 0, 8)
	weakSignals := make([]string, 0, 4)

	appendStrong := func(value string) {
		if value = normalizeCategoryQueryPart(value); value != "" {
			strongSignals = append(strongSignals, value)
		}
	}
	appendContext := func(value string) {
		if value = normalizeCategoryQueryPart(value); value != "" {
			contextSignals = append(contextSignals, value)
		}
	}
	appendWeak := func(value string) {
		if value = normalizeCategoryQueryPart(value); value != "" {
			weakSignals = append(weakSignals, value)
		}
	}

	if canonical != nil {
		if title := normalizeCategoryQueryPart(canonical.Title); title != "" {
			contextSignals = append(contextSignals, title)
		}
		if len(canonical.CategoryPath) > 0 {
			if path := normalizeCategoryQueryPart(strings.Join(canonical.CategoryPath, " > ")); shouldIncludeCategoryPath(path) {
				contextSignals = append(contextSignals, path)
			}
		}
		for _, key := range prioritizedCategoryAttributeKeys {
			if attr, ok := canonical.Attributes[key]; ok {
				appendStrong(key + ":" + attr.Value)
			}
		}
		for _, dim := range canonical.VariantDimensions {
			if strings.TrimSpace(dim.Name) == "" {
				continue
			}
			appendStrong(dim.Name + ":" + strings.Join(dim.Values, ","))
		}
		if shouldIncludeWeakCategoryText(canonical.Description, len(strongSignals)) {
			appendWeak(canonical.Description)
		}
	}
	if pkg != nil {
		if spuName := normalizeCategoryQueryPart(pkg.SpuName); shouldIncludePackageName(spuName, contextSignals) {
			contextSignals = append(contextSignals, spuName)
		}
		pkgPath := common.FirstNonEmpty(pkg.CategoryName, strings.Join(pkg.CategoryPath, " > "))
		if path := normalizeCategoryQueryPart(pkgPath); shouldIncludeCategoryPath(path) {
			contextSignals = append(contextSignals, path)
		}
		for _, key := range prioritizedCategoryAttributeKeys {
			if value := strings.TrimSpace(pkg.Attributes[key]); value != "" {
				appendStrong(key + ":" + value)
			}
		}
	}
	if req != nil {
		if shouldIncludeWeakCategoryText(req.Text, len(strongSignals)) {
			appendWeak(req.Text)
		}
		if includeHint {
			appendContext(req.TargetCategoryHint)
		}
	}

	parts := append([]string{}, common.UniqueStrings(strongSignals)...)
	parts = append(parts, common.UniqueStrings(contextSignals)...)
	if len(parts) < 4 {
		parts = append(parts, common.UniqueStrings(weakSignals)...)
	}
	return strings.Join(common.UniqueStrings(parts), " | ")
}

func normalizeCategoryQueryPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return strings.Join(strings.Fields(value), " ")
}

func canonicalTitle(canonical *productenrich.CanonicalProduct) string {
	if canonical == nil {
		return ""
	}
	return normalizeCategoryQueryPart(canonical.Title)
}

func packageTitle(pkg *Package) string {
	if pkg == nil {
		return ""
	}
	return normalizeCategoryQueryPart(pkg.SpuName)
}

func reqText(req *BuildRequest) string {
	if req == nil {
		return ""
	}
	return normalizeCategoryQueryPart(req.Text)
}

func firstNonGenericTitle(values ...string) string {
	for _, value := range values {
		value = normalizeCategoryQueryPart(value)
		if value == "" || isGenericCategoryPlaceholder(value) {
			continue
		}
		return value
	}
	return ""
}

func categoryLeafSeed(canonical *productenrich.CanonicalProduct, pkg *Package) string {
	if seed := compactCategorySeed(categoryPathFromCanonical(canonical)); seed != "" {
		return seed
	}
	return compactCategorySeed(categoryPathFromPackage(pkg))
}

func categoryPathFromCanonical(canonical *productenrich.CanonicalProduct) []string {
	if canonical == nil {
		return nil
	}
	return canonical.CategoryPath
}

func categoryPathFromPackage(pkg *Package) []string {
	if pkg == nil {
		return nil
	}
	if len(pkg.CategoryPath) > 0 {
		return pkg.CategoryPath
	}
	if path := normalizeCategoryQueryPart(pkg.CategoryName); path != "" {
		return []string{path}
	}
	return nil
}

func compactCategorySeed(path []string) string {
	if len(path) == 0 {
		return ""
	}
	filtered := make([]string, 0, len(path))
	for _, part := range path {
		part = normalizeCategoryQueryPart(part)
		if part == "" || isGenericCategoryPlaceholder(part) {
			continue
		}
		filtered = append(filtered, part)
	}
	if len(filtered) == 0 {
		return ""
	}
	if len(filtered) == 1 {
		return filtered[0]
	}
	return strings.Join(filtered[len(filtered)-2:], " ")
}

func buildCompactCategoryAttributeSeed(canonical *productenrich.CanonicalProduct, pkg *Package) string {
	parts := make([]string, 0, 3)
	appendPart := func(value string) {
		value = normalizeCategoryQueryPart(value)
		if value == "" {
			return
		}
		parts = append(parts, value)
	}
	for _, key := range []string{"产品类别", "category", "品类"} {
		if canonical != nil {
			if attr, ok := canonical.Attributes[key]; ok {
				appendPart(attr.Value)
			}
		}
		if pkg != nil {
			appendPart(pkg.Attributes[key])
		}
		if len(parts) > 0 {
			break
		}
	}
	for _, key := range []string{"材质", "material", "用途", "usage"} {
		if len(parts) >= 3 {
			break
		}
		if canonical != nil {
			if attr, ok := canonical.Attributes[key]; ok {
				appendPart(attr.Value)
			}
		}
		if pkg != nil && len(parts) < 3 {
			appendPart(pkg.Attributes[key])
		}
	}
	return strings.Join(common.UniqueStrings(parts), " ")
}

func extractCategoryProductType(canonical *productenrich.CanonicalProduct, pkg *Package) string {
	for _, key := range []string{"产品类别", "category", "品类"} {
		if canonical != nil {
			if attr, ok := canonical.Attributes[key]; ok {
				if value := normalizeCategoryQueryPart(attr.Value); value != "" {
					return value
				}
			}
		}
		if pkg != nil {
			if value := normalizeCategoryQueryPart(pkg.Attributes[key]); value != "" {
				return value
			}
		}
	}
	return ""
}

func extractCategorySuggestAttributes(canonical *productenrich.CanonicalProduct, pkg *Package) map[string]string {
	result := map[string]string{}
	appendAttr := func(key, value string) {
		value = normalizeCategoryQueryPart(value)
		if value == "" {
			return
		}
		if _, exists := result[key]; exists {
			return
		}
		result[key] = value
	}
	for _, key := range []string{"产品类别", "品类", "category", "空间", "用途", "材质", "style"} {
		if canonical != nil {
			if attr, ok := canonical.Attributes[key]; ok {
				appendAttr(key, attr.Value)
			}
		}
		if pkg != nil {
			appendAttr(key, pkg.Attributes[key])
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func shouldIncludeWeakCategoryText(value string, strongSignalCount int) bool {
	value = normalizeCategoryQueryPart(value)
	if value == "" {
		return false
	}
	if strongSignalCount >= 3 {
		return false
	}
	if isGenericCategoryPlaceholder(value) {
		return false
	}
	return true
}

func shouldIncludeCategoryPath(value string) bool {
	value = normalizeCategoryQueryPart(value)
	if value == "" {
		return false
	}
	return !isGenericCategoryPlaceholder(value)
}

func shouldIncludePackageName(value string, existing []string) bool {
	value = normalizeCategoryQueryPart(value)
	if value == "" || isGenericCategoryPlaceholder(value) {
		return false
	}
	lowerValue := strings.ToLower(value)
	for _, item := range existing {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return false
		}
		if strings.Contains(strings.ToLower(item), lowerValue) || strings.Contains(lowerValue, strings.ToLower(item)) {
			return false
		}
	}
	return true
}

func isGenericCategoryPlaceholder(value string) bool {
	normalized := strings.ToLower(normalizeCategoryQueryPart(value))
	switch normalized {
	case "product", "general", "general product", "general > product":
		return true
	}
	return false
}

var prioritizedCategoryAttributeKeys = []string{
	"产品类别",
	"category",
	"品类",
	"材质",
	"material",
	"填充物",
	"filling",
	"空间",
	"space",
	"风格",
	"style",
	"尺寸",
	"size",
	"用途",
	"usage",
}
