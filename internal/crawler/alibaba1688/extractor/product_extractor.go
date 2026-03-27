// Package extractor 提供1688产品数据提取功能
package extractor

import (
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/alibaba1688/model"
	"time"

	"github.com/playwright-community/playwright-go"
)

// ProductExtractor 1688产品数据提取器
type ProductExtractor struct {
	// 基础信息提取器
	basicInfoExtractor *BasicInfoExtractor
	// 物流信息提取器
	shippingExtractor *ShippingExtractor
	// 变体数据提取器
	variantExtractor *VariantExtractor
	// 优化的属性提取器
	attributeExtractor *AttributeExtractor
	// 商品详情提取器
	detailExtractor *DetailExtractor
	// 包装信息提取器
	packInfoExtractor *PackInfoExtractor
	// 变体值提取器
	variantValuesExtractor *VariantValuesExtractor
	// 价格范围提取器
	priceRangeExtractor *PriceExtractor
	// 传统提取器（保留作为备选）
	titleExtractor         *TitleExtractor
	priceExtractor         *PriceExtractor
	supplierExtractor      *SupplierExtractor
	imageExtractor         *ImageExtractor
	specificationExtractor *SpecificationExtractor
}

// NewProductExtractor 创建新的产品提取器
func NewProductExtractor() *ProductExtractor {
	return &ProductExtractor{
		// 优先使用优化的提取器
		basicInfoExtractor:     NewBasicInfoExtractor(),
		shippingExtractor:      NewShippingExtractor(),
		variantExtractor:       NewVariantExtractor(),
		attributeExtractor:     NewAttributeExtractor(),
		detailExtractor:        NewDetailExtractor(),
		packInfoExtractor:      NewPackInfoExtractor(),
		variantValuesExtractor: NewVariantValuesExtractor(),
		priceRangeExtractor:    NewPriceExtractor(),
		// 传统提取器作为备选
		titleExtractor:         NewTitleExtractor(),
		priceExtractor:         NewPriceExtractor(),
		supplierExtractor:      NewSupplierExtractor(),
		imageExtractor:         NewImageExtractor(),
		specificationExtractor: NewSpecificationExtractor(),
	}
}

// ExtractProductFromPage 从Playwright页面提取产品信息
func (pe *ProductExtractor) ExtractProductFromPage(page playwright.Page, url string) (*model.Product1688, error) {
	logger.GetGlobalLogger("crawler/alibaba1688").Infof("开始提取1688产品信息: %s", url)

	product := &model.Product1688{
		URL:       url,
		CrawledAt: time.Now(),
		UpdatedAt: time.Now(),
		Currency:  "CNY",
	}

	// 等待页面加载完成
	pe.waitForPageLoad(page)

	// 1. 优先使用优化的提取器（基于结构化数据）
	optimizedExtractors := []BaseExtractor{
		pe.basicInfoExtractor,     // 基础信息（标题、ID、销量等）
		pe.shippingExtractor,      // 物流信息（重量、发货地等）
		pe.variantExtractor,       // 变体数据（价格、库存、属性等）
		pe.attributeExtractor,     // 商品属性（材质、品牌等）
		pe.detailExtractor,        // 商品详情（详情图片、视频等）
		pe.packInfoExtractor,      // 包装信息（尺寸、重量等）
		pe.variantValuesExtractor, // 变体值（颜色、尺寸等选项）
		pe.priceRangeExtractor,    // 价格范围（批量价格等）
		pe.supplierExtractor,      // 供应商信息（已优化）
		pe.imageExtractor,         // 图片信息（已优化）
	}

	for _, extractor := range optimizedExtractors {
		if err := extractor.Extract(page, product); err != nil {
			logger.GetGlobalLogger("crawler/alibaba1688").Warnf("优化提取器执行失败: %v", err)
		}
	}

	// 2. 所有提取器现在都支持两种数据结构，不再需要单独的备选提取器
	// 每个提取器内部会自动检测并使用正确的数据源

	// 3. 如果仍然缺失信息，使用传统提取器作为最后备选
	if product.Title == "" {
		logger.GetGlobalLogger("crawler/alibaba1688").Debug("标题仍然缺失，使用传统标题提取器")
		if err := pe.titleExtractor.Extract(page, product); err != nil {
			logger.GetGlobalLogger("crawler/alibaba1688").Warnf("传统标题提取器失败: %v", err)
		}
	}

	if len(product.Variants) == 0 && product.MinPrice == 0 {
		logger.GetGlobalLogger("crawler/alibaba1688").Debug("价格信息缺失，使用传统价格提取器")
		if err := pe.priceExtractor.Extract(page, product); err != nil {
			logger.GetGlobalLogger("crawler/alibaba1688").Warnf("传统价格提取器失败: %v", err)
		}
	}

	if len(product.Specifications) == 0 {
		logger.GetGlobalLogger("crawler/alibaba1688").Debug("属性信息缺失，使用传统规格提取器")
		if err := pe.specificationExtractor.Extract(page, product); err != nil {
			logger.GetGlobalLogger("crawler/alibaba1688").Warnf("传统规格提取器失败: %v", err)
		}
	}

	logger.GetGlobalLogger("crawler/alibaba1688").Infof("成功提取产品信息: %s", product.Title)
	return product, nil
}

// waitForPageLoad 等待页面加载完成
func (pe *ProductExtractor) waitForPageLoad(page playwright.Page) {
	// 等待关键元素加载
	selectors := []string{
		"h1, .title, .product-title",
		".price, .offer-price",
		".supplier-info, .company-name",
	}

	for _, selector := range selectors {
		_, err := page.WaitForSelector(selector, playwright.PageWaitForSelectorOptions{
			Timeout: playwright.Float(5000), // 5秒超时
		})
		if err != nil {
			logger.GetGlobalLogger("crawler/alibaba1688").Debugf("等待元素 %s 超时，继续尝试其他元素", selector)
			continue
		}
		break
	}

	// 等待JavaScript执行完成
	time.Sleep(5 * time.Second) // 增加等待时间到5秒

	// 添加调试信息，检查页面数据是否存在
	hasInitData, err := page.Evaluate(`() => {
		return typeof window.__INIT_DATA !== 'undefined' && window.__INIT_DATA !== null;
	}`)
	if err == nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("页面__INIT_DATA存在: %v", hasInitData)
	}

	hasContext, err := page.Evaluate(`() => {
		return typeof window.context !== 'undefined' && window.context !== null;
	}`)
	if err == nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("页面context存在: %v", hasContext)
	}

	// 检查页面标题和URL
	title, _ := page.Title()
	url := page.URL()
	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("当前页面标题: %s", title)
	logger.GetGlobalLogger("crawler/alibaba1688").Debugf("当前页面URL: %s", url)

	// 检查页面是否包含商品信息
	hasProductInfo, err := page.Evaluate(`() => {
		// 检查是否有商品标题
		const titleElements = document.querySelectorAll('h1, .offer-title, .product-title, [class*="title"]');
		if (titleElements.length > 0) {
			for (let el of titleElements) {
				if (el.textContent && el.textContent.trim().length > 10) {
					return true;
				}
			}
		}
		
		// 检查是否有价格信息
		const priceElements = document.querySelectorAll('[class*="price"], .price, .cost');
		if (priceElements.length > 0) {
			return true;
		}
		
		// 检查是否有图片
		const imgElements = document.querySelectorAll('img[src*="alicdn.com"]');
		if (imgElements.length > 0) {
			return true;
		}
		
		return false;
	}`)
	if err == nil {
		logger.GetGlobalLogger("crawler/alibaba1688").Debugf("页面包含商品信息: %v", hasProductInfo)
	}
}
