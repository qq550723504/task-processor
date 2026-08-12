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
		targetInventory := platformAssetInventory(result, platform, inventory)
		imageBundle := builder.Build(assetbundleRequest(platform, targetInventory, recipes))
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
	for platform, summary := range result.AssetInventorySummariesByTarget {
		if summary != nil {
			summary.Platforms = uniqueStrings([]string{platform})
		}
	}
}

func platformAssetInventory(result *ListingKitResult, platform string, shared *asset.Inventory) *asset.Inventory {
	if result == nil || shared == nil || len(result.AssetBundlesByTarget) == 0 {
		return shared
	}
	targetBundle := result.AssetBundleForTarget(platform)
	if targetBundle == nil {
		return nil
	}
	targetInventory := asset.BuildInventory(shared.Ref.TaskID, targetBundle)
	if targetInventory == nil {
		return nil
	}
	baseRecords := targetBaseAssetRecordKeys(result)
	for _, record := range shared.Records {
		if _, isTargetBaseRecord := baseRecords[assetRecordKey(record.ID, record.Kind)]; isTargetBaseRecord {
			continue
		}
		if len(record.PlatformTags) > 0 && !assetRecordTargetsPlatform(record, platform) {
			continue
		}
		targetInventory.Records = append(targetInventory.Records, record)
	}
	targetInventory.Summary = asset.RebuildInventorySummary(targetInventory)
	return targetInventory
}

func targetBaseAssetRecordKeys(result *ListingKitResult) map[string]struct{} {
	keys := map[string]struct{}{}
	if result == nil {
		return keys
	}
	for _, bundle := range result.AssetBundlesByTarget {
		if bundle == nil {
			continue
		}
		for _, item := range bundle.Assets {
			keys[assetRecordKey(item.ID, item.Kind)] = struct{}{}
		}
	}
	return keys
}

func assetRecordKey(id string, kind asset.Kind) string {
	return id + "\x00" + string(kind)
}

func assetRecordTargetsPlatform(record asset.AssetRecord, platform string) bool {
	for _, tag := range record.PlatformTags {
		if tag == platform {
			return true
		}
	}
	return false
}

func platformGenerationTasks(platform string, plan *assetgeneration.Result) []assetgeneration.Task {
	if plan == nil || len(plan.Tasks) == 0 {
		return nil
	}
	out := make([]assetgeneration.Task, 0, len(plan.Tasks))
	for _, task := range plan.Tasks {
		if task.Platform == platform && task.ExecutionStatus != "completed" {
			out = append(out, assetgeneration.CloneTask(task))
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
		out = append(out, assetgeneration.CloneTasks(result.Amazon.ImageBundle.PendingGeneration)...)
	}
	if result.Shein != nil && result.Shein.ImageBundle != nil {
		out = append(out, assetgeneration.CloneTasks(result.Shein.ImageBundle.PendingGeneration)...)
	}
	if result.Temu != nil && result.Temu.ImageBundle != nil {
		out = append(out, assetgeneration.CloneTasks(result.Temu.ImageBundle.PendingGeneration)...)
	}
	if result.Walmart != nil && result.Walmart.ImageBundle != nil {
		out = append(out, assetgeneration.CloneTasks(result.Walmart.ImageBundle.PendingGeneration)...)
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
