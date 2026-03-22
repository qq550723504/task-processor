// Package amazon 提供Amazon实例处理功能
package amazon

import (
	"context"
	"fmt"
	"strings"
	"task-processor/internal/core/logger"
	"task-processor/internal/crawler/amazon/browser"
	"task-processor/internal/crawler/amazon/extractor"
	"task-processor/internal/model"
	"time"

	"github.com/playwright-community/playwright-go"
)

// InstanceProcessor 实例处理器
type InstanceProcessor struct {
	urlHelper      *URLHelper
	productChecker *ProductChecker
}

// NewInstanceProcessor 创建实例处理器
func NewInstanceProcessor(urlHelper *URLHelper, productChecker *ProductChecker) *InstanceProcessor {
	return &InstanceProcessor{
		urlHelper:      urlHelper,
		productChecker: productChecker,
	}
}

// ProcessWithInstance 使用指定浏览器实例处理产品，ctx 用于超时控制
func (ip *InstanceProcessor) ProcessWithInstance(ctx context.Context, instance *browser.BrowserInstance, url string, zipcode string) (*model.Product, error) {
	if instance == nil {
		return nil, fmt.Errorf("浏览器实例为空")
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("使用浏览器实例 %d 处理产品: %s", instance.ID, url)

	// 计算 ctx 剩余超时，用于 Playwright 各操作的 Timeout 参数
	playwrightTimeout := func() float64 {
		deadline, ok := ctx.Deadline()
		if !ok {
			return 30000 // 无 deadline 时默认 30s
		}
		remaining := time.Until(deadline).Milliseconds()
		if remaining <= 0 {
			return 1 // 已超时，给一个极小值让 Playwright 立即失败
		}
		return float64(remaining)
	}

	// 创建新页面（用 goroutine + channel 包装，防止 WebSocket 断连时 context.NewPage() 永久 hang）
	type newPageResult struct {
		page playwright.Page
		err  error
	}
	newPageChan := make(chan newPageResult, 1)
	go func() {
		p, err := instance.Manager.NewPage()
		newPageChan <- newPageResult{p, err}
	}()
	var page playwright.Page
	select {
	case r := <-newPageChan:
		if r.err != nil {
			return nil, fmt.Errorf("创建页面失败: %w", r.err)
		}
		page = r.page
	case <-ctx.Done():
		return nil, fmt.Errorf("创建页面超时: %w", ctx.Err())
	case <-time.After(15 * time.Second):
		return nil, fmt.Errorf("创建页面超时，WebSocket 可能已断连")
	}

	defer func() {
		if closeErr := page.Close(); closeErr != nil {
			logger.GetGlobalLogger("crawler/amazon").Warnf("关闭页面失败: %v", closeErr)
		}
	}()

	// 检查 ctx 是否已取消
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context 已取消: %w", err)
	}

	// 获取市场对应的默认货币
	currency := ip.urlHelper.GetCurrencyFromURL(url)

	// 添加语言和货币参数
	urlWithParams := ip.urlHelper.AddLanguageAndCurrencyParams(url, currency)

	// 导航到产品页面，使用 ctx 剩余时间作为超时
	_, err := page.Goto(urlWithParams, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(playwrightTimeout()),
	})
	if err != nil {
		return nil, fmt.Errorf("导航到页面失败: %w", err)
	}

	// 优先处理可能出现的"Continue shopping"按钮
	if err := ip.productChecker.HandleContinueShoppingButton(page); err != nil {
		logger.GetGlobalLogger("crawler/amazon").Warnf("处理Continue shopping按钮时出错: %v", err)
	}

	// 等待页面准备就绪，超时取 ctx 剩余时间与 15s 的较小值
	waitTimeout := 15 * time.Second
	if deadline, ok := ctx.Deadline(); ok {
		if remaining := time.Until(deadline); remaining < waitTimeout {
			waitTimeout = remaining
		}
	}
	if err := ip.productChecker.WaitForPageReady(page, waitTimeout); err != nil {
		return nil, fmt.Errorf("页面未准备就绪: %w", err)
	}

	// 检查是否为产品页面
	if !ip.productChecker.IsProductPage(page) {
		return nil, fmt.Errorf("不是有效的产品页面")
	}

	// 设置邮编
	if zipcode != "" {
		zipcodeSetter := browser.NewZipcodeSetter(instance.Manager)
		if err := zipcodeSetter.SetAndVerifyZipcode(page, zipcode); err != nil {
			logger.GetGlobalLogger("crawler/amazon").Errorf("设置邮编失败: %v", err)

			if ip.shouldRecreateInstance(err) {
				return nil, fmt.Errorf("需要重建浏览器实例: %w", err)
			}

			return nil, fmt.Errorf("邮编设置失败，终止数据抓取: %w", err)
		}
	}

	// 检查 ctx 是否已取消（邮编设置可能耗时较长）
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context 已取消: %w", err)
	}

	// 获取站点信息，用于设置货币
	marketplace := ip.urlHelper.GetMarketplaceFromURL(url)
	expectedCurrency := ip.urlHelper.GetCurrencyFromURL(url)

	// 创建产品对象
	now := time.Now()
	product := &model.Product{
		URL:       url,
		Zipcode:   zipcode,
		Asin:      ip.urlHelper.ExtractASINFromURL(url),
		Currency:  expectedCurrency,
		Timestamp: model.NullableTime{Time: &now},
	}

	// 提取产品信息
	ext := extractor.NewCompositeExtractor(marketplace)
	if err := ext.Extract(page, product); err != nil {
		return nil, fmt.Errorf("提取产品信息失败: %w", err)
	}

	// 检查货币是否匹配，不匹配时切换货币并重新提取
	if expectedCurrency != "" && product.Currency != expectedCurrency {
		logger.GetGlobalLogger("crawler/amazon").Warnf("货币不匹配 (页面: %s, 期望: %s)，尝试切换货币", product.Currency, expectedCurrency)
		currencySetter := browser.NewCurrencySetter(instance.Manager)
		if err := currencySetter.SetAndVerifyCurrency(page, expectedCurrency); err != nil {
			return nil, fmt.Errorf("货币切换失败，终止数据抓取: %w", err)
		}
		logger.GetGlobalLogger("crawler/amazon").Infof("货币切换成功，重新提取产品信息")
		product.Currency = expectedCurrency
		if err := ext.Extract(page, product); err != nil {
			return nil, fmt.Errorf("重新提取产品信息失败: %w", err)
		}
	}

	// 验证提取的数据
	if err := ip.validateProductData(product); err != nil {
		return nil, fmt.Errorf("产品数据验证失败: %w", err)
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("成功处理产品: (ASIN: %s)", product.Asin)
	return product, nil
}

// validateProductData 验证产品数据
func (ip *InstanceProcessor) validateProductData(product *model.Product) error {
	if product == nil {
		return fmt.Errorf("产品对象为空")
	}

	if product.Asin == "" {
		return fmt.Errorf("ASIN为空")
	}

	if product.Title == "" {
		return fmt.Errorf("产品标题为空")
	}

	// 检查价格信息
	if product.FinalPrice <= 0 {
		logger.GetGlobalLogger("crawler/amazon").Warnf("产品价格信息缺失或无效: ASIN=%s", product.Asin)
		// 不返回错误，某些产品可能暂时没有价格
	}

	return nil
}

// ProcessBatchWithInstance 使用指定实例批量处理产品
func (ip *InstanceProcessor) ProcessBatchWithInstance(ctx context.Context, instance *browser.BrowserInstance, requests []model.ProductRequest) []model.ProductResult {
	results := make([]model.ProductResult, len(requests))

	for i, req := range requests {
		product, err := ip.ProcessWithInstance(ctx, instance, req.URL, req.Zipcode)
		results[i] = model.ProductResult{
			Product: product,
			Error:   err,
		}

		if err != nil {
			logger.GetGlobalLogger("crawler/amazon").Errorf("批量处理 [%d/%d] 失败: %s - %v", i+1, len(requests), req.URL, err)
		} else {
			logger.GetGlobalLogger("crawler/amazon").Infof("批量处理 [%d/%d] 成功: %s", i+1, len(requests), product.Asin)
		}
	}

	return results
}

// ProcessWithRetry 带重试的产品处理
func (ip *InstanceProcessor) ProcessWithRetry(ctx context.Context, instance *browser.BrowserInstance, url string, zipcode string, maxRetries int) (*model.Product, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logger.GetGlobalLogger("crawler/amazon").Infof("重试处理产品 (第%d次): %s", attempt, url)
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		product, err := ip.ProcessWithInstance(ctx, instance, url, zipcode)
		if err == nil {
			return product, nil
		}

		lastErr = err
		logger.GetGlobalLogger("crawler/amazon").Warnf("处理产品失败 (第%d次尝试): %v", attempt+1, err)

		if ip.isSeriousError(err) {
			break
		}
	}

	return nil, fmt.Errorf("重试%d次后仍然失败: %w", maxRetries, lastErr)
}

// shouldRecreateInstance 判断是否需要重建浏览器实例
func (ip *InstanceProcessor) shouldRecreateInstance(err error) bool {
	if err == nil {
		return false
	}

	errorMsg := err.Error()
	recreateErrors := []string{
		"SIGN_IN_REQUIRED",
		"需要登录才能更新位置",
		"需要重建浏览器实例",
		// playwright WebSocket 断连
		"Failed to next",
		"next while nexting",
		"Socket connection to remote was closed",
		"Target closed",
		"target closed",
	}

	for _, recreateError := range recreateErrors {
		if strings.Contains(errorMsg, recreateError) {
			return true
		}
	}

	return false
}

// isSeriousError 判断是否为严重错误
func (ip *InstanceProcessor) isSeriousError(err error) bool {
	if err == nil {
		return false
	}

	errorMsg := err.Error()
	seriousErrors := []string{
		"访问被阻止",
		"遇到验证码",
		"浏览器实例为空",
		"创建页面失败",
		// playwright WebSocket 断连
		"Failed to next",
		"next while nexting",
		"Socket connection to remote was closed",
		"Target closed",
		"target closed",
	}

	for _, serious := range seriousErrors {
		if strings.Contains(errorMsg, serious) {
			return true
		}
	}

	return false
}
