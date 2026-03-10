# 函数清单 - Domain 模块

生成时间: 2026-03-10

## domain/model 模块

### task.go ⭐ 已增强
```go
// 业务方法（新增）
func (t *Task) IsValid() bool
func (t *Task) IsCrawlerTask() bool
func (t *Task) GetBasePlatform() string
func (t *Task) CanRetry() bool
func (t *Task) IsHighPriority() bool
func (t *Task) IsNormalPriority() bool
func (t *Task) IsLowPriority() bool
func (t *Task) GetPriorityLevel() string
func (t *Task) IsVariantTask() bool
func (t *Task) PlatformMatches(targetPlatform string) bool
```

### task_status.go
```go
func (s TaskStatus) String() string
func (s TaskStatus) Int16() int16
func (s TaskStatus) IsTerminal() bool
func (s TaskStatus) IsProcessing() bool
func (s TaskStatus) CanTransitionTo(target TaskStatus) bool
```

### amazon_product.go
```go
func (vv *VariationValue) UnmarshalJSON(data []byte) error
func (e *ProductNotFoundError) Error() string
func (e *ProductNotFoundError) Unwrap() error
func (nt *NullableTime) UnmarshalJSON(data []byte) error
func (nt NullableTime) MarshalJSON() ([]byte, error)
```

## domain/errors 模块 ⭐ 新增

### task_errors.go
```go
func NewTaskError(code ErrorCode, taskID int64, operation, message string, err error) *TaskError
func (e *TaskError) Error() string
func (e *TaskError) Unwrap() error
func (e *TaskError) IsRetryable() bool

// 便捷构造函数
func NewInvalidTaskError(taskID int64, message string) *TaskError
func NewPlatformMismatchError(taskID int64, taskPlatform, processorPlatform string) *TaskError
func NewProcessingError(taskID int64, operation string, err error) *TaskError
func NewStoreNotFoundError(taskID, storeID int64, err error) *TaskError
func NewConversionError(taskID int64, err error) *TaskError
```

## domain/message 模块 ⭐ 新增

### types.go
```go
func (s *SuccessData) ToMap() map[string]any
func NewSuccessData(platform, productID string, storeID int64) *SuccessData
```

## domain/queue 模块 ⭐ 新增

### naming.go
```go
func NewNamingService() *NamingService
func (s *NamingService) BuildCrawlerQueueName(platform string, priority int) string
func (s *NamingService) BuildTaskQueueName(platform string, priority int) string
func (s *NamingService) GetPriorityLevel(priority int) PriorityLevel
func (s *NamingService) IsCrawlerPlatform(platform string) bool
func (s *NamingService) getPriorityLevel(priority int) PriorityLevel
func (s *NamingService) extractBasePlatform(platform string) string
```

## domain/product 模块

### cache_manager.go
```go
func NewCacheManager(rawJsonDataClient RawJsonDataClient, logger *logrus.Entry) *CacheManager
func (c *CacheManager) GetFromCache(req *FetchRequest) (*model.Product, error)
func (c *CacheManager) SaveToCache(req *FetchRequest, product *model.Product) error
func (c *CacheManager) CacheProduct(req *FetchRequest, product *model.Product) error
func (c *CacheManager) CacheVariants(req *FetchRequest, variants []*model.Product) error
```

### crawler_client.go
```go
func NewCrawlerClient(amazonProcessor *amazon.AmazonProcessor, amazonConfig *config.AmazonConfig, logger *logrus.Entry) *CrawlerClient
func (c *CrawlerClient) ShouldUseCrawler(platform string) bool
func (c *CrawlerClient) FetchFromCrawler(req *FetchRequest) (*model.Product, error)
func (c *CrawlerClient) GetZipcodeForRegion(region string) string
func (c *CrawlerClient) getZipcodeForRegion(region string) string
```

### data_parser.go
```go
func NewDataParser(logger *logrus.Entry) *DataParser
func (p *DataParser) ParseAmazonProduct(jsonData string) (*model.Product, error)
func (p *DataParser) recalculateIsAvailable(product *model.Product) bool
func (p *DataParser) ValidateProductData(product *model.Product) error
func (p *DataParser) NormalizeProductData(product *model.Product)
```

### domain_resolver.go
```go
func NewDomainResolver() *DomainResolver
func (r *DomainResolver) GetAmazonDomainByRegion(region string) string
func (r *DomainResolver) BuildAmazonProductURL(region, asin string) string
```

### factory/product_factory.go
```go
func NewProductServiceFactory(logger *logrus.Entry) *ProductServiceFactory
func (f *ProductServiceFactory) CreateProductService(...) *ProductService
func (f *ProductServiceFactory) CreateLegacyProductFetcher(...) *ProductFetcher
```
