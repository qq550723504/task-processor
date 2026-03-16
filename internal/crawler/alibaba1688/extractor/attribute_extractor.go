// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"strings"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// AttributeExtractor 优化的商品属性提取器
type AttributeExtractor struct{}

// NewAttributeExtractor 创建优化的商品属性提取器
func NewAttributeExtractor() *AttributeExtractor {
	return &AttributeExtractor{}
}

// Extract 提取商品属性信息 - 支持两种数据结构
func (aeo *AttributeExtractor) Extract(page playwright.Page, product *model.Product1688) error {
	logrus.Debug("开始提取商品属性信息")

	// 直接从结构化数据中获取属性信息，支持两种数据结构
	attrResult, err := page.Evaluate(`() => {
		const result = {
			attributes: []
		};
		
		// 方案1：优先尝试从window.context结构化数据中获取（普通商品）
		if (window.context && window.context.result && window.context.result.global && 
			window.context.result.global.globalData && window.context.result.global.globalData.model &&
			window.context.result.global.globalData.model.offerDetail &&
			window.context.result.global.globalData.model.offerDetail.featureAttributes) {
			
			const featureAttributes = window.context.result.global.globalData.model.offerDetail.featureAttributes;
			
			// 直接使用featureAttributes数据
			if (Array.isArray(featureAttributes)) {
				featureAttributes.forEach(attr => {
					if (attr.name && attr.value && attr.name !== attr.value) {
						result.attributes.push({
							name: attr.name,
							value: attr.value,
							fid: attr.fid || null
						});
					}
				});
			}
		}
		// 方案2：备选方案 - 从window.__INIT_DATA获取（定制商品）
		else if (window.__INIT_DATA && window.__INIT_DATA.data) {
			const data = window.__INIT_DATA.data;
			
			// 遍历数据块查找属性信息
			for (let key in data) {
				const item = data[key];
				if (!item || !item.data) continue;
				
				const itemData = item.data;
				
				// 查找属性数组（通常是774504306799这种key）
				if (Array.isArray(itemData) && itemData.length > 0) {
					// 检查是否是属性数组（每个元素都有name、value、fid等字段）
					const firstItem = itemData[0];
					if (firstItem && firstItem.name && firstItem.value && firstItem.fid) {
						console.log('找到属性数组，数量:', itemData.length);
						
						itemData.forEach(attr => {
							if (attr.name && attr.value && attr.name !== attr.value) {
								// 过滤掉变体属性（颜色、尺码）
								if (attr.name !== '颜色' && attr.name !== '尺码') {
									result.attributes.push({
										name: attr.name,
										value: attr.value,
										fid: attr.fid || null
									});
								}
							}
						});
						
						console.log('提取到属性数量:', result.attributes.length);
						break;
					}
				}
			}
			
			// 如果没有找到属性数组，尝试从页面DOM中提取属性表格
			if (result.attributes.length === 0) {
				const attrTables = document.querySelectorAll('table, .attr-table, [class*="attr"], [class*="spec"]');
				
				attrTables.forEach(table => {
					const rows = table.querySelectorAll('tr, .attr-row, [class*="row"]');
					
					rows.forEach(row => {
						const cells = row.querySelectorAll('td, th, .attr-name, .attr-value, [class*="cell"]');
						
						if (cells.length >= 2) {
							const name = cells[0].textContent?.trim();
							const value = cells[1].textContent?.trim();
							
							if (name && value && name !== value && 
								!name.includes('：') && !name.includes(':') &&
								name.length < 50 && value.length < 200) {
								
								// 过滤掉一些无用的属性
								const skipNames = ['价格', '起订量', '供货总量', '发货期限', '所在地', '颜色', '尺码'];
								if (!skipNames.includes(name)) {
									result.attributes.push({
										name: name,
										value: value,
										fid: null
									});
								}
							}
						}
					});
				});
			}
			
			// 如果还是没有找到属性，添加一些基础属性
			if (result.attributes.length === 0) {
				result.attributes.push({
					name: '商品类型',
					value: '定制商品',
					fid: null
				});
				
				result.attributes.push({
					name: '定制服务',
					value: '支持个性化定制',
					fid: null
				});
			}
		}
		
		return result;
	}`, nil)

	if err != nil {
		logrus.Debugf("提取属性失败: %v", err)
		return err
	}

	var attributes []model.Specification

	if attrResult != nil {
		if attrData, ok := attrResult.(map[string]any); ok {
			if attrs, ok := attrData["attributes"].([]any); ok {
				for _, attr := range attrs {
					if attrInfo, ok := attr.(map[string]any); ok {
						if name, nameOk := attrInfo["name"].(string); nameOk {
							if value, valueOk := attrInfo["value"].(string); valueOk {
								attributes = append(attributes, model.Specification{
									Name:  strings.TrimSpace(name),
									Value: strings.TrimSpace(value),
								})
							}
						}
					}
				}
			}

			logrus.Debugf("提取到 %d 个商品属性", len(attributes))
		}
	}

	product.Specifications = attributes
	return nil
}
