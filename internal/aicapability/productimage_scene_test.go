package aicapability

import "testing"

func TestProductImageSceneCapabilityContract(t *testing.T) {
	if CapabilityProductImageScene != "productimage.scene_generation" {
		t.Fatalf("capability = %q", CapabilityProductImageScene)
	}
	if OperationProductImageSceneGenerate != "productimage_scene_generate" {
		t.Fatalf("operation = %q", OperationProductImageSceneGenerate)
	}
}

func TestProductImageModelOperationContracts(t *testing.T) {
	if OperationProductImageSubjectExtract != "productimage_subject_extract" ||
		OperationProductImageWhiteBackground != "productimage_white_background" ||
		OperationProductImageReview != "productimage_review" {
		t.Fatalf("unexpected product image operation contracts")
	}
}
