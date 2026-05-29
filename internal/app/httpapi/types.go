package httpapi

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"task-processor/internal/amazonlisting"
	appbootstrap "task-processor/internal/app/bootstrap"
	"task-processor/internal/core/config"
	"task-processor/internal/httproute"
	openaiclient "task-processor/internal/infra/clients/openai"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingkit"
	listingkithttpapi "task-processor/internal/listingkit/httpapi"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	"task-processor/internal/prompt"
	promptmgmtapi "task-processor/internal/promptmgmt/api"
	sdshttpapi "task-processor/internal/sds/httpapi"
	sdsusecase "task-processor/internal/sds/usecase"
	"task-processor/internal/sheinlogin"
	"task-processor/internal/taskrpcapi"
)

type Options struct {
	ConfigPath     string
	Port           int
	ShutdownSignal chan os.Signal
}

type runtimeDeps struct {
	cfg                        *config.Config
	closers                    []func() error
	openaiMgr                  *openaiclient.Manager
	aiCredentialStore          *openaiclient.GormCredentialResolver
	tenantPromptStore          prompt.TenantPromptStore
	llmMgr                     productenrich.LLMManager
	inputParser                productenrich.InputParser
	understanding              productenrich.ProductUnderstanding
	imageWorkDir               string
	shared                     *appbootstrap.SharedResources
	productService             productenrich.ProductService
	imageService               productimage.Service
	sdsSyncService             sdsusecase.Service
	sdsLoginStatusProvider     listingkit.SDSLoginStatusProvider
	sdsBaselineRemoteProvider  listingkit.SDSBaselineRemoteProvider
	imageSubjectExtractor      productimage.SubjectExtractor
	imageWhiteBgRenderer       productimage.WhiteBackgroundRenderer
	imageSceneRenderer         productimage.SceneRenderer
	listingKitSheinCookieStore *sheinlogin.RedisStore
}

type appBootstrap struct {
	productHandler        productenrich.ProductHandler
	imageHandler          productimage.Handler
	amazonListingHandler  amazonlisting.Handler
	listingKitHandler     listingkithttpapi.RouteHandler
	promptTemplateHandler promptTemplateRouteHandler
	studioSessionHandler  listingkit.StudioSessionHandler
	sdsCatalogHandler     sdsCatalogRouteHandler
	sheinLoginHandler     sheinLoginRouteHandler
	sdsLoginHandler       sdsLoginRouteHandler
	taskRPCHandler        taskrpcapi.Handler
	server                *http.Server
	routes                []routeDescriptor
	pools                 []worker.WorkerPool
	closers               []func() error
}

type productModule struct {
	handler productenrich.ProductHandler
	pool    worker.WorkerPool
}

type imageModule struct {
	handler productimage.Handler
	pool    worker.WorkerPool
}

type amazonListingModule struct {
	handler amazonlisting.Handler
	pool    worker.WorkerPool
}

type listingKitModule struct {
	handler              listingkithttpapi.RouteHandler
	studioSessionHandler listingkit.StudioSessionHandler
	pool                 worker.WorkerPool
}

type productRouteHandler = productenrich.ProductHandler
type imageRouteHandler = productimage.Handler
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

type sdsLoginRouteHandler interface {
	Health(c *gin.Context)
	Status(c *gin.Context)
	Login(c *gin.Context)
	ManualLogin(c *gin.Context)
	GetAuthState(c *gin.Context)
	ClearState(c *gin.Context)
}

type routeDescriptor = httproute.Descriptor
