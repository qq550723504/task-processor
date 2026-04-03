package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func buildHTTPServer(port int, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, taskRPCHandler taskRPCRouteHandler) *http.Server {
	router := gin.New()
	router.Use(gin.Recovery())
	RegisterRoutes(router, productHandler, imageHandler, amazonListingHandler, taskRPCHandler)
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func RegisterRoutes(r *gin.Engine, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, taskRPCHandler taskRPCRouteHandler) {
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	if productHandler != nil {
		v1 := r.Group("/api/v1/products")
		v1.POST("/generate", productHandler.GenerateProduct)
		v1.GET("/tasks/:task_id", productHandler.GetTaskResult)
	}

	if imageHandler != nil {
		v1 := r.Group("/api/v1/images")
		v1.POST("/process", imageHandler.ProcessImages)
		v1.GET("/tasks/:task_id", imageHandler.GetTaskResult)
		v1.POST("/tasks/:task_id/review", imageHandler.ReviewTask)
	}

	if amazonListingHandler != nil {
		v1 := r.Group("/api/v1/amazon/listings")
		v1.POST("/generate", amazonListingHandler.GenerateListing)
		v1.GET("/tasks/:task_id", amazonListingHandler.GetTaskResult)
		v1.GET("/tasks/:task_id/workbench", amazonListingHandler.GetTaskWorkbench)
		v1.POST("/tasks/:task_id/review", amazonListingHandler.ReviewTask)
		v1.POST("/tasks/:task_id/submit", amazonListingHandler.SubmitTask)
	}

	if taskRPCHandler != nil {
		v1 := r.Group("/api/v1/management/tasks")
		v1.GET("/health", taskRPCHandler.GetHealth)
		v1.GET("/:task_id/status", taskRPCHandler.GetTaskStatus)
		v1.POST("/:task_id/retry", taskRPCHandler.RetryTask)
		v1.POST("/:task_id/cancel", taskRPCHandler.CancelTask)
		v1.GET("/queue-stats", taskRPCHandler.GetQueueStats)
	}
}
