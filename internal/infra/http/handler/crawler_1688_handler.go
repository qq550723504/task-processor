// Package handler 提供 HTTP 处理器
package handler

import (
	"net/http"

	"task-processor/internal/domain/task"
	"task-processor/internal/infra/http/middleware"
	"task-processor/internal/infra/http/request"
	"task-processor/internal/infra/http/response"
	"task-processor/internal/infra/http/router"

	"github.com/sirupsen/logrus"
)

// Crawler1688Handler 1688爬虫 HTTP 处理器
type Crawler1688Handler struct {
	crawlerService CrawlerService
	logger         *logrus.Logger
}

// NewCrawler1688Handler 创建处理器
func NewCrawler1688Handler(crawlerService CrawlerService, logger *logrus.Logger) *Crawler1688Handler {
	return &Crawler1688Handler{
		crawlerService: crawlerService,
		logger:         logger,
	}
}

// RegisterRoutes 注册路由
func (h *Crawler1688Handler) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()

	// API 路由
	mux.HandleFunc("/api/v1/crawl", h.handleCrawl)
	mux.HandleFunc("/api/v1/tasks/", h.handleTask)
	mux.HandleFunc("/api/v1/tasks", h.handleTasks)
	mux.HandleFunc("/api/v1/stats", h.handleStats)

	// 健康检查路由（使用通用处理器）
	mux.HandleFunc("/health", HealthHandler())
	mux.HandleFunc("/ready", ReadyHandler(h.crawlerService))

	// 应用中间件
	httpHandler := middleware.CORSMiddleware()(mux)
	httpHandler = middleware.LoggingMiddleware(h.logger)(httpHandler)

	return httpHandler
}

// handleCrawl 处理爬虫请求
func (h *Crawler1688Handler) handleCrawl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w, "只支持 POST 方法")
		return
	}

	var req struct {
		URL      string `json:"url"`
		OfferID  string `json:"offer_id,omitempty"`
		Priority int    `json:"priority"`
	}

	// 使用通用的请求解析工具
	if err := request.ParseJSON(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	// 构造任务（领域模型）
	crawlerTask := task.NewCrawlerTask(req.URL)
	if req.OfferID != "" {
		crawlerTask.WithASIN(req.OfferID) // 复用ASIN字段存储OfferID
	}
	if req.Priority > 0 {
		crawlerTask.WithPriority(req.Priority)
	}

	// 提交任务
	if err := h.crawlerService.SubmitTask(crawlerTask); err != nil {
		response.ServiceUnavailable(w, err.Error())
		return
	}

	response.Success(w, "任务已提交", map[string]interface{}{
		"task_id": crawlerTask.TaskID,
		"url":     crawlerTask.URL,
	})
}

// handleTask 处理单个任务查询/删除
func (h *Crawler1688Handler) handleTask(w http.ResponseWriter, r *http.Request) {
	// 使用通用的路径参数提取工具
	taskID := router.ExtractPathParam(r, "/api/v1/tasks/")
	if taskID == "" {
		response.BadRequest(w, "任务 ID 不能为空")
		return
	}

	switch r.Method {
	case http.MethodGet:
		// 查询任务
		result, err := h.crawlerService.GetTask(taskID)
		if err != nil {
			response.NotFound(w, err.Error())
			return
		}
		response.Success(w, "查询成功", result)

	case http.MethodDelete:
		// 删除任务
		h.crawlerService.DeleteTask(taskID)
		response.Success(w, "任务已删除", nil)

	default:
		response.MethodNotAllowed(w, "不支持的方法")
	}
}

// handleTasks 处理所有任务查询
func (h *Crawler1688Handler) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w, "只支持 GET 方法")
		return
	}

	tasks := h.crawlerService.GetAllTasks()
	response.Success(w, "查询成功", map[string]interface{}{
		"total": len(tasks),
		"tasks": tasks,
	})
}

// handleStats 处理统计信息查询
func (h *Crawler1688Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w, "只支持 GET 方法")
		return
	}

	stats := h.crawlerService.GetStats()
	response.Success(w, "查询成功", stats)
}
