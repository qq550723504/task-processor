// Package alibaba1688 提供1688单个产品处理器
package alibaba1688

import (
	"task-processor/internal/core/logger"
	"fmt"
	"task-processor/internal/core/config"
	"task-processor/internal/crawler/alibaba1688/extractor"
	"task-processor/internal/crawler/alibaba1688/model"
	"time"

)

// SingleProcessor 单个产品处理器
type SingleProcessor struct {
	config         *config.Config
	urlHelper      *URLHelper
	productChecker *ProductChecker
	extractor      *extractor.ProductExtractor
	browserManager *BrowserManager
	pageOperator   *PageOperator
}

// NewSingleProcessor 创建新的单个产品处理器
func NewSingleProcessor(cfg *config.Config, urlHelper *URLHelper, productChecker *ProductChecker) *SingleProcessor {
	return &SingleProcessor{
		config:         cfg,
		urlHelper:      urlHelper,
		productChecker: productChecker,
		extractor:      extractor.NewProductExtractor(),
		browserManager: NewBrowserManager(cfg),
		pageOperator:   NewPageOperator(),
	}
}

// ProcessWithSingleBrowser 使用单个浏览器处理产品
func (sp *SingleProcessor) ProcessWithSingleBrowser(url string, startTime time.Time) (*model.Product1688, error) {
	logger.GetGlobalLogger("crawler/alibaba1688").Infof("使用单浏览器模式处理1688产品: %s", url)

	// 验证和标准化URL
	normalizedURL, err := sp.urlHelper.ValidateAndNormalizeURL(url)
	if err != nil {
		return nil, fmt.Errorf("URL验证失败: %w", err)
	}

	// 创建浏览器实例
	_, _, page, cleanup, err := sp.browserManager.CreateBrowser()
	if err != nil {
		return nil, err
	}
	defer cleanup()

	// 导航到产品页面
	if navErr := sp.pageOperator.NavigateToProduct(page, normalizedURL); navErr != nil {
		return nil, fmt.Errorf("导航到产品页面失败: %w", navErr)
	}

	// 提取产品信息
	product, err := sp.extractor.ExtractProductFromPage(page, normalizedURL)
	if err != nil {
		return nil, fmt.Errorf("提取产品信息失败: %w", err)
	}

	// 验证产品信息
	if validateErr := sp.productChecker.ValidateProduct(product); validateErr != nil {
		return nil, fmt.Errorf("产品信息验证失败: %w", validateErr)
	}

	duration := time.Since(startTime)
	logger.GetGlobalLogger("crawler/alibaba1688").Infof("单浏览器模式处理完成: %s, 耗时: %v", product.Title, duration)

	return product, nil
}
