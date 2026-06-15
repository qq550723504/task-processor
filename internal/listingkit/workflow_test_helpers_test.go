package listingkit

import (
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/listingkit/reviewstore"
	sdsusecase "task-processor/internal/sds/usecase"
)

type supportDependencySeed struct {
	sdsSyncService            sdsusecase.Service
	sdsBaselineRemoteProvider SDSBaselineRemoteProvider
	uploadedImageRepository   UploadedImageRepository
	assembler                 Assembler
	reviewRepository          reviewstore.Repository
}

type taskDependencySeed struct {
	sdsLoginStatusProvider SDSLoginStatusProvider
}

func seedWorkflowDeps(s *service) *service {
	return s
}

func seedSupportDeps(s *service, deps supportDependencySeed) *service {
	if s == nil {
		return nil
	}
	if s.supportDeps.sdsSyncService == nil {
		s.supportDeps.sdsSyncService = deps.sdsSyncService
	}
	if s.supportDeps.sdsBaselineRemoteProvider == nil {
		s.supportDeps.sdsBaselineRemoteProvider = deps.sdsBaselineRemoteProvider
	}
	if s.supportDeps.uploadedImageRepository == nil {
		s.supportDeps.uploadedImageRepository = deps.uploadedImageRepository
	}
	if s.supportDeps.assembler == nil {
		s.supportDeps.assembler = deps.assembler
	}
	if s.supportDeps.reviewRepository == nil {
		s.supportDeps.reviewRepository = deps.reviewRepository
	}
	return s
}

func seedTaskDeps(s *service, deps taskDependencySeed) *service {
	if s == nil {
		return nil
	}
	if s.taskDeps.sdsLoginStatusProvider == nil {
		s.taskDeps.sdsLoginStatusProvider = deps.sdsLoginStatusProvider
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

func seedWorkflowAssets(
	s *service,
	assetRepo assetrepo.Repository,
	assetRecipeResolver assetrecipe.Resolver,
	assetBundleBuilder assetbundle.Builder,
	assetGenerator assetgeneration.Service,
) *service {
	if s == nil {
		return nil
	}
	if s.workflowDeps.assetRepository == nil {
		s.workflowDeps.assetRepository = assetRepo
	}
	if s.workflowDeps.assetRecipeResolver == nil {
		s.workflowDeps.assetRecipeResolver = assetRecipeResolver
	}
	if s.workflowDeps.assetBundleBuilder == nil {
		s.workflowDeps.assetBundleBuilder = assetBundleBuilder
	}
	if s.workflowDeps.assetGenerationService == nil {
		s.workflowDeps.assetGenerationService = assetGenerator
	}
	return s
}
