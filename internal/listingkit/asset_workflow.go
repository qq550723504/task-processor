package listingkit

import (
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog/canonical"
	listingplatform "task-processor/internal/listing/platform"
)

func resolveRecipesForPlatforms(resolver assetrecipe.Resolver, platforms []string, canonical *canonical.Product) map[string][]assetrecipe.AssetRecipe {
	if resolver == nil {
		return nil
	}
	out := make(map[string][]assetrecipe.AssetRecipe, len(platforms))
	for _, platform := range listingplatform.NormalizeSupportedPlatforms(platforms) {
		out[platform] = resolver.Resolve(assetrecipe.ResolveRequest{
			Platform:     platform,
			CategoryPath: categoryPathOrNil(canonical),
		})
	}
	return out
}

func flattenRecipes(recipesByPlatform map[string][]assetrecipe.AssetRecipe) []assetrecipe.AssetRecipe {
	if len(recipesByPlatform) == 0 {
		return nil
	}
	out := make([]assetrecipe.AssetRecipe, 0, len(recipesByPlatform)*4)
	for _, items := range recipesByPlatform {
		out = append(out, items...)
	}
	return out
}

func baselineGenerationRecipes() []assetrecipe.AssetRecipe {
	return assetrecipe.BaseAssetRecipes()
}

func categoryPathOrNil(canonical *canonical.Product) []string {
	if canonical == nil {
		return nil
	}
	return append([]string(nil), canonical.CategoryPath...)
}
