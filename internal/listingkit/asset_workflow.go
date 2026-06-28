package listingkit

import (
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog/canonical"
	listingplatform "task-processor/internal/listing/platform"
)

func resolveRecipesForPlatforms(resolver assetrecipe.Resolver, platforms []string, canonical *canonical.Product) map[string][]assetrecipe.AssetRecipe {
	return assetrecipe.ResolveForPlatforms(
		resolver,
		listingplatform.NormalizeSupportedPlatforms(platforms),
		categoryPathOrNil(canonical),
	)
}

func categoryPathOrNil(canonical *canonical.Product) []string {
	if canonical == nil {
		return nil
	}
	return append([]string(nil), canonical.CategoryPath...)
}
