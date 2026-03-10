# 函数清单 - Pkg/Utils 模块

生成时间: 2026-03-10

## internal/pkg/amazon 模块

### domain_resolver.go
```go
func NewDomainResolver() *DomainResolver
func (r *DomainResolver) GetAmazonDomainByRegion(region string) string
func (r *DomainResolver) GetLanguageByRegion(region string) string
func (r *DomainResolver) BuildAmazonProductURL(region, asin string) string
func (r *DomainResolver) GetRegionByDomain(domain string) string
func (r *DomainResolver) IsValidRegion(region string) bool
```

## internal/pkg/downloader 模块

### image_downloader.go
```go
func NewImageDownloader() *ImageDownloader
func (d *ImageDownloader) DownloadImage(imageURL string) ([]byte, string, error)
func (d *ImageDownloader) DownloadImageWithRetry(imageURL string, maxRetries int) ([]byte, string, error)
func (d *ImageDownloader) GetImageInfo(imageURL string) (width, height int, size int64, err error)
func (d *ImageDownloader) parseImageConfig(resp *req.Response, _ string) (width, height int, size int64, err error)
func (d *ImageDownloader) getFilenameFromURL(imageURL string) string
func (d *ImageDownloader) validateImageURL(imageURL string) error
func (d *ImageDownloader) Close()
```

### anti_bot.go
```go
func (d *ImageDownloader) isBlocked() bool
func (d *ImageDownloader) getBlockRemainTime() time.Duration
func (d *ImageDownloader) applyRateLimit()
func (d *ImageDownloader) createDynamicRequest(targetURL string) *req.Request
func (d *ImageDownloader) detectBlock(resp *req.Response) bool
func (d *ImageDownloader) handleBlockDetection(resp *req.Response)
```

### image_processor.go
```go
func NewImageProcessor() *ImageProcessor
func (p *ImageProcessor) ProcessImageForPlatform(imageData []byte, platform string) ([]byte, error)
func (p *ImageProcessor) generateRandomID() string
func (p *ImageProcessor) processJPEGWithRandom(imageData []byte, platform, randomID string) ([]byte, error)
func (p *ImageProcessor) processPNGWithRandom(imageData []byte, platform, randomID string) ([]byte, error)
func (p *ImageProcessor) ResizeImage(imageData []byte, maxWidth, maxHeight int) ([]byte, error)
func (p *ImageProcessor) CompressImage(imageData []byte, quality int) ([]byte, error)
```

## internal/pkg/mathutil 模块

### math.go
```go
func Min(a, b int) int
func Max(a, b int) int
func MinInt64(a, b int64) int64
func MaxInt64(a, b int64) int64
func MinFloat64(a, b float64) float64
func MaxFloat64(a, b float64) float64
func Clamp(value, min, max int) int
func ClampFloat64(value, min, max float64) float64
```

### round.go
```go
func Round(value float64, precision int) float64
func RoundUp(value float64, precision int) float64
func RoundDown(value float64, precision int) float64
func RoundToNearest(value float64, multiple float64) float64
```

## internal/pkg/strutil 模块

### string.go
```go
func IsEmpty(s string) bool
func IsNotEmpty(s string) bool
func Trim(s string) string
func TrimSpace(s string) string
func Contains(s, substr string) bool
func ContainsAny(s string, substrs ...string) bool
func HasPrefix(s, prefix string) bool
func HasSuffix(s, suffix string) bool
```

### convert.go
```go
func ToInt(s string) (int, error)
func ToInt64(s string) (int64, error)
func ToFloat64(s string) (float64, error)
func ToBool(s string) (bool, error)
func ToString(v interface{}) string
func ToStringSlice(v interface{}) []string
```

### format.go
```go
func FormatPrice(price float64) string
func FormatPercentage(value float64) string
func FormatNumber(value int64) string
func TruncateString(s string, maxLen int) string
func PadLeft(s string, length int, pad string) string
func PadRight(s string, length int, pad string) string
```

## internal/pkg/pricing 模块

### calculator.go
```go
func NewPriceCalculator() *PriceCalculator
func (pc *PriceCalculator) CalculateSellingPrice(cost, margin float64) float64
func (pc *PriceCalculator) CalculateMargin(cost, sellingPrice float64) float64
func (pc *PriceCalculator) CalculateProfit(cost, sellingPrice float64) float64
func (pc *PriceCalculator) CalculateProfitMargin(cost, sellingPrice float64) float64
func (pc *PriceCalculator) ApplyDiscount(price, discountPercent float64) float64
```

### converter.go
```go
func NewCurrencyConverter() *CurrencyConverter
func (cc *CurrencyConverter) Convert(amount float64, from, to string) (float64, error)
func (cc *CurrencyConverter) GetExchangeRate(from, to string) (float64, error)
func (cc *CurrencyConverter) UpdateRates(rates map[string]float64)
func (cc *CurrencyConverter) SupportedCurrencies() []string
```

## internal/pkg/types 模块

### types.go
```go
func NewNullString(s string) NullString
func (ns *NullString) Scan(value interface{}) error
func (ns NullString) Value() (driver.Value, error)
func NewNullInt64(i int64) NullInt64
func (ni *NullInt64) Scan(value interface{}) error
func (ni NullInt64) Value() (driver.Value, error)
func NewNullFloat64(f float64) NullFloat64
func (nf *NullFloat64) Scan(value interface{}) error
func (nf NullFloat64) Value() (driver.Value, error)
```

## internal/pkg/utils 模块

### retry.go
```go
func Retry(fn func() error, maxRetries int, delay time.Duration) error
func RetryWithBackoff(fn func() error, maxRetries int, initialDelay time.Duration, maxDelay time.Duration) error
func RetryWithContext(ctx context.Context, fn func() error, maxRetries int, delay time.Duration) error
```

### timeout.go
```go
func WithTimeout(ctx context.Context, timeout time.Duration, fn func(context.Context) error) error
func WithDeadline(ctx context.Context, deadline time.Time, fn func(context.Context) error) error
```

### hash.go
```go
func MD5Hash(data []byte) string
func SHA256Hash(data []byte) string
func GenerateRandomString(length int) string
func GenerateUUID() string
```
