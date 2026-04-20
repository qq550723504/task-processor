package productimage

import (
	"testing"

	"task-processor/internal/prompt"
)

func TestBuildSceneGenerationPromptUsesRegistryTemplateWhenAvailable(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &promptRegistryStub{
		templates: map[string]string{
			prompt.KProductImageSceneDefault: "Registry scene prompt {{.product_type}} / {{.title}} / {{.scene_intent}}",
		},
	}
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	rendered := buildSceneGenerationPrompt(&SceneGenerationRequest{
		SceneIntent: "gallery_scene",
		ProductContext: &ProductContext{
			ProductType: "sneaker",
			Title:       "Red running shoe",
		},
	})

	if rendered != "Registry scene prompt sneaker / Red running shoe / gallery_scene" {
		t.Fatalf("prompt = %q", rendered)
	}
}

func TestBuildSceneGenerationResolvedPromptUsesRegistryMetadata(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &promptRegistryStub{
		templates: map[string]string{
			prompt.KProductImageSceneDefault: "Registry scene prompt {{.product_type}} / {{.title}} / {{.scene_intent}}",
		},
	}
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	resolved := buildSceneGenerationResolvedPrompt(&SceneGenerationRequest{
		SceneIntent: "gallery_scene",
		ProductContext: &ProductContext{
			ProductType: "sneaker",
			Title:       "Red running shoe",
		},
	})
	metadata := applyPromptObservabilityMetadata(map[string]string{}, resolved)

	if resolved.Text != "Registry scene prompt sneaker / Red running shoe / gallery_scene" {
		t.Fatalf("prompt = %q", resolved.Text)
	}
	if metadata["prompt_ref"] != prompt.KProductImageSceneDefault {
		t.Fatalf("prompt_ref = %q", metadata["prompt_ref"])
	}
	if metadata["prompt_key"] != prompt.KProductImageSceneDefault {
		t.Fatalf("prompt_key = %q", metadata["prompt_key"])
	}
	if metadata["prompt_source"] != "registry" {
		t.Fatalf("prompt_source = %q", metadata["prompt_source"])
	}
	if metadata["prompt_version"] != "default" {
		t.Fatalf("prompt_version = %q", metadata["prompt_version"])
	}
}

func TestBuildSceneGenerationResolvedPromptUsesFallbackMetadataWhenRegistryUnavailable(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = nil
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	resolved := buildSceneGenerationResolvedPrompt(&SceneGenerationRequest{
		SceneIntent: "gallery_scene",
		ProductContext: &ProductContext{
			ProductType: "sneaker",
			Title:       "Red running shoe",
		},
	})
	metadata := applyPromptObservabilityMetadata(map[string]string{}, resolved)

	if !containsInsensitive(resolved.Text, "scene intent: gallery_scene") {
		t.Fatalf("prompt = %q", resolved.Text)
	}
	if metadata["prompt_ref"] != prompt.KProductImageSceneDefault {
		t.Fatalf("prompt_ref = %q", metadata["prompt_ref"])
	}
	if metadata["prompt_key"] != prompt.KProductImageSceneDefault {
		t.Fatalf("prompt_key = %q", metadata["prompt_key"])
	}
	if metadata["prompt_source"] != "fallback" {
		t.Fatalf("prompt_source = %q", metadata["prompt_source"])
	}
	if metadata["prompt_version"] != "default" {
		t.Fatalf("prompt_version = %q", metadata["prompt_version"])
	}
}
