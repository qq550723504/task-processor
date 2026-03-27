// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"task-processor/internal/core/logger"
	"strconv"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/playwright-community/playwright-go"
)

// PriceExtractor 价格提取器（包含价格范围功能）
type PriceExtractor struct{}

// NewPriceExtractor 创建价格提取器
func NewPriceExtractor() *PriceExtractor {
	return &PriceExtractor{}
}

// Extract 提取价格信息 - 支持两种数据结构
func (pe *PriceExtractor) Extract(page playwright.Page, product *model.Product1688) error {
	logger.GetGlobalLogger("crawler/alibaba1688").Debug("开始提取价格信息")

	// 从结构化数据中获取价格信息，支持两种数据结构
	priceResult, err := page.Evaluate(`() => {
		const result = {
			priceRanges: [],
			minPrice: 0,
			maxPrice: 0,
			minOrderQuantity: 1
		};
		
		// 方案1：优先尝试从window.context结构化数据中获取（普通商品）
		if (window.context && window.context.result && window.context.result.data && 
			window.context.result.data.Root && window.context.result.data.Root.fields && 
			window.context.result.data.Root.fields.dataJson) {
			
			const dataJson = window.context.result.data.Root.fields.dataJson;
			
			// 方法1：优先从offerPriceRanges获取（包含endAmount）
			const contextData = window.context.result.data;
			for (const key in contextData) {
				const item = contextData[key];
				if (item && item.fields && item.fields.finalPriceModel && 
					item.fields.finalPriceModel.tradeWithoutPromotion && 
					item.fields.finalPriceModel.tradeWithoutPromotion.offerPriceRanges) {
					
					const offerPriceRanges = item.fields.finalPriceModel.tradeWithoutPromotion.offerPriceRanges;
					const prices = [];
					
					offerPriceRanges.forEach(priceItem => {
						const price = parseFloat(priceItem.price);
						const beginAmount = Math.max(1, parseInt(priceItem.beginAmount) || 1);
						const endAmount = parseInt(priceItem.endAmount) || 0; // 0表示无上限
						if (price > 0) {
							prices.push(price);
							result.priceRanges.push({
								minQuantity: beginAmount,
								maxQuantity: endAmount,
								price: price
							});
						}
					});
					
					if (prices.length > 0) {
						result.minPrice = Math.min(...prices);
						result.maxPrice = Math.max(...prices);
					}
					break;
				}
			}
			
			// 方法2：如果方法1没有获取到价格，从skuRangePrices获取价格区间
			if (result.minPrice === 0 && dataJson.orderParamModel && dataJson.orderParamModel.orderParam && 
				dataJson.orderParamModel.orderParam.skuParam && 
				dataJson.orderParamModel.orderParam.skuParam.skuRangePrices) {
				
				const rangePrices = dataJson.orderParamModel.orderParam.skuParam.skuRangePrices;
				const prices = [];
				
				rangePrices.forEach(item => {
					const price = parseFloat(item.price);
					const beginAmount = Math.max(1, parseInt(item.beginAmount) || 1);
					if (price > 0) {
						prices.push(price);
						result.priceRanges.push({
							minQuantity: beginAmount,
							maxQuantity: 0,
							price: price
						});
					}
				});
				
				if (prices.length > 0) {
					result.minPrice = Math.min(...prices);
					result.maxPrice = Math.max(...prices);
				}
			}
			
			// 方法3：如果还没有获取到价格，尝试从currentPrices获取
			if (result.minPrice === 0) {
				for (const key in contextData) {
					const item = contextData[key];
					if (item && item.fields && item.fields.priceModel && item.fields.priceModel.currentPrices) {
						const currentPrices = item.fields.priceModel.currentPrices;
						const prices = [];
						
						currentPrices.forEach(priceItem => {
							const price = parseFloat(priceItem.price);
							const beginAmount = Math.max(1, parseInt(priceItem.beginAmount) || 1);
							if (price > 0) {
								prices.push(price);
								result.priceRanges.push({
									minQuantity: beginAmount,
									maxQuantity: 0,
									price: price
								});
							}
						});
						
						if (prices.length > 0) {
							result.minPrice = Math.min(...prices);
							result.maxPrice = Math.max(...prices);
						}
						break;
					}
				}
			}
			
			// 获取起订量
			if (dataJson.orderParamModel && dataJson.orderParamModel.orderParam && 
				dataJson.orderParamModel.orderParam.beginNum) {
				result.minOrderQuantity = dataJson.orderParamModel.orderParam.beginNum;
			}
		}
		// 方案2：备选方案 - 从window.__INIT_DATA获取（定制商品）
		else if (window.__INIT_DATA && window.__INIT_DATA.globalData) {
			const globalData = window.__INIT_DATA.globalData;
			
			// 方法1：从orderParamModel.orderParam.skuParam.skuRangePrices获取价格区间
			if (globalData.orderParamModel && globalData.orderParamModel.orderParam && 
				globalData.orderParamModel.orderParam.skuParam && 
				globalData.orderParamModel.orderParam.skuParam.skuRangePrices) {
				
				const rangePrices = globalData.orderParamModel.orderParam.skuParam.skuRangePrices;
				const prices = [];
				
				rangePrices.forEach(item => {
					const price = parseFloat(item.price);
					const beginAmount = Math.max(1, parseInt(item.beginAmount) || 1);
					if (price > 0) {
						prices.push(price);
						result.priceRanges.push({
							minQuantity: beginAmount,
							maxQuantity: 0,
							price: price
						});
					}
				});
				
				if (prices.length > 0) {
					result.minPrice = Math.min(...prices);
					result.maxPrice = Math.max(...prices);
				}
				
				// 获取起订量
				if (globalData.orderParamModel.orderParam.beginNum) {
					result.minOrderQuantity = globalData.orderParamModel.orderParam.beginNum;
				}
			}
			
			// 方法2：如果方法1没有获取到价格，从data中查找priceModel
			if (result.minPrice === 0 && window.__INIT_DATA.data) {
				const data = window.__INIT_DATA.data;
				
				for (let key in data) {
					const item = data[key];
					if (!item || !item.data) continue;
					
					const itemData = item.data;
					
					if (itemData.priceModel && itemData.priceModel.currentPrices) {
						const currentPrices = itemData.priceModel.currentPrices;
						
						if (Array.isArray(currentPrices) && currentPrices.length > 0) {
							const prices = currentPrices.map(p => parseFloat(p.price || 0)).filter(p => p > 0);
							if (prices.length > 0) {
								result.minPrice = Math.min(...prices);
								result.maxPrice = Math.max(...prices);
								
								currentPrices.forEach(priceItem => {
									const price = parseFloat(priceItem.price || 0);
									const beginAmount = Math.max(1, parseInt(priceItem.beginAmount || 1));
									if (price > 0) {
										result.priceRanges.push({
											minQuantity: beginAmount,
											maxQuantity: 0,
											price: price
										});
									}
								});
								
								// 提取起订量
								const firstPrice = currentPrices[0];
								if (firstPrice.beginAmount) {
									result.minOrderQuantity = parseInt(firstPrice.beginAmount) || 1;
								}
							}
						}
						break;
					}
				}
			}
		}
		
		return result;
	}`, nil)

	if err != nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取价格信息失败: %v", err)
		return err
	}

	return pe.processPriceResult(priceResult, product)
}

// processPriceResult 处理JavaScript返回的价格数据
func (pe *PriceExtractor) processPriceResult(priceResult any, product *model.Product1688) error {
	if priceResult == nil {
		return nil
	}

	priceData, ok := priceResult.(map[string]any)
	if !ok {
		return nil
	}

	// 设置基础价格信息
	pe.setBasicPriceInfo(priceData, product)

	// 设置价格区间
	pe.setPriceRanges(priceData, product)

	// 验证和修复价格区间
	pe.validateAndFixPriceRanges(product)

	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("提取到价格信息: %.2f-%.2f, 起订量=%d, 价格区间数=%d",
		product.MinPrice, product.MaxPrice, product.MinOrderQuantity, len(product.PriceRanges))

	return nil
}

// setBasicPriceInfo 设置基础价格信息
func (pe *PriceExtractor) setBasicPriceInfo(priceData map[string]any, product *model.Product1688) {
	if minPrice, ok := priceData["minPrice"].(float64); ok && minPrice > 0 {
		product.MinPrice = minPrice
	}
	if maxPrice, ok := priceData["maxPrice"].(float64); ok && maxPrice > 0 {
		product.MaxPrice = maxPrice
	}

	if minOrderQty, ok := priceData["minOrderQuantity"].(float64); ok && minOrderQty > 0 {
		product.MinOrderQuantity = int(minOrderQty)
	} else {
		product.MinOrderQuantity = 1
	}
}

// setPriceRanges 设置价格区间
func (pe *PriceExtractor) setPriceRanges(priceData map[string]any, product *model.Product1688) {
	priceRanges, ok := priceData["priceRanges"].([]any)
	if !ok || len(priceRanges) == 0 {
		pe.createDefaultPriceRange(product)
		return
	}

	var ranges []model.PriceRange
	for _, rangeInterface := range priceRanges {
		if rangeData, ok := rangeInterface.(map[string]any); ok {
			priceRange := pe.parsePriceRange(rangeData)
			if priceRange.Price > 0 {
				ranges = append(ranges, priceRange)
			}
		}
	}
	product.PriceRanges = ranges

	if len(ranges) == 0 {
		pe.createDefaultPriceRange(product)
	}
}

// parsePriceRange 解析单个价格区间
func (pe *PriceExtractor) parsePriceRange(rangeData map[string]any) model.PriceRange {
	var priceRange model.PriceRange

	// 处理minQuantity
	if minQty, ok := rangeData["minQuantity"].(float64); ok {
		priceRange.MinQuantity = int(minQty)
	} else if minQty, ok := rangeData["minQuantity"].(int); ok {
		priceRange.MinQuantity = minQty
	}

	// 处理maxQuantity
	if maxQty, ok := rangeData["maxQuantity"].(float64); ok {
		priceRange.MaxQuantity = int(maxQty)
	} else if maxQty, ok := rangeData["maxQuantity"].(int); ok {
		priceRange.MaxQuantity = maxQty
	}

	// 处理price
	if price, ok := rangeData["price"].(float64); ok {
		priceRange.Price = price
	} else if priceInt, ok := rangeData["price"].(int); ok {
		priceRange.Price = float64(priceInt)
	} else if priceStr, ok := rangeData["price"].(string); ok {
		if parsedPrice, err := strconv.ParseFloat(priceStr, 64); err == nil {
			priceRange.Price = parsedPrice
		}
	}

	return priceRange
}

// createDefaultPriceRange 创建默认价格区间
func (pe *PriceExtractor) createDefaultPriceRange(product *model.Product1688) {
	if product.MinPrice <= 0 {
		return
	}

	minQty := product.MinOrderQuantity
	if minQty <= 0 {
		minQty = 1
	}

	priceRange := model.PriceRange{
		MinQuantity: minQty,
		MaxQuantity: 0,
		Price:       product.MinPrice,
	}
	product.PriceRanges = []model.PriceRange{priceRange}
}

// validateAndFixPriceRanges 验证和修复价格区间
func (pe *PriceExtractor) validateAndFixPriceRanges(product *model.Product1688) {
	var validRanges []model.PriceRange
	var validPrices []float64

	for _, priceRange := range product.PriceRanges {
		if priceRange.Price > 0 && priceRange.MinQuantity > 0 {
			validRanges = append(validRanges, priceRange)
			validPrices = append(validPrices, priceRange.Price)
		}
	}

	product.PriceRanges = validRanges

	// 重新计算价格范围
	if len(validPrices) > 0 {
		minPrice := validPrices[0]
		maxPrice := validPrices[0]
		for _, price := range validPrices {
			if price < minPrice {
				minPrice = price
			}
			if price > maxPrice {
				maxPrice = price
			}
		}
		product.MinPrice = minPrice
		product.MaxPrice = maxPrice
	}
}
