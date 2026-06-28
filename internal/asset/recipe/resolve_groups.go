package recipe

func ResolveForPlatforms(resolver Resolver, platforms []string, categoryPath []string) map[string][]AssetRecipe {
	if resolver == nil {
		return nil
	}
	out := make(map[string][]AssetRecipe, len(platforms))
	for _, platform := range platforms {
		out[platform] = resolver.Resolve(ResolveRequest{
			Platform:     platform,
			CategoryPath: append([]string(nil), categoryPath...),
		})
	}
	return out
}

func FlattenResolved(recipesByPlatform map[string][]AssetRecipe) []AssetRecipe {
	if len(recipesByPlatform) == 0 {
		return nil
	}
	out := make([]AssetRecipe, 0, len(recipesByPlatform)*4)
	for _, items := range recipesByPlatform {
		out = append(out, items...)
	}
	return out
}
