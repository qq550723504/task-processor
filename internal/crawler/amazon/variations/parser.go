package variations

import (
	"github.com/playwright-community/playwright-go"
)

// Parser JavaScript数据解析器
type Parser struct {
	config *Config
	mapper *Mapper
}

// NewParser 创建解析器
func NewParser(config *Config) *Parser {
	return &Parser{
		config: config,
		mapper: NewMapper(config),
	}
}

// ParseVariationsData 从页面提取产品变体值
func (p *Parser) ParseVariationsData(page playwright.Page) (*VariationsData, error) {
	result := &VariationsData{
		VariationsValues: make(map[string][]string),
		ASINMapping:      make(map[string]map[string]string),
	}

	// 从JavaScript数据中提取变体信息
	jsResult, err := page.Evaluate(p.getJavaScriptExtractor())
	if err != nil {
		return result, err
	}

	// 处理JavaScript返回的数据
	if jsResult != nil {
		if jsMap, ok := jsResult.(map[string]any); ok {
			p.processJavaScriptResult(jsMap, result)
		}
	}

	return result, nil
}

// getJavaScriptExtractor 返回JavaScript提取脚本
func (p *Parser) getJavaScriptExtractor() string {
	return `() => {
		const scripts = document.querySelectorAll('script');
		let variationsData = {};
		let debugInfo = [];
		let priceMapping = {};
		
		for (let script of scripts) {
			const content = script.textContent || script.innerHTML;
			
			// 查找价格相关数据
			if (content.includes('priceblock') || content.includes('priceToDisplay') || content.includes('displayPrice')) {
				const priceRegexes = [
					/priceToDisplay['"]*\s*:\s*['"]*([0-9.,]+)['"]*\s*[,}]/g,
					/displayPrice['"]*\s*:\s*['"]*([0-9.,]+)['"]*\s*[,}]/g,
					/price['"]*\s*:\s*['"]*([0-9.,]+)['"]*\s*[,}]/g
				];
				
				for (let regex of priceRegexes) {
					let match;
					while ((match = regex.exec(content)) !== null) {
						debugInfo.push('Found price: ' + match[1]);
					}
				}
			}
			
			// 查找 twisterData 中的价格信息
			if (content.includes('twisterData')) {
				debugInfo.push('Found script containing twisterData');
				const twisterRegex = /twisterData['"]*\s*:\s*({[\s\S]*?})\s*[,}]/;
				const twisterMatch = content.match(twisterRegex);
				
				if (twisterMatch) {
					debugInfo.push('Successfully matched twisterData regex');
					try {
						const twisterData = JSON.parse(twisterMatch[1]);
						debugInfo.push('twisterData keys: ' + Object.keys(twisterData).join(', '));
						
						if (twisterData.priceMap) {
							priceMapping = twisterData.priceMap;
							debugInfo.push('Found priceMap in twisterData');
						}
						
						if (twisterData.asinPriceMap) {
							priceMapping = twisterData.asinPriceMap;
							debugInfo.push('Found asinPriceMap in twisterData');
						}
					} catch (e) {
						debugInfo.push('Error parsing twisterData: ' + e.message);
					}
				}
			}
			
			// 查找 variationValues
			let realAttributeNames = [];
			if (content.includes('variationValues')) {
				const variationRegex = /variationValues['"]*\s*:\s*({[\s\S]*?})\s*[,}]/;
				const variationMatch = content.match(variationRegex);
				
				if (variationMatch) {
					try {
						const variationParsed = JSON.parse(variationMatch[1]);
						realAttributeNames = Object.keys(variationParsed);
						debugInfo.push('Found real attribute names: ' + realAttributeNames.join(', '));
						
						for (let attrName of realAttributeNames) {
							if (variationParsed[attrName] && Array.isArray(variationParsed[attrName])) {
								variationsData[attrName] = variationParsed[attrName];
							}
						}
					} catch (e) {
						debugInfo.push('Error parsing variationValues: ' + e.message);
					}
				}
			}
			
			// 查找 dimensionValuesDisplayData
			if (content.includes('dimensionValuesDisplayData')) {
				const regex = /dimensionValuesDisplayData['"]*\s*:\s*({[\s\S]*?})\s*[,}]/;
				const match = content.match(regex);
				
				if (match) {
					try {
						const parsed = JSON.parse(match[1]);
						let attributeValues = {};
						let asinMapping = {};
						
						for (let asin in parsed) {
							const values = parsed[asin];
							
							if (Array.isArray(values) && values.length > 0) {
								let asinAttrs = {};
								
								if (realAttributeNames.length > 0 && realAttributeNames.length >= values.length) {
									for (let i = 0; i < values.length; i++) {
										asinAttrs[realAttributeNames[i]] = values[i];
									}
								} else {
									if (values.length === 1) {
										const value = values[0];
										if (typeof value === 'string') {
											if (value.includes(':')) {
												const parts = value.split(':');
												asinAttrs['variant_code'] = parts[0].trim();
												asinAttrs['variant_style'] = parts[1].trim();
											} else {
												asinAttrs['variant'] = value;
											}
										}
									} else if (values.length === 2) {
										asinAttrs['attribute_1'] = values[0];
										asinAttrs['attribute_2'] = values[1];
									} else {
										for (let i = 0; i < values.length; i++) {
											asinAttrs['attribute_' + (i + 1)] = values[i];
										}
									}
								}
								
								if (priceMapping && priceMapping[asin]) {
									asinAttrs['price_info'] = priceMapping[asin];
								}
								
								asinMapping[asin] = asinAttrs;
								
								for (let attrKey in asinAttrs) {
									if (attrKey !== 'price_info') {
										if (!attributeValues[attrKey]) {
											attributeValues[attrKey] = new Set();
										}
										attributeValues[attrKey].add(asinAttrs[attrKey]);
									}
								}
							}
						}
						
						if (realAttributeNames.length === 0) {
							for (let attrKey in attributeValues) {
								variationsData[attrKey] = Array.from(attributeValues[attrKey]);
							}
						}
						
						variationsData['asin_mapping'] = asinMapping;
						
						if (Object.keys(priceMapping).length > 0) {
							variationsData['price_mapping'] = priceMapping;
						}
					} catch (e) {
						debugInfo.push('Error parsing dimensionValuesDisplayData: ' + e.message);
					}
				}
			}
			
			// 查找 colorImages
			if (content.includes('colorImages')) {
				const regex = /colorImages['"]*\s*:\s*({[\s\S]*?})\s*[,}]/;
				const match = content.match(regex);
				
				if (match) {
					try {
						const parsed = JSON.parse(match[1]);
						if (parsed.initial && typeof parsed.initial === 'object') {
							const colors = Object.keys(parsed.initial);
							if (colors.length > 0) {
								variationsData['color'] = colors;
							}
						}
					} catch (e) {
						debugInfo.push('Error parsing colorImages: ' + e.message);
					}
				}
			}
		}
		
		return {
			variationsData: Object.keys(variationsData).length > 0 ? variationsData : null,
			debugInfo: debugInfo
		};
	}`
}

// processJavaScriptResult 处理JavaScript返回的结果
func (p *Parser) processJavaScriptResult(jsMap map[string]any, result *VariationsData) {
	if variationsData, exists := jsMap["variationsData"]; exists && variationsData != nil {
		if variationsMap, ok := variationsData.(map[string]any); ok {
			for key, value := range variationsMap {
				if key == "asin_mapping" {
					p.processASINMapping(value, result)
				} else if key == "price_mapping" {
					p.processPriceMapping(value, result)
				} else if valueArray, ok := value.([]any); ok {
					p.processVariationValues(key, valueArray, result)
				}
			}
		}
	}
}

// processASINMapping 处理ASIN映射
func (p *Parser) processASINMapping(value any, result *VariationsData) {
	if asinMappingData, ok := value.(map[string]any); ok {
		for asin, attributes := range asinMappingData {
			if attrMap, ok := attributes.(map[string]any); ok {
				asinAttrs := make(map[string]string)
				for attrKey, attrValue := range attrMap {
					if str, ok := attrValue.(string); ok {
						asinAttrs[attrKey] = str
					}
				}
				result.ASINMapping[asin] = asinAttrs
			}
		}
	}
}

// processPriceMapping 处理价格映射
func (p *Parser) processPriceMapping(value any, result *VariationsData) {
	if priceMappingData, ok := value.(map[string]any); ok {
		result.PriceMapping = priceMappingData
	}
}

// processVariationValues 处理变体值
func (p *Parser) processVariationValues(key string, valueArray []any, result *VariationsData) {
	var stringArray []string
	for _, item := range valueArray {
		if str, ok := item.(string); ok {
			stringArray = append(stringArray, str)
		}
	}
	if len(stringArray) > 0 {
		normalizedKey := p.mapper.NormalizeKey(key)
		result.VariationsValues[normalizedKey] = removeDuplicates(stringArray)
	}
}

// removeDuplicates 去除重复值
func removeDuplicates(values []string) []string {
	seen := make(map[string]bool)
	var result []string

	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}

	return result
}
