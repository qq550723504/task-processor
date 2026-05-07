package shein

import "testing"

func TestValidateRevisionInputReportsInvalidCategory(t *testing.T) {
	invalidCategoryID := 0

	errs := ValidateRevisionInput(&RevisionInput{
		CategoryID: &invalidCategoryID,
	})

	if len(errs) != 1 {
		t.Fatalf("ValidateRevisionInput() returned %d errors, want 1", len(errs))
	}
	if errs[0].FieldPath != "shein.category_id" || errs[0].Code != "invalid_value" {
		t.Fatalf("ValidateRevisionInput() error = %+v", errs[0])
	}
}

func TestValidateRevisionInputReportsInvalidSKUPatch(t *testing.T) {
	negativeStock := -1

	errs := ValidateRevisionInput(&RevisionInput{
		SKCPatches: []SKCRevisionPatch{{
			SKUPatches: []SKURevisionPatch{{
				StockCount: &negativeStock,
			}},
		}},
	})

	if len(errs) != 3 {
		t.Fatalf("ValidateRevisionInput() returned %d errors, want 3: %+v", len(errs), errs)
	}
	if errs[0].FieldPath != "shein.skc_patches[0].supplier_code" {
		t.Fatalf("first field path = %q", errs[0].FieldPath)
	}
	if errs[1].FieldPath != "shein.skc_patches[0].sku_patches[0].supplier_sku" {
		t.Fatalf("second field path = %q", errs[1].FieldPath)
	}
	if errs[2].FieldPath != "shein.skc_patches[0].sku_patches[0].stock_count" {
		t.Fatalf("third field path = %q", errs[2].FieldPath)
	}
}
