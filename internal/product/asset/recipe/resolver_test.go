package recipe_test

import (
	"testing"

	"task-processor/internal/product/asset"
	assetrecipe "task-processor/internal/product/asset/recipe"
)

func TestBaseAssetRecipesExposeIndependentCommonInputs(t *testing.T) {
	t.Parallel()

	first := assetrecipe.BaseAssetRecipes()
	if len(first) != 3 || first[0].AssetKind != asset.KindCleanImage || first[1].AssetKind != asset.KindWhiteBgImage || first[2].AssetKind != asset.KindSubjectCutout {
		t.Fatalf("BaseAssetRecipes() = %+v, want clean, white-background, cutout", first)
	}
	first[0].Template.PreferredKinds[0] = asset.KindSceneImage
	second := assetrecipe.BaseAssetRecipes()
	if second[0].Template.PreferredKinds[0] != asset.KindCleanImage {
		t.Fatalf("BaseAssetRecipes() retained caller mutation: %+v", second[0])
	}
}

func TestStaticResolverResolvesPlatformRecipes(t *testing.T) {
	t.Parallel()

	recipes := assetrecipe.NewStaticResolver().Resolve(assetrecipe.ResolveRequest{
		Platform:     "amazon",
		CategoryPath: []string{"Home", "Storage"},
	})
	if len(recipes) != 4 {
		t.Fatalf("Resolve(amazon) returned %d recipes, want 4", len(recipes))
	}
	if recipes[0].ID != "amazon-main-white-bg" || recipes[0].Template == nil || recipes[0].Template.BundleSlot != "main" {
		t.Fatalf("first Amazon recipe = %+v, want white-background main", recipes[0])
	}
}

func TestResolveForPlatformsPassesDefensiveCategoryPathCopies(t *testing.T) {
	t.Parallel()

	resolver := &recordingResolver{}
	categoryPath := []string{"Home", "Storage"}
	got := assetrecipe.ResolveForPlatforms(resolver, []string{"amazon", "shein"}, categoryPath)
	if len(got) != 2 || len(resolver.requests) != 2 {
		t.Fatalf("ResolveForPlatforms() = %+v, requests = %+v", got, resolver.requests)
	}
	if categoryPath[0] != "Home" || resolver.requests[1].CategoryPath[0] != "Home" {
		t.Fatalf("category path was shared: input=%v requests=%v", categoryPath, resolver.requests)
	}
}

func TestFlattenResolvedIncludesEveryRecipe(t *testing.T) {
	t.Parallel()

	got := assetrecipe.FlattenResolved(map[string][]assetrecipe.AssetRecipe{
		"amazon": {{ID: "amazon-main"}},
		"shein":  {{ID: "shein-main"}, {ID: "shein-gallery"}},
	})
	if len(got) != 3 {
		t.Fatalf("FlattenResolved() returned %d recipes, want 3", len(got))
	}
}

type recordingResolver struct {
	requests []assetrecipe.ResolveRequest
}

func (r *recordingResolver) Resolve(req assetrecipe.ResolveRequest) []assetrecipe.AssetRecipe {
	r.requests = append(r.requests, assetrecipe.ResolveRequest{Platform: req.Platform, CategoryPath: append([]string(nil), req.CategoryPath...)})
	if len(req.CategoryPath) > 0 {
		req.CategoryPath[0] = "mutated"
	}
	return []assetrecipe.AssetRecipe{{ID: req.Platform + "-recipe", Platform: req.Platform}}
}
