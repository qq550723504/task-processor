package listingkit

import (
	"task-processor/internal/listingkit/reviewstore"
	sdsusecase "task-processor/internal/sds/usecase"
)

type supportDependencies struct {
	sdsSyncService            sdsusecase.Service
	sdsBaselineRemoteProvider SDSBaselineRemoteProvider
	uploadedImageRepository   UploadedImageRepository
	assembler                 Assembler
	reviewRepository          reviewstore.Repository
}

func resolveSDSSyncService(s *service) sdsusecase.Service {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.supportDeps.sdsSyncService, &s.mirrors.sdsSyncSvc)
}

func resolveSDSBaselineRemoteProvider(s *service) SDSBaselineRemoteProvider {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.supportDeps.sdsBaselineRemoteProvider, &s.mirrors.sdsBaselineRemoteProvider)
}

func resolveUploadedImageRepository(s *service) UploadedImageRepository {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.supportDeps.uploadedImageRepository, &s.mirrors.uploadedImageRepo)
}

func resolveAssembler(s *service) Assembler {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.supportDeps.assembler, &s.mirrors.assembler)
}

func resolveReviewRepository(s *service) reviewstore.Repository {
	if s == nil {
		return nil
	}
	return syncGroupedDependency(&s.supportDeps.reviewRepository, &s.mirrors.reviewRepo)
}
