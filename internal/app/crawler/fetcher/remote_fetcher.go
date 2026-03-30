package fetcher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"task-processor/internal/core/config"
	coreLogger "task-processor/internal/core/logger"
	"task-processor/internal/model"
	domainProduct "task-processor/internal/product"

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

type RemoteAPIProductFetcher struct {
	cacheManager   *domainProduct.CacheManager
	cacheEnabled   bool
	domainResolver *domainProduct.DomainResolver
	amazonConfig   *config.AmazonConfig
	client         *http.Client
	baseURL        string
	logger         *logrus.Entry
}

func NewRemoteAPIProductFetcher(
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
		cacheManager:   domainProduct.NewCacheManager(rawJsonDataClient, logger),
		cacheEnabled:   rawJsonDataClient != nil,
		domainResolver: domainProduct.NewDomainResolver(),
		amazonConfig:   amazonConfig,
		client: &http.Client{
			Timeout: time.Duration(amazonConfig.RemoteAPI.Timeout) * time.Second,
		},
		baseURL: strings.TrimRight(amazonConfig.RemoteAPI.BaseURL, "/"),
		logger:  logger,
	}, nil
}

func (f *RemoteAPIProductFetcher) FetchProduct(ctx context.Context, req *domainProduct.FetchRequest) (*model.Product, error) {
	if f.canUseCache() {
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

	var payload remoteFetchResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode crawler api response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		if payload.Error != "" {
			return nil, fmt.Errorf("crawler api returned %d: %s", resp.StatusCode, payload.Error)
		}
		return nil, fmt.Errorf("crawler api returned %d", resp.StatusCode)
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

	if f.canUseCache() {
		if err := f.cacheManager.SaveToCache(req, payload.Data.Product); err != nil {
			f.logger.WithError(err).Warn("save product cache failed")
		}
	}
	return payload.Data.Product, nil
}

func (f *RemoteAPIProductFetcher) canUseCache() bool {
	return f.cacheEnabled && f.cacheManager != nil
}

func (f *RemoteAPIProductFetcher) buildRequest(ctx context.Context, req *domainProduct.FetchRequest) (*http.Request, error) {
	region := strings.ToLower(req.Region)
	payload := remoteFetchRequest{
		ASIN:   req.ProductID,
		Region: region,
	}
	if region != "" {
		payload.Zipcode = f.domainResolver.GetZipcodeByRegion(region)
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

func (f *RemoteAPIProductFetcher) CacheProduct(req *domainProduct.FetchRequest, product *model.Product) error {
	if !f.canUseCache() {
		return nil
	}
	return f.cacheManager.CacheProduct(req, product)
}

func (f *RemoteAPIProductFetcher) CacheVariants(req *domainProduct.FetchRequest, variants []*model.Product) error {
	if !f.canUseCache() {
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
		variantReq := &domainProduct.FetchRequest{
			TenantID:   req.TenantID,
			Platform:   req.Platform,
			Region:     req.Region,
			ProductID:  asin,
			StoreID:    req.StoreID,
			CategoryID: req.CategoryID,
			Creator:    req.Creator,
		}
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
