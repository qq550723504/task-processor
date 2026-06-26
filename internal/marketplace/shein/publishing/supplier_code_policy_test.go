package publishing

import "testing"

func TestDeriveSubmitSupplierCodeUsesExistingNonRawCode(t *testing.T) {
	t.Parallel()

	got := DeriveSubmitSupplierCode(" SUPPLIER-CODE ", []string{"MG8089003001-V295977-TEEC9CE8E-RCODEX-4F2669C9"})
	if got != "SUPPLIER-CODE" {
		t.Fatalf("DeriveSubmitSupplierCode() = %q, want existing supplier code", got)
	}
}

func TestDeriveSubmitSupplierCodeDerivesFromSKUWhenProductCodeIsRaw(t *testing.T) {
	t.Parallel()

	got := DeriveSubmitSupplierCode("MG8089003001", []string{"MG8089003001-V295977-TEEC9CE8E-RCODEX-4F2669C9"})
	if got != "MG8089003001-4F2669C9" {
		t.Fatalf("DeriveSubmitSupplierCode() = %q, want derived supplier code", got)
	}
}

func TestDeriveSubmitSupplierCodeFallsBackToProductCode(t *testing.T) {
	t.Parallel()

	got := DeriveSubmitSupplierCode(" RAWCODE ", []string{"", "   "})
	if got != "RAWCODE" {
		t.Fatalf("DeriveSubmitSupplierCode() = %q, want trimmed fallback product code", got)
	}
}
