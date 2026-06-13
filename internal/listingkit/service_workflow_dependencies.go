package listingkit

import (
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	openaiclient "task-processor/internal/infra/clients/openai"
)

type workflowDependencies struct {
	productService         ProductService
	imageService           ImageService
	assetRepository        assetrepo.Repository
	assetRecipeResolver    assetrecipe.Resolver
	assetBundleBuilder     assetbundle.Builder
	assetGenerationService assetgeneration.Service
	sheinContentOptimizer  openaiclient.ChatCompleter
}

func resolveWorkflowProductService(s *service) ProductService {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.workflowDeps.productService, &s.mirrors.productSvc)
}

func resolveWorkflowImageService(s *service) ImageService {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.workflowDeps.imageService, &s.mirrors.imageSvc)
}

func resolveWorkflowAssetRepository(s *service) assetrepo.Repository {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.workflowDeps.assetRepository, &s.mirrors.assetRepo)
}

func resolveWorkflowAssetRecipeResolver(s *service) assetrecipe.Resolver {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.workflowDeps.assetRecipeResolver, &s.mirrors.assetRecipeResolver)
}

func resolveWorkflowAssetBundleBuilder(s *service) assetbundle.Builder {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.workflowDeps.assetBundleBuilder, &s.mirrors.assetBundleBuilder)
}

func resolveWorkflowAssetGenerationService(s *service) assetgeneration.Service {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.workflowDeps.assetGenerationService, &s.mirrors.assetGenerator)
}

func resolveWorkflowSheinContentOptimizer(s *service) openaiclient.ChatCompleter {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.workflowDeps.sheinContentOptimizer, &s.mirrors.sheinContentOptimizer)
}
