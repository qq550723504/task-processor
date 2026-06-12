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
	if s.supportDeps.sdsSyncService != nil {
		s.sdsSyncSvc = s.supportDeps.sdsSyncService
		return s.supportDeps.sdsSyncService
	}
	s.supportDeps.sdsSyncService = s.sdsSyncSvc
	return s.sdsSyncSvc
}

func resolveSDSBaselineRemoteProvider(s *service) SDSBaselineRemoteProvider {
	if s == nil {
		return nil
	}
	if s.supportDeps.sdsBaselineRemoteProvider != nil {
		s.sdsBaselineRemoteProvider = s.supportDeps.sdsBaselineRemoteProvider
		return s.supportDeps.sdsBaselineRemoteProvider
	}
	s.supportDeps.sdsBaselineRemoteProvider = s.sdsBaselineRemoteProvider
	return s.sdsBaselineRemoteProvider
}

func resolveUploadedImageRepository(s *service) UploadedImageRepository {
	if s == nil {
		return nil
	}
	if s.supportDeps.uploadedImageRepository != nil {
		s.uploadedImageRepo = s.supportDeps.uploadedImageRepository
		return s.supportDeps.uploadedImageRepository
	}
	s.supportDeps.uploadedImageRepository = s.uploadedImageRepo
	return s.uploadedImageRepo
}

func resolveAssembler(s *service) Assembler {
	if s == nil {
		return nil
	}
	if s.supportDeps.assembler != nil {
		s.assembler = s.supportDeps.assembler
		return s.supportDeps.assembler
	}
	s.supportDeps.assembler = s.assembler
	return s.assembler
}

func resolveReviewRepository(s *service) reviewstore.Repository {
	if s == nil {
		return nil
	}
	if s.supportDeps.reviewRepository != nil {
		s.reviewRepo = s.supportDeps.reviewRepository
		return s.supportDeps.reviewRepository
	}
	s.supportDeps.reviewRepository = s.reviewRepo
	return s.reviewRepo
}
