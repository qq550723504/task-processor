// Package httpx 提供 HTTP 处理器
package httpx

import (
	"context"
	"encoding/json"
	"net/http"

	"task-processor/internal/crawler/shared"
	"task-processor/internal/model"

	"github.com/sirupsen/logrus"
)

type directProductCrawler interface {
	FetchProduct(ctx context.Context, url, asin, region, zipcode string) (*model.Product, string, error)
}

type classifiedCrawlerError interface {
	error
	ErrorType() string
	RetryableError() bool
}

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
	mux.HandleFunc("/api/v1/products/fetch", h.handleFetchProduct)
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

// handleFetchProduct 处理同步商品抓取请求。
func (h *CrawlerHandler) handleFetchProduct(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		MethodNotAllowed(w, "只支持 POST 方法")
		return
	}

	productCrawler, ok := h.crawlerService.(directProductCrawler)
	if !ok {
		InternalServerError(w, "crawler service does not support direct product fetching")
		return
	}

	var req struct {
		URL     string `json:"url"`
		ASIN    string `json:"asin,omitempty"`
		Region  string `json:"region,omitempty"`
		Zipcode string `json:"zipcode,omitempty"`
	}

	if err := ParseJSON(r, &req); err != nil {
		BadRequest(w, err.Error())
		return
	}
	if req.URL == "" && req.ASIN == "" {
		BadRequest(w, "url 或 asin 至少需要提供一个")
		return
	}

	product, resolvedURL, err := productCrawler.FetchProduct(r.Context(), req.URL, req.ASIN, req.Region, req.Zipcode)
	if err != nil {
		errorType := ""
		retryable := false
		if classified, ok := err.(classifiedCrawlerError); ok {
			errorType = classified.ErrorType()
			retryable = classified.RetryableError()
		}
		statusCode := http.StatusServiceUnavailable
		if errorType == "invalid_request" {
			statusCode = http.StatusBadRequest
		}
		if errorType == "product_not_found" {
			statusCode = http.StatusNotFound
		}
		if errorType == "system_busy" {
			statusCode = http.StatusTooManyRequests
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(JSON{
			Success: false,
			Error:   err.Error(),
			Code:    errorType,
			Data: map[string]any{
				"retryable": retryable,
			},
		})
		return
	}
	if product == nil {
		NotFound(w, "product not found")
		return
	}

	Success(w, "抓取成功", map[string]any{
		"url":     resolvedURL,
		"product": product,
	})
}
