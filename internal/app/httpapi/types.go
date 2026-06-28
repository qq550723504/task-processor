package httpapi

import (
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/productenrich"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	"task-processor/internal/productimage"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	"task-processor/internal/sheinlogin"
)

type runtimeDeps struct {
	shared   *sharedRuntimeDeps
	features *featureRuntimeState
}

type featureRuntimeState struct {
	productService         productenrich.ProductService
	imageService           productimage.Service
	sdsLoginStatusProvider listingkit.SDSLoginStatusProvider
	imageSubjectExtractor  productimage.SubjectExtractor
	imageWhiteBgRenderer   productimage.WhiteBackgroundRenderer
	imageSceneRenderer     productimage.SceneRenderer
	listingKitSupport      *listingKitSupport
}

type listingKitSupport struct {
	sdsBaselineRemoteProvider listingkit.SDSBaselineRemoteProvider
	sheinCookieStore          *sheinlogin.RedisStore
}

type httpFeatureComposition struct {
	productModule       *productenrichhttpapi.Module
	imageModule         *productimagehttpapi.Module
	amazonListingModule *amazonlistinghttpapi.Module
	listingKitModule    *listingkithttpapi.Module
	promptModule        *promptModuleResult
	sdsModule           *sdsModuleResult
	taskRPCResult       *taskRPCModuleResult
	sheinLoginResult    *sheinLoginModuleResult
	sdsLoginResult      *sdsLoginModuleResult
}
