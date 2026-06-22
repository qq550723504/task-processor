package workspace

import sheinpub "task-processor/internal/publishing/shein"

type ValidationPayload[RestorePreview any] struct {
	DirtyHints                  *EditorDirtyHints       `json:"dirty_hints,omitempty"`
	CategoryPreviewEffects      []EditorEffect          `json:"category_preview_effects,omitempty"`
	AttributePreviewEffects     []EditorEffect          `json:"attribute_preview_effects,omitempty"`
	SaleAttributePreviewEffects []EditorEffect          `json:"sale_attribute_preview_effects,omitempty"`
	SuggestedMinimalRevision    *EditorRevisionSkeleton `json:"suggested_minimal_revision,omitempty"`
	RevisionDiffPreview         *RevisionDiffPreview    `json:"revision_diff_preview,omitempty"`
	RestorePreview              *RestorePreview         `json:"restore_preview,omitempty"`
}

type RepairValidationPreview[FieldError any] struct {
	Valid                       bool                 `json:"valid"`
	Status                      string               `json:"status,omitempty"`
	FieldErrors                 []FieldError         `json:"field_errors,omitempty"`
	RevisionDiffPreview         *RevisionDiffPreview `json:"revision_diff_preview,omitempty"`
	AffectedSections            []string             `json:"affected_sections,omitempty"`
	CategoryPreviewEffects      []EditorEffect       `json:"category_preview_effects,omitempty"`
	AttributePreviewEffects     []EditorEffect       `json:"attribute_preview_effects,omitempty"`
	SaleAttributePreviewEffects []EditorEffect       `json:"sale_attribute_preview_effects,omitempty"`
}

func CloneRevisionDiffPreview(src *RevisionDiffPreview) *RevisionDiffPreview {
	if src == nil {
		return nil
	}
	return &RevisionDiffPreview{
		ChangeCount: src.ChangeCount,
		Changes:     append([]RevisionFieldChange(nil), src.Changes...),
	}
}

func CloneRepairValidationPreview[FieldError any](src *RepairValidationPreview[FieldError]) *RepairValidationPreview[FieldError] {
	if src == nil {
		return nil
	}
	return &RepairValidationPreview[FieldError]{
		Valid:                       src.Valid,
		Status:                      src.Status,
		FieldErrors:                 append([]FieldError(nil), src.FieldErrors...),
		RevisionDiffPreview:         CloneRevisionDiffPreview(src.RevisionDiffPreview),
		AffectedSections:            append([]string(nil), src.AffectedSections...),
		CategoryPreviewEffects:      append([]EditorEffect(nil), src.CategoryPreviewEffects...),
		AttributePreviewEffects:     append([]EditorEffect(nil), src.AttributePreviewEffects...),
		SaleAttributePreviewEffects: append([]EditorEffect(nil), src.SaleAttributePreviewEffects...),
	}
}

func BuildValidationPayload[RestorePreview any](pkg *sheinpub.Package, restorePreview *RestorePreview) *ValidationPayload[RestorePreview] {
	if pkg == nil {
		return nil
	}
	minimal := BuildMinimalRevisionSkeleton(BuildEditorRevisionSkeleton(
		pkg,
		BuildCategoryResolutionPatch(pkg),
		BuildAttributeResolutionPatch(pkg),
		BuildSaleAttributeResolutionPatch(pkg),
		BuildEditorSKCPatches(pkg),
	))
	return &ValidationPayload[RestorePreview]{
		DirtyHints:                  BuildEditorDirtyHints(pkg),
		CategoryPreviewEffects:      BuildCategoryEffects(),
		AttributePreviewEffects:     BuildAttributeEffects(),
		SaleAttributePreviewEffects: BuildSaleAttributeEffects(),
		SuggestedMinimalRevision:    minimal,
		RevisionDiffPreview:         BuildRevisionDiffPreview(pkg, minimal),
		RestorePreview:              restorePreview,
	}
}

func BuildRepairValidationPreview[FieldError any](
	pkg *sheinpub.Package,
	editorSection string,
	skeleton *EditorRevisionSkeleton,
	valid bool,
	fieldErrors []FieldError,
) *RepairValidationPreview[FieldError] {
	if skeleton == nil || skeleton.Shein == nil {
		return nil
	}
	preview := &RepairValidationPreview[FieldError]{
		Valid:               valid,
		RevisionDiffPreview: BuildRevisionDiffPreview(pkg, skeleton),
		AffectedSections:    buildRepairAffectedSections(editorSection),
		FieldErrors:         append([]FieldError(nil), fieldErrors...),
	}
	switch editorSection {
	case "category":
		preview.CategoryPreviewEffects = BuildCategoryEffects()
	case "attributes":
		preview.AttributePreviewEffects = BuildAttributeEffects()
	case "sale_attributes":
		preview.SaleAttributePreviewEffects = BuildSaleAttributeEffects()
	case "basics":
		preview.CategoryPreviewEffects = BuildCategoryEffects()
		preview.AttributePreviewEffects = BuildAttributeEffects()
		preview.SaleAttributePreviewEffects = BuildSaleAttributeEffects()
	}
	if preview.Valid {
		preview.Status = "ready"
	} else {
		preview.Status = "invalid"
	}
	return preview
}

func buildRepairAffectedSections(editorSection string) []string {
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
