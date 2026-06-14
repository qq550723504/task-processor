package studio

import "testing"

func TestNextBatchNameUsesNextLargestTenantBatchNumber(t *testing.T) {
	got := NextBatchName([]string{"批次1", "批次7", "other", "批次x", " 批次3 "})

	if got != "批次8" {
		t.Fatalf("NextBatchName() = %q, want 批次8", got)
	}
}

func TestNextBatchNameStartsAtOne(t *testing.T) {
	got := NextBatchName(nil)

	if got != "批次1" {
		t.Fatalf("NextBatchName() = %q, want 批次1", got)
	}
}
