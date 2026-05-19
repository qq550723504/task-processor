package shein

type ReadinessReasonSpec struct {
	Code     string
	Category string
	Summary  string
}

type ReadinessHintSpec struct {
	Priority      string
	Target        string
	EditorSection string
	EditorFocus   []string
	RevisionPath  string
	Description   string
}

type ReadinessGuidanceSpec struct {
	Reason *ReadinessReasonSpec
	Hints  []ReadinessHintSpec
}

func BuildReadinessGuidanceSpec(key string, warningOnly bool) *ReadinessGuidanceSpec {
	switch key {
	case "category", "category_review":
		code := "category_unresolved"
		summary := "当前商品还没有确认到可提交的 SHEIN 类目骨架。"
		if key == "category_review" {
			code = "category_review_pending"
			summary = "当前 SHEIN 类目仍被建议复核，不能直接进入提交态。"
		}
		return &ReadinessGuidanceSpec{
			Reason: &ReadinessReasonSpec{
				Code:     code,
				Category: "classification",
				Summary:  summary,
			},
			Hints: []ReadinessHintSpec{{
				Priority:      "high",
				Target:        "editor.category",
				EditorSection: "category",
				EditorFocus:   []string{"category_id", "category_id_list", "product_type_id"},
				RevisionPath:  "shein.category_resolution",
				Description:   "先确认 category_id、category_id_list 和 product_type_id，再继续提交前校验。",
			}},
		}
	case "attributes", "attribute_review":
		code := "attributes_unmapped"
		summary := "普通属性还没有稳定映射到真实 attribute_id 或 attribute_value_id。"
		if key == "attribute_review" {
			code = "required_attributes_pending"
			summary = "普通属性仍有模板必填或重要属性未确认，不能直接进入提交态。"
		}
		return &ReadinessGuidanceSpec{
			Reason: &ReadinessReasonSpec{
				Code:     code,
				Category: "attributes",
				Summary:  summary,
			},
			Hints: []ReadinessHintSpec{{
				Priority:      "high",
				Target:        "editor.attributes",
				EditorSection: "attributes",
				EditorFocus:   []string{"resolved_attributes", "unresolved_count"},
				RevisionPath:  "shein.attribute_resolution",
				Description:   "先补齐未命中的普通属性映射，再重新检查提交状态。",
			}},
		}
	case "sale_attributes":
		return &ReadinessGuidanceSpec{
			Reason: &ReadinessReasonSpec{
				Code:     "sale_attributes_unresolved",
				Category: "variants",
				Summary:  "销售属性主副规格还没有稳定落到真实销售属性模板上。",
			},
			Hints: []ReadinessHintSpec{{
				Priority:      "high",
				Target:        "editor.sale_attributes",
				EditorSection: "sale_attributes",
				EditorFocus:   []string{"primary_attribute_id", "secondary_attribute_id", "skc_patches"},
				RevisionPath:  "shein.sale_attribute_resolution",
				Description:   "先确认主副销售属性和 SKC/SKU 规格结构，再继续提交。",
			}},
		}
	case "request_draft":
		return &ReadinessGuidanceSpec{
			Reason: &ReadinessReasonSpec{
				Code:     "request_draft_missing",
				Category: "payload",
				Summary:  "当前还没有生成 request_draft，无法继续作为提交草稿流转。",
			},
			Hints: []ReadinessHintSpec{{
				Priority:      "medium",
				Target:        "system.preview",
				EditorSection: "category",
				EditorFocus:   []string{"request_draft"},
				RevisionPath:  "shein.request_draft",
				Description:   "先重新生成 request_draft，再继续做提交前预览。",
			}},
		}
	case "preview_product":
		return &ReadinessGuidanceSpec{
			Reason: &ReadinessReasonSpec{
				Code:     "preview_product_missing",
				Category: "payload",
				Summary:  "当前还没有生成 preview_product，无法进入最终提交前载荷检查。",
			},
			Hints: []ReadinessHintSpec{{
				Priority:      "medium",
				Target:        "system.preview",
				EditorSection: "category",
				EditorFocus:   []string{"preview_product"},
				RevisionPath:  "shein.preview_product",
				Description:   "先重建 preview_product，再继续做提交前校验。",
			}},
		}
	case "images":
		return &ReadinessGuidanceSpec{
			Reason: &ReadinessReasonSpec{
				Code:     "images_missing",
				Category: "media",
				Summary:  "当前缺少提交前必须的主图资产。",
			},
			Hints: []ReadinessHintSpec{{
				Priority:      "high",
				Target:        "editor.basics.images",
				EditorSection: "basics",
				EditorFocus:   []string{"images.main_image", "images.gallery"},
				RevisionPath:  "shein.images",
				Description:   "至少补齐一张可用主图，再继续提交流程。",
			}},
		}
	case "variants":
		return &ReadinessGuidanceSpec{
			Reason: &ReadinessReasonSpec{
				Code:     "variants_incomplete",
				Category: "variants",
				Summary:  "当前 SKC/SKU 结构还不完整，不能作为稳定的提交规格。",
			},
			Hints: []ReadinessHintSpec{{
				Priority:      "high",
				Target:        "editor.sale_attributes",
				EditorSection: "sale_attributes",
				EditorFocus:   []string{"skc_patches", "sale_attribute_resolution"},
				RevisionPath:  "shein.skc_patches",
				Description:   "先补齐至少一个 SKC 和一个 SKU，并确认规格属性。",
			}},
		}
	case "manual_notes":
		category := "review"
		if warningOnly {
			category = "manual_review"
		}
		return &ReadinessGuidanceSpec{
			Reason: &ReadinessReasonSpec{
				Code:     "manual_review_pending",
				Category: category,
				Summary:  "当前仍有人工备注未处理，建议在提交前完成复核。",
			},
			Hints: []ReadinessHintSpec{{
				Priority:      "medium",
				Target:        "editor.basics.review_notes",
				EditorSection: "basics",
				EditorFocus:   []string{"review_notes"},
				RevisionPath:  "shein.review_notes",
				Description:   "逐条处理人工备注，确认这些说明不再阻塞提交。",
			}},
		}
	case "source_facts":
		return &ReadinessGuidanceSpec{
			Reason: &ReadinessReasonSpec{
				Code:     "source_fact_review_required",
				Category: "source_integrity",
				Summary:  "1688 来源商品存在缺少抓取依据的 LLM 推断字段，不能直接进入提交态。",
			},
			Hints: []ReadinessHintSpec{{
				Priority:      "high",
				Target:        "editor.basics.source_facts",
				EditorSection: "basics",
				EditorFocus:   []string{"metadata.source_fact_review_fields"},
				RevisionPath:  "shein.metadata",
				Description:   "先复核缺少抓取依据的字段，确认或补充来源后再提交。",
			}},
		}
	default:
		return nil
	}
}
