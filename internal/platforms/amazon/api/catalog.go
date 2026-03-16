// Package api 提供Amazon SP-API目录相关接口
package api

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/sirupsen/logrus"
)

// CatalogItem 目录商品信息
type CatalogItem struct {
	ASIN        string                 `json:"asin"`
	Title       string                 `json:"title"`
	Brand       string                 `json:"brand"`
	ProductType string                 `json:"productType"`
	Categories  []Category             `json:"categories"`
	Images      []Image                `json:"images"`
	Attributes  map[string]any `json:"attributes"`
}

// Category 商品分类
type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Image 商品图片
type Image struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

// SearchCatalogRequest 搜索目录请求
type SearchCatalogRequest struct {
	Keywords      string   `json:"keywords,omitempty"`     // 搜索关键词
	ProductTypes  []string `json:"productTypes,omitempty"` // 产品类型过滤
	Brands        []string `json:"brands,omitempty"`       // 品牌过滤
	MarketplaceID string   `json:"marketplaceId"`          // 市场ID
	PageSize      int      `json:"pageSize,omitempty"`     // 页面大小 (1-20)
	PageToken     string   `json:"pageToken,omitempty"`    // 分页令牌
}

// SearchCatalogResponse 搜索目录响应
type SearchCatalogResponse struct {
	Items         []CatalogItem `json:"items"`
	NextPageToken string        `json:"nextPageToken,omitempty"`
	TotalCount    int           `json:"totalCount"`
}

// GetSellerListingsRequest 获取卖家产品列表请求
type GetSellerListingsRequest struct {
	MarketplaceID string   `json:"marketplaceId"`          // 市场ID
	SKUs          []string `json:"skus,omitempty"`         // SKU过滤
	ProductTypes  []string `json:"productTypes,omitempty"` // 产品类型过滤
	Status        []string `json:"status,omitempty"`       // 状态过滤 (ACTIVE, INACTIVE, INCOMPLETE)
	PageSize      int      `json:"pageSize,omitempty"`     // 页面大小 (1-20)
	PageToken     string   `json:"pageToken,omitempty"`    // 分页令牌
}

// SellerListing 卖家产品信息
type SellerListing struct {
	SKU         string                 `json:"sku"`
	ASIN        string                 `json:"asin"`
	ProductType string                 `json:"productType"`
	Status      string                 `json:"status"`
	Title       string                 `json:"title"`
	Price       Price                  `json:"price"`
	Quantity    int                    `json:"quantity"`
	Attributes  map[string]any `json:"attributes"`
	Issues      []Issue                `json:"issues,omitempty"`
}

// Price 价格信息
type Price struct {
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
}

// Issue 产品问题
type Issue struct {
	Code     string `json:"code"`
	Message  string `json:"message"`
	Severity string `json:"severity"`
}

// GetSellerListingsResponse 获取卖家产品列表响应
type GetSellerListingsResponse struct {
	Listings      []SellerListing `json:"listings"`
	NextPageToken string          `json:"nextPageToken,omitempty"`
	TotalCount    int             `json:"totalCount"`
}

// SearchCatalog 搜索Amazon目录商品
func (c *Client) SearchCatalog(ctx context.Context, req *SearchCatalogRequest) (*SearchCatalogResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"keywords":     req.Keywords,
		"productTypes": req.ProductTypes,
		"marketplace":  req.MarketplaceID,
	}).Info("搜索Amazon目录")

	// 构建查询参数
	params := url.Values{}
	params.Set("marketplaceIds", req.MarketplaceID)

	if req.Keywords != "" {
		params.Set("keywords", req.Keywords)
	}

	if len(req.ProductTypes) > 0 {
		params.Set("productTypes", strings.Join(req.ProductTypes, ","))
	}

	if len(req.Brands) > 0 {
		params.Set("brandNames", strings.Join(req.Brands, ","))
	}

	if req.PageSize > 0 {
		params.Set("pageSize", fmt.Sprintf("%d", req.PageSize))
	} else {
		params.Set("pageSize", "20") // 默认页面大小
	}

	if req.PageToken != "" {
		params.Set("pageToken", req.PageToken)
	}

	// 构建请求路径
	path := fmt.Sprintf("/catalog/2022-04-01/items?%s", params.Encode())

	// 发送请求
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("搜索目录失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	// 解析响应
	var result SearchCatalogResponse
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	c.logger.WithFields(logrus.Fields{
		"itemCount":   len(result.Items),
		"totalCount":  result.TotalCount,
		"hasNextPage": result.NextPageToken != "",
	}).Info("目录搜索完成")

	return &result, nil
}

// GetSellerListings 获取卖家的产品列表
func (c *Client) GetSellerListings(ctx context.Context, req *GetSellerListingsRequest) (*GetSellerListingsResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"marketplace": req.MarketplaceID,
		"skus":        req.SKUs,
		"status":      req.Status,
	}).Info("获取卖家产品列表")

	// 获取SellerID
	sellerID, err := c.GetSellerID(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取SellerID失败: %w", err)
	}

	// 构建查询参数
	params := url.Values{}
	params.Set("marketplaceIds", req.MarketplaceID)

	if len(req.SKUs) > 0 {
		params.Set("skus", strings.Join(req.SKUs, ","))
	}

	if len(req.ProductTypes) > 0 {
		params.Set("productTypes", strings.Join(req.ProductTypes, ","))
	}

	if len(req.Status) > 0 {
		params.Set("status", strings.Join(req.Status, ","))
	}

	if req.PageSize > 0 {
		params.Set("pageSize", fmt.Sprintf("%d", req.PageSize))
	} else {
		params.Set("pageSize", "20") // 默认页面大小
	}

	if req.PageToken != "" {
		params.Set("pageToken", req.PageToken)
	}

	// 构建请求路径 - 根据SP-API文档，需要在路径中包含sellerID
	path := fmt.Sprintf("/listings/2021-08-01/items/%s?%s", sellerID, params.Encode())

	// 发送请求
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("获取产品列表失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	// 解析响应
	var result GetSellerListingsResponse
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	c.logger.WithFields(logrus.Fields{
		"listingCount": len(result.Listings),
		"totalCount":   result.TotalCount,
		"hasNextPage":  result.NextPageToken != "",
	}).Info("产品列表获取完成")

	return &result, nil
}

// GetCatalogItem 根据ASIN获取目录商品详情
func (c *Client) GetCatalogItem(ctx context.Context, asin string, marketplaceID string) (*CatalogItem, error) {
	c.logger.WithFields(logrus.Fields{
		"asin":        asin,
		"marketplace": marketplaceID,
	}).Info("获取目录商品详情")

	// 构建查询参数
	params := url.Values{}
	params.Set("marketplaceIds", marketplaceID)
	params.Set("includedData", "attributes,images,productTypes,salesRanks,summaries")

	// 构建请求路径
	path := fmt.Sprintf("/catalog/2022-04-01/items/%s?%s", asin, params.Encode())

	// 发送请求
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("获取商品详情失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	// 解析响应
	var result CatalogItem
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	c.logger.WithFields(logrus.Fields{
		"asin":  result.ASIN,
		"title": result.Title,
	}).Info("商品详情获取完成")

	return &result, nil
}
