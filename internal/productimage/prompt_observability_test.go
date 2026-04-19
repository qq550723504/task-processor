package productimage

import (
	"testing"

	"task-processor/internal/prompt"
)

func TestResolveProductImagePromptUsesRegistryWhenAvailable(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &promptRegistryStub{
		templates: map[string]string{
			prompt.KProductImageSceneDefault: "scene {{.product_type}}",
		},
	}
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	resolved := resolveProductImagePrompt("productimage.scene.default", prompt.KProductImageSceneDefault, map[string]any{
		"product_type": "sneaker",
	}, "fallback text")

	if resolved.Text != "scene sneaker" {
		t.Fatalf("Text = %q", resolved.Text)
	}
	if resolved.Key != "productimage.scene.default" || resolved.Source != "registry" || resolved.Version != "default" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestResolveProductImagePromptFallsBackWhenRegistryUnavailable(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = nil
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	resolved := resolveProductImagePrompt("", prompt.KProductImageWhiteBackgroundDefault, map[string]any{
		"product_type": "sneaker",
	}, "fallback text")

	if resolved.Text != "fallback text" {
		t.Fatalf("Text = %q", resolved.Text)
	}
	if resolved.Key != "productimage.white_background.default" || resolved.Source != "fallback" || resolved.Version != "default" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestResolveProductImagePromptFallsBackWhenRegistryKeyMissing(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &promptRegistryStub{
		templates: map[string]string{
			"productimage.scene.other": "scene {{.product_type}}",
		},
	}
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	resolved := resolveProductImagePrompt("productimage.scene.default", prompt.KProductImageSceneDefault, map[string]any{
		"product_type": "sneaker",
	}, "fallback text")

	if resolved.Text != "fallback text" {
		t.Fatalf("Text = %q", resolved.Text)
	}
	if resolved.Key != "productimage.scene.default" || resolved.Source != "fallback" || resolved.Version != "default" {
		t.Fatalf("resolved = %+v", resolved)
	}
}

func TestRenderProductImagePromptFallsBackWhenRegistryReturnsWhitespace(t *testing.T) {
	previous := prompt.GlobalRegistry
	prompt.GlobalRegistry = &promptRegistryStub{
		templates: map[string]string{
			prompt.KProductImageSceneDefault: "   ",
		},
	}
	t.Cleanup(func() {
		prompt.GlobalRegistry = previous
	})

	rendered := renderProductImagePrompt("productimage.scene.default", prompt.KProductImageSceneDefault, nil, "fallback text")

	if rendered != "fallback text" {
		t.Fatalf("rendered = %q", rendered)
	}
}
