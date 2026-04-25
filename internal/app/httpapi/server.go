package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func buildHTTPServer(port int, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) *http.Server {
	router := gin.New()
	router.Use(gin.Recovery())
	RegisterRoutes(router, productHandler, imageHandler, amazonListingHandler, listingKitHandler, taskRPCHandler, sdsCatalogHandlers...)
	return &http.Server{
		Addr:              fmt.Sprintf(":%d", port),
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func RegisterRoutes(r *gin.Engine, productHandler productRouteHandler, imageHandler imageRouteHandler, amazonListingHandler amazonListingRouteHandler, listingKitHandler listingKitRouteHandler, taskRPCHandler taskRPCRouteHandler, sdsCatalogHandlers ...sdsCatalogRouteHandler) {
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
		v1.GET("/tasks", amazonListingHandler.ListTaskQueue)
		v1.GET("/tasks/:task_id", amazonListingHandler.GetTaskResult)
		v1.GET("/tasks/:task_id/workbench", amazonListingHandler.GetTaskWorkbench)
		v1.POST("/tasks/:task_id/review", amazonListingHandler.ReviewTask)
		v1.POST("/tasks/:task_id/submit", amazonListingHandler.SubmitTask)
	}

	if listingKitHandler != nil {
		v1 := r.Group("/api/v1/listing-kits")
		v1.POST("/generate", listingKitHandler.GenerateListingKit)
		v1.POST("/uploads/images", listingKitHandler.UploadListingKitImages)
		v1.GET("/uploads/files/*key", listingKitHandler.GetUploadedListingKitImage)
		v1.GET("/tasks", listingKitHandler.ListTasks)
		v1.GET("/tasks/:task_id", listingKitHandler.GetTaskResult)
		v1.GET("/tasks/:task_id/preview", listingKitHandler.GetTaskPreview)
		v1.GET("/tasks/:task_id/generation-tasks", listingKitHandler.GetTaskGenerationTasks)
		v1.GET("/tasks/:task_id/generation-queue", listingKitHandler.GetTaskGenerationQueue)
		v1.GET("/tasks/:task_id/generation-review-session", listingKitHandler.GetTaskGenerationReviewSession)
		v1.GET("/tasks/:task_id/generation-review-preview", listingKitHandler.GetTaskGenerationReviewPreview)
		v1.POST("/tasks/:task_id/generation-navigation/dispatch", listingKitHandler.DispatchTaskGenerationNavigation)
		v1.POST("/tasks/:task_id/generation-tasks/retry", listingKitHandler.RetryTaskGenerationTasks)
		v1.POST("/tasks/:task_id/generation-actions/execute", listingKitHandler.ExecuteTaskGenerationAction)
		v1.GET("/tasks/:task_id/revision-history", listingKitHandler.GetTaskRevisionHistory)
		v1.GET("/tasks/:task_id/revision-history/:revision_id", listingKitHandler.GetTaskRevisionHistoryDetail)
		v1.GET("/tasks/:task_id/export", listingKitHandler.GetTaskExport)
		v1.POST("/tasks/:task_id/revision", listingKitHandler.ApplyTaskRevision)
		v1.POST("/tasks/:task_id/revision/validate", listingKitHandler.ValidateTaskRevision)
		v1.POST("/tasks/:task_id/submit", listingKitHandler.SubmitTask)
		v1.DELETE("/tasks/:task_id/shein-resolution-cache", listingKitHandler.ClearSheinResolutionCache)
	}

	var sdsCatalogHandler sdsCatalogRouteHandler
	if len(sdsCatalogHandlers) > 0 {
		sdsCatalogHandler = sdsCatalogHandlers[0]
	}
	if sdsCatalogHandler != nil {
		v1 := r.Group("/api/v1/sds")
		v1.GET("/products", sdsCatalogHandler.ListSDSProducts)
		v1.GET("/products/:product_id", sdsCatalogHandler.GetSDSProduct)
		v1.GET("/categories", sdsCatalogHandler.ListSDSCategories)
		v1.GET("/shipment-areas", sdsCatalogHandler.ListSDSShipmentAreas)
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
