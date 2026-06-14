package workspace

import "testing"

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
