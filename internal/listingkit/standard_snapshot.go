package listingkit

import (
	"task-processor/internal/asset"
	assetgeneration "task-processor/internal/asset/generation"
	"task-processor/internal/productimage"
)

func buildStandardProductSnapshot(result *ListingKitResult) *StandardProductSnapshot {
	result = normalizeListingKitResultSemanticFields(result)
	if result == nil {
		return nil
	}
	return normalizeStandardProductSnapshotSemanticFields(&StandardProductSnapshot{
		CatalogProduct:                  result.CatalogProduct,
		CanonicalProduct:                result.CanonicalProduct,
		AssetBundle:                     result.AssetBundle,
		AssetInventorySummary:           result.AssetInventorySummary,
		ImageAssets:                     result.ImageAssets,
		ImageAssetsByTarget:             cloneImageAssetsByTarget(result.ImageAssetsByTarget),
		AssetBundlesByTarget:            cloneAssetBundlesByTarget(result.AssetBundlesByTarget),
		AssetInventorySummariesByTarget: cloneAssetInventorySummariesByTarget(result.AssetInventorySummariesByTarget),
		PodExecution:                    clonePodExecutionSummary(result.PodExecution),
		SDSDesignResult:                 result.SDSDesignResult,
		Summary:                         cloneGenerationSummary(result.Summary),
		ChildTasks:                      append([]ChildTaskState(nil), result.ChildTasks...),
		WorkflowStages:                  append([]WorkflowStage(nil), result.WorkflowStages...),
		WorkflowIssues:                  append([]WorkflowIssue(nil), result.WorkflowIssues...),
	})
}

func applyStandardProductSnapshot(result *ListingKitResult, snapshot *StandardProductSnapshot) {
	result = normalizeListingKitResultSemanticFields(result)
	snapshot = normalizeStandardProductSnapshotSemanticFields(snapshot)
	if result == nil || snapshot == nil {
		return
	}
	result.StandardProductSnapshot = snapshot
	result.CatalogProduct = snapshot.CatalogProduct
	result.CanonicalProduct = snapshot.CanonicalProduct
	result.AssetBundle = snapshot.AssetBundle
	result.AssetInventorySummary = snapshot.AssetInventorySummary
	result.ImageAssets = snapshot.ImageAssets
	result.ImageAssetsByTarget = cloneImageAssetsByTarget(snapshot.ImageAssetsByTarget)
	result.AssetBundlesByTarget = cloneAssetBundlesByTarget(snapshot.AssetBundlesByTarget)
	result.AssetInventorySummariesByTarget = cloneAssetInventorySummariesByTarget(snapshot.AssetInventorySummariesByTarget)
	result.PodExecution = clonePodExecutionSummary(snapshot.PodExecution)
	result.SDSDesignResult = snapshot.SDSDesignResult
	result.ChildTasks = append([]ChildTaskState(nil), snapshot.ChildTasks...)
	result.WorkflowStages = append([]WorkflowStage(nil), snapshot.WorkflowStages...)
	result.WorkflowIssues = append([]WorkflowIssue(nil), snapshot.WorkflowIssues...)
	normalizeListingKitResultSemanticFields(result)
}

func mergeStandardProductLayerResult(existing, standard *ListingKitResult) *ListingKitResult {
	standard = normalizeListingKitResultSemanticFields(standard)
	if standard == nil {
		return normalizeListingKitResultSemanticFields(existing)
	}
	if existing == nil {
		return standard
	}
	merged := *normalizeListingKitResultSemanticFields(existing)
	merged.TaskID = standard.TaskID
	merged.Status = standard.Status
	merged.Platforms = append([]string(nil), standard.Platforms...)
	merged.Country = standard.Country
	merged.Language = standard.Language
	merged.PodExecution = clonePodExecutionSummary(standard.PodExecution)
	merged.StandardProductSnapshot = standard.StandardProductSnapshot
	merged.CatalogProduct = standard.CatalogProduct
	merged.AssetBundle = standard.AssetBundle
	merged.AssetInventorySummary = standard.AssetInventorySummary
	merged.AssetGenerationSummary = standard.AssetGenerationSummary
	merged.AssetGenerationTasks = append([]assetgeneration.Task(nil), standard.AssetGenerationTasks...)
	merged.AssetGenerationQueue = standard.AssetGenerationQueue
	merged.AssetGenerationOverview = standard.AssetGenerationOverview
	merged.CanonicalProduct = standard.CanonicalProduct
	merged.ImageAssets = standard.ImageAssets
	merged.ImageAssetsByTarget = cloneImageAssetsByTarget(standard.ImageAssetsByTarget)
	merged.AssetBundlesByTarget = cloneAssetBundlesByTarget(standard.AssetBundlesByTarget)
	merged.AssetInventorySummariesByTarget = cloneAssetInventorySummariesByTarget(standard.AssetInventorySummariesByTarget)
	if standard.SDSDesignResult != nil {
		merged.SDSDesignResult = standard.SDSDesignResult
	}
	merged.Summary = cloneGenerationSummary(standard.Summary)
	merged.ChildTasks = append([]ChildTaskState(nil), standard.ChildTasks...)
	merged.WorkflowStages = append([]WorkflowStage(nil), standard.WorkflowStages...)
	merged.WorkflowIssues = append([]WorkflowIssue(nil), standard.WorkflowIssues...)
	merged.CreatedAt = standard.CreatedAt
	merged.UpdatedAt = standard.UpdatedAt
	return normalizeListingKitResultSemanticFields(&merged)
}

func cloneImageAssetsByTarget(input map[string]*productimage.ImageProcessResult) map[string]*productimage.ImageProcessResult {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]*productimage.ImageProcessResult, len(input))
	for target, result := range input {
		out[target] = result
	}
	return out
}

func cloneAssetBundlesByTarget(input map[string]*asset.Bundle) map[string]*asset.Bundle {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]*asset.Bundle, len(input))
	for target, bundle := range input {
		out[target] = bundle
	}
	return out
}

func cloneAssetInventorySummariesByTarget(input map[string]*asset.InventorySummary) map[string]*asset.InventorySummary {
	if len(input) == 0 {
		return nil
	}
	out := make(map[string]*asset.InventorySummary, len(input))
	for target, summary := range input {
		out[target] = summary
	}
	return out
}

func cloneGenerationSummary(summary *GenerationSummary) *GenerationSummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	cloned.Warnings = append([]string(nil), summary.Warnings...)
	return &cloned
}

func standardProductSnapshotEmpty(snapshot *StandardProductSnapshot) bool {
	if snapshot == nil {
		return true
	}
	return snapshot.CatalogProduct == nil &&
		snapshot.CanonicalProduct == nil &&
		snapshot.AssetBundle == nil &&
		snapshot.AssetInventorySummary == nil &&
		snapshot.ImageAssets == nil &&
		len(snapshot.ImageAssetsByTarget) == 0 &&
		len(snapshot.AssetBundlesByTarget) == 0 &&
		len(snapshot.AssetInventorySummariesByTarget) == 0 &&
		snapshot.PodExecution == nil &&
		snapshot.SDSDesignResult == nil &&
		snapshot.Summary == nil &&
		len(snapshot.ChildTasks) == 0 &&
		len(snapshot.WorkflowStages) == 0 &&
		len(snapshot.WorkflowIssues) == 0
}
