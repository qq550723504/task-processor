// Package httpx 提供 HTTP 处理器
package httpx

import (
	"context"
	"net/http"

	"task-processor/internal/crawler/shared"

	"github.com/sirupsen/logrus"
)

// Crawler1688Handler 1688爬虫 HTTP 处理器
type Crawler1688Handler struct {
	baseCrawlerHandler
	tenantResolver TenantResolver
}

// TenantResolver resolves a tenant from caller-provided trusted request context.
type TenantResolver func(context.Context) (int64, bool)

// NewCrawler1688Handler 创建处理器
func NewCrawler1688Handler(crawlerService CrawlerService, logger *logrus.Logger, tenantResolvers ...TenantResolver) *Crawler1688Handler {
	var tenantResolver TenantResolver
	if len(tenantResolvers) > 0 {
		tenantResolver = tenantResolvers[0]
	}
	return &Crawler1688Handler{
		baseCrawlerHandler: baseCrawlerHandler{
			crawlerService: crawlerService,
			logger:         logger,
			tenantResolver: tenantResolver,
		},
		tenantResolver: tenantResolver,
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
	if req.SourceAccountID > 0 || h.tenantResolver != nil {
		var ok bool
		tenantID, ok = h.resolveTenant(r.Context())
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

func (h *Crawler1688Handler) resolveTenant(ctx context.Context) (int64, bool) {
	if h.tenantResolver == nil {
		return 0, false
	}
	tenantID, ok := h.tenantResolver(ctx)
	if !ok || tenantID <= 0 {
		return 0, false
	}
	return tenantID, true
}
