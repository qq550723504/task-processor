package api

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/sirupsen/logrus"
)

// InventoryRequest 库存更新请求
type InventoryRequest struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
}

// InventoryResponse 库存响应
type InventoryResponse struct {
	SKU      string `json:"sku"`
	Quantity int    `json:"quantity"`
	Status   string `json:"status"`
}

// UpdateInventory 更新库存 - 使用Feeds API
func (c *Client) UpdateInventory(ctx context.Context, req *InventoryRequest) (*InventoryResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"sku":      req.SKU,
		"quantity": req.Quantity,
	}).Info("更新Amazon库存")

	// 构建库存Feed XML内容
	feedContent := c.buildInventoryFeedXML(req)

	// 创建Feed
	feedID, err := c.createInventoryFeed(ctx, feedContent)
	if err != nil {
		return nil, fmt.Errorf("创建库存Feed失败: %w", err)
	}

	c.logger.WithFields(logrus.Fields{
		"sku":     req.SKU,
		"feed_id": feedID,
	}).Info("库存更新Feed已提交")

	return &InventoryResponse{
		SKU:      req.SKU,
		Quantity: req.Quantity,
		Status:   "SUBMITTED",
	}, nil
}

// buildInventoryFeedXML 构建库存Feed XML
func (c *Client) buildInventoryFeedXML(req *InventoryRequest) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<AmazonEnvelope xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:noNamespaceSchemaLocation="amzn-envelope.xsd">
	<Header>
		<DocumentVersion>1.01</DocumentVersion>
		<MerchantIdentifier>%s</MerchantIdentifier>
	</Header>
	<MessageType>Inventory</MessageType>
	<Message>
		<MessageID>1</MessageID>
		<Inventory>
			<SKU>%s</SKU>
			<Quantity>%d</Quantity>
		</Inventory>
	</Message>
</AmazonEnvelope>`, c.sellerID, req.SKU, req.Quantity)
}

// createInventoryFeed 创建库存Feed
func (c *Client) createInventoryFeed(ctx context.Context, feedContent string) (string, error) {
	// 构建Feed创建请求
	feedData := map[string]any{
		"feedType":            "POST_INVENTORY_AVAILABILITY_DATA",
		"marketplaceIds":      []string{c.marketplaceID},
		"inputFeedDocumentId": "", // 需要先上传文档
	}

	// 1. 先创建Feed文档
	docID, err := c.createFeedDocument(ctx, feedContent, "text/xml")
	if err != nil {
		return "", fmt.Errorf("创建Feed文档失败: %w", err)
	}

	feedData["inputFeedDocumentId"] = docID

	// 2. 创建Feed
	path := "/feeds/2021-06-30/feeds"
	resp, err := c.doRequest(ctx, "POST", path, feedData)
	if err != nil {
		return "", fmt.Errorf("创建Feed失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return "", err
	}

	// 解析响应
	var result struct {
		FeedID string `json:"feedId"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return "", err
	}

	return result.FeedID, nil
}

// createFeedDocument 创建Feed文档
func (c *Client) createFeedDocument(ctx context.Context, content, contentType string) (string, error) {
	// 构建文档创建请求
	docData := map[string]any{
		"contentType": contentType,
	}

	path := "/feeds/2021-06-30/documents"
	resp, err := c.doRequest(ctx, "POST", path, docData)
	if err != nil {
		return "", fmt.Errorf("创建文档失败: %w", err)
	}

	// 解析响应获取上传URL
	var result struct {
		DocumentID string `json:"feedDocumentId"`
		URL        string `json:"url"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return "", err
	}

	// 上传文档内容
	if err := c.uploadFeedDocument(ctx, result.URL, content, contentType); err != nil {
		return "", fmt.Errorf("上传文档内容失败: %w", err)
	}

	return result.DocumentID, nil
}

// uploadFeedDocument 上传Feed文档内容
func (c *Client) uploadFeedDocument(ctx context.Context, url, content, contentType string) error {
	req, err := http.NewRequestWithContext(ctx, "PUT", url, bytes.NewReader([]byte(content)))
	if err != nil {
		return fmt.Errorf("创建上传请求失败: %w", err)
	}

	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("上传失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("上传失败，状态码: %d", resp.StatusCode)
	}

	return nil
}

// GetInventory 获取库存信息
func (c *Client) GetInventory(ctx context.Context, sku string) (*InventoryResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"sku": sku,
	}).Info("获取Amazon库存")

	// 构建请求路径 - 使用FBA Inventory API
	path := fmt.Sprintf("/fba/inventory/v1/summaries?details=true&granularityType=Marketplace&granularityId=%s&marketplaceIds=%s&sellerSkus=%s",
		c.marketplaceID, c.marketplaceID, sku)

	// 发送请求
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("获取库存失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	// 解析响应
	var result struct {
		Payload struct {
			InventorySummaries []struct {
				SellerSKU                string `json:"sellerSku"`
				FulfillableQuantity      int    `json:"fulfillableQuantity"`
				InboundWorkingQuantity   int    `json:"inboundWorkingQuantity"`
				InboundShippedQuantity   int    `json:"inboundShippedQuantity"`
				InboundReceivingQuantity int    `json:"inboundReceivingQuantity"`
			} `json:"inventorySummaries"`
		} `json:"payload"`
	}

	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	// 提取库存信息
	var quantity int
	if len(result.Payload.InventorySummaries) > 0 {
		summary := result.Payload.InventorySummaries[0]
		quantity = summary.FulfillableQuantity
	}

	c.logger.WithFields(logrus.Fields{
		"sku":      sku,
		"quantity": quantity,
	}).Info("库存获取成功")

	return &InventoryResponse{
		SKU:      sku,
		Quantity: quantity,
		Status:   "SUCCESS",
	}, nil
}

// BatchUpdateInventory 批量更新库存
func (c *Client) BatchUpdateInventory(ctx context.Context, requests []*InventoryRequest) ([]*InventoryResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"count": len(requests),
	}).Info("批量更新Amazon库存")

	responses := make([]*InventoryResponse, 0, len(requests))

	for _, req := range requests {
		resp, err := c.UpdateInventory(ctx, req)
		if err != nil {
			c.logger.WithError(err).Warnf("更新SKU %s 库存失败", req.SKU)
			continue
		}
		responses = append(responses, resp)
	}

	return responses, nil
}
