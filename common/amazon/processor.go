package amazon

import (
	"fmt"
	"strings"
	"task-processor/common/config"
	"time"

	"github.com/playwright-community/playwright-go"
	"github.com/sirupsen/logrus"
)

// AmazonProcessor Amazon爬虫处理器
type AmazonProcessor struct {
	browserPool *BrowserPool
	config      *config.AmazonConfig
	usePool     bool
}

// NewAmazonProcessor 使用全局配置创建Amazon处理器
func NewAmazonProcessor(cfg *config.AmazonConfig) *AmazonProcessor {
	// 创建浏览器池
	poolConfig := DefaultBrowserPoolConfig()

	// 如果配置中的PoolSize为0，使用默认值
	if cfg.PoolSize > 0 {
		poolConfig.PoolSize = cfg.PoolSize
	}

	logrus.Infof("创建Amazon处理器，浏览器池大小: %d (配置值: %d)", poolConfig.PoolSize, cfg.PoolSize)
	browserPool := NewBrowserPool(cfg, poolConfig)

	// 初始化浏览器池
	if err := browserPool.Initialize(); err != nil {
		logrus.Infof("初始化浏览器池失败: %v，将使用单浏览器模式", err)
		return &AmazonProcessor{
			config:  cfg,
			usePool: false,
		}
	}

	logrus.Info("浏览器池初始化成功")

	return &AmazonProcessor{
		browserPool: browserPool,
		config:      cfg,
		usePool:     true,
	}
}

// Process 处理Amazon产品页面
func (ap *AmazonProcessor) Process(url string, zipcode string) (*Product, error) {
	startTime := time.Now()
	logrus.Infof("开始处理Amazon产品: %s", url)

	if ap.usePool {
		return ap.processWithPool(url, zipcode, startTime)
	}
	return ap.processWithSingleBrowser(url, zipcode, startTime)
}

// ProcessBatch 批量处理多个Amazon产品页面
// 使用同一个浏览器实例处理多个产品，提高效率
func (ap *AmazonProcessor) ProcessBatch(requests []ProductRequest) []ProductResult {
	if len(requests) == 0 {
		return []ProductResult{}
	}

	logrus.Infof("开始批量处理 %d 个Amazon产品", len(requests))
	startTime := time.Now()

	results := make([]ProductResult, len(requests))

	if ap.usePool {
		// 使用浏览器池批量处理
		instance, err := ap.browserPool.Acquire()
		if err != nil {
			// 如果获取实例失败，所有任务都失败
			for i := range results {
				results[i] = ProductResult{
					Product: nil,
					Error:   fmt.Errorf("获取浏览器实例失败: %w", err),
				}
			}
			return results
		}

		logrus.Infof("使用浏览器实例 %d 批量处理", instance.ID)

		// 跟踪是否有严重错误需要重建实例
		var lastError error

		// 使用同一个实例处理所有请求
		for i, req := range requests {
			product, err := ap.processWithInstance(instance, req.URL, req.Zipcode)
			results[i] = ProductResult{
				Product: product,
				Error:   err,
			}

			if err != nil {
				lastError = err
				logrus.Infof("批量处理 [%d/%d] 失败: %s - %v", i+1, len(requests), req.URL, err)

				// 如果检测到风控或严重错误，停止批量处理
				if ap.browserPool.isBlockedOrSeriousError(err) {
					logrus.Infof("检测到浏览器实例 %d 被风控，停止批量处理", instance.ID)
					// 将剩余任务标记为失败
					for j := i + 1; j < len(requests); j++ {
						results[j] = ProductResult{
							Product: nil,
							Error:   fmt.Errorf("浏览器实例被风控，跳过处理"),
						}
					}
					break
				}
			} else {
				logrus.Infof("批量处理 [%d/%d] 成功: %s", i+1, len(requests), product.Title)
			}
		}

		// 使用带错误检测的释放方法
		ap.browserPool.ReleaseWithError(instance, lastError)
	} else {
		// 单浏览器模式，逐个处理
		for i, req := range requests {
			product, err := ap.Process(req.URL, req.Zipcode)
			results[i] = ProductResult{
				Product: product,
				Error:   err,
			}
		}
	}

	duration := time.Since(startTime)
	successCount := 0
	for _, result := range results {
		if result.Error == nil {
			successCount++
		}
	}

	logrus.Infof("批量处理完成: 成功 %d/%d, 耗时: %v", successCount, len(requests), duration)

	return results
}

// processWithInstance 使用指定实例处理产品
func (ap *AmazonProcessor) processWithInstance(instance *BrowserInstance, url string, zipcode string) (*Product, error) {
	page := instance.Page
	browserManager := instance.Manager

	// 添加语言参数
	urlWithLang := ap.addLanguageParam(url)

	// 导航到页面
	if err := browserManager.NavigateTo(page, urlWithLang); err != nil {
		return nil, fmt.Errorf("导航失败: %w", err)
	}

	// 处理可能出现的"Continue shopping"按钮
	if err := ap.handleContinueShoppingButton(page); err != nil {
		logrus.Infof("处理Continue shopping按钮时出现警告: %v", err)
	}

	// 检查产品是否存在（在设置邮编之前检查，避免浪费时间）
	if err := ap.checkProductExists(page); err != nil {
		return nil, err // 产品不存在的错误不应触发浏览器重建
	}

	// 设置邮编（带缓存检查）
	if err := ap.setAndVerifyZipcodeWithCache(instance, zipcode); err != nil {
		return nil, fmt.Errorf("设置邮编失败: %w", err)
	}

	// 提取产品信息
	now := time.Now()
	product := &Product{
		URL:       url,
		Zipcode:   zipcode,
		Asin:      ap.extractASINFromURL(url),
		Currency:  ap.getCurrencyFromURL(url), // Set currency based on Amazon domain
		Timestamp: NullableTime{Time: &now},
	}

	extractor := NewCompositeExtractor()
	if err := extractor.Extract(page, product); err != nil {
		return nil, fmt.Errorf("提取产品信息失败: %w", err)
	}

	return product, nil
}

// processWithPool 使用浏览器池处理
func (ap *AmazonProcessor) processWithPool(url string, zipcode string, startTime time.Time) (*Product, error) {
	maxRetries := 2 // 最多重试2次（即总共尝试3次）

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			logrus.Infof("开始第 %d 次重试处理产品: %s", attempt, url)
		}

		// 从池中获取浏览器实例
		instance, err := ap.browserPool.Acquire()
		if err != nil {
			return nil, fmt.Errorf("获取浏览器实例失败: %w", err)
		}

		// 使用实例处理产品
		product, processErr := ap.processWithInstance(instance, url, zipcode)

		// 检查是否为严重错误
		if processErr != nil && ap.browserPool.isBlockedOrSeriousError(processErr) {
			logrus.Warnf("检测到浏览器实例 %d 出现严重错误: %v", instance.ID, processErr)

			// 同步重建浏览器实例
			newInstance := ap.browserPool.RecreateInstanceSync(instance)

			// 如果重建失败
			if newInstance == nil {
				logrus.Errorf("重建浏览器实例失败，任务失败: %s", url)
				return nil, fmt.Errorf("重建浏览器实例失败: %w", processErr)
			}

			// 如果是最后一次尝试，返回错误
			if attempt >= maxRetries {
				logrus.Errorf("已达到最大重试次数，任务失败: %s", url)
				// 将重建的实例放回池中
				ap.browserPool.Release(newInstance)
				return nil, processErr
			}

			// 否则继续下一次重试
			logrus.Infof("浏览器实例已重建为 %d，准备重试", newInstance.ID)
			continue
		}

		// 如果没有错误或不是严重错误，正常释放实例
		ap.browserPool.ReleaseWithError(instance, processErr)

		if processErr != nil {
			return nil, processErr
		}

		return product, nil
	}

	// 理论上不会到这里
	return nil, fmt.Errorf("处理产品失败，已达到最大重试次数")
}

// processWithSingleBrowser 使用单浏览器处理
func (ap *AmazonProcessor) processWithSingleBrowser(url string, zipcode string, startTime time.Time) (*Product, error) {
	browserManager := NewBrowserManager(ap.config)

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

	urlWithLang := ap.addLanguageParam(url)

	if err := browserManager.NavigateTo(page, urlWithLang); err != nil {
		return nil, fmt.Errorf("导航失败: %w", err)
	}

	// 处理可能出现的"Continue shopping"按钮
	if err := ap.handleContinueShoppingButton(page); err != nil {
		logrus.Infof("处理Continue shopping按钮时出现警告: %v", err)
	}

	// 设置邮编
	zipcodeSetter := NewZipcodeSetter(browserManager)
	if err := zipcodeSetter.SetAndVerifyZipcode(page, zipcode); err != nil {
		return nil, fmt.Errorf("设置邮编失败: %w", err)
	}

	now := time.Now()
	product := &Product{
		URL:       url,
		Zipcode:   zipcode,
		Asin:      ap.extractASINFromURL(url),
		Currency:  ap.getCurrencyFromURL(url), // Set currency based on Amazon domain
		Timestamp: NullableTime{Time: &now},
	}

	extractor := NewCompositeExtractor()
	if err := extractor.Extract(page, product); err != nil {
		return nil, fmt.Errorf("提取产品信息失败: %w", err)
	}

	return product, nil
}

// handleContinueShoppingButton 处理可能出现的"Continue shopping"按钮（多语言支持）
func (ap *AmazonProcessor) handleContinueShoppingButton(page playwright.Page) error {
	time.Sleep(2 * time.Second)

	continueShoppingSelectors := getContinueShoppingSelectors()

	for _, selector := range continueShoppingSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			visible, err := element.IsVisible()
			if err == nil && visible {
				if err := element.Click(); err == nil {
					logrus.Infof("成功点击Continue shopping按钮: %s", selector)
					time.Sleep(3 * time.Second)
					return nil
				}
			}
		}
	}

	return nil
}

// getContinueShoppingSelectors 获取"继续购物"按钮的多语言选择器
func getContinueShoppingSelectors() []string {
	return []string{
		// 英语
		"button:has-text('Continue Shopping')",
		"button:has-text('Continue shopping')",
		"a:has-text('Continue Shopping')",
		"a:has-text('Continue shopping')",
		"button:has-text('Continue')",
		"a:has-text('Continue')",
		// 中文
		"button:has-text('继续购物')",
		"a:has-text('继续购物')",
		"button:has-text('继续')",
		"a:has-text('继续')",
		// 日语
		"button:has-text('買い物を続ける')",
		"button:has-text('ショッピングを続ける')",
		"a:has-text('買い物を続ける')",
		"a:has-text('ショッピングを続ける')",
		"button:has-text('続ける')",
		"a:has-text('続ける')",
		// 西班牙语
		"button:has-text('Seguir comprando')",
		"button:has-text('Continuar comprando')",
		"a:has-text('Seguir comprando')",
		"a:has-text('Continuar comprando')",
		"button:has-text('Continuar')",
		"a:has-text('Continuar')",
		// 阿拉伯语
		"button:has-text('متابعة التسوق')",
		"a:has-text('متابعة التسوق')",
		// 德语
		"button:has-text('Weiter einkaufen')",
		"a:has-text('Weiter einkaufen')",
		"button:has-text('Weiter')",
		"a:has-text('Weiter')",
		// 法语
		"button:has-text('Continuer mes achats')",
		"a:has-text('Continuer mes achats')",
		"button:has-text('Continuer')",
		"a:has-text('Continuer')",
		// 意大利语
		"button:has-text('Continua lo shopping')",
		"a:has-text('Continua lo shopping')",
		"button:has-text('Continua')",
		"a:has-text('Continua')",
		// 葡萄牙语
		"button:has-text('Continuar comprando')",
		"a:has-text('Continuar comprando')",
		"button:has-text('Continuar')",
		"a:has-text('Continuar')",
		// 荷兰语
		"button:has-text('Verder winkelen')",
		"a:has-text('Verder winkelen')",
		"button:has-text('Verder')",
		"a:has-text('Verder')",
	}
}

// addLanguageParam 添加语言参数，强制使用英语
func (ap *AmazonProcessor) addLanguageParam(url string) string {
	// Remove existing language parameter if present
	if strings.Contains(url, "language=") {
		// Remove the language parameter
		parts := strings.Split(url, "?")
		if len(parts) == 2 {
			baseURL := parts[0]
			queryParams := strings.Split(parts[1], "&")
			var newParams []string
			for _, param := range queryParams {
				if !strings.HasPrefix(param, "language=") {
					newParams = append(newParams, param)
				}
			}
			if len(newParams) > 0 {
				url = baseURL + "?" + strings.Join(newParams, "&")
			} else {
				url = baseURL
			}
		}
	}

	separator := "?"
	if strings.Contains(url, "?") {
		separator = "&"
	}

	// Force English language
	return url + separator + "language=en_US"
}

// extractASINFromURL 从URL提取ASIN
func (ap *AmazonProcessor) extractASINFromURL(url string) string {
	parts := strings.Split(url, "/")
	for i, part := range parts {
		if part == "dp" && i+1 < len(parts) {
			asinPart := parts[i+1]
			if idx := strings.Index(asinPart, "?"); idx != -1 {
				asinPart = asinPart[:idx]
			}
			return asinPart
		}
	}
	return ""
}

// getCurrencyFromURL 从URL获取对应地区的货币
func (ap *AmazonProcessor) getCurrencyFromURL(url string) string {
	if strings.Contains(url, "amazon.co.jp") {
		return "JPY"
	} else if strings.Contains(url, "amazon.co.uk") {
		return "GBP"
	} else if strings.Contains(url, "amazon.de") {
		return "EUR"
	} else if strings.Contains(url, "amazon.fr") {
		return "EUR"
	} else if strings.Contains(url, "amazon.it") {
		return "EUR"
	} else if strings.Contains(url, "amazon.es") {
		return "EUR"
	} else if strings.Contains(url, "amazon.ca") {
		return "CAD"
	} else if strings.Contains(url, "amazon.com.au") {
		return "AUD"
	} else if strings.Contains(url, "amazon.com.mx") {
		return "MXN" // 墨西哥比索
	} else if strings.Contains(url, "amazon.sa") {
		return "SAR" // 沙特里亚尔
	} else if strings.Contains(url, "amazon.ae") {
		return "AED" // 阿联酋迪拉姆
	}
	return "USD" // Default to USD for amazon.com and other domains
}

// checkProductExists 检查产品是否存在
func (ap *AmazonProcessor) checkProductExists(page playwright.Page) error {
	// 获取当前页面URL用于日志
	currentURL := page.URL()
	logrus.Infof("开始检查产品页面有效性: URL=%s", currentURL)

	// 等待短暂时间让页面加载
	time.Sleep(1 * time.Second)

	// 获取页面标题用于诊断
	pageTitle, _ := page.Title()

	// 检查常见的"产品不存在"标识
	notFoundSelectors := []string{
		"text=Sorry, we couldn't find that page",
		"text=Page Not Found",
		"text=Dogs of Amazon",
		"#ap_email",                               // 登录页面
		"text=Enter the characters you see below", // 验证码页面
	}

	logrus.Debug("检查产品不存在标识...")
	for _, selector := range notFoundSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			visible, err := element.IsVisible()
			if err == nil && visible {
				logrus.Errorf("❌ 检测到产品不存在或页面异常: selector=%s, URL=%s, PageTitle=%s",
					selector, currentURL, pageTitle)
				return &ProductNotFoundError{Message: fmt.Sprintf("产品页面不存在或无法访问: %s, URL: %s", selector, currentURL)}
			}
		}
	}

	// 检查是否有产品标题元素（使用短超时）
	titleExists := false
	foundSelector := ""
	titleSelectors := []string{
		"#productTitle",
		"#title",
		"h1[id*='title']",
		"h1.product-title",
		"span#productTitle",
		"div#title_feature_div h1",
		"[data-feature-name='title'] h1",
	}

	logrus.Debug("检查产品标题元素...")
	for _, selector := range titleSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			titleExists = true
			foundSelector = selector
			break
		} else if err != nil {
			logrus.Debugf("查询选择器失败: selector=%s, error=%v", selector, err)
		}
	}

	if !titleExists {
		// 获取页面HTML的前500字符用于诊断
		//bodyHTML, _ := page.Evaluate("() => document.body.innerHTML.substring(0, 500)")
		logrus.Warnf("❌ 未找到产品标题元素，页面可能异常")
		logrus.Warnf("页面URL: %s", currentURL)
		logrus.Warnf("页面标题: %s", pageTitle)
		//logrus.Warnf("页面HTML前500字符: %v", bodyHTML)
		logrus.Warnf("尝试的选择器: %v", titleSelectors)

		return &ProductNotFoundError{Message: fmt.Sprintf("❌ 产品页面缺少必要元素 (URL: %s, Title: %s)", currentURL, pageTitle)}
	}

	logrus.Infof("产品页面有效性检查通过: selector=%s", foundSelector)

	return nil
}

// setAndVerifyZipcodeWithCache 设置并验证邮编（带缓存检查）
func (ap *AmazonProcessor) setAndVerifyZipcodeWithCache(instance *BrowserInstance, zipcode string) error {
	instance.mu.Lock()
	currentZipcode := instance.CurrentZipcode
	instance.mu.Unlock()

	// 如果当前邮编已经是目标邮编，跳过设置
	if currentZipcode == zipcode {
		logrus.Infof("浏览器实例 %d 的邮编已经是 %s，跳过设置", instance.ID, zipcode)
		return nil
	}

	logrus.Infof("浏览器实例 %d 需要设置邮编: %s -> %s", instance.ID, currentZipcode, zipcode)

	// 创建邮编设置器并设置邮编
	zipcodeSetter := NewZipcodeSetter(instance.Manager)
	if err := zipcodeSetter.SetAndVerifyZipcode(instance.Page, zipcode); err != nil {
		return err
	}

	// 更新缓存的邮编
	instance.mu.Lock()
	instance.CurrentZipcode = zipcode
	instance.mu.Unlock()

	logrus.Infof("浏览器实例 %d 邮编已更新为: %s", instance.ID, zipcode)
	return nil
}

// Shutdown 关闭处理器
func (ap *AmazonProcessor) Shutdown() {
	if ap.usePool && ap.browserPool != nil {
		logrus.Info("关闭浏览器池...")
		ap.browserPool.Shutdown()
	}
}
