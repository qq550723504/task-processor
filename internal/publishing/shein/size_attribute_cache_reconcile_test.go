package shein

import (
	"testing"

	sheinproduct "task-processor/internal/shein/api/product"
)

func TestReconcileSizeAttributeCacheReviewRemapsManualOverrideToCurrentSizeRow(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		RequestDraft: &RequestDraft{
			SizeAttributeList: []sheinproduct.SizeAttribute{
				{AttributeID: 15, AttributeExtraValue: "112", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 417},
			},
		},
	}
	review := &SizeAttributeReview{
		Attributes: []sheinproduct.SizeAttribute{
			{AttributeID: 15, AttributeExtraValue: "110", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 999},
		},
		ManualOverrides: map[string]string{
			"87:999|15": "114",
		},
		Ready: true,
	}

	got := ReconcileSizeAttributeCacheReview(pkg, review)

	if got != review {
		t.Fatal("ReconcileSizeAttributeCacheReview returned a different review pointer")
	}
	if len(got.Attributes) != 1 {
		t.Fatalf("attributes = %#v, want one current size attribute", got.Attributes)
	}
	attr := got.Attributes[0]
	if attr.RelateSaleAttributeValueID != 417 {
		t.Fatalf("relate sale attribute value id = %d, want current value id 417", attr.RelateSaleAttributeValueID)
	}
	if attr.AttributeExtraValue != "114" {
		t.Fatalf("attribute extra value = %q, want remapped manual value", attr.AttributeExtraValue)
	}
	if got.ManualOverrides["87:417|15"] != "114" {
		t.Fatalf("manual overrides = %#v, want current row override", got.ManualOverrides)
	}
	if _, exists := got.ManualOverrides["87:999|15"]; exists {
		t.Fatalf("manual overrides retained stale row: %#v", got.ManualOverrides)
	}
}

func TestSizeAttributeCacheKeyIgnoresMeasuredValuesButTracksShape(t *testing.T) {
	t.Parallel()

	req := &BuildRequest{SheinStoreID: 869}
	first := &Package{
		CategoryID:     3221,
		CategoryIDList: []int{1, 2, 3221},
		RequestDraft: &RequestDraft{
			SizeAttributeList: []sheinproduct.SizeAttribute{
				{AttributeID: 15, AttributeExtraValue: "112", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 417},
			},
		},
	}
	changedValue := &Package{
		CategoryID:     3221,
		CategoryIDList: []int{1, 2, 3221},
		RequestDraft: &RequestDraft{
			SizeAttributeList: []sheinproduct.SizeAttribute{
				{AttributeID: 15, AttributeExtraValue: "114", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 417},
			},
		},
	}
	changedShape := &Package{
		CategoryID:     3221,
		CategoryIDList: []int{1, 2, 3221},
		RequestDraft: &RequestDraft{
			SizeAttributeList: []sheinproduct.SizeAttribute{
				{AttributeID: 20, AttributeExtraValue: "72", RelateSaleAttributeID: 87, RelateSaleAttributeValueID: 417},
			},
		},
	}

	firstKey := SizeAttributeCacheKey(req, first)
	if firstKey == "" {
		t.Fatal("SizeAttributeCacheKey returned empty key")
	}
	if got := SizeAttributeCacheKey(req, changedValue); got != firstKey {
		t.Fatalf("SizeAttributeCacheKey changed after measured value edit: %q vs %q", firstKey, got)
	}
	if got := SizeAttributeCacheKey(req, changedShape); got == firstKey {
		t.Fatalf("SizeAttributeCacheKey did not change after shape changed: %q", got)
	}
}

func TestSizeAttributeReviewApplicableToSaleResolutionAcceptsCompletePublishedRows(t *testing.T) {
	t.Parallel()

	sID, fiveXLID := 568, 1430561
	pkg := &Package{SaleAttributeResolution: &SaleAttributeResolution{
		SecondaryAttributeID: 87,
		SKUValueAssignments: map[string]ResolvedSaleAttribute{
			"s":   {AttributeID: 87, AttributeValueID: &sID},
			"5xl": {AttributeID: 87, AttributeValueID: &fiveXLID},
		},
	}}
	review := &SizeAttributeReview{Ready: true, Attributes: []sheinproduct.SizeAttribute{
		{AttributeID: 55, RelateSaleAttributeID: 87, RelateSaleAttributeValueID: sID, AttributeExtraValue: "69.5"},
		{AttributeID: 55, RelateSaleAttributeID: 87, RelateSaleAttributeValueID: fiveXLID, AttributeExtraValue: "80"},
	}}

	if !SizeAttributeReviewApplicableToSaleResolution(pkg, review) {
		t.Fatal("complete published size rows were rejected")
	}
	review.Attributes[1].RelateSaleAttributeValueID = 999999
	if SizeAttributeReviewApplicableToSaleResolution(pkg, review) {
		t.Fatal("size row with unknown sale value id was accepted")
	}
}
