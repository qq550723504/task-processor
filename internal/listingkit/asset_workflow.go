package listingkit

import (
	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	"task-processor/internal/catalog/canonical"
	listingplatform "task-processor/internal/listing/platform"
)

func buildInventorySummaryFromBundle(bundle *asset.Bundle) *asset.InventorySummary {
	if bundle == nil {
		return nil
	}
	return asset.BuildInventory("", bundle).Summary
}

func rebuildInventorySummary(inventory *asset.Inventory) *asset.InventorySummary {
	if inventory == nil {
		return nil
	}
	summary := &asset.InventorySummary{
		TotalRecords: len(inventory.Records),
	}
	for _, record := range inventory.Records {
		switch record.Origin {
		case asset.OriginSource:
			summary.SourceRecords++
		case asset.OriginGenerated:
			summary.GeneratedRecords++
		default:
			summary.DerivedRecords++
		}
		if record.RecipeID != "" {
			summary.RecipeCount++
		}
	}
	return summary
}

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

func rebuildBundleWithGeneratedAssets(bundle *asset.Bundle, records []asset.AssetRecord) *asset.Bundle {
	if bundle == nil {
		bundle = &asset.Bundle{}
	}
	out := &asset.Bundle{
		Assets:     append([]asset.Asset(nil), bundle.Assets...),
		Selection:  bundle.Selection,
		Stats:      bundle.Stats,
		Review:     bundle.Review,
		Compliance: bundle.Compliance,
		Quality:    bundle.Quality,
		IPRisk:     bundle.IPRisk,
	}
	for _, record := range records {
		out.Assets = append(out.Assets, asset.Asset{
			ID:             record.ID,
			Kind:           record.Kind,
			URL:            record.URL,
			Role:           record.Role,
			Generator:      record.Generator,
			RecipeID:       record.RecipeID,
			SourceAssetIDs: sourceAssetIDsFromLineage(record.Lineage),
			Operations:     append([]string(nil), record.Operations...),
			Labels:         append([]string(nil), record.Labels...),
			PlatformTags:   append([]string(nil), record.PlatformTags...),
			Width:          record.Width,
			Height:         record.Height,
			Metadata:       cloneRecordMetadata(record.Metadata),
		})
	}
	out.Stats = rebuildBundleStats(out.Assets)
	return out
}

func rebuildBundleFromInventory(bundle *asset.Bundle, inventory *asset.Inventory) *asset.Bundle {
	if inventory == nil {
		return bundle
	}
	out := &asset.Bundle{}
	if bundle != nil {
		out.Selection = bundle.Selection
		out.Review = bundle.Review
		out.Compliance = bundle.Compliance
		out.Quality = bundle.Quality
		out.IPRisk = bundle.IPRisk
	}
	out.Assets = make([]asset.Asset, 0, len(inventory.Records))
	for _, record := range inventory.Records {
		item := asset.Asset{
			ID:             record.ID,
			Kind:           record.Kind,
			URL:            record.URL,
			Role:           record.Role,
			Generator:      record.Generator,
			RecipeID:       record.RecipeID,
			SourceAssetIDs: sourceAssetIDsFromLineage(record.Lineage),
			Operations:     append([]string(nil), record.Operations...),
			Labels:         append([]string(nil), record.Labels...),
			PlatformTags:   append([]string(nil), record.PlatformTags...),
			Width:          record.Width,
			Height:         record.Height,
			Metadata:       cloneRecordMetadata(record.Metadata),
		}
		if item.Metadata != nil {
			item.SourceURL = item.Metadata["source_url"]
		}
		out.Assets = append(out.Assets, item)
	}
	out.Stats = rebuildBundleStats(out.Assets)
	return out
}

func sourceAssetIDsFromLineage(lineage *asset.AssetLineage) []string {
	if lineage == nil {
		return nil
	}
	return append([]string(nil), lineage.SourceAssetIDs...)
}

func cloneRecordMetadata(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func rebuildBundleStats(items []asset.Asset) *asset.Stats {
	stats := &asset.Stats{TotalAssets: len(items)}
	for _, item := range items {
		switch {
		case item.Kind == asset.KindSourceImage:
			stats.SourceAssets++
		case item.Kind == asset.KindCleanImage || item.Kind == asset.KindDetailCrop || item.Kind == asset.KindSceneImage || item.Kind == asset.KindSellingPointImage || item.Kind == asset.KindSizeSceneImage || item.Kind == asset.KindModelImage:
			stats.GeneratedAssets++
		default:
			stats.DerivedAssets++
		}
	}
	return stats
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
	cloned.Metadata = cloneRecordMetadata(task.Metadata)
	return cloned
}
