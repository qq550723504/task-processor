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
	if s.workflowDeps.productService != nil {
		s.productSvc = s.workflowDeps.productService
		return s.workflowDeps.productService
	}
	s.workflowDeps.productService = s.productSvc
	return s.productSvc
}

func resolveWorkflowImageService(s *service) ImageService {
	if s == nil {
		return nil
	}
	if s.workflowDeps.imageService != nil {
		s.imageSvc = s.workflowDeps.imageService
		return s.workflowDeps.imageService
	}
	s.workflowDeps.imageService = s.imageSvc
	return s.imageSvc
}

func resolveWorkflowAssetRepository(s *service) assetrepo.Repository {
	if s == nil {
		return nil
	}
	if s.workflowDeps.assetRepository != nil {
		s.assetRepo = s.workflowDeps.assetRepository
		return s.workflowDeps.assetRepository
	}
	s.workflowDeps.assetRepository = s.assetRepo
	return s.assetRepo
}

func resolveWorkflowAssetRecipeResolver(s *service) assetrecipe.Resolver {
	if s == nil {
		return nil
	}
	if s.workflowDeps.assetRecipeResolver != nil {
		s.assetRecipeResolver = s.workflowDeps.assetRecipeResolver
		return s.workflowDeps.assetRecipeResolver
	}
	s.workflowDeps.assetRecipeResolver = s.assetRecipeResolver
	return s.assetRecipeResolver
}

func resolveWorkflowAssetBundleBuilder(s *service) assetbundle.Builder {
	if s == nil {
		return nil
	}
	if s.workflowDeps.assetBundleBuilder != nil {
		s.assetBundleBuilder = s.workflowDeps.assetBundleBuilder
		return s.workflowDeps.assetBundleBuilder
	}
	s.workflowDeps.assetBundleBuilder = s.assetBundleBuilder
	return s.assetBundleBuilder
}

func resolveWorkflowAssetGenerationService(s *service) assetgeneration.Service {
	if s == nil {
		return nil
	}
	if s.workflowDeps.assetGenerationService != nil {
		s.assetGenerator = s.workflowDeps.assetGenerationService
		return s.workflowDeps.assetGenerationService
	}
	s.workflowDeps.assetGenerationService = s.assetGenerator
	return s.assetGenerator
}

func resolveWorkflowSheinContentOptimizer(s *service) openaiclient.ChatCompleter {
	if s == nil {
		return nil
	}
	if s.workflowDeps.sheinContentOptimizer != nil {
		s.sheinContentOptimizer = s.workflowDeps.sheinContentOptimizer
		return s.workflowDeps.sheinContentOptimizer
	}
	s.workflowDeps.sheinContentOptimizer = s.sheinContentOptimizer
	return s.sheinContentOptimizer
}
