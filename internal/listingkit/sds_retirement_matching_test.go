package listingkit

import (
	"testing"

	"task-processor/internal/catalog/canonical"
)

func TestSDSRetirementTaskMatchesIdentityFromRequestOptions(t *testing.T) {
	task := Task{Request: &GenerateRequest{Options: &GenerateOptions{SDS: &SDSSyncOptions{
		ParentProductID:  238915,
		PrototypeGroupID: 28345,
		VariantID:        238916,
	}}}}
	identity := SDSBaselineIdentity{ParentProductID: 238915, PrototypeGroupID: 28345, VariantID: 238916}
	if !sdsRetirementTaskMatchesIdentity(&task, identity) {
		t.Fatal("expected task to match SDS identity")
	}
}

func TestSDSRetirementExtractsSourceSDSSKUsFromCanonicalAttributes(t *testing.T) {
	product := &canonical.Product{
		Variants: []canonical.Variant{{
			SKU: "generated-sku",
			Attributes: map[string]canonical.Attribute{
				"source_sds_sku": {Value: "MG8006905001"},
			},
		}},
	}
	got := sdsRetirementSourceSKUs(product)
	if len(got) != 1 || got[0] != "MG8006905001" {
		t.Fatalf("source skus = %#v", got)
	}
}
