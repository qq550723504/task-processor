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
		Variants: []SDSSyncVariantOption{
			{VariantID: 3002},
			{VariantID: 3001},
		},
	}}}}
	identity := SDSBaselineIdentity{
		ParentProductID:    238915,
		PrototypeGroupID:   28345,
		VariantID:          238916,
		SelectedVariantIDs: []int64{3001, 3002},
	}
	if !sdsRetirementTaskMatchesIdentity(&task, identity) {
		t.Fatal("expected task to match SDS identity")
	}
}

func TestSDSRetirementTaskMatchingRequiresSelectedVariantIdentity(t *testing.T) {
	task := Task{Request: &GenerateRequest{Options: &GenerateOptions{SDS: &SDSSyncOptions{
		ParentProductID:  238915,
		PrototypeGroupID: 28345,
		VariantID:        238916,
		Variants: []SDSSyncVariantOption{
			{VariantID: 3001},
		},
	}}}}
	identity := SDSBaselineIdentity{
		ParentProductID:    238915,
		PrototypeGroupID:   28345,
		VariantID:          238916,
		SelectedVariantIDs: []int64{3002},
	}

	if sdsRetirementTaskMatchesIdentity(&task, identity) {
		t.Fatal("expected different selected variant sets not to match")
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
