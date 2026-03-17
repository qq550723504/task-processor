// Package httpx 提供 HTTP 处理器
package httpx

import (
	"net/http"

	"task-processor/internal/crawler/shared"

	"github.com/sirupsen/logrus"
)

// CrawlerHandler Amazon爬虫 HTTP 处理器
type CrawlerHandler struct {
	baseCrawlerHandler
}

// NewCrawlerHandler 创建Amazon爬虫处理器
func NewCrawlerHandler(crawlerService CrawlerService, logger *logrus.Logger) *CrawlerHandler {
	return &CrawlerHandler{
		baseCrawlerHandler: baseCrawlerHandler{
			crawlerService: crawlerService,
			logger:         logger,
		},
	}
}

// RegisterRoutes 注册路由
func (h *CrawlerHandler) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()
	return h.registerCommonRoutes(mux, h.handleCrawl)
}

// handleCrawl 处理Amazon爬虫请求
func (h *CrawlerHandler) handleCrawl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowed(w, "只支持 POST 方法")
		return
	}

	var req struct {
		URL      string `json:"url"`
		ASIN     string `json:"asin,omitempty"`
		Region   string `json:"region,omitempty"`
		Zipcode  string `json:"zipcode,omitempty"`
		Priority int    `json:"priority"`
	}

	if err := ParseJSON(r, &req); err != nil {
		BadRequest(w, err.Error())
		return
	}

	crawlerTask := shared.NewCrawlerTask(req.URL)
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

	if err := h.crawlerService.SubmitTask(crawlerTask); err != nil {
		ServiceUnavailable(w, err.Error())
		return
	}

	Success(w, "任务已提交", map[string]any{
		"task_id": crawlerTask.TaskID,
		"url":     crawlerTask.URL,
	})
}
