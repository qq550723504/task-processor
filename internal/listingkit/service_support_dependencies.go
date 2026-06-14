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
	return s.supportDeps.sdsSyncService
}

func resolveSDSBaselineRemoteProvider(s *service) SDSBaselineRemoteProvider {
	if s == nil {
		return nil
	}
	return s.supportDeps.sdsBaselineRemoteProvider
}

func resolveUploadedImageRepository(s *service) UploadedImageRepository {
	if s == nil {
		return nil
	}
	return s.supportDeps.uploadedImageRepository
}

func resolveAssembler(s *service) Assembler {
	if s == nil {
		return nil
	}
	return s.supportDeps.assembler
}

func resolveReviewRepository(s *service) reviewstore.Repository {
	if s == nil {
		return nil
	}
	return s.supportDeps.reviewRepository
}
