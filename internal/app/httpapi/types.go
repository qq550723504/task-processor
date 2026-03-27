package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/amazonlisting"
	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

type Options struct {
	ConfigPath string
	Port       int
}

type runtimeDeps struct {
	cfg            *config.Config
	closers        []func() error
	llmMgr         productenrich.LLMManager
	inputParser    productenrich.InputParser
	understanding  productenrich.ProductUnderstanding
	imageWorkDir   string
	productService productenrich.ProductService
	imageService   productimage.Service
}

type appBootstrap struct {
	productHandler       productenrich.ProductHandler
	imageHandler         productimage.Handler
	amazonListingHandler amazonlisting.Handler
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
	GetTaskResult(c *gin.Context)
	GetTaskWorkbench(c *gin.Context)
	ReviewTask(c *gin.Context)
	SubmitTask(c *gin.Context)
}
