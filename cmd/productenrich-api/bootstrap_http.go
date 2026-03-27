package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"task-processor/internal/core/config"
	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func buildHTTPServer(productHandler productenrich.ProductHandler, imageHandler productimage.Handler) *http.Server {
	router := gin.New()
	router.Use(gin.Recovery())
	registerRoutes(router, productHandler, imageHandler)
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", *port),
		Handler: router,
	}
}

func newWorkerPool(processor worker.Processor, cfg *config.Config) worker.WorkerPool {
	return worker.NewPoolWithConfig(processor, worker.PoolConfig{
		Concurrency:     cfg.Worker.Concurrency,
		BufferSize:      cfg.Worker.BufferSize,
		TaskTimeout:     15 * time.Minute,
		EnableMetrics:   true,
		ShutdownTimeout: 30 * time.Second,
	})
}

func registerRoutes(r *gin.Engine, productHandler productenrich.ProductHandler, imageHandler productimage.Handler) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if productHandler != nil {
		v1 := r.Group("/api/v1/products")
		{
			v1.POST("/generate", productHandler.GenerateProduct)
			v1.GET("/tasks/:task_id", productHandler.GetTaskResult)
		}
	}

	if imageHandler != nil {
		v1 := r.Group("/api/v1/images")
		{
			v1.POST("/process", imageHandler.ProcessImages)
			v1.GET("/tasks/:task_id", imageHandler.GetTaskResult)
			v1.POST("/tasks/:task_id/review", imageHandler.ReviewTask)
		}
	}
}
