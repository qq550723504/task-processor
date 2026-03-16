// Package handler 提供通用 HTTP 处理器
package handler

import (
	"net/http"
	"time"

	"task-processor/internal/infra/http/response"
)

// HealthChecker 健康检查接口
type HealthChecker interface {
	IsHealthy() bool
	IsReady() bool
}

// HealthHandler 创建健康检查处理器
// 返回一个简单的健康检查处理器，总是返回 healthy
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		response.Success(w, "healthy", map[string]any{
			"timestamp": time.Now().Format(time.RFC3339),
		})
	}
}

// ReadyHandler 创建就绪检查处理器
// 参数:
//   - checker: 健康检查器，如果为 nil 则总是返回 ready
//
// 返回值:
//   - http.HandlerFunc: 就绪检查处理器
func ReadyHandler(checker HealthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if checker == nil || checker.IsReady() {
			response.Success(w, "ready", map[string]any{
				"timestamp": time.Now().Format(time.RFC3339),
			})
		} else {
			response.ServiceUnavailable(w, "服务未就绪")
		}
	}
}

// HealthAndReadyHandlers 创建健康检查和就绪检查处理器
// 参数:
//   - checker: 健康检查器
//
// 返回值:
//   - health: 健康检查处理器
//   - ready: 就绪检查处理器
func HealthAndReadyHandlers(checker HealthChecker) (health, ready http.HandlerFunc) {
	return HealthHandler(), ReadyHandler(checker)
}
