# 函数清单 - Crawler/Alibaba1688 模块

生成时间: 2026-03-10

## crawler/alibaba1688 模块

### browser_manager.go
```go
func NewBrowserManager(cfg *config.Config) *BrowserManager
func (bm *BrowserManager) CreateBrowser() (playwright.Browser, playwright.BrowserContext, playwright.Page, func(), error)
func (bm *BrowserManager) Close()
```

### captcha_handler.go
```go
func (ch *CaptchaHandler) HandlePageCaptcha(page playwright.Page) error
```

### captcha_human_behavior.go
```go
func (ch *CaptchaHandler) randomDelay(maxMs int) int
func (ch *CaptchaHandler) easeInOutCubic(t float64) float64
func (ch *CaptchaHandler) complexEasing(t float64) float64
func (ch *CaptchaHandler) simulateHumanMouseMove(page playwright.Page, fromX, fromY, toX, toY float64) error
```

### captcha_other.go
```go
func (ch *CaptchaHandler) handleLoginPrompt(page playwright.Page) error
func (ch *CaptchaHandler) handleOtherCaptcha(page playwright.Page) error
func (ch *CaptchaHandler) waitForManualCaptcha(page playwright.Page, captchaType string) error
```

### captcha_slider.go
```go
func (ch *CaptchaHandler) handleSliderCaptcha(page playwright.Page) error
func (ch *CaptchaHandler) performSliderAction(page playwright.Page, sliderBtn playwright.ElementHandle, strategy string) error
func (ch *CaptchaHandler) calculateSlideDistance(page playwright.Page, buttonBox *playwright.Rect) (float64, error)
func (ch *CaptchaHandler) checkSliderSuccess(page playwright.Page) bool
func (ch *CaptchaHandler) waitForManualSlider(page playwright.Page) error
```

### captcha_types.go
```go
func NewCaptchaHandler() *CaptchaHandler
func (ch *CaptchaHandler) DetectCaptchaType(page playwright.Page) CaptchaType
```

### extractor/attribute_extractor.go
```go
func NewAttributeExtractor() *AttributeExtractor
func (aeo *AttributeExtractor) Extract(page playwright.Page, product *model.Product1688) error
func (aeo *AttributeExtractor) extractAttributes(page playwright.Page) (map[string]string, error)
```

### extractor/basic_info_extractor.go
```go
func NewBasicInfoExtractor() *BasicInfoExtractor
func (bie *BasicInfoExtractor) Extract(page playwright.Page, product *model.Product1688) error
func (bie *BasicInfoExtractor) extractTitle(page playwright.Page) (string, error)
func (bie *BasicInfoExtractor) extractPrice(page playwright.Page) (float64, error)
func (bie *BasicInfoExtractor) extractImages(page playwright.Page) ([]string, error)
```

### extractor/detail_extractor.go
```go
func NewDetailExtractor() *DetailExtractor
func (de *DetailExtractor) Extract(page playwright.Page, product *model.Product1688) error
func (de *DetailExtractor) extractDescription(page playwright.Page) (string, error)
func (de *DetailExtractor) extractSpecifications(page playwright.Page) (map[string]string, error)
```

### extractor/price_extractor.go
```go
func NewPriceExtractor() *PriceExtractor
func (pe *PriceExtractor) Extract(page playwright.Page, product *model.Product1688) error
func (pe *PriceExtractor) extractPriceRange(page playwright.Page) (float64, float64, error)
func (pe *PriceExtractor) extractMOQ(page playwright.Page) (int, error)
```

### extractor/supplier_extractor.go
```go
func NewSupplierExtractor() *SupplierExtractor
func (se *SupplierExtractor) Extract(page playwright.Page, product *model.Product1688) error
func (se *SupplierExtractor) extractSupplierInfo(page playwright.Page) (*SupplierInfo, error)
```

### processor.go
```go
func NewAlibaba1688Processor(cfg *config.Config) *Alibaba1688Processor
func (ap *Alibaba1688Processor) ProcessProduct(url string) (*model.Product1688, error)
func (ap *Alibaba1688Processor) ProcessProductWithTimeout(ctx context.Context, url string, timeout time.Duration) (*model.Product1688, error)
func (ap *Alibaba1688Processor) Close()
```

### page_handler.go
```go
func NewPageHandler(cfg *config.Config) *PageHandler
func (ph *PageHandler) HandlePage(page playwright.Page, url string) (*model.Product1688, error)
func (ph *PageHandler) navigateToProduct(page playwright.Page, url string) error
func (ph *PageHandler) extractProductData(page playwright.Page) (*model.Product1688, error)
```
