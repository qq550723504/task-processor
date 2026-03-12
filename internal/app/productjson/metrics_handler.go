// Package productjson 提供 Prometheus 指标导出 handler
package productjson

import (
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// MetricsHandler Prometheus 指标导出处理器
type MetricsHandler struct {
	handler gin.HandlerFunc
}

// NewMetricsHandler 创建 Prometheus 指标导出处理器
func NewMetricsHandler() *MetricsHandler {
	// 使用 Prometheus 的 HTTP handler
	promHandler := promhttp.Handler()

	// 包装为 Gin handler
	ginHandler := func(c *gin.Context) {
		promHandler.ServeHTTP(c.Writer, c.Request)
	}

	return &MetricsHandler{
		handler: ginHandler,
	}
}

// ServeMetrics 处理指标导出请求
// @Summary Prometheus 指标
// @Description 导出 Prometheus 格式的指标数据
// @Tags monitoring
// @Produce plain
// @Success 200 {string} string "Prometheus metrics"
// @Router /metrics [get]
func (h *MetricsHandler) ServeMetrics(c *gin.Context) {
	h.handler(c)
}
