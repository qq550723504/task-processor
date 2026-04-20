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

func TestBuildSceneGenerationResolvedPromptUsesCategorySpecificTemplateWhenAvailable(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &promptRegistryStub{
		templates: map[string]string{
			"productimage.scene.shoes": "Shoes scene {{.product_type}} / {{.scene_style}} / {{.background_tone}} / {{.custom_scene_hint}}",
		},
	}
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	resolved := buildSceneGenerationResolvedPrompt(&SceneGenerationRequest{
		SceneIntent:     "gallery_scene",
		SceneStyle:      "lifestyle",
		BackgroundTone:  "warm",
		CustomSceneHint: "show subtle motion energy",
		ProductContext: &ProductContext{
			ProductType: "running sneaker",
			Title:       "Red running shoe",
		},
	})

	if resolved.Key != "productimage.scene.shoes" {
		t.Fatalf("prompt_key = %q", resolved.Key)
	}
	if resolved.Source != "registry" || resolved.Version != "default" {
		t.Fatalf("resolved = %+v", resolved)
	}
	if resolved.Text != "Shoes scene running sneaker / lifestyle / warm / show subtle motion energy" {
		t.Fatalf("prompt = %q", resolved.Text)
	}
}

func TestBuildSceneGenerationResolvedPromptFallsBackToDefaultTemplateWhenCategoryTemplateMissing(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &promptRegistryStub{
		templates: map[string]string{
			prompt.KProductImageSceneDefault: "Default scene {{.product_type}} / {{.scene_intent}}",
		},
	}
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	resolved := buildSceneGenerationResolvedPrompt(&SceneGenerationRequest{
		SceneIntent: "gallery_scene",
		ProductContext: &ProductContext{
			ProductType: "desk lamp",
			Title:       "Minimalist table lamp",
		},
	})

	if resolved.Key != prompt.KProductImageSceneDefault {
		t.Fatalf("prompt_key = %q", resolved.Key)
	}
	if resolved.Source != "registry" {
		t.Fatalf("resolved = %+v", resolved)
	}
	if resolved.Text != "Default scene desk lamp / gallery_scene" {
		t.Fatalf("prompt = %q", resolved.Text)
	}
}

func TestBuildSceneGenerationResolvedPromptUsesRequestAndContextCustomizations(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &promptRegistryStub{
		templates: map[string]string{
			"productimage.scene.jewelry": "Jewelry scene {{.scene_style}} / {{.background_tone}} / {{.composition}} / {{.props_level}} / {{.audience_hint}} / {{.custom_scene_hint}}",
		},
	}
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	resolved := buildSceneGenerationResolvedPrompt(&SceneGenerationRequest{
		SceneIntent: "gallery_scene",
		SceneStyle:  "studio",
		ProductContext: &ProductContext{
			ProductType: "necklace",
			Attributes: map[string]string{
				"background_tone":   "cool",
				"composition":       "close_up",
				"props_level":       "light",
				"audience_hint":     "premium",
				"custom_scene_hint": "keep reflective surfaces restrained",
			},
		},
	})

	if resolved.Key != "productimage.scene.jewelry" {
		t.Fatalf("prompt_key = %q", resolved.Key)
	}
	if resolved.Text != "Jewelry scene studio / cool / close_up / light / premium / keep reflective surfaces restrained" {
		t.Fatalf("prompt = %q", resolved.Text)
	}
}
