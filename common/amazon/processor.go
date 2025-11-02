package amazon

import (
	"fmt"
	"log"
	"strings"
	"task-processor/common/config"
	"time"

	"github.com/playwright-community/playwright-go"
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
	poolConfig.PoolSize = cfg.PoolSize
	browserPool := NewBrowserPool(cfg, poolConfig)

	// 初始化浏览器池
	if err := browserPool.Initialize(); err != nil {
		log.Printf("初始化浏览器池失败: %v，将使用单浏览器模式", err)
		return &AmazonProcessor{
			config:  cfg,
			usePool: false,
		}
	}

	log.Println("浏览器池初始化成功")

	return &AmazonProcessor{
		browserPool: browserPool,
		config:      cfg,
		usePool:     true,
	}
}

// Process 处理Amazon产品页面
func (ap *AmazonProcessor) Process(url string, zipcode string) (*Product, error) {
	startTime := time.Now()
	log.Printf("开始处理Amazon产品: %s", url)

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

	log.Printf("开始批量处理 %d 个Amazon产品", len(requests))
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

		log.Printf("使用浏览器实例 %d 批量处理", instance.ID)

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
				log.Printf("批量处理 [%d/%d] 失败: %s - %v", i+1, len(requests), req.URL, err)

				// 如果检测到风控或严重错误，停止批量处理
				if ap.browserPool.isBlockedOrSeriousError(err) {
					log.Printf("检测到浏览器实例 %d 被风控，停止批量处理", instance.ID)
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
				log.Printf("批量处理 [%d/%d] 成功: %s", i+1, len(requests), product.Title)
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

	log.Printf("批量处理完成: 成功 %d/%d, 耗时: %v", successCount, len(requests), duration)

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
		log.Printf("处理Continue shopping按钮时出现警告: %v", err)
	}

	// 设置邮编（带缓存检查）
	if err := ap.setAndVerifyZipcodeWithCache(instance, zipcode); err != nil {
		return nil, fmt.Errorf("设置邮编失败: %w", err)
	}

	// 提取产品信息
	product := &Product{
		URL:       url,
		Zipcode:   zipcode,
		Asin:      ap.extractASINFromURL(url),
		Currency:  ap.getCurrencyFromURL(url), // Set currency based on Amazon domain
		Timestamp: time.Now(),
	}

	extractor := NewCompositeExtractor()
	if err := extractor.Extract(page, product); err != nil {
		return nil, fmt.Errorf("提取产品信息失败: %w", err)
	}

	return product, nil
}

// processWithPool 使用浏览器池处理
func (ap *AmazonProcessor) processWithPool(url string, zipcode string, startTime time.Time) (*Product, error) {
	// 从池中获取浏览器实例
	instance, err := ap.browserPool.Acquire()
	if err != nil {
		return nil, fmt.Errorf("获取浏览器实例失败: %w", err)
	}

	// 使用带错误检测的释放方法
	var processErr error
	defer func() {
		ap.browserPool.ReleaseWithError(instance, processErr)
	}()

	// 使用实例处理产品
	product, processErr := ap.processWithInstance(instance, url, zipcode)
	if processErr != nil {
		return nil, processErr
	}

	duration := time.Since(startTime)
	log.Printf("成功处理产品: %s (耗时: %v, 实例: %d)", product.Title, duration, instance.ID)

	return product, nil
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
		log.Printf("处理Continue shopping按钮时出现警告: %v", err)
	}

	// 设置邮编
	zipcodeSetter := NewZipcodeSetter(browserManager)
	if err := zipcodeSetter.SetAndVerifyZipcode(page, zipcode); err != nil {
		return nil, fmt.Errorf("设置邮编失败: %w", err)
	}

	product := &Product{
		URL:       url,
		Zipcode:   zipcode,
		Asin:      ap.extractASINFromURL(url),
		Currency:  ap.getCurrencyFromURL(url), // Set currency based on Amazon domain
		Timestamp: time.Now(),
	}

	extractor := NewCompositeExtractor()
	if err := extractor.Extract(page, product); err != nil {
		return nil, fmt.Errorf("提取产品信息失败: %w", err)
	}

	duration := time.Since(startTime)
	log.Printf("成功处理产品: %s (耗时: %v)", product.Title, duration)

	return product, nil
}

// handleContinueShoppingButton 处理可能出现的"Continue shopping"按钮
func (ap *AmazonProcessor) handleContinueShoppingButton(page playwright.Page) error {
	time.Sleep(2 * time.Second)

	continueShoppingSelectors := []string{
		"button:text('Continue shopping')",
		"button:text('继续购物')",
		"a:text('Continue shopping')",
	}

	for _, selector := range continueShoppingSelectors {
		element, err := page.QuerySelector(selector)
		if err == nil && element != nil {
			visible, err := element.IsVisible()
			if err == nil && visible {
				if err := element.Click(); err == nil {
					log.Printf("成功点击Continue shopping按钮: %s", selector)
					time.Sleep(3 * time.Second)
					return nil
				}
			}
		}
	}

	return nil
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
	}
	return "USD" // Default to USD for amazon.com and other domains
}

// setAndVerifyZipcodeWithCache 设置并验证邮编（带缓存检查）
func (ap *AmazonProcessor) setAndVerifyZipcodeWithCache(instance *BrowserInstance, zipcode string) error {
	instance.mu.Lock()
	currentZipcode := instance.CurrentZipcode
	instance.mu.Unlock()

	// 如果当前邮编已经是目标邮编，跳过设置
	if currentZipcode == zipcode {
		log.Printf("浏览器实例 %d 的邮编已经是 %s，跳过设置", instance.ID, zipcode)
		return nil
	}

	log.Printf("浏览器实例 %d 需要设置邮编: %s -> %s", instance.ID, currentZipcode, zipcode)

	// 创建邮编设置器并设置邮编
	zipcodeSetter := NewZipcodeSetter(instance.Manager)
	if err := zipcodeSetter.SetAndVerifyZipcode(instance.Page, zipcode); err != nil {
		return err
	}

	// 更新缓存的邮编
	instance.mu.Lock()
	instance.CurrentZipcode = zipcode
	instance.mu.Unlock()

	log.Printf("浏览器实例 %d 邮编已更新为: %s", instance.ID, zipcode)
	return nil
}

// Shutdown 关闭处理器
func (ap *AmazonProcessor) Shutdown() {
	if ap.usePool && ap.browserPool != nil {
		log.Println("关闭浏览器池...")
		ap.browserPool.Shutdown()
	}
}
