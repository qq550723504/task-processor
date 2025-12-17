package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/sirupsen/logrus"
)

// ListingRequest 创建listing请求
type ListingRequest struct {
	SKU           string         `json:"-"` // SKU在URL路径中，不在请求体中
	ProductType   string         `json:"productType"`
	Requirements  string         `json:"requirements"`
	Attributes    map[string]any `json:"attributes"`
	MarketplaceID string         `json:"-"` // MarketplaceID在查询参数中，不在请求体中
}

// ListingResponse listing响应
type ListingResponse struct {
	SKU    string `json:"sku"`
	Status string `json:"status"`
	Issues []struct {
		Code     string `json:"code"`
		Message  string `json:"message"`
		Severity string `json:"severity"`
	} `json:"issues,omitempty"`
}

// PostmanListingRequest 基于Amazon官方Postman集合的listing请求格式
type PostmanListingRequest struct {
	SKU           string         `json:"-"` // SKU在URL路径中
	MarketplaceID string         `json:"-"` // MarketplaceID在查询参数中
	ProductType   string         `json:"productType"`
	Requirements  string         `json:"requirements,omitempty"`
	Attributes    map[string]any `json:"attributes"`
}

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

// GetDetailedListing 获取包含所有详细信息的listing
func (c *Client) GetDetailedListing(ctx context.Context, sku, marketplaceID string) (*ListingResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"sku":         sku,
		"marketplace": marketplaceID,
	}).Info("获取详细Amazon listing信息")

	// 获取SellerID
	sellerID, err := c.GetSellerID(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取SellerID失败: %w", err)
	}

	// 构建请求路径，包含所有可用的数据类型
	path := fmt.Sprintf("/listings/2021-08-01/items/%s/%s", sellerID, url.PathEscape(sku))

	// 添加所有可用的includedData参数来获取完整信息
	queryParams := fmt.Sprintf("?marketplaceIds=%s&issueLocale=en_US&includedData=summaries,attributes,issues,offers,fulfillmentAvailability,procurement", marketplaceID)
	fullPath := path + queryParams

	c.logger.WithFields(logrus.Fields{
		"path":     fullPath,
		"sellerID": sellerID,
	}).Info("发送详细信息请求")

	// 发送请求
	resp, err := c.doRequest(ctx, "GET", fullPath, nil)
	if err != nil {
		return nil, fmt.Errorf("获取详细listing失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	// 解析响应为通用map以便查看所有数据
	var detailedResult map[string]interface{}
	if err := c.parseResponse(resp, &detailedResult); err != nil {
		return nil, err
	}

	c.logger.WithFields(logrus.Fields{
		"sku":      sku,
		"response": detailedResult,
	}).Info("详细listing信息获取成功")

	// 打印详细的产品信息供用户参考
	c.printProductDetails(detailedResult)

	// 返回基本的ListingResponse结构
	result := &ListingResponse{
		SKU:    sku,
		Status: "SUCCESS",
	}

	return result, nil
}

// printProductDetails 打印产品详细信息
func (c *Client) printProductDetails(data map[string]interface{}) {
	c.logger.Info("📋 ===== 产品详细信息 =====")

	if sku, ok := data["sku"].(string); ok {
		c.logger.Infof("🏷️  SKU: %s", sku)
	}

	// 解析summaries信息
	if summaries, ok := data["summaries"].([]interface{}); ok && len(summaries) > 0 {
		if summary, ok := summaries[0].(map[string]interface{}); ok {
			c.logger.Info("📦 基本信息:")

			if asin, ok := summary["asin"].(string); ok {
				c.logger.Infof("  🔗 ASIN: %s", asin)
			}

			if productType, ok := summary["productType"].(string); ok {
				c.logger.Infof("  📂 产品类型: %s", productType)
			}

			if itemName, ok := summary["itemName"].(string); ok {
				c.logger.Infof("  📝 产品名称: %s", itemName)
			}

			if conditionType, ok := summary["conditionType"].(string); ok {
				c.logger.Infof("  🏷️  商品状态: %s", conditionType)
			}

			if status, ok := summary["status"].([]interface{}); ok {
				statusList := make([]string, len(status))
				for i, s := range status {
					if str, ok := s.(string); ok {
						statusList[i] = str
					}
				}
				c.logger.Infof("  ✅ 状态: %v", statusList)
			}

			if mainImage, ok := summary["mainImage"].(map[string]interface{}); ok {
				if link, ok := mainImage["link"].(string); ok {
					c.logger.Infof("  🖼️  主图: %s", link)
				}
				if height, ok := mainImage["height"].(float64); ok {
					if width, ok := mainImage["width"].(float64); ok {
						c.logger.Infof("  📐 图片尺寸: %.0fx%.0f", width, height)
					}
				}
			}

			if createdDate, ok := summary["createdDate"].(string); ok {
				c.logger.Infof("  📅 创建时间: %s", createdDate)
			}

			if lastUpdatedDate, ok := summary["lastUpdatedDate"].(string); ok {
				c.logger.Infof("  🔄 更新时间: %s", lastUpdatedDate)
			}
		}
	}

	// 解析attributes信息
	if attributes, ok := data["attributes"].(map[string]interface{}); ok {
		c.logger.Info("🔧 产品属性:")
		for key, value := range attributes {
			c.logger.Infof("  %s: %v", key, value)
		}
	}

	// 解析offers信息
	if offers, ok := data["offers"].([]interface{}); ok && len(offers) > 0 {
		c.logger.Info("💰 价格信息:")
		for i, offer := range offers {
			if offerMap, ok := offer.(map[string]interface{}); ok {
				c.logger.Infof("  报价 %d: %v", i+1, offerMap)
			}
		}
	}

	// 解析issues信息
	if issues, ok := data["issues"].([]interface{}); ok && len(issues) > 0 {
		c.logger.Info("⚠️  问题列表:")
		for i, issue := range issues {
			if issueMap, ok := issue.(map[string]interface{}); ok {
				c.logger.Infof("  问题 %d: %v", i+1, issueMap)
			}
		}
	} else {
		c.logger.Info("✅ 无发现问题")
	}

	c.logger.Info("📋 ===== 详细信息结束 =====")
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
