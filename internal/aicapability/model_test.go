package aicapability

import "testing"

func TestProductEnrichTextCapabilityContracts(t *testing.T) {
	if CapabilityProductEnrichText != "productenrich.text_understanding" {
		t.Fatalf("capability = %q", CapabilityProductEnrichText)
	}
	if OperationProductEnrichTextExtract != "productenrich_text_extract_attributes" {
		t.Fatalf("operation = %q", OperationProductEnrichTextExtract)
	}
	if FeatureTextGenerate != "text_generate" {
		t.Fatalf("feature = %q", FeatureTextGenerate)
	}
}
