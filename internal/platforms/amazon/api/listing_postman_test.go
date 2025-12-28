// Package api 提供基于Amazon官方Postman集合的测试功能
package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/sirupsen/logrus"
)

// TestPostmanGetListing 基于Amazon官方Postman集合格式测试GetListing
func (c *Client) TestPostmanGetListing(ctx context.Context, sku, marketplaceID string) error {
	c.logger.WithFields(logrus.Fields{
		"sku":         sku,
		"marketplace": marketplaceID,
	}).Info("Postman格式测试GetListing")

	// 获取SellerID
	sellerID, err := c.GetSellerID(ctx)
	if err != nil {
		return fmt.Errorf("获取SellerID失败: %w", err)
	}

	// 完全按照Postman集合中的格式构建请求
	// 参考: https://github.com/amzn/selling-partner-api-postman
	path := fmt.Sprintf("/listings/2021-08-01/items/%s/%s", sellerID, url.PathEscape(sku))

	// Postman集合中使用的查询参数格式
	queryParams := fmt.Sprintf("?marketplaceIds=%s&issueLocale=en_US", marketplaceID)
	fullPath := path + queryParams

	c.logger.WithFields(logrus.Fields{
		"path":     fullPath,
		"sellerID": sellerID,
	}).Info("发送Postman格式请求")

	// 发送请求
	resp, err := c.doRequest(ctx, "GET", fullPath, nil)
	if err != nil {
		return fmt.Errorf("Postman格式请求失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return err
	}

	// 解析响应
	var result map[string]interface{}
	if err := c.parseResponse(resp, &result); err != nil {
		return err
	}

	c.logger.WithFields(logrus.Fields{
		"response": result,
	}).Info("Postman格式响应")

	return nil
}

// TestPostmanPutListing 基于Amazon官方Postman集合格式测试PutListing
func (c *Client) TestPostmanPutListing(ctx context.Context, req *PostmanListingRequest) (*ListingResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"sku":         req.SKU,
		"productType": req.ProductType,
		"marketplace": req.MarketplaceID,
	}).Info("Postman格式测试PutListing")

	// 获取SellerID
	sellerID, err := c.GetSellerID(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取SellerID失败: %w", err)
	}

	// 完全按照Postman集合中的格式构建请求
	path := fmt.Sprintf("/listings/2021-08-01/items/%s/%s", sellerID, url.PathEscape(req.SKU))

	// Postman集合中使用的查询参数格式 - 添加验证模式
	queryParams := fmt.Sprintf("?marketplaceIds=%s&issueLocale=en_US&mode=VALIDATION_PREVIEW", req.MarketplaceID)
	fullPath := path + queryParams

	c.logger.WithFields(logrus.Fields{
		"path":     fullPath,
		"sellerID": sellerID,
		"body":     req,
	}).Info("发送Postman格式PUT请求")

	// 发送请求
	resp, err := c.doRequest(ctx, "PUT", fullPath, req)
	if err != nil {
		return nil, fmt.Errorf("Postman格式PUT请求失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	// 解析响应
	var result ListingResponse
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	// 如果响应中没有 SKU，使用请求中的 SKU
	if result.SKU == "" {
		result.SKU = req.SKU
	}

	c.logger.WithFields(logrus.Fields{
		"sku":    result.SKU,
		"status": result.Status,
	}).Info("Postman格式PUT响应")

	return &result, nil
}
