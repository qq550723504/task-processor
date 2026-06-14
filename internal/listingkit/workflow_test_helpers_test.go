package listingkit

func seedWorkflowDepsFromMirrors(s *service) *service {
	if s == nil {
		return nil
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
	if s.supportDeps.sdsSyncService == nil {
		s.supportDeps.sdsSyncService = s.mirrors.sdsSyncSvc
	}
	if s.supportDeps.sdsBaselineRemoteProvider == nil {
		s.supportDeps.sdsBaselineRemoteProvider = s.mirrors.sdsBaselineRemoteProvider
	}
	if s.supportDeps.uploadedImageRepository == nil {
		s.supportDeps.uploadedImageRepository = s.mirrors.uploadedImageRepo
	}
	if s.supportDeps.assembler == nil {
		s.supportDeps.assembler = s.mirrors.assembler
	}
	if s.supportDeps.reviewRepository == nil {
		s.supportDeps.reviewRepository = s.mirrors.reviewRepo
	}
	if s.taskDeps.sdsLoginStatusProvider == nil {
		s.taskDeps.sdsLoginStatusProvider = s.mirrors.sdsLoginStatusProvider
	}
	return s
}

func seedWorkflowServices(s *service, productSvc ProductService, imageSvc ImageService) *service {
	if s == nil {
		return nil
	}
	if s.workflowDeps.productService == nil {
		s.workflowDeps.productService = productSvc
	}
	if s.workflowDeps.imageService == nil {
		s.workflowDeps.imageService = imageSvc
	}
	return s
}
