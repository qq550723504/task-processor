package listingkit

import sheinworkspace "task-processor/internal/marketplace/shein/workspace"

func refreshSheinReviewState(pkg *SheinPackage, extraNotes ...string) {
	if pkg == nil {
		return
	}

	notes := sheinworkspace.FilterManualReviewNotes(pkg.ReviewNotes)
	notes = append(notes, extraNotes...)

	if !sheinworkspace.IsCategoryResolved(pkg) {
		if pkg.CategoryID > 0 {
			notes = append(notes, "SHEIN 类目已解析 category_id，但类目路径、product_type 或语义一致性仍需要人工确认")
		} else {
			notes = append(notes, "SHEIN 类目解析尚未命中真实 category_id，当前仍需要人工确认类目")
		}
	}
	if !sheinworkspace.IsAttributeResolved(pkg) {
		notes = append(notes, "SHEIN 属性模板尚未完成真实 attribute_id 映射，当前仍需要人工确认属性")
	}
	if !sheinworkspace.IsSaleAttributeResolved(pkg) {
		notes = append(notes, "SHEIN 销售属性尚未完成真实 sale attribute 映射，当前仍需要人工确认变体规格")
	}

	pkg.ReviewNotes = uniqueStrings(notes)
	pkg.Inspection = sheinworkspace.BuildInspection(pkg)
}
