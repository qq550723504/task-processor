package listingkit

func buildStandardProductSnapshot(result *ListingKitResult) *StandardProductSnapshot {
	if result == nil {
		return nil
	}
	return &StandardProductSnapshot{
		CatalogProduct:        result.CatalogProduct,
		CanonicalProduct:      result.CanonicalProduct,
		AssetBundle:           result.AssetBundle,
		AssetInventorySummary: result.AssetInventorySummary,
		ImageAssets:           result.ImageAssets,
		SDSSync:               result.SDSSync,
		Summary:               cloneGenerationSummary(result.Summary),
		ChildTasks:            append([]ChildTaskState(nil), result.ChildTasks...),
		WorkflowStages:        append([]WorkflowStage(nil), result.WorkflowStages...),
		WorkflowIssues:        append([]WorkflowIssue(nil), result.WorkflowIssues...),
	}
}

func applyStandardProductSnapshot(result *ListingKitResult, snapshot *StandardProductSnapshot) {
	if result == nil || snapshot == nil {
		return
	}
	result.StandardProductSnapshot = snapshot
	result.CatalogProduct = snapshot.CatalogProduct
	result.CanonicalProduct = snapshot.CanonicalProduct
	result.AssetBundle = snapshot.AssetBundle
	result.AssetInventorySummary = snapshot.AssetInventorySummary
	result.ImageAssets = snapshot.ImageAssets
	result.SDSSync = snapshot.SDSSync
	result.ChildTasks = append([]ChildTaskState(nil), snapshot.ChildTasks...)
	result.WorkflowStages = append([]WorkflowStage(nil), snapshot.WorkflowStages...)
	result.WorkflowIssues = append([]WorkflowIssue(nil), snapshot.WorkflowIssues...)
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
		snapshot.SDSSync == nil &&
		snapshot.Summary == nil &&
		len(snapshot.ChildTasks) == 0 &&
		len(snapshot.WorkflowStages) == 0 &&
		len(snapshot.WorkflowIssues) == 0
}
