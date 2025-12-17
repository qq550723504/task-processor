package api

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

// PriceRequest 价格更新请求
type PriceRequest struct {
	SKU      string  `json:"sku"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
}

// PriceResponse 价格响应
type PriceResponse struct {
	SKU      string  `json:"sku"`
	Price    float64 `json:"price"`
	Currency string  `json:"currency"`
	Status   string  `json:"status"`
}

// UpdatePrice 更新价格
func (c *Client) UpdatePrice(ctx context.Context, req *PriceRequest) (*PriceResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"sku":      req.SKU,
		"price":    req.Price,
		"currency": req.Currency,
	}).Info("更新Amazon价格")

	// 构建价格更新请求体
	priceData := map[string]any{
		"offers": []map[string]any{
			{
				"marketplaceId": c.marketplaceID,
				"offerType":     "B2C",
				"buyingPrice": map[string]any{
					"listPrice": map[string]any{
						"currencyCode": req.Currency,
						"amount":       req.Price,
					},
				},
			},
		},
	}

	// 构建请求路径
	path := fmt.Sprintf("/products/pricing/v0/items/%s/offers", req.SKU)

	// 发送请求
	resp, err := c.doRequest(ctx, "PUT", path, priceData)
	if err != nil {
		return nil, fmt.Errorf("更新价格失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	// 解析响应
	var result map[string]any
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	c.logger.WithFields(logrus.Fields{
		"sku":      req.SKU,
		"price":    req.Price,
		"response": result,
	}).Info("价格更新成功")

	return &PriceResponse{
		SKU:      req.SKU,
		Price:    req.Price,
		Currency: req.Currency,
		Status:   "SUCCESS",
	}, nil
}

// GetPrice 获取价格信息
func (c *Client) GetPrice(ctx context.Context, sku string) (*PriceResponse, error) {
	c.logger.WithFields(logrus.Fields{
		"sku": sku,
	}).Info("获取Amazon价格")

	// 构建请求路径
	path := fmt.Sprintf("/products/pricing/v0/items/%s/offers?MarketplaceId=%s&ItemCondition=New",
		sku, c.marketplaceID)

	// 发送请求
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("获取价格失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	// 解析响应
	var result struct {
		Payload struct {
			ASIN   string `json:"ASIN"`
			Status string `json:"status"`
			Offers []struct {
				BuyingPrice struct {
					ListPrice struct {
						CurrencyCode string  `json:"CurrencyCode"`
						Amount       float64 `json:"Amount"`
					} `json:"ListPrice"`
				} `json:"BuyingPrice"`
			} `json:"Offers"`
		} `json:"payload"`
	}

	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	// 提取价格信息
	var price float64
	var currency string = "USD"

	if len(result.Payload.Offers) > 0 {
		offer := result.Payload.Offers[0]
		price = offer.BuyingPrice.ListPrice.Amount
		currency = offer.BuyingPrice.ListPrice.CurrencyCode
	}

	c.logger.WithFields(logrus.Fields{
		"sku":      sku,
		"price":    price,
		"currency": currency,
	}).Info("价格获取成功")

	return &PriceResponse{
		SKU:      sku,
		Price:    price,
		Currency: currency,
		Status:   "SUCCESS",
	}, nil
}
