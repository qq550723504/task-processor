package main

import (
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"task-processor/internal/app/httpapi"
	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
	"task-processor/internal/productimage"
)

func registerRoutes(r *gin.Engine, productHandler productenrich.ProductHandler, imageHandler productimage.Handler) {
	httpapi.RegisterRoutes(r, productHandler, imageHandler, nil)
}

func buildHandlers(logger *logrus.Logger) (productenrich.ProductHandler, productimage.Handler, []worker.WorkerPool, []func() error, error) {
	return httpapi.BuildHandlers(logger, httpapi.Options{
		ConfigPath: *configPath,
		Port:       *port,
	})
}
