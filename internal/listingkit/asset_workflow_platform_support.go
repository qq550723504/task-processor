package listingkit

import (
	"task-processor/internal/asset"
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
)

func attachPlatformImageBundles(result *ListingKitResult, inventory *asset.Inventory, recipesByPlatform map[string][]assetrecipe.AssetRecipe, generationPlan *assetgeneration.Result, builder assetbundle.Builder) {
	if result == nil || inventory == nil || builder == nil {
		return
	}
	platforms := make([]string, 0, len(recipesByPlatform))
	for platform, recipes := range recipesByPlatform {
		platforms = append(platforms, platform)
		imageBundle := builder.Build(assetbundleRequest(platform, inventory, recipes))
		if len(platformGenerationTasks(platform, generationPlan)) > 0 {
			imageBundle.PendingGeneration = platformGenerationTasks(platform, generationPlan)
		}
		switch platform {
		case "amazon":
			if result.Amazon != nil {
				result.Amazon.ImageBundle = imageBundle
			}
		case "shein":
			if result.Shein != nil {
				result.Shein.ImageBundle = imageBundle
			}
		case "temu":
			if result.Temu != nil {
				result.Temu.ImageBundle = imageBundle
			}
		case "walmart":
			if result.Walmart != nil {
				result.Walmart.ImageBundle = imageBundle
			}
		}
	}
	if result.AssetInventorySummary != nil {
		result.AssetInventorySummary.Platforms = uniqueStrings(platforms)
	}
}

func platformGenerationTasks(platform string, plan *assetgeneration.Result) []assetgeneration.Task {
	if plan == nil || len(plan.Tasks) == 0 {
		return nil
	}
	out := make([]assetgeneration.Task, 0, len(plan.Tasks))
	for _, task := range plan.Tasks {
		if task.Platform == platform && task.ExecutionStatus != "completed" {
			out = append(out, cloneGenerationTask(task))
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func collectPlatformGenerationTasks(result *ListingKitResult) []assetgeneration.Task {
	if result == nil {
		return nil
	}
	out := make([]assetgeneration.Task, 0, 8)
	if result.Amazon != nil && result.Amazon.ImageBundle != nil {
		out = append(out, cloneGenerationTasks(result.Amazon.ImageBundle.PendingGeneration)...)
	}
	if result.Shein != nil && result.Shein.ImageBundle != nil {
		out = append(out, cloneGenerationTasks(result.Shein.ImageBundle.PendingGeneration)...)
	}
	if result.Temu != nil && result.Temu.ImageBundle != nil {
		out = append(out, cloneGenerationTasks(result.Temu.ImageBundle.PendingGeneration)...)
	}
	if result.Walmart != nil && result.Walmart.ImageBundle != nil {
		out = append(out, cloneGenerationTasks(result.Walmart.ImageBundle.PendingGeneration)...)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func assetbundleRequest(platform string, inventory *asset.Inventory, recipes []assetrecipe.AssetRecipe) assetbundle.BuildRequest {
	return assetbundle.BuildRequest{
		Platform:  platform,
		Inventory: inventory,
		Recipes:   append([]assetrecipe.AssetRecipe(nil), recipes...),
	}
}
