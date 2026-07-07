package listingkit

import "testing"

func TestStudioSpecificationsIncludesSDSProductIdentity(t *testing.T) {
	t.Parallel()

	specs := studioSpecifications(&SDSSyncOptions{
		ParentProductID:  238915,
		VariantID:        238916,
		PrototypeGroupID: 28345,
		VariantSize:      "L",
	})

	if specs == nil || specs.Technical == nil {
		t.Fatal("expected technical specs")
	}
	if got := specs.Technical["parent_product_id"]; got != "238915" {
		t.Fatalf("parent_product_id = %q, want 238915", got)
	}
	if got := specs.Technical["variant_id"]; got != "238916" {
		t.Fatalf("variant_id = %q, want 238916", got)
	}
	if got := specs.Technical["prototype_group_id"]; got != "28345" {
		t.Fatalf("prototype_group_id = %q, want 28345", got)
	}
}
