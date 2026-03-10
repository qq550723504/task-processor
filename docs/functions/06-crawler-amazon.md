# 函数清单 - Crawler/Amazon 模块

生成时间: 2026-03-10

## crawler/amazon 模块

### batch_processor.go
```go
func NewBatchProcessor(browserPool *browser.BrowserPool, urlHelper *URLHelper, productChecker *ProductChecker) *BatchProcessor
func (bp *BatchProcessor) ProcessWithPool(requests []model.ProductRequest, browserPool *browser.BrowserPool) []model.ProductResult
func (bp *BatchProcessor) ProcessWithSingleBrowser(requests []model.ProductRequest, processor interface{...}) []model.ProductResult
```

### browser/browser_pool.go
```go
func DefaultBrowserPoolConfig() *BrowserPoolConfig
func NewBrowserPool(cfg *config.Config, poolConfig *BrowserPoolConfig) *BrowserPool
func (bp *BrowserPool) Initialize() error
func (bp *BrowserPool) Acquire() (*BrowserInstance, error)
func (bp *BrowserPool) Release(instance *BrowserInstance)
func (bp *BrowserPool) Close()
func (bp *BrowserPool) GetStats() PoolStats
func (bp *BrowserPool) GetAvailableCount() int
func (bp *BrowserPool) GetTotalCount() int
```

### browser/config_manager.go
```go
func NewConfigManager() *ConfigManager
func (cm *ConfigManager) GenerateBrowserConfig(cfg *config.Config, strategy string, presetName string, instanceID int) *sharedbrowser.BrowserConfig
func (cm *ConfigManager) applyBaseConfig(browserConfig *sharedbrowser.BrowserConfig, cfg *config.Config)
func (cm *ConfigManager) copyBrowserConfig(src *sharedbrowser.BrowserConfig) *sharedbrowser.BrowserConfig
func (cm *ConfigManager) ShouldUseRandomConfig(cfg *config.AmazonConfig) bool
func (cm *ConfigManager) GetPresetConfig(presetName string) *sharedbrowser.BrowserConfig
```

### browser/error_detector.go
```go
func NewErrorDetector() *ErrorDetector
func (ed *ErrorDetector) IsBlockedOrSeriousError(err error) bool
func (ed *ErrorDetector) IsTimeoutError(err error) bool
func (ed *ErrorDetector) IsNetworkError(err error) bool
func (ed *ErrorDetector) IsCaptchaError(err error) bool
func (ed *ErrorDetector) IsRecoverableError(err error) bool
func (ed *ErrorDetector) GetErrorType(err error) ErrorType
```

### browser/health_checker.go
```go
func NewHealthChecker(pool *BrowserPool) *HealthChecker
func (hc *HealthChecker) HealthCheck(instance *BrowserInstance) bool
func (hc *HealthChecker) GetPoolStats() map[string]interface{}
func (hc *HealthChecker) LogPoolStats()
func (hc *HealthChecker) StartHealthCheckRoutine(ctx context.Context)
func (hc *HealthChecker) CheckInstanceHealth(instance *BrowserInstance) error
```

### browser/instance_manager.go
```go
func NewInstanceManager(pool *BrowserPool) *InstanceManager
func (im *InstanceManager) CreateInstance(id int) (*BrowserInstance, error)
func (im *InstanceManager) RecreateInstanceSync(oldInstance *BrowserInstance) *BrowserInstance
func (im *InstanceManager) RecreateInstanceAsync(oldInstance *BrowserInstance)
func (im *InstanceManager) CloseInstance(instance *BrowserInstance)
func (im *InstanceManager) GetInstanceStats(instance *BrowserInstance) InstanceStats
```

### browser/manager.go
```go
func NewBrowserManager(cfg *config.Config) *BrowserManager
func NewBrowserManagerWithConfig(cfg *config.Config, strategy string, presetName string, instanceID int) *BrowserManager
func (bm *BrowserManager) GetConfigManager() *ConfigManager
func (bm *BrowserManager) NavigateTo(page playwright.Page, url string) error
func (bm *BrowserManager) setLanguageCookies(url string) error
func (bm *BrowserManager) Close()
```

### browser/pool_manager.go
```go
func NewPoolManager(pool *BrowserPool) *PoolManager
func (pm *PoolManager) ProcessWithTimeout(ctx context.Context, url, zipcode string, timeout time.Duration, processor ProductProcessor) (*model.Product, error)
func (pm *PoolManager) processProduct(ctx context.Context, url, zipcode string, processor ProductProcessor) *ProcessResult
func (pm *PoolManager) acquireInstanceWithTimeout(ctx context.Context, timeout time.Duration) (*BrowserInstance, error)
func (pm *PoolManager) releaseInstanceSafely(instance *BrowserInstance, err error)
```

### browser/zipcode_getter.go
```go
func NewZipcodeGetter() *ZipcodeGetter
func (zg *ZipcodeGetter) GetCurrentZipcode(page playwright.Page) (string, error)
func extractCityName(text string) string
func (zg *ZipcodeGetter) SetZipcode(page playwright.Page, zipcode string) error
```

### processor.go
```go
func NewAmazonProcessor(cfg *config.Config) *AmazonProcessor
func (ap *AmazonProcessor) ProcessProduct(url, zipcode string) (*model.Product, error)
func (ap *AmazonProcessor) ProcessProductWithTimeout(ctx context.Context, url, zipcode string, timeout time.Duration) (*model.Product, error)
func (ap *AmazonProcessor) GetBrowserPool() *browser.BrowserPool
func (ap *AmazonProcessor) Close()
```

### page_handler.go
```go
func NewPageHandler(cfg *config.Config) *PageHandler
func (ph *PageHandler) HandlePage(page playwright.Page, url, zipcode string) (*model.Product, error)
func (ph *PageHandler) navigateToProduct(page playwright.Page, url string) error
func (ph *PageHandler) handleZipcode(page playwright.Page, zipcode string) error
func (ph *PageHandler) extractProductData(page playwright.Page) (*model.Product, error)
```

### data_extractor.go
```go
func NewDataExtractor() *DataExtractor
func (de *DataExtractor) ExtractProductData(page playwright.Page) (*model.Product, error)
func (de *DataExtractor) extractTitle(page playwright.Page) (string, error)
func (de *DataExtractor) extractPrice(page playwright.Page) (float64, error)
func (de *DataExtractor) extractImages(page playwright.Page) ([]string, error)
func (de *DataExtractor) extractDescription(page playwright.Page) (string, error)
func (de *DataExtractor) extractVariations(page playwright.Page) ([]model.Variation, error)
```

### url_helper.go
```go
func NewURLHelper() *URLHelper
func (uh *URLHelper) BuildProductURL(region, asin string) string
func (uh *URLHelper) ExtractASIN(url string) string
func (uh *URLHelper) IsValidProductURL(url string) bool
func (uh *URLHelper) NormalizeURL(url string) string
```

### product_checker.go
```go
func NewProductChecker() *ProductChecker
func (pc *ProductChecker) IsProductAvailable(page playwright.Page) bool
func (pc *ProductChecker) CheckProductStatus(page playwright.Page) ProductStatus
func (pc *ProductChecker) HasCaptcha(page playwright.Page) bool
```
