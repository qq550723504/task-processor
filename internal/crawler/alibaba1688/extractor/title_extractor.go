// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/playwright-community/playwright-go"
)

// TitleExtractor 标题提取器
type TitleExtractor struct{}

// NewTitleExtractor 创建标题提取器
func NewTitleExtractor() *TitleExtractor {
	return &TitleExtractor{}
}

// Extract 提取产品标题和基础信息 - 直接从结构化数据获取
func (te *TitleExtractor) Extract(page playwright.Page, product *model.Product1688) error {
	// 从结构化数据中获取标题和ID信息
	titleResult, err := page.Evaluate(`() => {
		const result = {
			title: '',
			id: '',
			url: window.location.href
		};
		
		// 从window.context结构化数据中获取
		if (window.context && window.context.result && window.context.result.data) {
			const data = window.context.result.data;
			
			// 方法1：从productTitle.fields获取标题
			if (data.productTitle && data.productTitle.fields && data.productTitle.fields.title) {
				result.title = data.productTitle.fields.title;
			}
			
			// 方法2：从Root.fields.dataJson.tempModel获取标题和ID
			if (data.Root && data.Root.fields && data.Root.fields.dataJson && 
				data.Root.fields.dataJson.tempModel) {
				const tempModel = data.Root.fields.dataJson.tempModel;
				
				if (!result.title && tempModel.offerTitle) {
					result.title = tempModel.offerTitle;
				}
				if (tempModel.offerId) {
					result.id = tempModel.offerId.toString();
				}
			}
		}
		
		return result;
	}`, nil)

	if err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取标题信息失败: %v", err)
		return err
	}

	if titleResult != nil {
		if titleData, ok := titleResult.(map[string]any); ok {
			// 设置标题
			if title, ok := titleData["title"].(string); ok && title != "" {
				product.Title = title
				logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取到产品标题: %s", title)
			}

			// 设置ID
			if id, ok := titleData["id"].(string); ok && id != "" {
				product.ID = id
				logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取到产品ID: %s", id)
			}

			// 设置URL
			if url, ok := titleData["url"].(string); ok && url != "" {
				product.URL = url
			}
		}
	}

	return nil
}
