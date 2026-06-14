package listingkit

import (
	assetbundle "task-processor/internal/asset/bundle"
	assetgeneration "task-processor/internal/asset/generation"
	assetrecipe "task-processor/internal/asset/recipe"
	assetrepo "task-processor/internal/asset/repository"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/listingkit/reviewstore"
	sheinpub "task-processor/internal/publishing/shein"
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

	sheinCategoryResolver      sheinpub.CategoryResolver
	sheinResolutionCacheStore  sheinpub.ResolutionCacheStore
	sheinStoreCatalog          SheinStoreCatalog
	sheinAPIClientFactory      SheinAPIClientFactory
	sheinAttributeResolver     sheinpub.AttributeResolver
	sheinSaleAttributeResolver sheinpub.SaleAttributeResolver
	sheinPricingPolicy         sheinpub.PricingPolicy
	sheinProductAPIBuilder     sheinpub.ProductAPIBuilder
	sheinImageAPIBuilder       sheinpub.ImageAPIBuilder
	sheinTranslateAPIBuilder   sheinpub.TranslateAPIBuilder
	sheinContentOptimizer      openaiclient.ChatCompleter

	aiCredentialStore   AIClientCredentialStore
	assetRepo           assetrepo.Repository
	reviewRepo          reviewstore.Repository
	assetRecipeResolver assetrecipe.Resolver
	assetBundleBuilder  assetbundle.Builder
	assetGenerator      assetgeneration.Service
	storeProfileRepo    StoreProfileRepository
}
