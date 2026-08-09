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
