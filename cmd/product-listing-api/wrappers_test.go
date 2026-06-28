package main

import (
	"net/http"

	"github.com/gin-gonic/gin"

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
