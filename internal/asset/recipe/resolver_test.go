package recipe_test

import (
	"testing"

	assetrecipe "task-processor/internal/asset/recipe"
)

func TestStaticResolverResolvesPlatformRecipes(t *testing.T) {
	t.Parallel()

	resolver := assetrecipe.NewStaticResolver()
	recipes := resolver.Resolve(assetrecipe.ResolveRequest{
		Platform:     "amazon",
		CategoryPath: []string{"Home", "Storage"},
	})

	if len(recipes) == 0 {
		t.Fatal("expected recipes")
	}
	if recipes[0].Platform != "amazon" {
		t.Fatalf("recipe platform = %q, want amazon", recipes[0].Platform)
	}
	if recipes[0].Template == nil || recipes[0].Template.BundleSlot == "" {
		t.Fatalf("recipe template = %+v", recipes[0].Template)
	}
	if recipes[0].Template.TemplateLabel == "" {
		t.Fatalf("recipe template = %+v, want template label", recipes[0].Template)
	}
	if recipes[0].Template.RenderProfile == "" {
		t.Fatalf("recipe template = %+v, want render profile", recipes[0].Template)
	}
}
