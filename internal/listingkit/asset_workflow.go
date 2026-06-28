package listingkit

import (
	assetgeneration "task-processor/internal/asset/generation"
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

func mergeGenerationTasks(existing []assetgeneration.Task, updates []assetgeneration.Task) []assetgeneration.Task {
	if len(existing) == 0 {
		return cloneGenerationTasks(updates)
	}
	byID := make(map[string]assetgeneration.Task, len(existing)+len(updates))
	for _, item := range existing {
		byID[item.ID] = cloneGenerationTask(item)
	}
	for _, item := range updates {
		byID[item.ID] = cloneGenerationTask(item)
	}
	out := make([]assetgeneration.Task, 0, len(byID))
	for _, item := range existing {
		out = append(out, cloneGenerationTask(byID[item.ID]))
		delete(byID, item.ID)
	}
	for _, item := range updates {
		if _, ok := byID[item.ID]; !ok {
			continue
		}
		out = append(out, cloneGenerationTask(byID[item.ID]))
		delete(byID, item.ID)
	}
	return out
}

func cloneGenerationTasks(tasks []assetgeneration.Task) []assetgeneration.Task {
	if len(tasks) == 0 {
		return nil
	}
	cloned := make([]assetgeneration.Task, 0, len(tasks))
	for _, task := range tasks {
		cloned = append(cloned, cloneGenerationTask(task))
	}
	return cloned
}

func cloneGenerationTask(task assetgeneration.Task) assetgeneration.Task {
	cloned := task
	cloned.Lineage = append([]string(nil), task.Lineage...)
	cloned.SourceAssetIDs = append([]string(nil), task.SourceAssetIDs...)
	cloned.Metadata = cloneGenerationTaskMetadata(task.Metadata)
	return cloned
}

func cloneGenerationTaskMetadata(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
