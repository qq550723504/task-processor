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

// CrawlerHandler Amazon爬虫 HTTP 处理器
type CrawlerHandler struct {
	crawlerService CrawlerService
	logger         *logrus.Logger
}

// NewCrawlerHandler 创建Amazon爬虫处理器
func NewCrawlerHandler(crawlerService CrawlerService, logger *logrus.Logger) *CrawlerHandler {
	return &CrawlerHandler{
		crawlerService: crawlerService,
		logger:         logger,
	}
}

// RegisterRoutes 注册路由
func (h *CrawlerHandler) RegisterRoutes() http.Handler {
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
func (h *CrawlerHandler) handleCrawl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response.MethodNotAllowed(w, "只支持 POST 方法")
		return
	}

	var req struct {
		URL      string `json:"url"`
		ASIN     string `json:"asin,omitempty"`
		Region   string `json:"region,omitempty"`
		Zipcode  string `json:"zipcode,omitempty"`
		Priority int    `json:"priority"`
	}

	// 使用通用的请求解析工具
	if err := request.ParseJSON(r, &req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	// 构造任务（领域模型）
	crawlerTask := task.NewCrawlerTask(req.URL)
	if req.ASIN != "" {
		crawlerTask.WithASIN(req.ASIN)
	}
	if req.Region != "" {
		crawlerTask.WithRegion(req.Region)
	}
	if req.Zipcode != "" {
		crawlerTask.WithZipcode(req.Zipcode)
	}
	if req.Priority > 0 {
		crawlerTask.WithPriority(req.Priority)
	}

	// 提交任务（应用层会处理 URL 构造）
	if err := h.crawlerService.SubmitTask(crawlerTask); err != nil {
		response.ServiceUnavailable(w, err.Error())
		return
	}

	response.Success(w, "任务已提交", map[string]any{
		"task_id": crawlerTask.TaskID,
		"url":     crawlerTask.URL,
	})
}

// handleTask 处理单个任务查询/删除
func (h *CrawlerHandler) handleTask(w http.ResponseWriter, r *http.Request) {
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
func (h *CrawlerHandler) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w, "只支持 GET 方法")
		return
	}

	tasks := h.crawlerService.GetAllTasks()
	response.Success(w, "查询成功", map[string]any{
		"total": len(tasks),
		"tasks": tasks,
	})
}

// handleStats 处理统计信息查询
func (h *CrawlerHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		response.MethodNotAllowed(w, "只支持 GET 方法")
		return
	}

	stats := h.crawlerService.GetStats()
	response.Success(w, "查询成功", stats)
}
