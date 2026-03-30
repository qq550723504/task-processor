// Package httpx 提供 HTTP 处理器
package httpx

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

// baseCrawlerHandler 爬虫处理器公共基础，包含所有平台共享的路由逻辑。
// 各平台处理器通过嵌入此结构体复用公共方法，只需实现自己的 handleCrawl。
type baseCrawlerHandler struct {
	crawlerService CrawlerService
	logger         *logrus.Logger
}

// registerCommonRoutes 注册公共路由（任务查询/删除、统计、健康检查）。
// crawlHandler 由各平台处理器提供，用于处理平台特定的爬取请求。
func (b *baseCrawlerHandler) registerCommonRoutes(mux *http.ServeMux, crawlHandler http.HandlerFunc) http.Handler {
	mux.HandleFunc("/api/v1/crawl", crawlHandler)
	mux.HandleFunc("/api/v1/tasks/", b.handleTask)
	mux.HandleFunc("/api/v1/tasks", b.handleTasks)
	mux.HandleFunc("/api/v1/stats", b.handleStats)
	mux.HandleFunc("/metrics", b.handleMetrics)
	mux.HandleFunc("/health", HealthHandler())
	mux.HandleFunc("/ready", ReadyHandler(b.crawlerService))

	httpHandler := CORSMiddleware()(mux)
	return LoggingMiddleware(b.logger)(httpHandler)
}

// handleTask 处理单个任务查询/删除
func (b *baseCrawlerHandler) handleTask(w http.ResponseWriter, r *http.Request) {
	taskID := ExtractPathParam(r, "/api/v1/tasks/")
	if taskID == "" {
		BadRequest(w, "任务 ID 不能为空")
		return
	}

	switch r.Method {
	case http.MethodGet:
		result, err := b.crawlerService.GetTask(taskID)
		if err != nil {
			NotFound(w, err.Error())
			return
		}
		Success(w, "查询成功", result)
	case http.MethodDelete:
		b.crawlerService.DeleteTask(taskID)
		Success(w, "任务已删除", nil)
	default:
		MethodNotAllowed(w, "不支持的方法")
	}
}

// handleTasks 处理所有任务查询
func (b *baseCrawlerHandler) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, "只支持 GET 方法")
		return
	}
	tasks := b.crawlerService.GetAllTasks()
	Success(w, "查询成功", map[string]any{
		"total": len(tasks),
		"tasks": tasks,
	})
}

// handleStats 处理统计信息查询
func (b *baseCrawlerHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, "只支持 GET 方法")
		return
	}
	Success(w, "查询成功", b.crawlerService.GetStats())
}

func (b *baseCrawlerHandler) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		MethodNotAllowed(w, "只支持 GET 方法")
		return
	}
	writeMetricsResponse(w, b.crawlerService.GetStats())
}
