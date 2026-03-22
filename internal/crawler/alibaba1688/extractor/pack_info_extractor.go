// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/playwright-community/playwright-go"
)

// PackInfoExtractor 产品包装信息提取器
type PackInfoExtractor struct{}

// NewPackInfoExtractor 创建产品包装信息提取器
func NewPackInfoExtractor() *PackInfoExtractor {
	return &PackInfoExtractor{}
}

// Extract 提取产品包装信息 - 支持两种数据结构
func (pie *PackInfoExtractor) Extract(page playwright.Page, product *model.Product1688) error {
	logger.GetGlobalLogger("crawler/alibaba1688").Debug("开始提取产品包装信息")

	// 从结构化数据中获取包装信息，支持两种数据结构
	packInfoResult, err := page.Evaluate(`() => {
		const result = {
			unitWeight: 0
		};
		
		// 方案1：优先尝试从window.context结构化数据中获取（普通商品）
		if (window.context && window.context.result && window.context.result.data && 
			window.context.result.data.productPackInfo && 
			window.context.result.data.productPackInfo.fields) {
			
			const packInfoFields = window.context.result.data.productPackInfo.fields;
			result.unitWeight = packInfoFields.unitWeight || 0;
		}
		// 方案2：备选方案 - 从window.__INIT_DATA获取（定制商品）
		else if (window.__INIT_DATA && window.__INIT_DATA.data) {
			const data = window.__INIT_DATA.data;
			
			// 遍历数据块查找包装信息
			for (let key in data) {
				const item = data[key];
				if (!item || !item.data) continue;
				
				const itemData = item.data;
				
				// 查找包装相关信息
				if (itemData.packInfo && itemData.packInfo.unitWeight) {
					result.unitWeight = itemData.packInfo.unitWeight;
					break;
				}
				
				// 从unitWeight字段获取重量信息
				if (itemData.unitWeight) {
					result.unitWeight = parseFloat(itemData.unitWeight) || 0;
					break;
				}
				
				// 从freightInfo中获取重量信息
				if (itemData.freightInfo && itemData.freightInfo.unitWeight) {
					result.unitWeight = parseFloat(itemData.freightInfo.unitWeight) || 0;
					break;
				}
				
				// 或者从其他字段查找重量信息
				if (itemData.weight) {
					result.unitWeight = parseFloat(itemData.weight) || 0;
					break;
				}
			}
		}
		
		return result;
	}`, nil)

	if err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取包装信息失败: %v", err)
		return err
	}

	if packInfoResult != nil {
		if packData, ok := packInfoResult.(map[string]any); ok {
			if unitWeight, ok := packData["unitWeight"].(float64); ok && unitWeight > 0 {
				packInfo := &model.PackInfo{
					PackageType:     "标准包装",
					Weight:          unitWeight,
					PackageContents: []string{},
				}

				product.PackInfo = packInfo
				logger.GetGlobalLogger("crawler/alibaba1688").Debugf("成功提取包装信息，重量: %.0fg", unitWeight)
			}
		}
	}

	return nil
}
