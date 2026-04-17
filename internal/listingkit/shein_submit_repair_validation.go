package listingkit

type SheinRepairValidationPreview struct {
	Valid                       bool                 `json:"valid"`
	Status                      string               `json:"status,omitempty"`
	FieldErrors                 []RevisionFieldError `json:"field_errors,omitempty"`
	RevisionDiffPreview         *RevisionDiffPreview `json:"revision_diff_preview,omitempty"`
	AffectedSections            []string             `json:"affected_sections,omitempty"`
	CategoryPreviewEffects      []SheinEditorEffect  `json:"category_preview_effects,omitempty"`
	AttributePreviewEffects     []SheinEditorEffect  `json:"attribute_preview_effects,omitempty"`
	SaleAttributePreviewEffects []SheinEditorEffect  `json:"sale_attribute_preview_effects,omitempty"`
}

func buildSheinRepairValidationPreview(pkg *SheinPackage, editorSection string, revision *ApplyRevisionRequest, skeleton *SheinEditorRevisionSkeleton) *SheinRepairValidationPreview {
	if revision == nil || skeleton == nil || skeleton.Shein == nil {
		return nil
	}

	preview := &SheinRepairValidationPreview{
		Valid:               true,
		RevisionDiffPreview: buildSheinRevisionDiffPreview(pkg, skeleton),
		AffectedSections:    buildSheinRepairAffectedSections(editorSection),
	}
	if validationErr, ok := validateApplyRevisionRequest(revision).(*RevisionValidationError); ok {
		preview.Valid = false
		preview.FieldErrors = append([]RevisionFieldError(nil), validationErr.Fields...)
	}
	switch editorSection {
	case "category":
		preview.CategoryPreviewEffects = buildSheinCategoryEffects()
	case "attributes":
		preview.AttributePreviewEffects = buildSheinAttributeEffects()
	case "sale_attributes":
		preview.SaleAttributePreviewEffects = buildSheinSaleAttributeEffects()
	case "basics":
		preview.CategoryPreviewEffects = buildSheinCategoryEffects()
		preview.AttributePreviewEffects = buildSheinAttributeEffects()
		preview.SaleAttributePreviewEffects = buildSheinSaleAttributeEffects()
	}
	if preview.Valid {
		preview.Status = "ready"
	} else {
		preview.Status = "invalid"
	}
	return preview
}

func cloneSheinRepairValidationPreview(src *SheinRepairValidationPreview) *SheinRepairValidationPreview {
	if src == nil {
		return nil
	}
	return &SheinRepairValidationPreview{
		Valid:                       src.Valid,
		Status:                      src.Status,
		FieldErrors:                 append([]RevisionFieldError(nil), src.FieldErrors...),
		RevisionDiffPreview:         cloneRevisionDiffPreview(src.RevisionDiffPreview),
		AffectedSections:            append([]string(nil), src.AffectedSections...),
		CategoryPreviewEffects:      append([]SheinEditorEffect(nil), src.CategoryPreviewEffects...),
		AttributePreviewEffects:     append([]SheinEditorEffect(nil), src.AttributePreviewEffects...),
		SaleAttributePreviewEffects: append([]SheinEditorEffect(nil), src.SaleAttributePreviewEffects...),
	}
}

func cloneRevisionDiffPreview(src *RevisionDiffPreview) *RevisionDiffPreview {
	if src == nil {
		return nil
	}
	cloned := &RevisionDiffPreview{
		ChangeCount: src.ChangeCount,
	}
	if len(src.Changes) > 0 {
		cloned.Changes = append([]RevisionFieldChange(nil), src.Changes...)
	}
	return cloned
}

func buildSheinRepairAffectedSections(editorSection string) []string {
	switch editorSection {
	case "category":
		return []string{"category", "inspection", "submit_readiness", "submit_checklist"}
	case "attributes":
		return []string{"attributes", "inspection", "submit_readiness", "submit_checklist"}
	case "sale_attributes":
		return []string{"sale_attributes", "inspection", "submit_readiness", "submit_checklist", "preview_product"}
	case "basics":
		return []string{"basics", "inspection", "submit_readiness", "submit_checklist", "preview_product"}
	default:
		return []string{"inspection", "submit_readiness", "submit_checklist"}
	}
}
