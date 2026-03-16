// Package api 提供Amazon listing详细信息处理功能
package api

import (
	"context"
	"fmt"
	"net/url"

	"github.com/sirupsen/logrus"
)

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
	var detailedResult map[string]any
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
func (c *Client) printProductDetails(data map[string]any) {
	c.logger.Info("📋 ===== 产品详细信息 =====")

	if sku, ok := data["sku"].(string); ok {
		c.logger.Infof("🏷️  SKU: %s", sku)
	}

	// 解析summaries信息
	if summaries, ok := data["summaries"].([]any); ok && len(summaries) > 0 {
		if summary, ok := summaries[0].(map[string]any); ok {
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

			if status, ok := summary["status"].([]any); ok {
				statusList := make([]string, len(status))
				for i, s := range status {
					if str, ok := s.(string); ok {
						statusList[i] = str
					}
				}
				c.logger.Infof("  ✅ 状态: %v", statusList)
			}

			if mainImage, ok := summary["mainImage"].(map[string]any); ok {
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
	if attributes, ok := data["attributes"].(map[string]any); ok {
		c.logger.Info("🔧 产品属性:")
		for key, value := range attributes {
			c.logger.Infof("  %s: %v", key, value)
		}
	}

	// 解析offers信息
	if offers, ok := data["offers"].([]any); ok && len(offers) > 0 {
		c.logger.Info("💰 价格信息:")
		for i, offer := range offers {
			if offerMap, ok := offer.(map[string]any); ok {
				c.logger.Infof("  报价 %d: %v", i+1, offerMap)
			}
		}
	}

	// 解析issues信息
	if issues, ok := data["issues"].([]any); ok && len(issues) > 0 {
		c.logger.Info("⚠️  问题列表:")
		for i, issue := range issues {
			if issueMap, ok := issue.(map[string]any); ok {
				c.logger.Infof("  问题 %d: %v", i+1, issueMap)
			}
		}
	} else {
		c.logger.Info("✅ 无发现问题")
	}

	c.logger.Info("📋 ===== 详细信息结束 =====")
}
