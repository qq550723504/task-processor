package httpapi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	"task-processor/internal/infra/worker"
)

type stubWorkerPool struct {
	stats   worker.QueueStats
	metrics *worker.Metrics
}

func (s stubWorkerPool) Start(context.Context) {}
func (s stubWorkerPool) Stop(context.Context)  {}
func (s stubWorkerPool) Submit(worker.WorkerJob) error {
	return nil
}
func (s stubWorkerPool) AvailableSlots() int {
	return s.stats.AvailableSlots
}
func (s stubWorkerPool) GetQueueStats() worker.QueueStats {
	return s.stats
}
func (s stubWorkerPool) SetJobHandler(worker.JobHandler) {}
func (s stubWorkerPool) GetMetrics() *worker.Metrics {
	return s.metrics
}

type stubSDSCatalogRouteHandler struct{}

func (stubSDSCatalogRouteHandler) ListSDSProducts(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (stubSDSCatalogRouteHandler) GetSDSProduct(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"id": c.Param("product_id")})
}

func (stubSDSCatalogRouteHandler) ListSDSCategories(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (stubSDSCatalogRouteHandler) ListSDSShipmentAreas(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

type stubStudioSessionHandler struct{}

func (stubStudioSessionHandler) ListStudioSessionGallery(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (stubStudioSessionHandler) ListStudioBatches(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"items": []any{}})
}

func (stubStudioSessionHandler) GetStudioBatch(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"batch_id": c.Param("batch_id")})
}

func (stubStudioSessionHandler) StartStudioBatchGeneration(c *gin.Context) {
	c.JSON(http.StatusAccepted, gin.H{"batch_id": c.Param("batch_id")})
}

func (stubStudioSessionHandler) RetryStudioBatchItems(c *gin.Context) {
	c.JSON(http.StatusAccepted, gin.H{"batch_id": c.Param("batch_id")})
}

func (stubStudioSessionHandler) ApproveStudioBatchDesigns(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"batch_id": c.Param("batch_id")})
}

func (stubStudioSessionHandler) CreateStudioBatchTasks(c *gin.Context) {
	c.JSON(http.StatusAccepted, gin.H{"batch_id": c.Param("batch_id")})
}

func (stubStudioSessionHandler) UpsertStudioBatch(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"saved": true})
}

func (stubStudioSessionHandler) DeleteStudioBatch(c *gin.Context) {
	c.Status(http.StatusNoContent)
}
