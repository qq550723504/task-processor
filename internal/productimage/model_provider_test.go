package productimage_test

import (
	"testing"

	"task-processor/internal/productimage"
)

func TestGenerationMetadataClonePreservesModelFields(t *testing.T) {
	src := &productimage.GenerationMetadata{
		Provider:         "openai",
		ModelFamily:      "gpt-image",
		GenerationMode:   "scene_generation",
		PromptRef:        "preset:selling_point/default",
		ReviewConfidence: 0.82,
	}

	cloned := src.Clone()
	if cloned == nil {
		t.Fatal("Clone() = nil")
	}
	if cloned == src {
		t.Fatal("Clone() returned original pointer")
	}
	if cloned.Provider != "openai" || cloned.ModelFamily != "gpt-image" || cloned.GenerationMode != "scene_generation" {
		t.Fatalf("Clone() lost fields: %+v", cloned)
	}
	if cloned.PromptRef != "preset:selling_point/default" || cloned.ReviewConfidence != 0.82 {
		t.Fatalf("Clone() lost metadata: %+v", cloned)
	}
}
