package listingkit

import (
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	"task-processor/internal/listingkit/reviewstore"
	sdsusecase "task-processor/internal/sds/usecase"
)

type serviceDependencyMirrors struct {
	productSvc                ProductService
	imageSvc                  ImageService
	sdsSyncSvc                sdsusecase.Service
	sdsLoginStatusProvider    SDSLoginStatusProvider
	sdsBaselineRemoteProvider SDSBaselineRemoteProvider
	uploadStore               ImageUploadStore
	uploadedImageRepo         UploadedImageRepository
	assembler                 Assembler

	assetRepo           assetrepo.Repository
	reviewRepo          reviewstore.Repository
	assetRecipeResolver assetrecipe.Resolver
	assetBundleBuilder  assetbundle.Builder
	assetGenerator      assetgeneration.Service
}
