package httpapi

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"

	worker "task-processor/internal/platform/workerpool"
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
