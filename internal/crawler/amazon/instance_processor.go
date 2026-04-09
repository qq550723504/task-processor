// Package amazon 提供Amazon实例处理功能
package amazon

import (
	"context"
	"errors"
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
	urlHelper            *URLHelper
	domainResolver       *DomainResolver
	productChecker       *ProductChecker
	resultValidator      *ProductResultValidator
	failureArtifactStore *FailureArtifactStore
	qualityControl       qualityControlOptions
	qualityMetrics       qualityMetricsRecorder
	extractProduct       func(ctx context.Context, page playwright.Page, url, zipcode string) (*model.Product, error)
	prepareRetry         func(ctx context.Context, page playwright.Page, waitTimeout time.Duration) error
	matchTargetContext   func(page playwright.Page, targetURL string) (bool, error)
}

type qualityControlOptions struct {
	retryOnValidationFailure bool
	validationRetryAttempts  int
}

// NewInstanceProcessor 创建实例处理器。
// resultValidator 可选，省略时使用默认验证器，兼容旧调用点。
func NewInstanceProcessor(urlHelper *URLHelper, productChecker *ProductChecker, resultValidator ...*ProductResultValidator) *InstanceProcessor {
	validator := NewProductResultValidator()
	if len(resultValidator) > 0 && resultValidator[0] != nil {
		validator = resultValidator[0]
	}
	return &InstanceProcessor{
		urlHelper:            urlHelper,
		domainResolver:       NewDomainResolver(),
		productChecker:       productChecker,
		resultValidator:      validator,
		failureArtifactStore: nil,
		qualityControl: qualityControlOptions{
			retryOnValidationFailure: false,
			validationRetryAttempts:  1,
		},
	}
}

func (ip *InstanceProcessor) SetFailureArtifactStore(store *FailureArtifactStore) {
	ip.failureArtifactStore = store
}

func (ip *InstanceProcessor) SetQualityControlOptions(retryOnValidationFailure bool, validationRetryAttempts int) {
	if validationRetryAttempts <= 0 {
		validationRetryAttempts = 1
	}
	ip.qualityControl = qualityControlOptions{
		retryOnValidationFailure: retryOnValidationFailure,
		validationRetryAttempts:  validationRetryAttempts,
	}
}

func (ip *InstanceProcessor) SetQualityMetricsRecorder(recorder qualityMetricsRecorder) {
	ip.qualityMetrics = recorder
}

// ProcessWithInstance 使用指定浏览器实例处理产品，ctx 用于超时控制
func (ip *InstanceProcessor) ProcessWithInstance(ctx context.Context, instance *browser.BrowserInstance, url string, zipcode string) (product *model.Product, retErr error) {
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
	defer func() {
		ip.captureFailureArtifacts(page, instance, url, zipcode, retErr)
	}()

	// 检查 ctx 是否已取消
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context 已取消: %w", err)
	}

	// 添加语言与防跳站参数，尽量避免被 Amazon 根据来源 IP 重定向到其他站点。
	urlWithParams := ip.urlHelper.AddLanguageParam(url)
	urlWithParams = ip.urlHelper.AddNoRedirectParam(urlWithParams)

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
	zipcodeToSet := strings.TrimSpace(zipcode)
	if zipcodeToSet == "" {
		if fallbackZipcode := ip.resolveDefaultZipcodeForTargetContext(page, urlWithParams); fallbackZipcode != "" {
			zipcodeToSet = fallbackZipcode
			logger.GetGlobalLogger("crawler/amazon").Infof("当前不在目标配送上下文，回退使用默认邮编切换目标站点上下文: %s", zipcodeToSet)
		} else {
			logger.GetGlobalLogger("crawler/amazon").Info("当前配送上下文已满足目标站点要求，跳过默认邮编设置")
		}
	}

	if zipcodeToSet != "" {
		zipcodeSetter := browser.NewZipcodeSetter(instance.Manager)
		zipcodeSetter.SetTargetURL(urlWithParams)
		if err := zipcodeSetter.SetAndVerifyZipcode(page, zipcodeToSet); err != nil {
			logger.GetGlobalLogger("crawler/amazon").Errorf("设置邮编失败: %v", err)
			// 严重错误（WebSocket 断连、需要登录等）交由上层重建实例
			return nil, fmt.Errorf("邮编设置失败，终止数据抓取: %w", err)
		}
	}

	// 检查 ctx 是否已取消（邮编设置可能耗时较长）
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context 已取消: %w", err)
	}

	product, retErr = ip.extractWithQualityRetry(ctx, page, url, zipcode, waitTimeout)
	if retErr != nil {
		return nil, retErr
	}

	logger.GetGlobalLogger("crawler/amazon").Infof("成功处理产品: (ASIN: %s)", product.Asin)
	return product, nil
}

func (ip *InstanceProcessor) resolveDefaultZipcodeForTargetContext(page playwright.Page, targetURL string) string {
	if strings.TrimSpace(targetURL) == "" {
		return ""
	}
	if page == nil && (ip == nil || ip.matchTargetContext == nil) {
		return ""
	}

	resolver := ip.domainResolver
	if resolver == nil {
		resolver = NewDomainResolver()
	}
	region := strings.ToLower(strings.TrimSpace(resolver.ExtractRegionFromURL(targetURL)))
	if region != "us" {
		logger.GetGlobalLogger("crawler/amazon").Infof("目标站点 %s(region=%s) 不启用默认邮编回退，仅使用当前配送上下文", targetURL, region)
		return ""
	}

	inTargetContext, err := ip.matchesTargetContext(page, targetURL)
	if err != nil {
		if ip != nil && ip.qualityMetrics != nil {
			ip.qualityMetrics.RecordTargetContextCheckError(region)
			ip.qualityMetrics.RecordTargetContextFallback(region)
		}
		logger.GetGlobalLogger("crawler/amazon").Warnf("判断目标配送上下文失败，继续使用默认美国邮编兜底: target_url=%s region=%s err=%v", targetURL, region, err)
		return resolver.GetZipcodeByRegion(region)
	}
	if inTargetContext {
		if ip != nil && ip.qualityMetrics != nil {
			ip.qualityMetrics.RecordTargetContextSkip(region)
		}
		logger.GetGlobalLogger("crawler/amazon").Infof("当前配送上下文已命中目标站点，无需回退默认邮编: target_url=%s region=%s", targetURL, region)
		return ""
	}

	if ip != nil && ip.qualityMetrics != nil {
		ip.qualityMetrics.RecordTargetContextFallback(region)
	}
	logger.GetGlobalLogger("crawler/amazon").Infof("当前配送上下文未命中目标站点，回退默认邮编: target_url=%s region=%s zipcode=%s", targetURL, region, resolver.GetZipcodeByRegion(region))
	return resolver.GetZipcodeByRegion(region)
}

func (ip *InstanceProcessor) matchesTargetContext(page playwright.Page, targetURL string) (bool, error) {
	if ip != nil && ip.matchTargetContext != nil {
		return ip.matchTargetContext(page, targetURL)
	}

	validator := browser.NewZipcodeValidator()
	validator.SetTargetURL(targetURL)
	return validator.MatchesTargetContext(page)
}

func (ip *InstanceProcessor) extractWithQualityRetry(ctx context.Context, page playwright.Page, url, zipcode string, waitTimeout time.Duration) (*model.Product, error) {
	attempts := 1
	hadRetry := false
	if ip.qualityControl.retryOnValidationFailure && ip.qualityControl.validationRetryAttempts > 1 {
		attempts = ip.qualityControl.validationRetryAttempts
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ip.alignTargetContext(page, url, waitTimeout); err != nil {
			logger.GetGlobalLogger("crawler/amazon").Warnf("目标站点上下文对齐失败，继续提取: %v", err)
		}

		product, err := ip.extractAndValidateProduct(ctx, page, url, zipcode)
		if err == nil {
			if hadRetry && ip.qualityMetrics != nil {
				ip.qualityMetrics.RecordValidationRetryRecovered()
			}
			return product, nil
		}

		var qualityErr *ProductQualityError
		if !errors.As(err, &qualityErr) || attempt == attempts {
			return nil, err
		}
		hadRetry = true
		if ip.qualityMetrics != nil {
			ip.qualityMetrics.RecordValidationRetryAttempt()
		}

		logger.GetGlobalLogger("crawler/amazon").Warnf("产品质量校验未通过，准备第 %d/%d 次重抓: %v", attempt+1, attempts, err)
		if retryErr := ip.preparePageForQualityRetry(ctx, page, waitTimeout); retryErr != nil {
			return nil, fmt.Errorf("产品质量校验失败且页面重试准备失败: %w", retryErr)
		}
	}

	return nil, fmt.Errorf("产品质量重抓失败")
}

func (ip *InstanceProcessor) alignTargetContext(page playwright.Page, url string, waitTimeout time.Duration) error {
	if page == nil || ip.urlHelper == nil {
		return nil
	}

	expectedCurrency := strings.TrimSpace(ip.urlHelper.GetCurrencyFromURL(url))
	if expectedCurrency == "" {
		return nil
	}

	currencySetter := browser.NewCurrencySetter(nil)
	if err := currencySetter.SetAndVerifyCurrency(page, expectedCurrency); err != nil {
		return err
	}

	if err := ip.productChecker.HandleContinueShoppingButton(page); err != nil {
		logger.GetGlobalLogger("crawler/amazon").Warnf("货币对齐后处理Continue shopping按钮时出错: %v", err)
	}
	if waitTimeout > 0 {
		if err := ip.productChecker.WaitForPageReady(page, waitTimeout); err != nil {
			return fmt.Errorf("货币对齐后页面未准备就绪: %w", err)
		}
	}

	return nil
}

func (ip *InstanceProcessor) extractAndValidateProduct(ctx context.Context, page playwright.Page, url, zipcode string) (*model.Product, error) {
	if customExtractor := ip.extractProduct; customExtractor != nil {
		return customExtractor(ctx, page, url, zipcode)
	}

	marketplace := ip.urlHelper.GetMarketplaceFromURL(url)
	expectedCurrency := ip.urlHelper.GetCurrencyFromURL(url)

	now := time.Now()
	product := &model.Product{
		URL:       url,
		Zipcode:   zipcode,
		Asin:      ip.urlHelper.ExtractASINFromURL(url),
		Currency:  expectedCurrency,
		Timestamp: model.NullableTime{Time: &now},
	}

	ext := extractor.NewCompositeExtractor(marketplace)
	if err := ext.ExtractWithContext(ctx, page, product); err != nil {
		return nil, fmt.Errorf("提取产品信息失败: %w", err)
	}

	if err := ip.validateProductData(product); err != nil {
		return nil, fmt.Errorf("产品数据验证失败: %w", err)
	}

	return product, nil
}

func (ip *InstanceProcessor) preparePageForQualityRetry(ctx context.Context, page playwright.Page, waitTimeout time.Duration) error {
	if ip.prepareRetry != nil {
		return ip.prepareRetry(ctx, page, waitTimeout)
	}

	if page == nil {
		return fmt.Errorf("页面对象为空")
	}

	reloadTimeout := float64(waitTimeout.Milliseconds())
	if reloadTimeout <= 0 {
		reloadTimeout = 1000
	}

	if _, err := page.Reload(playwright.PageReloadOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		Timeout:   playwright.Float(reloadTimeout),
	}); err != nil {
		return fmt.Errorf("重新加载页面失败: %w", err)
	}

	if err := ip.productChecker.HandleContinueShoppingButton(page); err != nil {
		logger.GetGlobalLogger("crawler/amazon").Warnf("重试前处理Continue shopping按钮时出错: %v", err)
	}
	if err := ip.productChecker.WaitForPageReady(page, waitTimeout); err != nil {
		return fmt.Errorf("重试后页面未准备就绪: %w", err)
	}

	return nil
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
	if ip.resultValidator != nil {
		if err := ip.resultValidator.Validate(product); err != nil {
			return err
		}
	}

	expectedCurrency := ""
	if ip.urlHelper != nil && product.URL != "" {
		expectedCurrency = ip.urlHelper.GetCurrencyFromURL(product.URL)
	}
	if expectedCurrency != "" && product.FinalPrice > 0 && strings.TrimSpace(product.Currency) != "" &&
		!strings.EqualFold(strings.TrimSpace(product.Currency), expectedCurrency) {
		return &ProductQualityError{Reasons: []string{
			fmt.Sprintf("currency mismatch: expected %s got %s", expectedCurrency, strings.TrimSpace(product.Currency)),
		}}
	}

	return nil
}

func (ip *InstanceProcessor) captureFailureArtifacts(page playwright.Page, instance *browser.BrowserInstance, url, zipcode string, cause error) {
	if cause == nil || ip.failureArtifactStore == nil || page == nil {
		return
	}

	instanceID := -1
	if instance != nil {
		instanceID = instance.ID
	}
	fetchErr := ClassifyFetchError(cause)

	if err := ip.failureArtifactStore.Capture(page, FailureArtifactInput{
		URL:        url,
		Zipcode:    zipcode,
		ASIN:       ip.urlHelper.ExtractASINFromURL(url),
		Error:      cause.Error(),
		ErrorType:  fetchErr.ErrorType(),
		Retryable:  fetchErr.RetryableError(),
		InstanceID: instanceID,
	}); err != nil {
		logger.GetGlobalLogger("crawler/amazon").Warnf("保存失败样本失败: %v", err)
	}
}

// ProcessBatchWithInstance、ProcessWithRetry、shouldRecreateInstance、isSeriousError
// 已删除：无调用方，错误判断逻辑由 browser.ErrorDetector 统一负责
