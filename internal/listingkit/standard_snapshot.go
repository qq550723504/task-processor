package listingkit

import (
	productasset "task-processor/internal/product/asset"
	"task-processor/internal/product/catalog/canonical"
)

func buildStandardProductSnapshot(result *ListingKitResult) *StandardProductSnapshot {
	result = normalizeListingKitResultSemanticFields(result)
	if result == nil {
		return nil
	}
	return normalizeStandardProductSnapshotSemanticFields(&StandardProductSnapshot{
		CatalogProduct:         result.CatalogProduct,
		ApprovedAssetInventory: cloneApprovedAssetInventory(result.ApprovedAssetInventory),
		PodExecution:           clonePodExecutionSummary(result.PodExecution),
		SDSDesignResult:        result.SDSDesignResult,
		Summary:                cloneGenerationSummary(result.Summary),
		ChildTasks:             append([]ChildTaskState(nil), result.ChildTasks...),
		WorkflowStages:         append([]WorkflowStage(nil), result.WorkflowStages...),
		WorkflowIssues:         append([]WorkflowIssue(nil), result.WorkflowIssues...),
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
	result.ApprovedAssetInventory = cloneApprovedAssetInventory(snapshot.ApprovedAssetInventory)
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
	merged.ApprovedAssetInventory = cloneApprovedAssetInventory(standard.ApprovedAssetInventory)
	merged.CanonicalProduct = standard.CanonicalProduct
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

func cloneApprovedAssetInventory(input *productasset.ApprovedAssetInventory) *productasset.ApprovedAssetInventory {
	if input == nil {
		return nil
	}
	cloned := productasset.CloneApprovedAssetInventory(*input)
	return &cloned
}

func cloneGenerationSummary(summary *GenerationSummary) *GenerationSummary {
	if summary == nil {
		return nil
	}
	cloned := *summary
	cloned.Warnings = append([]string(nil), summary.Warnings...)
	return &cloned
}

func canonicalProductFromStandardSnapshot(snapshot *StandardProductSnapshot) *canonical.Product {
	if snapshot == nil || snapshot.CatalogProduct == nil {
		return nil
	}
	return canonicalProductFromApprovedAssets(*snapshot.CatalogProduct, snapshot.ApprovedAssetInventory)
}

func standardProductSnapshotEmpty(snapshot *StandardProductSnapshot) bool {
	if snapshot == nil {
		return true
	}
	return snapshot.CatalogProduct == nil &&
		snapshot.ApprovedAssetInventory == nil &&
		snapshot.PodExecution == nil &&
		snapshot.SDSDesignResult == nil &&
		snapshot.Summary == nil &&
		len(snapshot.ChildTasks) == 0 &&
		len(snapshot.WorkflowStages) == 0 &&
		len(snapshot.WorkflowIssues) == 0
}
