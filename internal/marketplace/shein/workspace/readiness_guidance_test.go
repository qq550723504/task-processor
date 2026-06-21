package workspace

import (
	"testing"

	common "task-processor/internal/publishing/common"
	sheinpub "task-processor/internal/publishing/shein"
)

func TestBuildReadinessPatchPayloadMarketplace(t *testing.T) {
	t.Parallel()

	categoryID := 3001
	pkg := &sheinpub.Package{
		CategoryID:     categoryID,
		CategoryIDList: []int{3001, 3002},
		CategoryResolution: &sheinpub.CategoryResolution{
			Status: "resolved",
		},
		Images: &common.ImageSet{
			MainImage: "https://cdn.example.com/main.jpg",
		},
		ReviewNotes: []string{"manual review"},
	}

	categoryPatch := BuildReadinessPatchPayload(pkg, "category_review")
	if categoryPatch == nil || categoryPatch.CategoryResolution == nil || categoryPatch.CategoryResolution.CategoryID == nil || *categoryPatch.CategoryResolution.CategoryID != categoryID {
		t.Fatalf("category patch = %+v, want category resolution patch", categoryPatch)
	}

	imagePatch := BuildReadinessPatchPayload(pkg, "images")
	if imagePatch == nil || imagePatch.Images == nil || imagePatch.Images.MainImage != "https://cdn.example.com/main.jpg" {
		t.Fatalf("image patch = %+v, want images patch", imagePatch)
	}
	pkg.Images.MainImage = "https://cdn.example.com/changed.jpg"
	if imagePatch.Images.MainImage != "https://cdn.example.com/main.jpg" {
		t.Fatalf("image patch main = %q, want deep clone", imagePatch.Images.MainImage)
	}

	notesPatch := BuildReadinessPatchPayload(pkg, "manual_notes")
	if notesPatch == nil || len(notesPatch.ReviewNotes) != 1 || notesPatch.ReviewNotes[0] != "manual review" {
		t.Fatalf("notes patch = %+v, want review notes patch", notesPatch)
	}
	pkg.ReviewNotes[0] = "changed"
	if notesPatch.ReviewNotes[0] != "manual review" {
		t.Fatalf("notes patch note = %q, want deep clone", notesPatch.ReviewNotes[0])
	}

	if patch := BuildReadinessPatchPayload(pkg, "source_facts"); patch != nil {
		t.Fatalf("source facts patch = %+v, want nil direct repair patch", patch)
	}
}

func TestBuildReadinessGuidanceSpecMarketplace(t *testing.T) {
	spec := BuildReadinessGuidanceSpec("category_review", false)
	if spec == nil || spec.Reason == nil {
		t.Fatal("expected guidance spec")
	}
	if spec.Reason.Code != "category_review_pending" {
		t.Fatalf("reason code = %q", spec.Reason.Code)
	}
	if len(spec.Hints) != 1 || spec.Hints[0].Target != "editor.category" {
		t.Fatalf("hints = %+v", spec.Hints)
	}
}

func TestBuildReadinessGuidanceSpecMarketplaceManualNotesWarningCategory(t *testing.T) {
	spec := BuildReadinessGuidanceSpec("manual_notes", true)
	if spec == nil || spec.Reason == nil {
		t.Fatal("expected guidance spec")
	}
	if spec.Reason.Category != "manual_review" {
		t.Fatalf("reason category = %q", spec.Reason.Category)
	}
}

func TestBuildReadinessGuidanceSpecMarketplaceUnknown(t *testing.T) {
	if spec := BuildReadinessGuidanceSpec("unknown", false); spec != nil {
		t.Fatalf("spec = %+v, want nil", spec)
	}
}
