package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"task-processor/internal/app/httpapi"
	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	"task-processor/internal/productimage"
)

func registerRoutes(r *gin.Engine, productHandler productenrich.ProductHandler, imageHandler productimage.Handler) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	for _, route := range productenrichhttpapi.AppendProductRouteDescriptors(nil, productHandler, imageHandler) {
		r.Handle(route.Method, route.Path, route.Handler)
	}
}

func buildHandlers(logger *logrus.Logger) (productenrich.ProductHandler, productimage.Handler, []worker.WorkerPool, []func() error, error) {
	return httpapi.BuildHandlers(logger, httpapi.Options{
		ConfigPath: *configPath,
		Port:       *port,
	})
}
