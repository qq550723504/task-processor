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
	if CapabilityProductEnrichVision != "productenrich.vision_understanding" {
		t.Fatalf("vision capability = %q", CapabilityProductEnrichVision)
	}
	if OperationProductEnrichImageAnalyze != "productenrich_image_analyze" {
		t.Fatalf("image operation = %q", OperationProductEnrichImageAnalyze)
	}
	if FeatureVisionAnalyze != "vision_analyze" {
		t.Fatalf("vision feature = %q", FeatureVisionAnalyze)
	}
	if CapabilityProductEnrichListing != "productenrich.listing_generation" {
		t.Fatalf("listing capability = %q", CapabilityProductEnrichListing)
	}
	if OperationProductEnrichJSONGenerate != "productenrich_json_generate" {
		t.Fatalf("json operation = %q", OperationProductEnrichJSONGenerate)
	}
	if OperationProductEnrichSpecsGenerate != "productenrich_specs_generate" {
		t.Fatalf("specs operation = %q", OperationProductEnrichSpecsGenerate)
	}
}
