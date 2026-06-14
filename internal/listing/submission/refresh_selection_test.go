package submission

import "testing"

func TestResolveRefreshActionPrefersLastAction(t *testing.T) {
	t.Parallel()

	got := ResolveRefreshAction(" save_draft ", true, true)
	if got != "save_draft" {
		t.Fatalf("ResolveRefreshAction() = %q, want save_draft", got)
	}
}

func TestResolveRefreshActionFallsBackToPublishThenSaveDraft(t *testing.T) {
	t.Parallel()

	if got := ResolveRefreshAction("", true, true); got != "publish" {
		t.Fatalf("ResolveRefreshAction() = %q, want publish", got)
	}
	if got := ResolveRefreshAction("", false, true); got != "save_draft" {
		t.Fatalf("ResolveRefreshAction() = %q, want save_draft", got)
	}
}

func TestResolveRefreshSupplierCodePrefersRecordValue(t *testing.T) {
	t.Parallel()

	got := ResolveRefreshSupplierCode(" record-sku ", "package-sku")
	if got != "record-sku" {
		t.Fatalf("ResolveRefreshSupplierCode() = %q, want record-sku", got)
	}
}

func TestResolveRefreshSupplierCodeFallsBackToPackageValue(t *testing.T) {
	t.Parallel()

	got := ResolveRefreshSupplierCode("", " package-sku ")
	if got != "package-sku" {
		t.Fatalf("ResolveRefreshSupplierCode() = %q, want package-sku", got)
	}
}
