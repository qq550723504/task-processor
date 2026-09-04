package listingkit

type workflowDependencies struct {
	productSnapshots ProductSnapshotReader
	approvedAssets   ApprovedAssetInventoryReader
}

func resolveWorkflowApprovedAssets(s *service) ApprovedAssetInventoryReader {
	if s == nil {
		return nil
	}
	return s.workflowDeps.approvedAssets
}

func resolveWorkflowProductSnapshots(s *service) ProductSnapshotReader {
	if s == nil {
		return nil
	}
	return s.workflowDeps.productSnapshots
}

func resolveWorkflowSheinContentOptimizer(s *service) AIChatCompleter {
	return resolveSheinContentOptimizer(s)
}
