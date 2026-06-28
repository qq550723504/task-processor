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

func TestResolveForPlatformsPassesDefensiveCategoryPathCopies(t *testing.T) {
	t.Parallel()

	resolver := &recordingResolver{}
	categoryPath := []string{"Home", "Storage"}
	recipesByPlatform := assetrecipe.ResolveForPlatforms(resolver, []string{"amazon", "shein"}, categoryPath)

	if len(recipesByPlatform) != 2 {
		t.Fatalf("ResolveForPlatforms() = %+v, want recipes for two platforms", recipesByPlatform)
	}
	if categoryPath[0] != "Home" {
		t.Fatalf("category path mutated = %+v, want original preserved", categoryPath)
	}
	if len(resolver.requests) != 2 {
		t.Fatalf("resolver requests = %+v, want one request per platform", resolver.requests)
	}
	if resolver.requests[1].CategoryPath[0] != "Home" {
		t.Fatalf("second request category path = %+v, want independent copy", resolver.requests[1].CategoryPath)
	}
}

func TestResolveForPlatformsReturnsNilWithoutResolver(t *testing.T) {
	t.Parallel()

	if recipes := assetrecipe.ResolveForPlatforms(nil, []string{"amazon"}, []string{"Home"}); recipes != nil {
		t.Fatalf("ResolveForPlatforms(nil) = %+v, want nil", recipes)
	}
}

func TestFlattenResolvedRecipes(t *testing.T) {
	t.Parallel()

	flattened := assetrecipe.FlattenResolved(map[string][]assetrecipe.AssetRecipe{
		"amazon": {
			{ID: "amazon-main"},
		},
		"shein": {
			{ID: "shein-main"},
			{ID: "shein-gallery"},
		},
	})

	if len(flattened) != 3 {
		t.Fatalf("FlattenResolved() = %+v, want three recipes", flattened)
	}
	ids := map[string]bool{}
	for _, recipe := range flattened {
		ids[recipe.ID] = true
	}
	for _, id := range []string{"amazon-main", "shein-main", "shein-gallery"} {
		if !ids[id] {
			t.Fatalf("FlattenResolved() = %+v, missing %s", flattened, id)
		}
	}
}

type recordingResolver struct {
	requests []assetrecipe.ResolveRequest
}

func (r *recordingResolver) Resolve(req assetrecipe.ResolveRequest) []assetrecipe.AssetRecipe {
	r.requests = append(r.requests, assetrecipe.ResolveRequest{
		Platform:     req.Platform,
		CategoryPath: append([]string(nil), req.CategoryPath...),
	})
	if len(req.CategoryPath) > 0 {
		req.CategoryPath[0] = "mutated"
	}
	return []assetrecipe.AssetRecipe{{ID: req.Platform + "-recipe", Platform: req.Platform}}
}
