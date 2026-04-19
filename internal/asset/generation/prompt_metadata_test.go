package generation

import (
	"testing"

	"task-processor/internal/productimage"
)

func TestAttachGenerationMetadataCopiesPromptObservability(t *testing.T) {
	t.Parallel()

	task := Task{
		Metadata: map[string]string{
			"custom": "keep",
		},
	}

	AttachGenerationMetadata(&task, &productimage.GenerationMetadata{
		Provider:       "openai_compatible",
		ModelFamily:    "nano-banana-fast",
		GenerationMode: "scene_generation",
		PromptRef:      "productimage.scene.default",
		PromptKey:      "productimage.scene.default",
		PromptSource:   "registry",
		PromptVersion:  "default",
	})

	if task.Metadata["custom"] != "keep" {
		t.Fatalf("custom metadata lost: %+v", task.Metadata)
	}
	if task.Metadata["model_provider"] != "openai_compatible" {
		t.Fatalf("model_provider = %q", task.Metadata["model_provider"])
	}
	if task.Metadata["model_family"] != "nano-banana-fast" {
		t.Fatalf("model_family = %q", task.Metadata["model_family"])
	}
	if task.Metadata["generation_mode"] != "scene_generation" {
		t.Fatalf("generation_mode = %q", task.Metadata["generation_mode"])
	}
	if task.Metadata["prompt_ref"] != "productimage.scene.default" {
		t.Fatalf("prompt_ref = %q", task.Metadata["prompt_ref"])
	}
	if task.Metadata["prompt_key"] != "productimage.scene.default" {
		t.Fatalf("prompt_key = %q", task.Metadata["prompt_key"])
	}
	if task.Metadata["prompt_source"] != "registry" {
		t.Fatalf("prompt_source = %q", task.Metadata["prompt_source"])
	}
	if task.Metadata["prompt_version"] != "default" {
		t.Fatalf("prompt_version = %q", task.Metadata["prompt_version"])
	}
}

func TestAttachGenerationMetadataSkipsEmptyFields(t *testing.T) {
	t.Parallel()

	task := Task{
		Metadata: map[string]string{
			"prompt_source":  "registry",
			"prompt_version": "default",
		},
	}

	AttachGenerationMetadata(&task, &productimage.GenerationMetadata{
		Provider:    "openai_compatible",
		ModelFamily: "nano-banana-fast",
	})

	if task.Metadata["model_provider"] != "openai_compatible" {
		t.Fatalf("model_provider = %q", task.Metadata["model_provider"])
	}
	if task.Metadata["model_family"] != "nano-banana-fast" {
		t.Fatalf("model_family = %q", task.Metadata["model_family"])
	}
	if task.Metadata["prompt_source"] != "registry" {
		t.Fatalf("prompt_source = %q", task.Metadata["prompt_source"])
	}
	if task.Metadata["prompt_version"] != "default" {
		t.Fatalf("prompt_version = %q", task.Metadata["prompt_version"])
	}
	if _, ok := task.Metadata["prompt_key"]; ok {
		t.Fatalf("prompt_key should not be set: %+v", task.Metadata)
	}
}
