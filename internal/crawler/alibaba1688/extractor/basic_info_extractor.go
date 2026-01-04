// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"strconv"
	"strings"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// BasicInfoExtractor 基础信息提取器
type BasicInfoExtractor struct{}

// NewBasicInfoExtractor 创建基础信息提取器
func NewBasicInfoExtractor() *BasicInfoExtractor {
	return &BasicInfoExtractor{}
}

// Extract 提取商品基础信息 - 支持两种数据结构
func (bie *BasicInfoExtractor) Extract(page playwright.Page, product *model.Product1688) error {
	// 从结构化数据中获取基础信息，支持两种数据结构
	basicInfoResult, err := page.Evaluate(`() => {
		const result = {
			title: '',
			id: '',
			url: window.location.href,
			saleCount: '',
			rating: 0,
			reviewCount: 0,
			unit: '',
			isCustomized: false
		};
		
		// 方案1：优先尝试从window.context结构化数据中获取（普通商品）
		if (window.context && window.context.result && window.context.result.data) {
			const data = window.context.result.data;
			
			// 获取商品标题和销量信息
			if (data.productTitle && data.productTitle.fields) {
				const titleFields = data.productTitle.fields;
				result.title = titleFields.title || '';
				result.saleCount = titleFields.saleNum || '';
				result.unit = titleFields.unit || '';
				
				// 获取评价信息
				if (titleFields.rateInfo) {
					result.rating = titleFields.rateInfo.goodsGrade || 0;
					result.reviewCount = titleFields.rateInfo.goodRates || 0;
				}
			}
			
			// 获取商品ID
			if (data.Root && data.Root.fields && data.Root.fields.dataJson && 
				data.Root.fields.dataJson.tempModel) {
				const tempModel = data.Root.fields.dataJson.tempModel;
				result.id = tempModel.offerId ? tempModel.offerId.toString() : '';
				result.isCustomized = data.Root.fields.dataJson.isCustomMade || false;
			}
		}
		// 方案2：备选方案 - 从window.__INIT_DATA获取（定制商品）
		else if (window.__INIT_DATA && window.__INIT_DATA.data) {
			const data = window.__INIT_DATA.data;
			
			// 遍历所有数据块，提取基础信息
			for (let key in data) {
				const item = data[key];
				if (!item || !item.data) continue;
				
				const itemData = item.data;
				
				// 提取标题信息
				if (itemData.title && !result.title) {
					result.title = itemData.title;
				}
				
				// 提取商品ID
				if (itemData.offerId && !result.id) {
					result.id = itemData.offerId.toString();
				}
				
				// 提取销量信息（如果有的话）
				if (itemData.saleNum && !result.saleCount) {
					result.saleCount = itemData.saleNum;
				}
				
				// 提取单位信息（如果有的话）
				if (itemData.unit && !result.unit) {
					result.unit = itemData.unit;
				}
				
				// 定制商品默认支持定制
				result.isCustomized = true;
			}
		}
		
		return result;
	}`, nil)

	if err == nil && basicInfoResult != nil {
		if basicData, ok := basicInfoResult.(map[string]any); ok {
			// 设置商品标题
			if title, ok := basicData["title"].(string); ok && title != "" {
				product.Title = title
			}

			// 设置商品ID
			if id, ok := basicData["id"].(string); ok && id != "" {
				product.ID = id
			}

			// 设置商品URL
			if url, ok := basicData["url"].(string); ok && url != "" {
				product.URL = url
			}

			// 设置销量
			if saleCount, ok := basicData["saleCount"].(string); ok && saleCount != "" {
				// 解析销量字符串，如 "60+"
				saleCountStr := strings.ReplaceAll(saleCount, "+", "")
				if saleNum, err := strconv.Atoi(saleCountStr); err == nil {
					product.SalesVolume = saleNum
				}
			}

			// 设置评分
			if rating, ok := basicData["rating"].(float64); ok {
				product.Rating = rating
			}

			// 设置评价数量
			if reviewCount, ok := basicData["reviewCount"].(float64); ok {
				product.ReviewCount = int(reviewCount)
			}

			// 设置单位
			if unit, ok := basicData["unit"].(string); ok && unit != "" {
				product.Unit = unit
			}

			// 设置是否支持定制
			if isCustomized, ok := basicData["isCustomized"].(bool); ok {
				product.IsCustomized = isCustomized
			}

			logrus.Debugf("提取到基础信息: 标题=%s, ID=%s", product.Title, product.ID)
		}
	}

	// 如果没有获取到ID，从URL中提取
	if product.ID == "" {
		currentURL := page.URL()
		if currentURL != "" {
			// 从URL中提取商品ID，格式如：https://detail.1688.com/offer/981645030344.html
			parts := strings.Split(currentURL, "/")
			for i, part := range parts {
				if part == "offer" && i+1 < len(parts) {
					idPart := parts[i+1]
					if strings.Contains(idPart, ".html") {
						product.ID = strings.Replace(idPart, ".html", "", 1)
						break
					}
				}
			}
		}
		product.URL = currentURL
	}

	return nil
}
