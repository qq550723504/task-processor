package httpapi

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"task-processor/internal/amazonlisting"
	amazonlistinghttpapi "task-processor/internal/amazonlisting/httpapi"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	"task-processor/internal/infra/worker"
	kernelmodule "task-processor/internal/kernel/module"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/productenrich"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	"task-processor/internal/productimage"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
	"task-processor/internal/sdslogin"
	sdsloginbootstrap "task-processor/internal/sdslogin/bootstrap"
	"task-processor/internal/sheinlogin"
	sheinloginbootstrap "task-processor/internal/sheinlogin/bootstrap"
	"task-processor/internal/taskrpcapi"
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
	productHandler productenrich.ProductHandler
	imageHandler   productimagehttpapi.RouteHandler
	server         *http.Server
	routes         []routeDescriptor
	pools          []worker.WorkerPool
	closers        []func() error
}

type httpFeatureComposition struct {
	productModule       *productenrichhttpapi.Module
	imageModule         *productimagehttpapi.Module
	amazonListingModule *amazonlistinghttpapi.Module
	listingKitModule    *listingkithttpapi.Module
	promptModule        *promptmgmtapi.BuildResult
	sdsModule           *sdshttpapi.BuildResult
	taskRPCResult       *taskrpcapi.BuildResult
	sheinLoginResult    *sheinloginbootstrap.BuildResult
	sdsLoginResult      *sdsloginbootstrap.BuildResult
}

func (c httpFeatureComposition) runtimeModules() []kernelmodule.Module {
	return []kernelmodule.Module{
		newCoreHTTPModule(),
		newProductHTTPModule(httpModuleHandlers{
			product: c.productHandler(),
			image:   c.imageHandler(),
		}, c.productModule, c.imageModule),
		newAmazonListingHTTPModule(httpModuleHandlers{
			amazonListing: c.amazonListingHandler(),
		}, c.amazonListingModule),
		newListingKitHTTPModule(httpModuleHandlers{
			listingKit: c.listingKitHandler(),
		}, c.listingKitModule),
		c.promptHTTPModule(),
		newListingKitStudioHTTPModule(httpModuleHandlers{
			studioSession: c.studioSessionHandler(),
		}, c.listingKitModule),
		c.sdsHTTPModule(),
		c.sheinLoginHTTPModule(),
		c.sdsLoginHTTPModule(),
	}
}

func (c httpFeatureComposition) routeModules() []kernelmodule.Module {
	modules := c.runtimeModules()
	modules = append(modules, c.taskRPCHTTPModule())
	return modules
}

func (c httpFeatureComposition) buildRuntimeBundle(cfg *config.Config) (runtimeBundle, error) {
	return buildRuntimeBundleFromModules(cfg, c.routeModules())
}

func (c httpFeatureComposition) buildServerBundle(port int, cfg *config.Config) (*http.Server, []routeDescriptor, error) {
	bundle, err := c.buildRuntimeBundle(cfg)
	if err != nil {
		return nil, nil, err
	}
	server, routes := bundle.buildServerBundle(port)
	return server, routes, nil
}

func (c httpFeatureComposition) productHandler() productenrich.ProductHandler {
	if c.productModule == nil {
		return nil
	}
	return c.productModule.Handler
}

func (c httpFeatureComposition) imageHandler() productimagehttpapi.RouteHandler {
	if c.imageModule == nil {
		return nil
	}
	return c.imageModule.Handler
}

func (c httpFeatureComposition) amazonListingHandler() amazonlisting.Handler {
	if c.amazonListingModule == nil {
		return nil
	}
	return c.amazonListingModule.Handler
}

func (c httpFeatureComposition) listingKitHandler() listingKitRouteHandler {
	if c.listingKitModule == nil {
		return nil
	}
	return c.listingKitModule.Handler
}

func (c httpFeatureComposition) studioSessionHandler() studioSessionRouteHandler {
	if c.listingKitModule == nil {
		return nil
	}
	return c.listingKitModule.StudioSessionHandler
}

func (c httpFeatureComposition) promptHTTPModule() kernelmodule.Module {
	if c.promptModule == nil {
		return nil
	}
	return c.promptModule.Module
}

func (c httpFeatureComposition) sdsHTTPModule() kernelmodule.Module {
	if c.sdsModule == nil {
		return nil
	}
	return c.sdsModule.Module
}

func (c httpFeatureComposition) taskRPCHTTPModule() kernelmodule.Module {
	if c.taskRPCResult == nil {
		return nil
	}
	return c.taskRPCResult.Module
}

func (c httpFeatureComposition) sheinLoginHTTPModule() kernelmodule.Module {
	if c.sheinLoginResult == nil {
		return nil
	}
	return c.sheinLoginResult.Module
}

func (c httpFeatureComposition) sdsLoginHTTPModule() kernelmodule.Module {
	if c.sdsLoginResult == nil {
		return nil
	}
	return c.sdsLoginResult.Module
}

type productRouteHandler = productenrich.ProductHandler
type imageRouteHandler = productimagehttpapi.RouteHandler
type amazonListingRouteHandler = amazonlisting.Handler
type listingKitRouteHandler = listingkithttpapi.RouteHandler
type studioSessionRouteHandler = listingkit.StudioSessionHandler
type taskRPCRouteHandler = taskrpcapi.Handler

type promptTemplateRouteHandler = promptmgmtapi.HTTPRouteHandler

type sdsCatalogRouteHandler = sdshttpapi.HTTPRouteHandler

type sheinLoginRouteHandler interface {
	Health(c *gin.Context)
	ListAccounts(c *gin.Context)
	Login(c *gin.Context)
	Status(c *gin.Context)
	ListWarehouses(c *gin.Context)
	SubmitVerifyCode(c *gin.Context)
	CancelVerifyCodeWait(c *gin.Context)
	ClearCookie(c *gin.Context)
	GetLastFailure(c *gin.Context)
	ClearLastFailure(c *gin.Context)
}

type sdsLoginRouteHandler = sdslogin.HTTPRouteHandler

type routeDescriptor = httproute.Descriptor
