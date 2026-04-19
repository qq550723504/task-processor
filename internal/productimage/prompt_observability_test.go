package productimage_test

import (
	"testing"

	"task-processor/internal/productimage"
)

func TestGenerationMetadataClonePreservesPromptObservability(t *testing.T) {
	src := &productimage.GenerationMetadata{
		Provider:       "openai_compatible",
		ModelFamily:    "nano-banana-fast",
		GenerationMode: "scene_generation",
		PromptRef:      "productimage.scene.default",
		PromptKey:      "productimage.scene.default",
		PromptSource:   "registry",
		PromptVersion:  "default",
	}

	cloned := src.Clone()
	if cloned == nil {
		t.Fatal("Clone() = nil")
	}
	if cloned.PromptKey != "productimage.scene.default" || cloned.PromptSource != "registry" || cloned.PromptVersion != "default" {
		t.Fatalf("Clone() lost prompt observability fields: %+v", cloned)
	}
}
