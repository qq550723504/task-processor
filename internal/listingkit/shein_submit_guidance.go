package listingkit

type SheinReadinessReason struct {
	Code     string `json:"code,omitempty"`
	Category string `json:"category,omitempty"`
	Summary  string `json:"summary,omitempty"`
}

type SheinRepairHint struct {
	Action        string                        `json:"action,omitempty"`
	Priority      string                        `json:"priority,omitempty"`
	Target        string                        `json:"target,omitempty"`
	EditorSection string                        `json:"editor_section,omitempty"`
	EditorFocus   []string                      `json:"editor_focus,omitempty"`
	RevisionPath  string                        `json:"revision_path,omitempty"`
	Description   string                        `json:"description,omitempty"`
	FieldPaths    []string                      `json:"field_paths,omitempty"`
	Patch         *SheinRepairPatchPayload      `json:"patch,omitempty"`
	Skeleton      *SheinEditorRevisionSkeleton  `json:"skeleton,omitempty"`
	Revision      *ApplyRevisionRequest         `json:"revision,omitempty"`
	Validation    *SheinRepairValidationPreview `json:"validation,omitempty"`
}

type sheinReadinessGuidance struct {
	reason      *SheinReadinessReason
	repairHints []SheinRepairHint
}

func buildSheinReadinessGuidance(pkg *SheinPackage, key string, fieldPaths []string, suggestedAction string, warningOnly bool) sheinReadinessGuidance {
	newHint := func(priority, target, editorSection string, editorFocus []string, revisionPath string, description string, patch *SheinRepairPatchPayload) SheinRepairHint {
		skeleton := buildSheinRepairRevisionSkeleton(suggestedAction, patch)
		revision := buildSheinRepairApplyRequest(suggestedAction, patch)
		return SheinRepairHint{
			Action:        suggestedAction,
			Priority:      priority,
			Target:        target,
			EditorSection: editorSection,
			EditorFocus:   append([]string(nil), editorFocus...),
			RevisionPath:  revisionPath,
			Description:   description,
			FieldPaths:    append([]string(nil), fieldPaths...),
			Patch:         cloneSheinRepairPatchPayload(patch),
			Skeleton:      skeleton,
			Revision:      revision,
			Validation:    buildSheinRepairValidationPreview(pkg, editorSection, revision, skeleton),
		}
	}

	switch key {
	case "category":
		return sheinReadinessGuidance{
			reason: &SheinReadinessReason{
				Code:     "category_unresolved",
				Category: "classification",
				Summary:  "当前商品还没有确认到可提交的 SHEIN 类目骨架。",
			},
			repairHints: []SheinRepairHint{
				newHint("high", "editor.category", "category", []string{"category_id", "category_id_list", "product_type_id"}, "shein.category_resolution", "先确认 category_id、category_id_list 和 product_type_id，再继续提交前校验。", &SheinRepairPatchPayload{
					CategoryResolution: buildSheinCategoryResolutionPatch(pkg),
				}),
			},
		}
	case "attributes":
		return sheinReadinessGuidance{
			reason: &SheinReadinessReason{
				Code:     "attributes_unmapped",
				Category: "attributes",
				Summary:  "普通属性还没有稳定映射到真实 attribute_id 或 attribute_value_id。",
			},
			repairHints: []SheinRepairHint{
				newHint("high", "editor.attributes", "attributes", []string{"resolved_attributes", "unresolved_count"}, "shein.attribute_resolution", "先补齐未命中的普通属性映射，再重新检查提交状态。", &SheinRepairPatchPayload{
					AttributeResolution: buildSheinAttributeResolutionPatch(pkg),
				}),
			},
		}
	case "sale_attributes":
		return sheinReadinessGuidance{
			reason: &SheinReadinessReason{
				Code:     "sale_attributes_unresolved",
				Category: "variants",
				Summary:  "销售属性主副规格还没有稳定落到真实销售属性模板上。",
			},
			repairHints: []SheinRepairHint{
				newHint("high", "editor.sale_attributes", "sale_attributes", []string{"primary_attribute_id", "secondary_attribute_id", "skc_patches"}, "shein.sale_attribute_resolution", "先确认主副销售属性和 SKC/SKU 规格结构，再继续提交。", &SheinRepairPatchPayload{
					SaleAttributeResolution: buildSheinSaleAttributeResolutionPatch(pkg),
					SKCPatches:              buildSheinEditorSKCPatches(pkg),
				}),
			},
		}
	case "request_draft":
		return sheinReadinessGuidance{
			reason: &SheinReadinessReason{
				Code:     "request_draft_missing",
				Category: "payload",
				Summary:  "当前还没有生成 request_draft，无法继续作为提交草稿流转。",
			},
			repairHints: []SheinRepairHint{
				newHint("medium", "system.preview", "category", []string{"request_draft"}, "shein.request_draft", "先重新生成 request_draft，再继续做提交前预览。", nil),
			},
		}
	case "preview_product":
		return sheinReadinessGuidance{
			reason: &SheinReadinessReason{
				Code:     "preview_product_missing",
				Category: "payload",
				Summary:  "当前还没有生成 preview_product，无法进入最终提交前载荷检查。",
			},
			repairHints: []SheinRepairHint{
				newHint("medium", "system.preview", "category", []string{"preview_product"}, "shein.preview_product", "先重建 preview_product，再继续做提交前校验。", nil),
			},
		}
	case "images":
		return sheinReadinessGuidance{
			reason: &SheinReadinessReason{
				Code:     "images_missing",
				Category: "media",
				Summary:  "当前缺少提交前必须的主图资产。",
			},
			repairHints: []SheinRepairHint{
				newHint("high", "editor.basics.images", "basics", []string{"images.main_image", "images.gallery"}, "shein.images", "至少补齐一张可用主图，再继续提交流程。", &SheinRepairPatchPayload{
					Images: clonePlatformImageSetForEditor(pkg.Images),
				}),
			},
		}
	case "variants":
		return sheinReadinessGuidance{
			reason: &SheinReadinessReason{
				Code:     "variants_incomplete",
				Category: "variants",
				Summary:  "当前 SKC/SKU 结构还不完整，不能作为稳定的提交规格。",
			},
			repairHints: []SheinRepairHint{
				newHint("high", "editor.sale_attributes", "sale_attributes", []string{"skc_patches", "sale_attribute_resolution"}, "shein.skc_patches", "先补齐至少一个 SKC 和一个 SKU，并确认规格属性。", &SheinRepairPatchPayload{
					SaleAttributeResolution: buildSheinSaleAttributeResolutionPatch(pkg),
					SKCPatches:              buildSheinEditorSKCPatches(pkg),
				}),
			},
		}
	case "manual_notes":
		category := "review"
		if warningOnly {
			category = "manual_review"
		}
		return sheinReadinessGuidance{
			reason: &SheinReadinessReason{
				Code:     "manual_review_pending",
				Category: category,
				Summary:  "当前仍有人工备注未处理，建议在提交前完成复核。",
			},
			repairHints: []SheinRepairHint{
				newHint("medium", "editor.basics.review_notes", "basics", []string{"review_notes"}, "shein.review_notes", "逐条处理人工备注，确认这些说明不再阻塞提交。", &SheinRepairPatchPayload{
					ReviewNotes: append([]string(nil), pkg.ReviewNotes...),
				}),
			},
		}
	default:
		return sheinReadinessGuidance{}
	}
}

func cloneSheinReadinessReason(reason *SheinReadinessReason) *SheinReadinessReason {
	if reason == nil {
		return nil
	}
	cloned := *reason
	return &cloned
}

func cloneSheinRepairHints(items []SheinRepairHint) []SheinRepairHint {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]SheinRepairHint, 0, len(items))
	for _, item := range items {
		cloned = append(cloned, SheinRepairHint{
			Action:        item.Action,
			Priority:      item.Priority,
			Target:        item.Target,
			EditorSection: item.EditorSection,
			EditorFocus:   append([]string(nil), item.EditorFocus...),
			RevisionPath:  item.RevisionPath,
			Description:   item.Description,
			FieldPaths:    append([]string(nil), item.FieldPaths...),
			Patch:         cloneSheinRepairPatchPayload(item.Patch),
			Skeleton:      cloneSheinEditorRevisionSkeleton(item.Skeleton),
			Revision:      cloneApplyRevisionRequest(item.Revision),
			Validation:    cloneSheinRepairValidationPreview(item.Validation),
		})
	}
	return cloned
}
