package shein

import sheinpub "task-processor/internal/publishing/shein"

var AutoReviewNotes = []string{
	"SHEIN 资料包已贴近 SPU/SKC 结构，但类目 ID、销售属性 ID 仍需接 SHEIN 类目/属性中心二次映射",
	"SHEIN 类目解析尚未命中真实 category_id，当前仍需要人工确认类目",
	"SHEIN 属性模板尚未完成真实 attribute_id 映射，当前仍需要人工确认属性",
	"SHEIN 销售属性尚未完成真实 sale attribute 映射，当前仍需要人工确认变体规格",
}

func FilterManualReviewNotes(notes []string) []string {
	if len(notes) == 0 {
		return nil
	}
	auto := make(map[string]struct{}, len(AutoReviewNotes))
	for _, note := range AutoReviewNotes {
		auto[note] = struct{}{}
	}
	filtered := make([]string, 0, len(notes))
	for _, note := range uniqueStrings(notes) {
		if _, ok := auto[note]; ok {
			continue
		}
		filtered = append(filtered, note)
	}
	return filtered
}

func IsCategoryResolved(pkg *sheinpub.Package) bool {
	if pkg == nil || pkg.CategoryResolution == nil {
		return false
	}
	if pkg.CategoryID <= 0 {
		return false
	}
	return firstNonEmpty(pkg.CategoryResolution.Status, "unresolved") == "resolved"
}

func IsAttributeResolved(pkg *sheinpub.Package) bool {
	if pkg == nil || pkg.AttributeResolution == nil {
		return false
	}
	if pkg.AttributeResolution.ResolvedCount <= 0 && len(pkg.ResolvedAttributes) == 0 {
		return false
	}
	return firstNonEmpty(pkg.AttributeResolution.Status, "unresolved") == "resolved"
}

func IsSaleAttributeResolved(pkg *sheinpub.Package) bool {
	if pkg == nil || pkg.SaleAttributeResolution == nil {
		return false
	}
	if pkg.SaleAttributeResolution.PrimaryAttributeID <= 0 {
		return false
	}
	return firstNonEmpty(pkg.SaleAttributeResolution.Status, "unresolved") == "resolved"
}
