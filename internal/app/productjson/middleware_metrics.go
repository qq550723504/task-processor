// Package productjson 提供指标收集中间件
package productjson

import (
	"task-processor/internal/core/metrics"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsMiddleware 指标收集中间件
func MetricsMiddleware(collector metrics.MetricsCollector) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()
		c.Next()
		duration := time.Since(startTime)
		collector.RecordAPIRequest(c.Request.Method, c.FullPath(), c.Writer.Status(), duration)
	}
}
