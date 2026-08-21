// Package alibaba1688 提供1688单个产品处理器
package alibaba1688

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"task-processor/internal/core/config"
	"task-processor/internal/core/logger"
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
	pageOperator   *PageOperator
}

// NewSingleProcessor 创建新的单个产品处理器
func NewSingleProcessor(cfg *config.Config, urlHelper *URLHelper, productChecker *ProductChecker) *SingleProcessor {
	return &SingleProcessor{
		config:         cfg,
		urlHelper:      urlHelper,
		productChecker: productChecker,
		extractor:      extractor.NewProductExtractor(),
		pageOperator:   NewPageOperator(),
	}
}

// Prepare provisions the browser runtime without navigating to a product.
func (sp *SingleProcessor) Prepare(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sp == nil || sp.config == nil {
		return errors.New("1688 single processor is not configured")
	}
	manager := sp.newPublicBrowserManager()
	defer manager.Close()
	if _, _, _, _, err := manager.CreateBrowser(); err != nil {
		return err
	}
	return ctx.Err()
}

// ProcessWithSingleBrowser 使用单个浏览器处理产品
func (sp *SingleProcessor) ProcessWithSingleBrowser(url string, startTime time.Time) (*model.Product1688, error) {
	return sp.processWithBrowserManager(url, startTime, sp.newPublicBrowserManager(), false)
}

// ProcessWithAccountProfile uses a short-lived browser manager configured for one 1688 login account.
func (sp *SingleProcessor) ProcessWithAccountProfile(url string, startTime time.Time, profile AccountProfile) (*model.Product1688, error) {
	return sp.processWithBrowserManager(url, startTime, NewBrowserManagerForAccountProfile(sp.config, profile), true)
}

func (sp *SingleProcessor) newPublicBrowserManager() *BrowserManager {
	return NewPublicBrowserManager(sp.config)
}

func (sp *SingleProcessor) processWithBrowserManager(url string, startTime time.Time, browserManager *BrowserManager, allowManualCaptcha bool) (*model.Product1688, error) {
	logger.GetGlobalLogger("crawler/alibaba1688").Infof("使用单浏览器模式处理1688产品: %s", url)

	// 验证和标准化URL
	normalizedURL, err := sp.urlHelper.ValidateAndNormalizeURL(url)
	if err != nil {
		return nil, NewPublicAccessError(PublicAccessFailureInvalidURL, fmt.Errorf("URL验证失败: %w", err))
	}

	// 创建浏览器实例
	_, _, page, cleanup, err := browserManager.CreateBrowser()
	if err != nil {
		return nil, NewPublicAccessError(PublicAccessFailureBrowser, err)
	}
	defer cleanup()

	// 导航到产品页面
	var navErr error
	if allowManualCaptcha {
		navErr = sp.pageOperator.NavigateToProduct(page, normalizedURL)
	} else {
		navErr = sp.pageOperator.NavigateToProductWithoutManualCaptcha(page, normalizedURL)
	}
	if navErr != nil {
		kind := PublicAccessFailureTransport
		if isChallengeError(navErr) {
			kind = PublicAccessFailureChallenge
		}
		return nil, NewPublicAccessError(kind, fmt.Errorf("导航到产品页面失败: %w", navErr))
	}

	// 提取产品信息
	product, err := sp.extractor.ExtractProductFromPage(page, normalizedURL)
	if err != nil {
		return nil, NewPublicAccessError(PublicAccessFailureMissingFields, fmt.Errorf("提取产品信息失败: %w", err))
	}

	// 验证产品信息
	if validateErr := sp.productChecker.ValidateProduct(product); validateErr != nil {
		return nil, NewPublicAccessError(PublicAccessFailureValidation, fmt.Errorf("产品信息验证失败: %w", validateErr))
	}

	duration := time.Since(startTime)
	logger.GetGlobalLogger("crawler/alibaba1688").Infof("单浏览器模式处理完成: %s, 耗时: %v", product.Title, duration)

	return product, nil
}

func isChallengeError(err error) bool {
	if err == nil {
		return false
	}
	var accessErr *PublicAccessError
	if errors.As(err, &accessErr) && accessErr != nil {
		return accessErr.Kind == PublicAccessFailureChallenge
	}
	message := strings.ToLower(err.Error())
	for _, marker := range []string{"captcha", "challenge", "验证码", "登录", "拦截"} {
		if strings.Contains(message, strings.ToLower(marker)) {
			return true
		}
	}
	return false
}
