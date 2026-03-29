// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"strings"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/playwright-community/playwright-go"
)

// VariantValuesExtractor 变体值提取器
type VariantValuesExtractor struct{}

// NewVariantValuesExtractor 创建变体值提取器
func NewVariantValuesExtractor() *VariantValuesExtractor {
	return &VariantValuesExtractor{}
}

// Extract 提取变体值信息 - 支持两种数据结构
func (vve *VariantValuesExtractor) Extract(page playwright.Page, product *model.Product1688) error {
	// 从结构化数据中获取变体值信息，支持两种数据结构
	result, err := page.Evaluate(`() => {
		const variationGroups = [];
		
		// 方案1：优先尝试从window.context结构化数据中获取（普通商品）
		if (window.context && window.context.result && window.context.result.data && 
			window.context.result.data.Root && window.context.result.data.Root.fields && 
			window.context.result.data.Root.fields.dataJson && 
			window.context.result.data.Root.fields.dataJson.skuModel && 
			window.context.result.data.Root.fields.dataJson.skuModel.skuProps) {
			
			const skuProps = window.context.result.data.Root.fields.dataJson.skuModel.skuProps;
			
			skuProps.forEach(prop => {
				if (prop.prop && prop.value && Array.isArray(prop.value)) {
					const values = prop.value.map(item => item.name).filter(name => name && name.trim());
					
					if (values.length > 0) {
						variationGroups.push({
							variant_name: prop.prop,
							values: values
						});
					}
				}
			});
		}
		// 方案2：备选方案 - 从window.__INIT_DATA获取（定制商品）
		else if (window.__INIT_DATA && window.__INIT_DATA.data) {
			const data = window.__INIT_DATA.data;
			
			// 查找包含skuModel的数据块
			let foundSkuModel = false;
			
			// 遍历所有数据块
			for (let key in data) {
				const item = data[key];
				if (!item || !item.data) continue;
				
				const itemData = item.data;
				
				// 检查多种可能的SKU模型结构
				const skuModels = [
					itemData.skuModel,
					itemData.skuModelOrigin, 
					itemData.nySkuModel
				];
				
				for (let skuModel of skuModels) {
					if (skuModel && skuModel.skuProps && Array.isArray(skuModel.skuProps)) {
						skuModel.skuProps.forEach(prop => {
							if (prop.prop && prop.value && Array.isArray(prop.value)) {
								const values = prop.value.map(item => {
									// 处理不同的数据格式
									if (typeof item === 'string') {
										return item;
									} else if (item && item.name) {
										return item.name;
									}
									return null;
								}).filter(name => name && name.trim());
								
								if (values.length > 0) {
									variationGroups.push({
										variant_name: prop.prop,
										values: values
									});
								}
							}
						});
						foundSkuModel = true;
						break;
					}
				}
				
				if (foundSkuModel) break;
			}
			
			// 如果没有找到SKU模型，尝试从属性数据中提取
			if (!foundSkuModel) {
				for (let key in data) {
					const item = data[key];
					if (!item || !item.data) continue;
					
					// 查找属性数据（通常是数组格式的属性信息）
					if (Array.isArray(item.data)) {
						const attributes = item.data;
						
						// 遍历所有属性，查找有多个值的属性（可能是变体属性）
						attributes.forEach(attr => {
							if (attr.name && attr.values && Array.isArray(attr.values) && attr.values.length > 1) {
								const values = attr.values.filter(v => v && v.trim());
								if (values.length > 1) {
									variationGroups.push({
										variant_name: attr.name,
										values: values
									});
								}
							}
						});
						
						if (variationGroups.length > 0) {
							foundSkuModel = true;
							break;
						}
					}
				}
			}
		}
		
		return variationGroups;
	}`, nil)

	if err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取变体值失败: %v", err)
		return err
	}

	var variationValues []model.VariationValue

	if result != nil {
		if variationGroups, ok := result.([]any); ok {
			// 转换为VariationValue结构
			for _, groupInterface := range variationGroups {
				if group, ok := groupInterface.(map[string]any); ok {
					var variationValue model.VariationValue

					// 提取变体名称
					if variantName, ok := group["variant_name"].(string); ok {
						variationValue.VariantName = strings.TrimSpace(variantName)
					}

					// 提取变体值
					if valuesInterface, ok := group["values"].([]any); ok {
						var stringValues []string
						for _, value := range valuesInterface {
							if strValue, ok := value.(string); ok {
								strValue = strings.TrimSpace(strValue)
								if strValue != "" {
									stringValues = append(stringValues, strValue)
								}
							}
						}

						variationValue.Values = stringValues
					}

					// 只添加有效的变体值
					if variationValue.VariantName != "" && len(variationValue.Values) > 0 {
						variationValues = append(variationValues, variationValue)
					}
				}
			}
		}
	}

	product.VariationsValues = variationValues
	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取到 %d 个变体值", len(variationValues))

	return nil
}
