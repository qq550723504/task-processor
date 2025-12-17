// Package api 提供Amazon SP-API卖家信息相关功能
package api

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
)

// MarketplaceParticipation 市场参与信息
type MarketplaceParticipation struct {
	Marketplace   Marketplace   `json:"marketplace"`
	Participation Participation `json:"participation"`
}

// Marketplace 市场信息
type Marketplace struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	CountryCode         string `json:"countryCode"`
	DefaultCurrencyCode string `json:"defaultCurrencyCode"`
	DefaultLanguageCode string `json:"defaultLanguageCode"`
	DomainName          string `json:"domainName"`
}

// Participation 参与信息
type Participation struct {
	IsParticipating      bool   `json:"isParticipating"`
	HasSuspendedListings bool   `json:"hasSuspendedListings"`
	SellerId             string `json:"sellerId"`
}

// GetMarketplaceParticipations 获取市场参与信息（包含SellerID）
func (c *Client) GetMarketplaceParticipations(ctx context.Context) ([]MarketplaceParticipation, error) {
	c.logger.Info("获取市场参与信息")

	// 构建请求路径
	path := "/sellers/v1/marketplaceParticipations"

	// 发送请求
	resp, err := c.doRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("获取市场参与信息失败: %w", err)
	}

	// 检查速率限制
	if err := c.handleRateLimit(resp); err != nil {
		return nil, err
	}

	// 解析响应
	var result struct {
		Payload []MarketplaceParticipation `json:"payload"`
	}
	if err := c.parseResponse(resp, &result); err != nil {
		return nil, err
	}

	c.logger.WithFields(logrus.Fields{
		"count": len(result.Payload),
	}).Info("市场参与信息获取成功")

	return result.Payload, nil
}

// GetSellerID 获取当前卖家ID
func (c *Client) GetSellerID(ctx context.Context) (string, error) {
	// 如果已经配置了 sellerID，直接返回
	if c.sellerID != "" {
		return c.sellerID, nil
	}

	// Amazon的市场参与API响应中不包含SellerID字段
	// 这是Amazon API的限制，SellerID必须通过配置提供
	return "", fmt.Errorf("SellerID未配置，请在配置文件中设置sellerID字段")
}

// GetAvailableMarketplaces 获取所有可用的市场信息
func (c *Client) GetAvailableMarketplaces(ctx context.Context) ([]Marketplace, error) {
	c.logger.Info("获取可用市场列表")

	// 获取市场参与信息
	participations, err := c.GetMarketplaceParticipations(ctx)
	if err != nil {
		return nil, fmt.Errorf("获取市场信息失败: %w", err)
	}

	var marketplaces []Marketplace
	for _, participation := range participations {
		if participation.Participation.IsParticipating {
			marketplaces = append(marketplaces, participation.Marketplace)
		}
	}

	c.logger.WithFields(logrus.Fields{
		"count": len(marketplaces),
	}).Info("可用市场获取成功")

	return marketplaces, nil
}

// GetMarketplaceByRegion 根据区域获取推荐的市场ID
func (c *Client) GetMarketplaceByRegion(ctx context.Context, region string) (string, error) {
	c.logger.WithFields(logrus.Fields{
		"region": region,
	}).Info("根据区域获取市场ID")

	// 获取可用市场
	marketplaces, err := c.GetAvailableMarketplaces(ctx)
	if err != nil {
		return "", err
	}

	// 区域到市场的映射
	regionToMarketplace := map[string][]string{
		"us-east-1": {"ATVPDKIKX0DER"},                                                                         // 美国
		"us-west-2": {"A2EUQ1WTGCTBG2", "A1AM78C64UM0Y8"},                                                      // 加拿大、墨西哥
		"eu-west-1": {"A1PA6795UKMFR9", "A1RKKUPIHCS9HS", "A13V1IB3VIYZZH", "A1F83G8C2ARO7P", "APJ6JRA9NG5V4"}, // 德国、西班牙、法国、英国、意大利
	}

	// 查找匹配的市场
	if expectedMarketplaces, exists := regionToMarketplace[region]; exists {
		for _, marketplace := range marketplaces {
			for _, expectedID := range expectedMarketplaces {
				if marketplace.ID == expectedID {
					c.logger.WithFields(logrus.Fields{
						"marketplaceId": marketplace.ID,
						"name":          marketplace.Name,
						"region":        region,
					}).Info("找到匹配的市场")
					return marketplace.ID, nil
				}
			}
		}
	}

	// 如果没有找到匹配的，返回第一个可用的市场
	if len(marketplaces) > 0 {
		marketplace := marketplaces[0]
		c.logger.WithFields(logrus.Fields{
			"marketplaceId": marketplace.ID,
			"name":          marketplace.Name,
			"region":        region,
		}).Warn("未找到区域匹配的市场，使用第一个可用市场")
		return marketplace.ID, nil
	}

	return "", fmt.Errorf("未找到区域 %s 的可用市场", region)
}

// SetMarketplaceID 动态设置市场ID
func (c *Client) SetMarketplaceID(marketplaceID string) {
	c.marketplaceID = marketplaceID
	// 清空缓存的 sellerID，因为不同市场可能有不同的 sellerID
	c.sellerID = ""

	c.logger.WithFields(logrus.Fields{
		"marketplaceId": marketplaceID,
	}).Info("市场ID已更新")
}

// GetCurrentMarketplaceID 获取当前市场ID（智能获取）
func (c *Client) GetCurrentMarketplaceID(ctx context.Context) (string, error) {
	// 如果已经配置了 marketplaceID，直接返回
	if c.marketplaceID != "" {
		return c.marketplaceID, nil
	}

	c.logger.Info("动态获取MarketplaceID")

	// 根据区域获取推荐的市场ID
	marketplaceID, err := c.GetMarketplaceByRegion(ctx, c.region)
	if err != nil {
		return "", fmt.Errorf("获取MarketplaceID失败: %w", err)
	}

	// 缓存到客户端
	c.marketplaceID = marketplaceID
	return marketplaceID, nil
}
