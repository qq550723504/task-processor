// Package productjson 提供指标收集中间件
package productjson

import (
	utils "task-processor/internal/core/metrics"
	"time"

	"github.com/gin-gonic/gin"
)

// MetricsMiddleware 指标收集中间件
func MetricsMiddleware(metrics utils.MetricsCollector) gin.HandlerFunc {
	return func(c *gin.Context) {
		startTime := time.Now()

		// 处理请求
		c.Next()

		// 记录指标
		duration := time.Since(startTime)
		method := c.Request.Method
		path := c.FullPath()
		statusCode := c.Writer.Status()

		metrics.RecordAPIRequest(method, path, statusCode, duration)
	}
}
