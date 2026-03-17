// Package httpx 提供 HTTP 处理器
package httpx

import (
	"net/http"

	"task-processor/internal/crawler/shared"

	"github.com/sirupsen/logrus"
)

// Crawler1688Handler 1688爬虫 HTTP 处理器
type Crawler1688Handler struct {
	baseCrawlerHandler
}

// NewCrawler1688Handler 创建处理器
func NewCrawler1688Handler(crawlerService CrawlerService, logger *logrus.Logger) *Crawler1688Handler {
	return &Crawler1688Handler{
		baseCrawlerHandler: baseCrawlerHandler{
			crawlerService: crawlerService,
			logger:         logger,
		},
	}
}

// RegisterRoutes 注册路由
func (h *Crawler1688Handler) RegisterRoutes() http.Handler {
	mux := http.NewServeMux()
	return h.registerCommonRoutes(mux, h.handleCrawl)
}

// handleCrawl 处理1688爬虫请求
func (h *Crawler1688Handler) handleCrawl(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowed(w, "只支持 POST 方法")
		return
	}

	var req struct {
		URL      string `json:"url"`
		OfferID  string `json:"offer_id,omitempty"`
		Priority int    `json:"priority"`
	}

	if err := ParseJSON(r, &req); err != nil {
		BadRequest(w, err.Error())
		return
	}

	crawlerTask := shared.NewCrawlerTask(req.URL)
	if req.OfferID != "" {
		crawlerTask.WithASIN(req.OfferID) // 复用ASIN字段存储OfferID
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
