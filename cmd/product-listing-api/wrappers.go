package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"task-processor/internal/app/httpapi"
	"task-processor/internal/infra/worker"
	"task-processor/internal/productenrich"
	productenrichhttpapi "task-processor/internal/productenrich/httpapi"
	productimagehttpapi "task-processor/internal/productimage/httpapi"
)

func registerRoutes(r *gin.Engine, productHandler productenrich.ProductHandler, imageHandler productimagehttpapi.RouteHandler) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	for _, route := range productenrichhttpapi.AppendProductRouteDescriptors(nil, productHandler, imageHandler) {
		r.Handle(route.Method, route.Path, route.Handler)
	}
}

func buildHandlers(logger *logrus.Logger) (productenrich.ProductHandler, productimagehttpapi.RouteHandler, []worker.WorkerPool, []func() error, error) {
	return httpapi.BuildHandlers(logger, httpapi.Options{
		ConfigPath: *configPath,
		Port:       *port,
	})
}
