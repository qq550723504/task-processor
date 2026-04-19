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
