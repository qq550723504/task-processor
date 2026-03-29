// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"strconv"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/playwright-community/playwright-go"
)

// VariantExtractor 新的变体数据提取器
type VariantExtractor struct{}

// NewVariantExtractorNew 创建新的变体数据提取器
func NewVariantExtractor() *VariantExtractor {
	return &VariantExtractor{}
}

// Extract 提取变体数据（包含属性、价格、库存等）- 支持两种数据结构
func (ve *VariantExtractor) Extract(page playwright.Page, product *model.Product1688) error {
	logger.GetGlobalLogger("crawler/alibaba1688").Debug("开始提取变体数据")

	// 直接从结构化数据中提取变体信息，支持两种数据结构
	result, err := page.Evaluate(`() => {
		const variants = [];
		
		console.log('=== 开始变体提取调试 ===');
		
		// 方案1：优先尝试从window.context结构化数据中获取（普通商品）
		if (window.context && window.context.result && window.context.result.data) {
			console.log('发现window.context数据结构');
			const data = window.context.result.data;
			
			// 获取SKU属性定义和SKU信息映射
			let skuProps = [];
			let skuInfoMap = {};
			
			if (data.Root && data.Root.fields && data.Root.fields.dataJson && 
				data.Root.fields.dataJson.skuModel) {
				const skuModel = data.Root.fields.dataJson.skuModel;
				skuProps = skuModel.skuProps || [];
				skuInfoMap = skuModel.skuInfoMap || {};
			}
			
			// 如果没有SKU信息，返回空数组
			if (Object.keys(skuInfoMap).length === 0) {
				console.log('window.context中没有找到SKU信息');
				return variants;
			}
			
			// 构建属性名称映射
			const propNames = skuProps.map(prop => prop.prop || '属性');
			
			// 构建变体图片映射（从skuProps中的value获取）
			const imageMap = {};
			skuProps.forEach(prop => {
				if (prop.value && Array.isArray(prop.value)) {
					prop.value.forEach((item, index) => {
						if (item.imageUrl && item.name) {
							imageMap[item.name] = item.imageUrl;
						}
					});
				}
			});
			
			// 处理每个SKU
			for (const specAttrs in skuInfoMap) {
				const sku = skuInfoMap[specAttrs];
				
				// 解析specAttrs，格式如："双岩板丨颜色备注&gt;45*40*55CM"
				const parts = specAttrs.split('&gt;'); // HTML编码的 >
				if (parts.length === 0) continue;
				
				// 构建属性对象
				const attributes = {};
				const attributeValues = [];
				let variantImage = '';
				
				parts.forEach((part, partIndex) => {
					const value = part.trim();
					if (value) {
						const propName = propNames[partIndex] || '属性' + (partIndex + 1);
						attributes[propName] = value;
						attributeValues.push(value);
						
						// 查找对应的图片
						if (!variantImage && imageMap[value]) {
							variantImage = imageMap[value];
						}
					}
				});
				
				// 创建变体名称
				const variantName = attributeValues.length > 0 ? attributeValues.join(' - ') : specAttrs;
				
				// 创建变体对象
				const variant = {
					name: variantName,
					price: parseFloat(sku.price || sku.discountPrice || '0'),
					stock: parseInt(sku.canBookCount || '0'),
					attributes: attributes,
					image: variantImage,
					skuId: sku.skuId,
					specId: sku.specId
				};
				
				variants.push(variant);
			}
		}
		// 方案2：备选方案 - 从window.__INIT_DATA获取（定制商品）
		else if (window.__INIT_DATA && window.__INIT_DATA.data) {
			console.log('发现window.__INIT_DATA数据结构');
			const data = window.__INIT_DATA.data;
			let foundSkuModel = false;
			let defaultPrice = '10.00'; // 默认价格
			
			console.log('开始查找SKU数据，数据块数量:', Object.keys(data).length);
			
			// 首先查找价格信息
			for (let key in data) {
				const item = data[key];
				if (!item || !item.data) continue;
				
				const itemData = item.data;
				
				// 查找价格信息
				if (itemData.disPriceRanges && Array.isArray(itemData.disPriceRanges) && itemData.disPriceRanges.length > 0) {
					const priceRange = itemData.disPriceRanges[0];
					if (priceRange.price || priceRange.discountPrice) {
						defaultPrice = priceRange.price || priceRange.discountPrice;
						console.log('找到价格信息:', defaultPrice);
						break;
					}
				}
				
				// 也可以从其他价格字段获取
				if (itemData.offerMinPrice) {
					defaultPrice = itemData.offerMinPrice;
					console.log('从offerMinPrice获取价格:', defaultPrice);
					break;
				}
				
				// 从customTradeAttributes获取价格
				if (itemData.customTradeAttributes && itemData.customTradeAttributes.minPrice) {
					defaultPrice = itemData.customTradeAttributes.minPrice.toString();
					console.log('从customTradeAttributes获取价格:', defaultPrice);
					break;
				}
			}
			
			// 优先检查globalData中的nySkuModel（定制商品的主要数据结构）
			if (window.__INIT_DATA.globalData && window.__INIT_DATA.globalData.nySkuModel) {
				console.log('在globalData中找到nySkuModel');
				const skuModel = window.__INIT_DATA.globalData.nySkuModel;
				
				if (skuModel.skuProps && skuModel.skuInfoMap) {
					console.log('globalData.nySkuModel包含完整SKU数据');
					console.log('SKU数量:', Object.keys(skuModel.skuInfoMap).length);
					
					const skuProps = skuModel.skuProps;
					const skuInfoMap = skuModel.skuInfoMap;
					
					// 构建属性名称映射
					const propNames = skuProps.map(prop => prop.prop || '属性');
					console.log('属性名称:', propNames);
					
					// 构建变体图片映射
					const imageMap = {};
					skuProps.forEach(prop => {
						if (prop.value && Array.isArray(prop.value)) {
							prop.value.forEach(item => {
								if (item.imageUrl && item.name) {
									imageMap[item.name] = item.imageUrl;
								}
							});
						}
					});
					
					// 处理每个SKU
					let processedCount = 0;
					for (const specAttrs in skuInfoMap) {
						const sku = skuInfoMap[specAttrs];
						
						// 解析specAttrs，格式如："100%新疆棉-白色&gt;S"
						const parts = specAttrs.split('&gt;');
						
						// 构建属性对象
						const attributes = {};
						const attributeValues = [];
						let variantImage = '';
						
						parts.forEach((part, partIndex) => {
							const value = part.trim();
							if (value) {
								const propName = propNames[partIndex] || '属性' + (partIndex + 1);
								attributes[propName] = value;
								attributeValues.push(value);
								
								// 查找对应的图片
								if (!variantImage && imageMap[value]) {
									variantImage = imageMap[value];
								}
							}
						});
						
						// 创建变体名称
						const variantName = attributeValues.length > 0 ? attributeValues.join(' - ') : specAttrs;
						
						// 创建变体对象，使用找到的默认价格
						const variant = {
							name: variantName,
							price: parseFloat(sku.price || sku.discountPrice || defaultPrice),
							stock: parseInt(sku.canBookCount || '0'),
							attributes: attributes,
							image: variantImage,
							skuId: sku.skuId,
							specId: sku.specId
						};
						
						variants.push(variant);
						processedCount++;
						
						// 只显示前几个SKU的详细信息
						if (processedCount <= 3) {
							console.log('SKU示例', processedCount, ':', specAttrs, '->', variantName, '价格:', variant.price);
						}
					}
					
					console.log('处理完成，变体数量:', variants.length);
					foundSkuModel = true;
				}
			}
			
			// 如果globalData中没有找到，再遍历数据块查找SKU信息
			if (!foundSkuModel) {
				for (let key in data) {
					const item = data[key];
					if (!item || !item.data) continue;
					
					const itemData = item.data;
					
					// 检查多种可能的SKU模型结构，优先检查nySkuModel
					const skuModels = [
						{ model: itemData.nySkuModel, name: 'nySkuModel' },
						{ model: itemData.skuModel, name: 'skuModel' },
						{ model: itemData.skuModelOrigin, name: 'skuModelOrigin' }
					];
					
					for (let skuModelInfo of skuModels) {
						const skuModel = skuModelInfo.model;
						if (skuModel && skuModel.skuProps && skuModel.skuInfoMap) {
							console.log('找到完整SKU数据在:', key, skuModelInfo.name);
							console.log('SKU数量:', Object.keys(skuModel.skuInfoMap).length);
							
							const skuProps = skuModel.skuProps;
							const skuInfoMap = skuModel.skuInfoMap;
							
							// 构建属性名称映射
							const propNames = skuProps.map(prop => prop.prop || '属性');
							console.log('属性名称:', propNames);
							
							// 构建变体图片映射
							const imageMap = {};
							skuProps.forEach(prop => {
								if (prop.value && Array.isArray(prop.value)) {
									prop.value.forEach(item => {
										if (item.imageUrl && item.name) {
											imageMap[item.name] = item.imageUrl;
										}
									});
								}
							});
							
							// 处理每个SKU
							let processedCount = 0;
							for (const specAttrs in skuInfoMap) {
								const sku = skuInfoMap[specAttrs];
								
								// 解析specAttrs，格式如："100%新疆棉-白色&gt;S"
								const parts = specAttrs.split('&gt;');
								
								// 构建属性对象
								const attributes = {};
								const attributeValues = [];
								let variantImage = '';
								
								parts.forEach((part, partIndex) => {
									const value = part.trim();
									if (value) {
										const propName = propNames[partIndex] || '属性' + (partIndex + 1);
										attributes[propName] = value;
										attributeValues.push(value);
										
										// 查找对应的图片
										if (!variantImage && imageMap[value]) {
											variantImage = imageMap[value];
										}
									}
								});
								
								// 创建变体名称
								const variantName = attributeValues.length > 0 ? attributeValues.join(' - ') : specAttrs;
								
								// 创建变体对象，使用找到的默认价格
								const variant = {
									name: variantName,
									price: parseFloat(sku.price || sku.discountPrice || defaultPrice),
									stock: parseInt(sku.canBookCount || '0'),
									attributes: attributes,
									image: variantImage,
									skuId: sku.skuId,
									specId: sku.specId
								};
								
								variants.push(variant);
								processedCount++;
								
								// 只显示前几个SKU的详细信息
								if (processedCount <= 3) {
									console.log('SKU示例', processedCount, ':', specAttrs, '->', variantName, '价格:', variant.price);
								}
							}
							
							console.log('处理完成，变体数量:', variants.length);
							foundSkuModel = true;
							break;
						}
					}
					
					if (foundSkuModel) break;
				}
			}
			
			if (!foundSkuModel) {
				console.log('未找到任何完整的SKU数据');
				// 列出所有数据块的key，帮助调试
				console.log('所有数据块key:', Object.keys(data).slice(0, 20));
				
				// 检查每个数据块是否包含SKU相关信息
				for (let key in data) {
					const item = data[key];
					if (!item || !item.data) continue;
					
					const itemData = item.data;
					if (itemData.nySkuModel || itemData.skuModel || itemData.skuModelOrigin) {
						console.log('数据块', key, '包含SKU模型:');
						if (itemData.nySkuModel) {
							console.log('  - nySkuModel存在, skuProps:', !!itemData.nySkuModel.skuProps, 'skuInfoMap:', !!itemData.nySkuModel.skuInfoMap);
							if (itemData.nySkuModel.skuInfoMap) {
								console.log('    - skuInfoMap键数量:', Object.keys(itemData.nySkuModel.skuInfoMap).length);
								console.log('    - 前3个键:', Object.keys(itemData.nySkuModel.skuInfoMap).slice(0, 3));
							}
						}
						if (itemData.skuModel) {
							console.log('  - skuModel存在, skuProps:', !!itemData.skuModel.skuProps, 'skuInfoMap:', !!itemData.skuModel.skuInfoMap);
						}
						if (itemData.skuModelOrigin) {
							console.log('  - skuModelOrigin存在, skuProps:', !!itemData.skuModelOrigin.skuProps, 'skuInfoMap:', !!itemData.skuModelOrigin.skuInfoMap);
						}
					}
				}
			}
		} else {
			console.log('未找到任何数据结构 (window.context 或 window.__INIT_DATA)');
		}
		
		console.log('=== 变体提取调试结束，最终变体数量:', variants.length, '===');
		return variants;
	}`, nil)

	if err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("JavaScript执行失败: %v", err)
		return err
	}

	// 解析JavaScript返回的结果
	if result != nil {
		if variantArray, ok := result.([]any); ok {
			variants := make([]model.Variant, 0, len(variantArray))

			for _, variantInterface := range variantArray {
				if variantData, ok := variantInterface.(map[string]any); ok {
					name, _ := variantData["name"].(string)
					image, _ := variantData["image"].(string)

					// 直接使用JavaScript返回的attributes
					var attributes map[string]any
					if attrs, exists := variantData["attributes"]; exists {
						if attrsMap, ok := attrs.(map[string]any); ok {
							attributes = attrsMap
						}
					}
					if attributes == nil {
						attributes = make(map[string]any)
					}

					// 处理价格类型转换
					var price float64
					if priceVal, exists := variantData["price"]; exists {
						switch v := priceVal.(type) {
						case float64:
							price = v
						case int:
							price = float64(v)
						case string:
							if p, err := strconv.ParseFloat(v, 64); err == nil {
								price = p
							}
						}
					}

					// 处理库存类型转换
					var stock int
					if stockVal, exists := variantData["stock"]; exists {
						switch v := stockVal.(type) {
						case float64:
							stock = int(v)
						case int:
							stock = v
						case string:
							if s, err := strconv.Atoi(v); err == nil {
								stock = s
							}
						}
					}

					if name != "" {
						variant := model.Variant{
							Name:       name,
							Image:      image,
							Stock:      stock,
							Price:      price,
							Attributes: attributes,
						}
						variants = append(variants, variant)
					}
				}
			}

			product.Variants = variants

			// 计算价格范围
			if len(variants) > 0 {
				minPrice := variants[0].Price
				maxPrice := variants[0].Price

				for _, variant := range variants {
					if variant.Price > 0 {
						if minPrice == 0 || variant.Price < minPrice {
							minPrice = variant.Price
						}
						if variant.Price > maxPrice {
							maxPrice = variant.Price
						}
					}
				}

				if minPrice > 0 {
					product.MinPrice = minPrice
					product.MaxPrice = maxPrice
					logger.GetGlobalLogger("crawler/alibaba1688").Debugf("设置价格范围: %.2f - %.2f", minPrice, maxPrice)
				}
			}

			logger.GetGlobalLogger("crawler/alibaba1688").Debugf("成功提取 %d 个变体", len(variants))
		}
	}

	return nil
}
