package httpapi

import (
	"net/http"
	"os"

	"github.com/gin-gonic/gin"

	"task-processor/internal/amazonlisting"
	appbootstrap "task-processor/internal/app/bootstrap"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/clients/management"
	"task-processor/internal/infra/worker"
	"task-processor/internal/listingkit"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
	"task-processor/internal/taskrpcapi"
)

type Options struct {
	ConfigPath     string
	Port           int
	ShutdownSignal chan os.Signal
}

type runtimeDeps struct {
	cfg              *config.Config
	closers          []func() error
	llmMgr           productenrich.LLMManager
	inputParser      productenrich.InputParser
	understanding    productenrich.ProductUnderstanding
	imageWorkDir     string
	shared           *appbootstrap.SharedResources
	managementClient *management.ClientManager
	productService   productenrich.ProductService
	imageService     productimage.Service
}

type appBootstrap struct {
	productHandler       productenrich.ProductHandler
	imageHandler         productimage.Handler
	amazonListingHandler amazonlisting.Handler
	listingKitHandler    listingkit.Handler
	taskRPCHandler       taskrpcapi.Handler
	server               *http.Server
	pools                []worker.WorkerPool
	closers              []func() error
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
	handler listingkit.Handler
	pool    worker.WorkerPool
}

type productRouteHandler interface {
	GenerateProduct(c *gin.Context)
	GetTaskResult(c *gin.Context)
}

type imageRouteHandler interface {
	ProcessImages(c *gin.Context)
	GetTaskResult(c *gin.Context)
	ReviewTask(c *gin.Context)
}

type amazonListingRouteHandler interface {
	GenerateListing(c *gin.Context)
	ListTaskQueue(c *gin.Context)
	GetTaskResult(c *gin.Context)
	GetTaskWorkbench(c *gin.Context)
	ReviewTask(c *gin.Context)
	SubmitTask(c *gin.Context)
}

type listingKitRouteHandler interface {
	GenerateListingKit(c *gin.Context)
	GetTaskResult(c *gin.Context)
	GetTaskPreview(c *gin.Context)
	GetTaskRevisionHistory(c *gin.Context)
	GetTaskRevisionHistoryDetail(c *gin.Context)
	GetTaskExport(c *gin.Context)
	ApplyTaskRevision(c *gin.Context)
	ValidateTaskRevision(c *gin.Context)
}

type taskRPCRouteHandler interface {
	GetTaskStatus(c *gin.Context)
	RetryTask(c *gin.Context)
	CancelTask(c *gin.Context)
	GetQueueStats(c *gin.Context)
	GetHealth(c *gin.Context)
}
