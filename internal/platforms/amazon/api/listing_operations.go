// Package api 提供Amazon API的核心listing操作功能
package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/sirupsen/logrus"
)

// CreateListing 创建产品listing
func (c *Client) CreateListing(ctx context.Context, req *ListingRequest) (*ListingResponse, error) {
	return c.createListingWithMode(ctx, req, "")
}

// ValidateListing 验证listing（不实际创建）
func (c *Client) ValidateListing(ctx context.Context, req *ListingRequest) (*ListingResponse, error) {
	return c.createListingWithMode(ctx, req, "VALIDATION_PREVIEW")
}

// createListingWithMode 创建或验证listing
func (c *Client) createListingWithMode(ctx context.Context, req *ListingRequest, mode string) (*ListingResponse, error) {
	// 获取SellerID（如果未配置则动态获取）
	sellerID, err := c.GetSellerID(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取SellerID失败: %w", err)
	}

	// 对SKU进行URL编码
	encodedSKU := url.PathEscape(req.SKU)

	// 构建请求路径，添加 marketplaceIds 和 issueLocale 查询参数
	path := fmt.Sprintf("/listings/2021-08-01/items/%s/%s?marketplaceIds=%s&issueLocale=en_US",
		sellerID,
		encodedSKU,
		c.marketplaceID)

	// 如果指定了验证模式，添加 mode 参数
	if mode != "" {
		path += "&mode=" + mode
	}

	c.logger.WithFields(logrus.Fields{
		"sku":         req.SKU,
		"productType": req.ProductType,
		"marketplace": c.marketplaceID,
		"sellerID":    sellerID,
		"mode":        mode,
		"path":        path,
	}).Info("创建Amazon listing")

	// 发送请求
	resp, err := c.doRequest(ctx, "PUT", path, req)
	if err != nil {
		return nil, fmt.Errorf("创建listing失败: %w", err)
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
	}).Info("Listing 创建成功")

	return &result, nil
}

// UpdateListing 更新产品listing
func (c *Client) UpdateListing(ctx context.Context, req *ListingRequest) (*ListingResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"sku": req.SKU,
	}).Info("更新Amazon listing")

	// 更新和创建使用相同的API端点
	return c.CreateListing(ctx, req)
}

// DeleteListing 删除产品listing
func (c *Client) DeleteListing(ctx context.Context, sku string) error {
	c.logger.WithFields(logrus.Fields{
		"sku": sku,
	}).Info("删除Amazon listing")

	// 获取SellerID（如果未配置则动态获取）
	sellerID, err := c.GetSellerID(ctx)
	if err != nil {
		return fmt.Errorf("获取SellerID失败: %w", err)
	}

	// 构建请求路径
	path := fmt.Sprintf("/listings/2021-08-01/items/%s/%s", sellerID, sku)

	// 发送请求
	resp, err := c.doRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("删除listing失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return err
	}

	// 解析响应（DELETE 通常没有响应体）
	if err := c.parseResponse(resp, nil); err != nil {
		return err
	}

	c.logger.Info("Listing 删除成功")
	return nil
}

// GetListing 获取产品listing信息
func (c *Client) GetListing(ctx context.Context, sku string) (*ListingResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"sku": sku,
	}).Info("获取Amazon listing")

	// 获取SellerID（如果未配置则动态获取）
	sellerID, err := c.GetSellerID(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取SellerID失败: %w", err)
	}

	// 构建请求路径，添加 marketplaceIds 查询参数
	path := fmt.Sprintf("/listings/2021-08-01/items/%s/%s?marketplaceIds=%s&issueLocale=en_US",
		sellerID, sku, c.marketplaceID)

	// 发送请求
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("获取listing失败: %w", err)
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

	c.logger.WithFields(logrus.Fields{
		"sku":    result.SKU,
		"status": result.Status,
	}).Info("Listing 获取成功")

	return &result, nil
}
