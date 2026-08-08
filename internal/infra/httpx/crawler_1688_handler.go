// Package httpx 提供 HTTP 处理器
package httpx

import (
	"net/http"
	"strconv"
	"strings"

	"task-processor/internal/crawler/shared"
	"task-processor/internal/shared/tenantctx"

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
		URL             string `json:"url"`
		OfferID         string `json:"offer_id,omitempty"`
		Priority        int    `json:"priority"`
		SourceAccountID int64  `json:"source_account_id,omitempty"`
	}

	if err := ParseJSON(r, &req); err != nil {
		BadRequest(w, err.Error())
		return
	}
	if req.SourceAccountID < 0 {
		BadRequest(w, "source_account_id must not be negative")
		return
	}
	var tenantID int64
	if req.SourceAccountID > 0 {
		var ok bool
		tenantID, ok = trustedCrawlerTenantID(r)
		if !ok {
			BadRequest(w, "trusted tenant context is required for source_account_id")
			return
		}
	}
	crawlerTask := shared.NewCrawlerTask(req.URL)
	crawlerTask.SourceAccountID = req.SourceAccountID
	crawlerTask.TenantID = tenantID
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

func trustedCrawlerTenantID(r *http.Request) (int64, bool) {
	if r == nil {
		return 0, false
	}
	tenantScope, ok := tenantctx.TenantScopeFromContext(r.Context())
	if !ok {
		return 0, false
	}
	tenantID, err := strconv.ParseInt(strings.TrimSpace(tenantScope), 10, 64)
	if err != nil || tenantID <= 0 {
		return 0, false
	}
	return tenantID, true
}
