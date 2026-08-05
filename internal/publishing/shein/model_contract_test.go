package shein

import (
	"testing"

	common "task-processor/internal/publishing/common"
)

func TestDraftPayloadOfNormalizesLegacyRequestDraft(t *testing.T) {
	legacy := &RequestDraft{SpuName: "Legacy product"}
	pkg := &Package{RequestDraft: legacy}

	got := DraftPayloadOf(pkg)
	if got != legacy {
		t.Fatalf("DraftPayloadOf() = %p, want legacy draft %p", got, legacy)
	}
	if pkg.DraftPayload != legacy {
		t.Fatalf("DraftPayload = %p, want legacy draft %p", pkg.DraftPayload, legacy)
	}
}

func TestSetDraftPayloadMirrorsLegacyCompatibilityAlias(t *testing.T) {
	pkg := &Package{}
	draft := &RequestDraft{SpuName: "Semantic product"}

	if got := SetDraftPayload(pkg, draft); got != pkg {
		t.Fatalf("SetDraftPayload() returned %p, want package %p", got, pkg)
	}
	if pkg.DraftPayload != draft || pkg.RequestDraft != draft {
		t.Fatalf("draft aliases = semantic %p, legacy %p; want both %p", pkg.DraftPayload, pkg.RequestDraft, draft)
	}
}

func TestEnsureDraftPayloadCreatesSemanticDraftAndAliasesIt(t *testing.T) {
	pkg := &Package{}

	draft := EnsureDraftPayload(pkg)
	if draft == nil {
		t.Fatal("EnsureDraftPayload() returned nil")
	}
	if pkg.DraftPayload != draft || pkg.RequestDraft != draft {
		t.Fatalf("draft aliases = semantic %p, legacy %p; want both %p", pkg.DraftPayload, pkg.RequestDraft, draft)
	}
	if EnsureDraftPayload(pkg) != draft {
		t.Fatal("EnsureDraftPayload() replaced an existing draft")
	}
}

func TestValidateDraftPayloadReportsStructuralIssues(t *testing.T) {
	issues := ValidateDraftPayload(&RequestDraft{
		SKCList: []SKCRequestDraft{{SupplierCode: "SKC-1"}},
	})

	if !hasDraftIssueCode(issues, "skc_sku_list_required") {
		t.Fatalf("issues = %+v, want skc_sku_list_required", issues)
	}
}

func TestValidateDraftPayloadAcceptsStructurallyCompleteDraft(t *testing.T) {
	issues := ValidateDraftPayload(&RequestDraft{
		SpuName: "Complete product",
		SKCList: []SKCRequestDraft{{
			SupplierCode: "SKC-1",
			SKUList:      []SKUDraft{{SupplierSKU: "SKU-1"}},
		}},
	})

	if len(issues) != 0 {
		t.Fatalf("issues = %+v, want none", issues)
	}
}

func TestHasAnySubmitSKUUsesDraftAsSourceWhenBothViewsExist(t *testing.T) {
	pkg := &Package{
		SkcList: []SKCPackage{{SKUs: []common.Variant{{SKU: "stale-package-sku"}}}},
		DraftPayload: &RequestDraft{SKCList: []SKCRequestDraft{{
			SKUList: nil,
		}}},
	}

	if HasAnySubmitSKU(pkg) {
		t.Fatal("HasAnySubmitSKU() = true, want false when semantic draft has no SKU")
	}
}

func hasDraftIssueCode(issues []DraftPayloadIssue, want string) bool {
	for _, issue := range issues {
		if issue.Code == want {
			return true
		}
	}
	return false
}
