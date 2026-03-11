// Package amazon 提供Amazon实例处理功能
package amazon

import (
	"fmt"
	"strings"
	"task-processor/internal/crawler/amazon/browser"
	"task-processor/internal/crawler/amazon/extractor"
	"task-processor/internal/domain/model"
	"time"

	"github.com/sirupsen/logrus"
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

// ProcessWithInstance 使用指定浏览器实例处理产品
func (ip *InstanceProcessor) ProcessWithInstance(instance *browser.BrowserInstance, url string, zipcode string) (*model.Product, error) {
	if instance == nil {
		return nil, fmt.Errorf("浏览器实例为空")
	}

	logrus.Infof("使用浏览器实例 %d 处理产品: %s", instance.ID, url)

	// 创建新页面
	page, err := instance.Manager.NewPage()
	if err != nil {
		return nil, fmt.Errorf("创建页面失败: %w", err)
	}
	defer func() {
		if closeErr := page.Close(); closeErr != nil {
			logrus.Warnf("关闭页面失败: %v", closeErr)
		}
	}()

	// 获取市场对应的默认货币
	currency := ip.urlHelper.GetCurrencyFromURL(url)

	// 添加语言和货币参数
	urlWithParams := ip.urlHelper.AddLanguageAndCurrencyParams(url, currency)

	// 导航到产品页面
	_, err = page.Goto(urlWithParams)
	if err != nil {
		return nil, fmt.Errorf("导航到页面失败: %w", err)
	}

	// 优先处理可能出现的"Continue shopping"按钮
	if err := ip.productChecker.HandleContinueShoppingButton(page); err != nil {
		logrus.Warnf("处理Continue shopping按钮时出错: %v", err)
		// 不返回错误，继续处理
	}

	// 再等待页面准备就绪，使用更短的超时时间避免长时间卡死
	if err := ip.productChecker.WaitForPageReady(page, 15*time.Second); err != nil {
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
			logrus.Errorf("设置邮编失败: %v", err)

			// 检查是否需要重建浏览器实例
			if ip.shouldRecreateInstance(err) {
				return nil, fmt.Errorf("需要重建浏览器实例: %w", err)
			}

			// 邮编设置失败，不应该继续抓取数据，因为价格和货币信息会不准确
			return nil, fmt.Errorf("邮编设置失败，终止数据抓取: %w", err)
		}
	}

	// 获取站点信息，用于设置货币
	marketplace := ip.urlHelper.GetMarketplaceFromURL(url)
	expectedCurrency := ip.urlHelper.GetCurrencyFromURL(url)

	// 设置并验证货币
	if expectedCurrency != "" {
		currencySetter := browser.NewCurrencySetter(instance.Manager)
		if err := currencySetter.SetAndVerifyCurrency(page, expectedCurrency); err != nil {
			logrus.Warnf("设置货币失败: %v (将继续抓取，但货币可能不准确)", err)
			// 货币设置失败不应该终止抓取，只记录警告
		}
	}

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

	// 验证提取的数据
	if err := ip.validateProductData(product); err != nil {
		return nil, fmt.Errorf("产品数据验证失败: %w", err)
	}

	logrus.Infof("成功处理产品: (ASIN: %s)", product.Asin)
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
		logrus.Warnf("产品价格信息缺失或无效: ASIN=%s", product.Asin)
		// 不返回错误，某些产品可能暂时没有价格
	}

	return nil
}

// ProcessBatchWithInstance 使用指定实例批量处理产品
func (ip *InstanceProcessor) ProcessBatchWithInstance(instance *browser.BrowserInstance, requests []model.ProductRequest) []model.ProductResult {
	results := make([]model.ProductResult, len(requests))

	for i, req := range requests {
		product, err := ip.ProcessWithInstance(instance, req.URL, req.Zipcode)
		results[i] = model.ProductResult{
			Product: product,
			Error:   err,
		}

		if err != nil {
			logrus.Errorf("批量处理 [%d/%d] 失败: %s - %v", i+1, len(requests), req.URL, err)
		} else {
			logrus.Infof("批量处理 [%d/%d] 成功: %s", i+1, len(requests), product.Asin)
		}
	}

	return results
}

// ProcessWithRetry 带重试的产品处理
func (ip *InstanceProcessor) ProcessWithRetry(instance *browser.BrowserInstance, url string, zipcode string, maxRetries int) (*model.Product, error) {
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logrus.Infof("重试处理产品 (第%d次): %s", attempt, url)
			// 重试前等待一段时间
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		product, err := ip.ProcessWithInstance(instance, url, zipcode)
		if err == nil {
			return product, nil
		}

		lastErr = err
		logrus.Warnf("处理产品失败 (第%d次尝试): %v", attempt+1, err)

		// 如果是严重错误（如被阻止），不再重试
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
	}

	for _, serious := range seriousErrors {
		if strings.Contains(errorMsg, serious) {
			return true
		}
	}

	return false
}
