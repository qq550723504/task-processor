// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"strings"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/alibaba1688/model"

	"github.com/mxschmitt/playwright-go"
)

// ShippingExtractor 物流信息提取器
type ShippingExtractor struct{}

// NewShippingExtractor 创建物流信息提取器
func NewShippingExtractor() *ShippingExtractor {
	return &ShippingExtractor{}
}

// Extract 提取物流信息 - 支持两种数据结构
func (se *ShippingExtractor) Extract(page playwright.Page, product *model.Product1688) error {
	shippingInfo := &model.ShippingInfo{}

	// 从结构化数据中获取物流信息，支持两种数据结构
	shippingResult, err := page.Evaluate(`() => {
		const result = {
			source: '',
			weight: 0,
			shippingFrom: '',
			isFreeShipping: false,
			processingTime: '',
			buyerProtection: []
		};
		
		// 方案1：优先从window.context结构化数据中获取（普通商品）
		if (window.context && window.context.result && window.context.result.data) {
			const data = window.context.result.data;
			result.source = 'structured_data';
			
			// 获取物流信息
			if (data.shippingServices && data.shippingServices.fields) {
				const shippingFields = data.shippingServices.fields;
				
				// 获取商品重量
				result.weight = shippingFields.unitWeight || 0;
				
				// 获取发货地 - 直接使用location字段
				result.shippingFrom = shippingFields.location || '';
				
				// 获取买家保护服务
				if (shippingFields.buyerProtectionModel && Array.isArray(shippingFields.buyerProtectionModel)) {
					result.buyerProtection = shippingFields.buyerProtectionModel
						.filter(service => service && service.packageBuyerDesc)
						.map(service => service.packageBuyerDesc);
				}
				
				// 检查是否包邮
				result.isFreeShipping = shippingFields.freeDeliverFee || false;
			}
			
			// 如果shippingServices中没有发货地，从供应商信息中获取
			if (!result.shippingFrom && data.productTitle && data.productTitle.fields && data.productTitle.fields.shopInfo) {
				const shopInfo = data.productTitle.fields.shopInfo;
				result.shippingFrom = shopInfo.location || '';
			}
			
			// 如果获取到物流信息，返回结果
			if (result.shippingFrom || result.weight > 0 || result.buyerProtection.length > 0) {
				return result;
			}
		}
		// 方案2：备选方案 - 从window.__INIT_DATA获取（定制商品）
		else if (window.__INIT_DATA && window.__INIT_DATA.data) {
			const data = window.__INIT_DATA.data;
			result.source = 'init_data';
			
			// 遍历数据块查找物流信息
			for (let key in data) {
				const item = data[key];
				if (!item || !item.data) continue;
				
				const itemData = item.data;
				
				// 查找发货地信息
				if (itemData.location && !result.shippingFrom) {
					result.shippingFrom = itemData.location;
				}
				
				// 查找重量信息
				if (itemData.weight && !result.weight) {
					result.weight = parseFloat(itemData.weight) || 0;
				}
				
				// 查找供应商地址信息
				if (itemData.supplierInfo && itemData.supplierInfo.location && !result.shippingFrom) {
					result.shippingFrom = itemData.supplierInfo.location;
				}
			}
			
			// 如果找到了信息，返回结果
			if (result.shippingFrom || result.weight > 0) {
				return result;
			}
		}
		
		// 方案3：备选方案 - 从DOM中提取
		result.source = 'dom_extraction';
		
		// 查找发货地信息 - 使用更精确的选择器
		const locationSelectors = [
			'span.location',
			'.shipping-from',
			'.delivery-from',
			'[class*="location"]',
			'[class*="address"]'
		];
		
		for (const selector of locationSelectors) {
			const element = document.querySelector(selector);
			if (element && element.textContent.trim()) {
				result.shippingFrom = element.textContent.trim();
				break;
			}
		}
		
		// 查找包邮信息
		const freeShippingElements = document.querySelectorAll('[class*="free"], [class*="shipping"]');
		for (const element of freeShippingElements) {
			const text = element.textContent.trim();
			if (text.includes('包邮') || text.includes('免运费')) {
				result.isFreeShipping = true;
				break;
			}
		}
		
		return result;
	}`, nil)

	if err == nil && shippingResult != nil {
		if shippingData, ok := shippingResult.(map[string]any); ok {
			source, _ := shippingData["source"].(string)

			// 设置商品重量
			if weight, ok := shippingData["weight"].(float64); ok && weight > 0 {
				// 将重量信息存储到产品包装信息中
				if product.PackInfo == nil {
					product.PackInfo = &model.PackInfo{}
				}
				product.PackInfo.Weight = weight
			}

			// 设置发货地
			if shippingFrom, ok := shippingData["shippingFrom"].(string); ok && shippingFrom != "" {
				// 简单清理发货地信息，移除明显的无关内容
				cleanedLocation := se.cleanLocationText(shippingFrom)
				shippingInfo.ShippingFrom = cleanedLocation
			}

			// 设置是否包邮
			if isFreeShipping, ok := shippingData["isFreeShipping"].(bool); ok {
				shippingInfo.IsFreeShipping = isFreeShipping
			}

			// 设置处理时间
			if processingTime, ok := shippingData["processingTime"].(string); ok && processingTime != "" {
				shippingInfo.ProcessingTime = processingTime
			}

			// 设置买家保护服务
			if buyerProtection, ok := shippingData["buyerProtection"].([]any); ok && len(buyerProtection) > 0 {
				var methods []model.ShippingMethod
				for _, protection := range buyerProtection {
					if protectionStr, ok := protection.(string); ok && protectionStr != "" {
						methods = append(methods, model.ShippingMethod{
							Name:         protectionStr,
							Cost:         0,
							DeliveryTime: "",
						})
					}
				}
				shippingInfo.ShippingMethods = methods
			}

			logger.GetGlobalLogger("crawler/alibaba1688").Debugf("通过%s提取到物流信息: 发货地=%s, 重量=%.0fg", source, shippingInfo.ShippingFrom, func() float64 {
				if product.PackInfo != nil {
					return product.PackInfo.Weight
				}
				return 0
			}())
		}
	}

	// 如果没有从结构化数据获取到发货地，尝试从DOM中提取
	if shippingInfo.ShippingFrom == "" {
		se.extractShippingFromDOM(page, shippingInfo)
	} else {
		// 清理发货地信息，移除无关内容
		shippingInfo.ShippingFrom = se.cleanLocationText(shippingInfo.ShippingFrom)
	}

	product.ShippingInfo = *shippingInfo
	return nil
}

// extractShippingFromDOM 从DOM中提取发货地信息
func (se *ShippingExtractor) extractShippingFromDOM(page playwright.Page, shippingInfo *model.ShippingInfo) {
	// 发货地信息的选择器
	shippingSelectors := []string{
		"[class*='shipping-from']",
		"[class*='delivery-from']",
		"[class*='ship-from']",
		".shipping-info",
		".delivery-info",
	}

	for _, selector := range shippingSelectors {
		locator := page.Locator(selector)
		count, err := locator.Count()
		if err != nil || count == 0 {
			continue
		}

		shippingText, err := locator.First().TextContent()
		if err == nil && strings.TrimSpace(shippingText) != "" {
			cleanText := strings.TrimSpace(shippingText)
			if se.isValidShippingLocation(cleanText) {
				shippingInfo.ShippingFrom = cleanText
				logger.GetGlobalLogger("crawler/alibaba1688").Debugf("通过选择器 %s 提取到发货地: %s", selector, cleanText)
				return
			}
		}
	}

	// 如果选择器方法失败，使用JavaScript查找地区信息
	shippingResult, err := page.Evaluate(`() => {
		// 查找包含地区相关关键词的文本
		const elements = document.querySelectorAll('*');
		for (const element of elements) {
			const text = element.textContent;
			if (text && (text.includes('发货') || text.includes('发出') || text.includes('送至'))) {
				// 查找包含"市"、"省"、"区"等地区标识的文本
				const locationMatch = text.match(/[\u4e00-\u9fff]{2,6}[市省区县]/);
				if (locationMatch) {
					return locationMatch[0];
				}
			}
		}
		
		return '';
	}`, nil)

	if err == nil && shippingResult != nil {
		if location, ok := shippingResult.(string); ok && location != "" {
			// 清理发货地信息
			cleanedLocation := se.cleanLocationText(location)
			shippingInfo.ShippingFrom = cleanedLocation
			logger.GetGlobalLogger("crawler/alibaba1688").Debugf("通过JavaScript方法提取到发货地: %s", cleanedLocation)
		}
	}
}

// isValidShippingLocation 验证发货地信息是否有效
func (se *ShippingExtractor) isValidShippingLocation(location string) bool {
	// 基本长度检查
	if len(location) < 2 || len(location) > 30 {
		return false
	}

	// 检查是否包含地区标识符
	locationSuffixes := []string{"市", "省", "区", "县", "镇"}
	for _, suffix := range locationSuffixes {
		if strings.Contains(location, suffix) {
			return true
		}
	}

	// 检查是否主要由中文字符组成
	chineseCount := 0
	totalCount := 0
	for _, r := range location {
		totalCount++
		if r >= '\u4e00' && r <= '\u9fff' {
			chineseCount++
		}
	}

	// 至少70%是中文字符
	return float64(chineseCount)/float64(totalCount) >= 0.7
}

// cleanLocationText 清理地区文本，移除无关信息
func (se *ShippingExtractor) cleanLocationText(text string) string {
	if text == "" {
		return ""
	}

	// 移除明显的无关内容
	unwantedKeywords := []string{
		"关注", "客服", "进店铺", "进工厂", "全部商品", "服务", "分", "回头率",
		"评价", "价格实惠", "质量好", "小户型", "超适合", "有限公司", "公司",
		"店铺", "工厂", "商品", "联系", "咨询", "电话", "微信", "QQ",
	}

	// 移除包含无关关键词的部分
	cleanedText := text
	for _, keyword := range unwantedKeywords {
		if strings.Contains(cleanedText, keyword) {
			// 找到关键词位置，截取之前的部分
			if index := strings.Index(cleanedText, keyword); index > 0 {
				cleanedText = cleanedText[:index]
			} else if index == 0 {
				// 如果关键词在开头，尝试找到第一个中文地名
				cleanedText = se.extractLocationFromText(text)
			}
		}
	}

	// 清理空白字符
	cleanedText = strings.TrimSpace(cleanedText)

	// 如果清理后为空或太短，尝试从原文本中提取地区
	if len(cleanedText) < 2 {
		cleanedText = se.extractLocationFromText(text)
	}

	return cleanedText
}

// extractLocationFromText 从文本中提取地区信息
func (se *ShippingExtractor) extractLocationFromText(text string) string {
	// 查找常见的地区模式
	locationPatterns := []string{
		"市", "省", "区", "县", "镇", "村", "街道",
	}

	words := strings.Fields(text)
	for _, word := range words {
		// 检查是否包含地区后缀
		for _, pattern := range locationPatterns {
			if strings.Contains(word, pattern) && len(word) >= 2 && len(word) <= 10 {
				// 进一步验证是否为有效地区名
				if se.isValidLocationName(word) {
					return word
				}
			}
		}
	}

	// 如果没有找到明确的地区，返回前几个字符（可能是省份简称）
	if len(text) >= 2 {
		// 取前面的字符，但不超过6个字符
		maxLen := 6
		if len(text) < maxLen {
			maxLen = len(text)
		}

		firstPart := text[:maxLen]
		// 移除数字和特殊字符
		var result strings.Builder
		for _, r := range firstPart {
			if (r >= '\u4e00' && r <= '\u9fff') || // 中文字符
				(r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') { // 英文字符
				result.WriteRune(r)
			}
		}

		if result.Len() >= 2 {
			return result.String()
		}
	}

	return ""
}

// isValidLocationName 验证是否为有效的地区名称
func (se *ShippingExtractor) isValidLocationName(name string) bool {
	// 基本长度检查
	if len(name) < 2 || len(name) > 10 {
		return false
	}

	// 排除明显不是地区的词汇
	invalidKeywords := []string{
		"服务", "评价", "质量", "价格", "商品", "客服", "联系", "咨询",
		"电话", "微信", "QQ", "店铺", "工厂", "公司", "有限",
	}

	for _, keyword := range invalidKeywords {
		if strings.Contains(name, keyword) {
			return false
		}
	}

	// 检查是否主要由中文字符组成
	chineseCount := 0
	totalCount := 0
	for _, r := range name {
		totalCount++
		if r >= '\u4e00' && r <= '\u9fff' {
			chineseCount++
		}
	}

	// 至少50%是中文字符
	return float64(chineseCount)/float64(totalCount) >= 0.5
}
