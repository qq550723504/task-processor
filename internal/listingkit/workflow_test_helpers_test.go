package listingkit

func seedWorkflowDepsFromMirrors(s *service) *service {
	if s == nil {
		return nil
	}
	if s.workflowDeps.productService == nil {
		s.workflowDeps.productService = s.mirrors.productSvc
	}
	if s.workflowDeps.imageService == nil {
		s.workflowDeps.imageService = s.mirrors.imageSvc
	}
	if s.workflowDeps.assetRepository == nil {
		s.workflowDeps.assetRepository = s.mirrors.assetRepo
	}
	if s.workflowDeps.assetRecipeResolver == nil {
		s.workflowDeps.assetRecipeResolver = s.mirrors.assetRecipeResolver
	}
	if s.workflowDeps.assetBundleBuilder == nil {
		s.workflowDeps.assetBundleBuilder = s.mirrors.assetBundleBuilder
	}
	if s.workflowDeps.assetGenerationService == nil {
		s.workflowDeps.assetGenerationService = s.mirrors.assetGenerator
	}
	return s
}
