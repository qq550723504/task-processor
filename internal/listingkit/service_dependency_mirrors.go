package listingkit

import (
	"task-processor/internal/listingkit/reviewstore"
	sdsusecase "task-processor/internal/sds/usecase"
)

type serviceDependencyMirrors struct {
	sdsSyncSvc                sdsusecase.Service
	sdsLoginStatusProvider    SDSLoginStatusProvider
	sdsBaselineRemoteProvider SDSBaselineRemoteProvider
	uploadStore               ImageUploadStore
	uploadedImageRepo         UploadedImageRepository
	assembler                 Assembler

	reviewRepo          reviewstore.Repository
}
