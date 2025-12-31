// Package amazon 提供Amazon单浏览器处理功能
package amazon

import (
	"fmt"
	"task-processor/internal/common/amazon/browser"
	"task-processor/internal/common/amazon/extractor"
	"task-processor/internal/core/config"
	"task-processor/internal/model"
	"time"
)

// SingleProcessor 单浏览器处理器
type SingleProcessor struct {
	config         *config.AmazonConfig
	urlHelper      *URLHelper
	productChecker *ProductChecker
}

// NewSingleProcessor 创建单浏览器处理器
func NewSingleProcessor(config *config.AmazonConfig, urlHelper *URLHelper, productChecker *ProductChecker) *SingleProcessor {
	return &SingleProcessor{
		config:         config,
		urlHelper:      urlHelper,
		productChecker: productChecker,
	}
}

// ProcessWithSingleBrowser 使用单浏览器处理
func (sp *SingleProcessor) ProcessWithSingleBrowser(url string, zipcode string, startTime time.Time) (*model.Product, error) {
	browserManager := browser.NewBrowserManager(sp.config)

	if err := browserManager.Install(); err != nil {
		return nil, fmt.Errorf("初始化Playwright失败: %w", err)
	}

	if err := browserManager.Launch(); err != nil {
		return nil, fmt.Errorf("启动浏览器失败: %w", err)
	}
	defer browserManager.Close()

	page, err := browserManager.NewPage()
	if err != nil {
		return nil, fmt.Errorf("创建页面失败: %w", err)
	}

	urlWithLang := sp.urlHelper.AddLanguageParam(url)

	if err := browserManager.NavigateTo(page, urlWithLang); err != nil {
		return nil, fmt.Errorf("导航失败: %w", err)
	}

	// 处理可能出现的"Continue shopping"按钮
	if err := sp.productChecker.HandleContinueShoppingButton(page); err != nil {
		// 这里只记录警告，不返回错误
	}

	// 设置邮编
	zipcodeSetter := browser.NewZipcodeSetter(browserManager)
	if err := zipcodeSetter.SetAndVerifyZipcode(page, zipcode); err != nil {
		return nil, fmt.Errorf("设置邮编失败: %w", err)
	}

	now := time.Now()
	product := &model.Product{
		URL:       url,
		Zipcode:   zipcode,
		Asin:      sp.urlHelper.ExtractASINFromURL(url),
		Currency:  sp.urlHelper.GetCurrencyFromURL(url),
		Timestamp: model.NullableTime{Time: &now},
	}

	marketplace := sp.urlHelper.GetMarketplaceFromURL(url)
	ext := extractor.NewCompositeExtractor(marketplace)
	if err := ext.Extract(page, product); err != nil {
		return nil, fmt.Errorf("提取产品信息失败: %w", err)
	}

	return product, nil
}
