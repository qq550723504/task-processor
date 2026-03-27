// Package amazon 提供Amazon实例处理功能
package amazon

import (
	"context"
	"fmt"
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
		// page.Close() 在 WebSocket 断连时可能永久 hang，加超时保护
		done := make(chan error, 1)
		go func() { done <- page.Close() }()
		select {
		case closeErr := <-done:
			if closeErr != nil {
				logger.GetGlobalLogger("crawler/amazon").Warnf("关闭页面失败: %v", closeErr)
			}
		case <-time.After(5 * time.Second):
			logger.GetGlobalLogger("crawler/amazon").Warnf("关闭页面超时（5s），WebSocket 可能已断连")
		}
	}()

	// 检查 ctx 是否已取消
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context 已取消: %w", err)
	}

	// 填加语言参数
	urlWithParams := ip.urlHelper.AddLanguageParam(url)

	// 导航到产品页面，用 goroutine+channel+select 包装，防止 WebSocket 断连时永久 hang
	type gotoResult struct {
		err error
	}
	gotoChan := make(chan gotoResult, 1)
	go func() {
		_, err := page.Goto(urlWithParams, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
			Timeout:   playwright.Float(playwrightTimeout()),
		})
		gotoChan <- gotoResult{err}
	}()
	select {
	case r := <-gotoChan:
		if r.err != nil {
			return nil, fmt.Errorf("导航到页面失败: %w", r.err)
		}
	case <-ctx.Done():
		return nil, fmt.Errorf("导航超时: %w", ctx.Err())
	case <-time.After(90 * time.Second):
		return nil, fmt.Errorf("导航超时，WebSocket 可能已断连")
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
			// 严重错误（WebSocket 断连、需要登录等）交由上层重建实例
			return nil, fmt.Errorf("邮编设置失败，终止数据抓取: %w", err)
		}
	}

	// 检查 ctx 是否已取消（邮编设置可能耗时较长）
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context 已取消: %w", err)
	}

	// 获取站点信息
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

	// 提取产品信息（传入 ctx，确保超时时并行提取器能及时退出）
	ext := extractor.NewCompositeExtractor(marketplace)
	if err := ext.ExtractWithContext(ctx, page, product); err != nil {
		return nil, fmt.Errorf("提取产品信息失败: %w", err)
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

// ProcessBatchWithInstance、ProcessWithRetry、shouldRecreateInstance、isSeriousError
// 已删除：无调用方，错误判断逻辑由 browser.ErrorDetector 统一负责
