package httpapi

import (
	"net/http"
	"os"

	"task-processor/internal/infra/worker"
	"task-processor/internal/listingkit"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	"task-processor/internal/sheinlogin"
)

type Options struct {
	ConfigPath     string
	Port           int
	ShutdownSignal chan os.Signal
}

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

type appBootstrap struct {
	productHandler productRouteHandler
	imageHandler   imageRouteHandler
	server         *http.Server
	routes         []routeDescriptor
	pools          []worker.WorkerPool
	closers        []func() error
}

type httpFeatureComposition struct {
	productModule       *productModuleResult
	imageModule         *imageModuleResult
	amazonListingModule *amazonListingModuleResult
	listingKitModule    *listingKitModuleResult
	promptModule        *promptModuleResult
	sdsModule           *sdsModuleResult
	taskRPCResult       *taskRPCModuleResult
	sheinLoginResult    *sheinLoginModuleResult
	sdsLoginResult      *sdsLoginModuleResult
}
