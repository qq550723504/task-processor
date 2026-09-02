package listingkit

import (
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
)

type workflowDependencies struct {
	productSnapshots       ProductSnapshotReader
	imageService           ImageService
	assetRepository        assetrepo.Repository
	assetRecipeResolver    assetrecipe.Resolver
	assetBundleBuilder     assetbundle.Builder
	assetGenerationService assetgeneration.Service
}

func resolveWorkflowProductSnapshots(s *service) ProductSnapshotReader {
	if s == nil {
		return nil
	}
	return s.workflowDeps.productSnapshots
}

func resolveWorkflowImageService(s *service) ImageService {
	if s == nil {
		return nil
	}
	return s.workflowDeps.imageService
}

func resolveWorkflowAssetRepository(s *service) assetrepo.Repository {
	if s == nil {
		return nil
	}
	return s.workflowDeps.assetRepository
}

func resolveWorkflowAssetRecipeResolver(s *service) assetrecipe.Resolver {
	if s == nil {
		return nil
	}
	return s.workflowDeps.assetRecipeResolver
}

func resolveWorkflowAssetBundleBuilder(s *service) assetbundle.Builder {
	if s == nil {
		return nil
	}
	return s.workflowDeps.assetBundleBuilder
}

func resolveWorkflowAssetGenerationService(s *service) assetgeneration.Service {
	if s == nil {
		return nil
	}
	return s.workflowDeps.assetGenerationService
}

func resolveWorkflowSheinContentOptimizer(s *service) AIChatCompleter {
	return resolveSheinContentOptimizer(s)
}
