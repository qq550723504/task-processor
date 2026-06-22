package workspace

import (
	"testing"

	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildValidationPayloadProjectsEditorHelpers(t *testing.T) {
	restorePreview := "restore"
	pkg := &sheinpub.Package{
		SpuName: "before",
	}

	payload := BuildValidationPayload(pkg, &restorePreview)

	if payload == nil {
		t.Fatal("payload = nil")
	}
	if payload.DirtyHints == nil || payload.SuggestedMinimalRevision == nil || payload.RevisionDiffPreview == nil {
		t.Fatalf("payload helpers = %#v", payload)
	}
	if payload.RestorePreview == nil || *payload.RestorePreview != restorePreview {
		t.Fatalf("restore preview = %#v", payload.RestorePreview)
	}
	if len(payload.CategoryPreviewEffects) == 0 || len(payload.AttributePreviewEffects) == 0 || len(payload.SaleAttributePreviewEffects) == 0 {
		t.Fatalf("effects = %#v %#v %#v", payload.CategoryPreviewEffects, payload.AttributePreviewEffects, payload.SaleAttributePreviewEffects)
	}
}

func TestBuildRepairValidationPreviewProjectsAffectedSectionsAndStatus(t *testing.T) {
	fieldErrors := []string{"missing category"}
	spuName := "SPU-1"

	preview := BuildRepairValidationPreview(
		&sheinpub.Package{},
		"category",
		&EditorRevisionSkeleton{Shein: &RevisionInput{SpuName: &spuName}},
		false,
		fieldErrors,
	)

	if preview == nil {
		t.Fatal("preview = nil")
	}
	if preview.Status != "invalid" || preview.Valid {
		t.Fatalf("status = %#v", preview)
	}
	if len(preview.AffectedSections) != 4 || preview.AffectedSections[0] != "category" {
		t.Fatalf("affected sections = %#v", preview.AffectedSections)
	}
	if len(preview.CategoryPreviewEffects) == 0 || len(preview.FieldErrors) != 1 {
		t.Fatalf("preview details = %#v", preview)
	}
	fieldErrors[0] = "changed"
	if preview.FieldErrors[0] != "missing category" {
		t.Fatalf("field errors not cloned = %#v", preview.FieldErrors)
	}
}

func TestCloneRepairValidationPreviewDeepCopiesSlicesAndDiff(t *testing.T) {
	t.Parallel()

	src := &RepairValidationPreview[string]{
		Valid:       true,
		Status:      "ready",
		FieldErrors: []string{"missing category"},
		RevisionDiffPreview: &RevisionDiffPreview{
			ChangeCount: 1,
			Changes: []RevisionFieldChange{{
				FieldPath: "shein.category_resolution",
				Label:     "category",
			}},
		},
		AffectedSections:            []string{"category"},
		CategoryPreviewEffects:      []EditorEffect{{Label: "category"}},
		AttributePreviewEffects:     []EditorEffect{{Label: "attributes"}},
		SaleAttributePreviewEffects: []EditorEffect{{Label: "sale attributes"}},
	}

	cloned := CloneRepairValidationPreview(src)
	if cloned == nil {
		t.Fatal("CloneRepairValidationPreview() = nil, want clone")
	}
	src.FieldErrors[0] = "changed"
	src.RevisionDiffPreview.Changes[0].Label = "changed"
	src.AffectedSections[0] = "changed"
	src.CategoryPreviewEffects[0].Label = "changed"
	src.AttributePreviewEffects[0].Label = "changed"
	src.SaleAttributePreviewEffects[0].Label = "changed"

	if cloned.FieldErrors[0] != "missing category" {
		t.Fatalf("field errors = %+v, want deep clone", cloned.FieldErrors)
	}
	if cloned.RevisionDiffPreview.Changes[0].Label != "category" {
		t.Fatalf("revision diff = %+v, want deep clone", cloned.RevisionDiffPreview)
	}
	if cloned.AffectedSections[0] != "category" {
		t.Fatalf("affected sections = %+v, want deep clone", cloned.AffectedSections)
	}
	if cloned.CategoryPreviewEffects[0].Label != "category" || cloned.AttributePreviewEffects[0].Label != "attributes" || cloned.SaleAttributePreviewEffects[0].Label != "sale attributes" {
		t.Fatalf("effects = %+v/%+v/%+v, want deep clone", cloned.CategoryPreviewEffects, cloned.AttributePreviewEffects, cloned.SaleAttributePreviewEffects)
	}
}
