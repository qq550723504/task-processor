package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"task-processor/internal/core/config"
	coreLogger "task-processor/internal/core/logger"
	"task-processor/internal/model"
	domainProduct "task-processor/internal/product"
	"task-processor/internal/product/sourcing"

	"github.com/sirupsen/logrus"
)

type remoteFetchRequest struct {
	URL     string `json:"url,omitempty"`
	ASIN    string `json:"asin,omitempty"`
	Region  string `json:"region,omitempty"`
	Zipcode string `json:"zipcode,omitempty"`
}

type remoteFetchResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    struct {
		URL     string         `json:"url"`
		Product *model.Product `json:"product"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
}

type remoteErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
	Data    struct {
		Retryable bool `json:"retryable"`
	} `json:"data"`
}

type remoteAsyncSubmitResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    struct {
		TaskID string `json:"task_id"`
		URL    string `json:"url"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
	Code  string `json:"code,omitempty"`
}

type remoteTaskResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    struct {
		TaskID      string         `json:"TaskID"`
		Status      string         `json:"Status"`
		ProductData map[string]any `json:"ProductData"`
		Error       string         `json:"Error"`
	} `json:"data"`
	Error string `json:"error,omitempty"`
	Code  string `json:"code,omitempty"`
}

type RemoteAPIProductFetcher struct {
	cacheManager *domainProduct.CacheManager
	cacheEnabled bool
	amazonConfig *config.AmazonConfig
	client       *http.Client
	baseURL      string
	logger       *logrus.Entry
}

func newRemoteAPIProductFetcher(
	rawJsonDataClient domainProduct.RawJsonDataClient,
	amazonConfig *config.AmazonConfig,
) (*RemoteAPIProductFetcher, error) {
	if amazonConfig == nil {
		return nil, fmt.Errorf("amazon config is required")
	}
	if !amazonConfig.RemoteAPI.Enabled {
		return nil, fmt.Errorf("amazon remote API is not enabled")
	}
	if strings.TrimSpace(amazonConfig.RemoteAPI.BaseURL) == "" {
		return nil, fmt.Errorf("amazon remote API baseURL is empty")
	}

	logger := coreLogger.GetGlobalLogger("RemoteAPIProductFetcher")
	return &RemoteAPIProductFetcher{
		cacheManager: domainProduct.NewCacheManagerWithFreshness(rawJsonDataClient, logger, amazonConfig.DataFreshnessDays),
		cacheEnabled: rawJsonDataClient != nil,
		amazonConfig: amazonConfig,
		client: &http.Client{
			Timeout: time.Duration(amazonConfig.RemoteAPI.Timeout) * time.Second,
			Transport: &http.Transport{
				Proxy:             http.ProxyFromEnvironment,
				DisableKeepAlives: true,
			},
		},
		baseURL: strings.TrimRight(amazonConfig.RemoteAPI.BaseURL, "/"),
		logger:  logger,
	}, nil
}

func (f *RemoteAPIProductFetcher) FetchProduct(ctx context.Context, req *domainProduct.FetchRequest) (*model.Product, error) {
	if f.canUseCache(req) {
		if product, err := f.cacheManager.GetFromCache(req); err == nil && product != nil {
			f.logger.Debugf("got product from cache: %s", req.ProductID)
			return product, nil
		}
	}

	httpReq, err := f.buildRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("call crawler api: %w", err)
	}
	defer resp.Body.Close()

	product, err := f.handleFetchResponse(ctx, resp, req)
	if err != nil {
		return nil, err
	}

	if f.canUseCache(req) {
		if err := f.cacheManager.SaveToCache(req, product); err != nil {
			f.logger.WithError(err).Warn("save product cache failed")
		}
	}
	return product, nil
}

func (f *RemoteAPIProductFetcher) canUseCache(req *domainProduct.FetchRequest) bool {
	if !f.cacheEnabled || f.cacheManager == nil {
		return false
	}
	return req == nil || strings.TrimSpace(req.Zipcode) == ""
}

func (f *RemoteAPIProductFetcher) buildRequest(ctx context.Context, req *domainProduct.FetchRequest) (*http.Request, error) {
	region := strings.ToLower(req.Region)
	payload := remoteFetchRequest{
		ASIN:   req.ProductID,
		Region: region,
	}
	if zipcode := f.resolveZipcode(req); zipcode != "" {
		payload.Zipcode = zipcode
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal crawler api request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, f.baseURL+"/api/v1/products/fetch", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build crawler api request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	return httpReq, nil
}

func (f *RemoteAPIProductFetcher) handleFetchResponse(ctx context.Context, resp *http.Response, req *domainProduct.FetchRequest) (*model.Product, error) {
	if resp.StatusCode == http.StatusOK {
		var payload remoteFetchResponse
		if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
			return nil, fmt.Errorf("decode crawler api response: %w", err)
		}
		if !payload.Success {
			if payload.Error != "" {
				return nil, fmt.Errorf("crawler api failed: %s", payload.Error)
			}
			return nil, fmt.Errorf("crawler api failed")
		}
		if payload.Data.Product == nil {
			return nil, fmt.Errorf("crawler api returned empty product")
		}
		return payload.Data.Product, nil
	}

	var errorPayload remoteErrorResponse
	if err := json.NewDecoder(resp.Body).Decode(&errorPayload); err != nil {
		return nil, fmt.Errorf("decode crawler api error response: %w", err)
	}
	if f.shouldFallbackToAsync(resp.StatusCode, errorPayload) {
		f.logger.WithFields(logrus.Fields{
			"status_code": resp.StatusCode,
			"code":        errorPayload.Code,
			"tenant_id":   req.TenantID,
			"store_id":    req.StoreID,
			"platform":    req.Platform,
			"region":      req.Region,
			"product_id":  req.ProductID,
		}).Warn("crawler api is busy, falling back to async crawl polling")
		return f.fetchProductAsync(ctx, req)
	}

	if errorPayload.Error != "" {
		return nil, fmt.Errorf("crawler api returned %d: %s", resp.StatusCode, errorPayload.Error)
	}
	return nil, fmt.Errorf("crawler api returned %d", resp.StatusCode)
}

func (f *RemoteAPIProductFetcher) shouldFallbackToAsync(statusCode int, payload remoteErrorResponse) bool {
	if statusCode == http.StatusTooManyRequests {
		return true
	}
	if !payload.Data.Retryable {
		return false
	}
	switch payload.Code {
	case "system_busy", "timeout", "network", "server_error":
		return true
	default:
		return false
	}
}

func (f *RemoteAPIProductFetcher) fetchProductAsync(ctx context.Context, req *domainProduct.FetchRequest) (*model.Product, error) {
	taskID, err := f.submitAsyncCrawl(ctx, req)
	if err != nil {
		return nil, err
	}
	return f.pollAsyncResult(ctx, taskID)
}

func (f *RemoteAPIProductFetcher) submitAsyncCrawl(ctx context.Context, req *domainProduct.FetchRequest) (string, error) {
	region := strings.ToLower(req.Region)
	payload := remoteFetchRequest{
		ASIN:   req.ProductID,
		Region: region,
	}
	if zipcode := f.resolveZipcode(req); zipcode != "" {
		payload.Zipcode = zipcode
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal async crawl request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, f.baseURL+"/api/v1/crawl", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("build async crawl request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("submit async crawl task: %w", err)
	}
	defer resp.Body.Close()

	var payloadResp remoteAsyncSubmitResponse
	if err := json.NewDecoder(resp.Body).Decode(&payloadResp); err != nil {
		return "", fmt.Errorf("decode async crawl response: %w", err)
	}
	if resp.StatusCode != http.StatusOK || !payloadResp.Success || strings.TrimSpace(payloadResp.Data.TaskID) == "" {
		if payloadResp.Error != "" {
			return "", fmt.Errorf("submit async crawl failed: %s", payloadResp.Error)
		}
		return "", fmt.Errorf("submit async crawl failed with status %d", resp.StatusCode)
	}

	f.logger.WithFields(logrus.Fields{
		"crawler_task_id": payloadResp.Data.TaskID,
		"tenant_id":       req.TenantID,
		"store_id":        req.StoreID,
		"platform":        req.Platform,
		"region":          req.Region,
		"product_id":      req.ProductID,
		"creator":         req.Creator,
	}).Info("submitted async crawl task")
	return payloadResp.Data.TaskID, nil
}

func (f *RemoteAPIProductFetcher) resolveZipcode(req *domainProduct.FetchRequest) string {
	if req == nil {
		return ""
	}

	var zipcodes map[string]string
	if f.amazonConfig != nil {
		zipcodes = f.amazonConfig.Zipcodes
	}
	planner := sourcing.AmazonCrawlRequestPlanner{
		ZipcodePolicy: sourcing.AmazonDefaultZipcodePolicy{},
		Zipcodes:      zipcodes,
	}
	return planner.ResolveZipcode(req.Region, req.Zipcode)
}

func (f *RemoteAPIProductFetcher) pollAsyncResult(ctx context.Context, taskID string) (*model.Product, error) {
	pollInterval := f.asyncPollInterval()
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		product, done, err := f.fetchAsyncTask(ctx, taskID)
		if err != nil {
			return nil, err
		}
		if done {
			return product, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func (f *RemoteAPIProductFetcher) fetchAsyncTask(ctx context.Context, taskID string) (*model.Product, bool, error) {
	taskURL := f.baseURL + "/api/v1/tasks/" + url.PathEscape(taskID)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, taskURL, nil)
	if err != nil {
		return nil, false, fmt.Errorf("build async task request: %w", err)
	}

	resp, err := f.client.Do(httpReq)
	if err != nil {
		return nil, false, fmt.Errorf("poll async task: %w", err)
	}
	defer resp.Body.Close()

	var taskResp remoteTaskResponse
	if err := json.NewDecoder(resp.Body).Decode(&taskResp); err != nil {
		return nil, false, fmt.Errorf("decode async task response: %w", err)
	}
	if resp.StatusCode != http.StatusOK || !taskResp.Success {
		logFields := logrus.Fields{
			"crawler_task_id": taskID,
			"task_url":        taskURL,
			"status_code":     resp.StatusCode,
			"response_code":   taskResp.Code,
		}
		if taskResp.Error != "" {
			logFields["error"] = taskResp.Error
			f.logger.WithFields(logFields).Warn("async crawl task query failed")
			return nil, false, fmt.Errorf("async task query failed: %s", taskResp.Error)
		}
		f.logger.WithFields(logFields).Warn("async crawl task query failed without error payload")
		return nil, false, fmt.Errorf("async task query failed with status %d", resp.StatusCode)
	}

	switch strings.ToLower(taskResp.Data.Status) {
	case "pending", "processing":
		return nil, false, nil
	case "failed":
		f.logger.WithFields(logrus.Fields{
			"crawler_task_id": taskID,
			"task_status":     taskResp.Data.Status,
			"error":           taskResp.Data.Error,
		}).Warn("async crawl task finished with failure")
		if taskResp.Data.Error != "" {
			return nil, true, fmt.Errorf("async crawl failed: %s", taskResp.Data.Error)
		}
		return nil, true, fmt.Errorf("async crawl failed")
	case "success":
		f.logger.WithFields(logrus.Fields{
			"crawler_task_id": taskID,
			"task_status":     taskResp.Data.Status,
		}).Info("async crawl task finished successfully")
		product, err := decodeProductMap(taskResp.Data.ProductData)
		if err != nil {
			return nil, true, err
		}
		return product, true, nil
	default:
		return nil, false, nil
	}
}

func (f *RemoteAPIProductFetcher) asyncPollInterval() time.Duration {
	return 2 * time.Second
}

func decodeProductMap(data map[string]any) (*model.Product, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("async crawl returned empty product")
	}
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("marshal async product payload: %w", err)
	}
	var product model.Product
	if err := json.Unmarshal(raw, &product); err != nil {
		return nil, fmt.Errorf("decode async product payload: %w", err)
	}
	return &product, nil
}

func (f *RemoteAPIProductFetcher) CacheProduct(req *domainProduct.FetchRequest, product *model.Product) error {
	if !f.canUseCache(req) {
		return nil
	}
	return f.cacheManager.CacheProduct(req, product)
}

func (f *RemoteAPIProductFetcher) CacheVariants(req *domainProduct.FetchRequest, variants []*model.Product) error {
	if !f.canUseCache(req) {
		return nil
	}
	return f.cacheManager.CacheVariants(req, variants)
}

func (f *RemoteAPIProductFetcher) FetchVariants(ctx context.Context, req *domainProduct.FetchRequest, variantASINs []string) ([]*model.Product, error) {
	if len(variantASINs) == 0 {
		return []*model.Product{}, nil
	}

	var variants []*model.Product
	for _, asin := range variantASINs {
		variantReq := domainProduct.FetchRequestFromSource(sourcing.VariantSourceRequest(domainProduct.SourceRequestFromFetch(req), asin))
		product, err := f.FetchProduct(ctx, variantReq)
		if err != nil {
			f.logger.WithError(err).Warnf("fetch variant via crawler api failed: %s", asin)
			continue
		}
		if product != nil {
			variants = append(variants, product)
		}
	}
	if len(variants) == 0 {
		return nil, fmt.Errorf("all variants fetch failed, total=%d", len(variantASINs))
	}
	return variants, nil
}

func (f *RemoteAPIProductFetcher) GetStats() map[string]any {
	return map[string]any{
		"type":     "remote-api",
		"base_url": f.baseURL,
	}
}
