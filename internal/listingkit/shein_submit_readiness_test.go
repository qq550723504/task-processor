package listingkit

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestBuildSheinSubmitReadinessBlockedWhenCoreFieldsMissing(t *testing.T) {
	t.Parallel()

	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryPath: []string{"Home", "Kitchen", "Bottle"},
		ReviewNotes:  []string{"人工确认尺寸映射"},
	})
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if readiness.Ready {
		t.Fatalf("ready = true, want false; readiness=%+v", readiness)
	}
	if readiness.Status != "blocked" {
		t.Fatalf("status = %q, want blocked", readiness.Status)
	}
	if len(readiness.BlockingItems) < 4 {
		t.Fatalf("blocking items = %+v, want multiple blockers", readiness.BlockingItems)
	}
	categoryBlocker := readiness.BlockingItems[0]
	if categoryBlocker.Key != "category" {
		t.Fatalf("first blocker key = %q, want category", categoryBlocker.Key)
	}
	if categoryBlocker.Reason == nil || categoryBlocker.Reason.Code != "category_unresolved" {
		t.Fatalf("category blocker reason = %+v", categoryBlocker.Reason)
	}
	if len(categoryBlocker.RepairHints) != 1 || categoryBlocker.RepairHints[0].Target != "editor.category" {
		t.Fatalf("category blocker repair hints = %+v", categoryBlocker.RepairHints)
	}
	if categoryBlocker.RepairHints[0].EditorSection != "category" || categoryBlocker.RepairHints[0].RevisionPath != "shein.category_resolution" {
		t.Fatalf("category blocker editor metadata = %+v", categoryBlocker.RepairHints[0])
	}
	if categoryBlocker.RepairHints[0].Patch == nil || categoryBlocker.RepairHints[0].Patch.CategoryResolution == nil {
		t.Fatalf("category blocker patch = %+v", categoryBlocker.RepairHints[0].Patch)
	}
	if categoryBlocker.RepairHints[0].Skeleton == nil || categoryBlocker.RepairHints[0].Skeleton.Shein == nil || categoryBlocker.RepairHints[0].Skeleton.Shein.CategoryResolution == nil {
		t.Fatalf("category blocker skeleton = %+v", categoryBlocker.RepairHints[0].Skeleton)
	}
	if categoryBlocker.RepairHints[0].Revision == nil || categoryBlocker.RepairHints[0].Revision.Shein == nil || categoryBlocker.RepairHints[0].Revision.Shein.CategoryResolution == nil {
		t.Fatalf("category blocker revision = %+v", categoryBlocker.RepairHints[0].Revision)
	}
	if categoryBlocker.RepairHints[0].Validation == nil || !categoryBlocker.RepairHints[0].Validation.Valid || categoryBlocker.RepairHints[0].Validation.Status != "ready" {
		t.Fatalf("category blocker validation = %+v", categoryBlocker.RepairHints[0].Validation)
	}
	if len(categoryBlocker.RepairHints[0].Validation.CategoryPreviewEffects) == 0 {
		t.Fatalf("category blocker validation effects = %+v", categoryBlocker.RepairHints[0].Validation)
	}
	if len(readiness.WarningItems) != 1 || readiness.WarningItems[0].Key != "manual_notes" {
		t.Fatalf("warning items = %+v", readiness.WarningItems)
	}
	if readiness.WarningItems[0].Reason == nil || readiness.WarningItems[0].Reason.Code != "manual_review_pending" {
		t.Fatalf("warning reason = %+v", readiness.WarningItems[0].Reason)
	}
}

func TestBuildSheinSubmitReadinessReadyWithWarningsAfterManualNotes(t *testing.T) {
	t.Parallel()

	productTypeID := 901
	readiness := buildSheinSubmitReadiness(&SheinPackage{
		CategoryID:    3001,
		CategoryPath:  []string{"Home", "Kitchen", "Bottle"},
		ProductTypeID: &productTypeID,
		Images: &PlatformImageSet{
			MainImage: "https://cdn.example.com/main.jpg",
		},
		ResolvedAttributes: []SheinResolvedAttribute{{
			Name:        "material",
			AttributeID: 7001,
		}},
		CategoryResolution: &SheinCategoryResolution{
			Status:     "resolved",
			CategoryID: 3001,
		},
		AttributeResolution: &SheinAttributeResolution{
			Status:        "resolved",
			ResolvedCount: 1,
		},
		SaleAttributeResolution: &SheinSaleAttributeResolution{
			Status:             "resolved",
			PrimaryAttributeID: 501,
		},
		RequestDraft: &SheinRequestDraft{
			ResolvedAttributes: []SheinResolvedAttribute{{
				Name:        "material",
				AttributeID: 7001,
			}},
			SKCList: []SheinSKCRequestDraft{{
				SupplierCode: "SKC-1",
				SKUList: []SheinSKUDraft{{
					SupplierSKU: "SKU-1",
				}},
			}},
		},
		PreviewProduct: &sheinproduct.Product{},
		SkcList: []SheinSKCPackage{{
			SupplierCode: "SKC-1",
			SKUs: []PlatformVariant{{
				SKU: "SKU-1",
			}},
		}},
		ReviewNotes: []string{"人工确认站点价格"},
	})
	if readiness == nil {
		t.Fatal("expected readiness")
	}
	if readiness.Ready != true {
		t.Fatalf("ready = false, want true; readiness=%+v", readiness)
	}
	if readiness.Status != "ready_with_warnings" {
		t.Fatalf("status = %q, want ready_with_warnings", readiness.Status)
	}
	if len(readiness.BlockingItems) != 0 {
		t.Fatalf("blocking items = %+v, want none", readiness.BlockingItems)
	}
	if len(readiness.WarningItems) != 1 || readiness.WarningItems[0].Key != "manual_notes" {
		t.Fatalf("warning items = %+v", readiness.WarningItems)
	}
	if readiness.WarningItems[0].Reason == nil || readiness.WarningItems[0].Reason.Category != "manual_review" {
		t.Fatalf("warning reason = %+v", readiness.WarningItems[0].Reason)
	}
	if len(readiness.WarningItems[0].RepairHints) != 1 || readiness.WarningItems[0].RepairHints[0].Target != "editor.basics.review_notes" {
		t.Fatalf("warning repair hints = %+v", readiness.WarningItems[0].RepairHints)
	}
	if readiness.WarningItems[0].RepairHints[0].Patch == nil || len(readiness.WarningItems[0].RepairHints[0].Patch.ReviewNotes) != 1 {
		t.Fatalf("warning patch = %+v", readiness.WarningItems[0].RepairHints[0].Patch)
	}
	if readiness.WarningItems[0].RepairHints[0].Revision == nil || readiness.WarningItems[0].RepairHints[0].Revision.Shein == nil || len(readiness.WarningItems[0].RepairHints[0].Revision.Shein.ReviewNotes) != 1 {
		t.Fatalf("warning revision = %+v", readiness.WarningItems[0].RepairHints[0].Revision)
	}
	if readiness.WarningItems[0].RepairHints[0].Validation == nil || !readiness.WarningItems[0].RepairHints[0].Validation.Valid {
		t.Fatalf("warning validation = %+v", readiness.WarningItems[0].RepairHints[0].Validation)
	}
	if len(readiness.WarningItems[0].RepairHints[0].Validation.AffectedSections) == 0 {
		t.Fatalf("warning validation sections = %+v", readiness.WarningItems[0].RepairHints[0].Validation)
	}
}
