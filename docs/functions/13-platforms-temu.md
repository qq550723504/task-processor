# 函数清单 - Platforms/TEMU 模块

生成时间: 2026-03-10

## platforms/temu/api/client 模块

### auth.go
```go
func NewAuthManager(logger *logrus.Entry) *AuthManager
func NewAuthManagerWithDependencies(...) *AuthManager
func (a *AuthManager) SendRequestWithAuth(client APIClientInterface, request map[string]any, result any) error
func (a *AuthManager) validateCookies(client APIClientInterface) error
```

### auth_config.go
```go
func DefaultAuthConfig() *AuthConfig
func NewAuthContext(storeID int64) *AuthContext
func (ctx *AuthContext) IncrementAttempt()
func (ctx *AuthContext) IncrementAuthError(err error)
func (ctx *AuthContext) ResetAuthError()
func (ctx *AuthContext) ShouldRetry() bool
func (ctx *AuthContext) GetAttemptCount() int
```

### auth_error_detector.go
```go
func NewTemuErrorDetector(config *AuthConfig, logger *logrus.Entry) *TemuErrorDetector
func (d *TemuErrorDetector) IsAuthenticationError(err error) bool
func (d *TemuErrorDetector) containsTemuAuthErrorCode(errStr string) bool
func (d *TemuErrorDetector) containsAuthErrorKeyword(errStr string) bool
func (d *TemuErrorDetector) GetErrorType(err error) ErrorType
```

### auth_factory.go
```go
func NewAuthManagerFactory(logger *logrus.Entry) *AuthManagerFactory
func (f *AuthManagerFactory) CreateAuthManager() *AuthManager
func (f *AuthManagerFactory) CreateAuthManagerWithCustomConfig(config *AuthConfig) *AuthManager
func (f *AuthManagerFactory) CreateComponents(config *AuthConfig) (...)
```

### auth_pause_handler.go
```go
func NewTemuPauseHandler(logger *logrus.Entry) *TemuPauseHandler
func (h *TemuPauseHandler) SetPauseKeyForAuthExpired(client APIClientInterface, reason string) error
func (h *TemuPauseHandler) validateClient(client APIClientInterface) error
func (h *TemuPauseHandler) ClearPauseKey(client APIClientInterface) error
```

### auth_request_sender.go
```go
func NewRequestSender(logger *logrus.Entry) *RequestSender
func (s *RequestSender) SendRequest(client APIClientInterface, request map[string]any, result any) error
func (s *RequestSender) extractRequestParams(request map[string]any) (*RequestParams, error)
func (s *RequestSender) buildFullURL(client APIClientInterface, url string) (string, error)
func (s *RequestSender) validateResponse(response *req.Response, method, fullURL string) error
```

### auth_retry_handler.go
```go
func NewTemuRetryHandler(...) *TemuRetryHandler
func (h *TemuRetryHandler) SendRequestWithRetry(client APIClientInterface, request map[string]any, result any) error
func (h *TemuRetryHandler) handleAuthError(client APIClientInterface, ctx *AuthContext, err error) error
func (h *TemuRetryHandler) handleFinalAuthFailure(client APIClientInterface, ctx *AuthContext, reason string) error
func (h *TemuRetryHandler) sendRequestOnce(client APIClientInterface, request map[string]any, result any) error
```

### client.go
```go
func NewAPIClient(storeID int64, managementClient *management.ClientManager) *APIClient
func (c *APIClient) initializeMallID(managementClient *management.ClientManager)
func (c *APIClient) SetCookies(cookies []*http.Cookie)
func (c *APIClient) ReloadCookies() error
func (c *APIClient) HasCookies() bool
func (c *APIClient) GetStoreID() int64
func (c *APIClient) GetMallID() string
func (c *APIClient) SendRequest(request map[string]any, result any) error
```

### config.go
```go
func DefaultConfig() *Config
func NewConfigFromSettings(timeout, maxTimeout, retryInterval int, retryCount int) *Config
func GetDefaultHeaders() map[string]string
func (c *Config) Validate() error
```

## platforms/temu 模块

### processor.go
```go
func NewTemuProcessor(apiClient *client.APIClient, logger *logrus.Logger) *TemuProcessor
func (tp *TemuProcessor) Process(ctx context.Context, task *model.Task) error
func (tp *TemuProcessor) ProcessProduct(ctx context.Context, product *model.Product) error
func (tp *TemuProcessor) ValidateProduct(product *model.Product) error
func (tp *TemuProcessor) Close() error
```

### product_mapper.go
```go
func NewProductMapper(logger *logrus.Logger) *ProductMapper
func (pm *ProductMapper) MapToTemuProduct(product *model.Product) (*TemuProduct, error)
func (pm *ProductMapper) MapFromTemuProduct(temuProduct *TemuProduct) (*model.Product, error)
func (pm *ProductMapper) MapImages(images []string) []TemuImage
func (pm *ProductMapper) MapAttributes(product *model.Product) map[string]interface{}
```

### listing_builder.go
```go
func NewListingBuilder() *ListingBuilder
func (lb *ListingBuilder) BuildListing(product *model.Product) (*TemuListing, error)
func (lb *ListingBuilder) BuildProductInfo(product *model.Product) *ProductInfo
func (lb *ListingBuilder) BuildPriceInfo(product *model.Product) *PriceInfo
func (lb *ListingBuilder) ValidateListing(listing *TemuListing) error
```

### api_operations.go
```go
func (c *APIClient) CreateProduct(ctx context.Context, product *TemuProduct) (*CreateProductResponse, error)
func (c *APIClient) UpdateProduct(ctx context.Context, productID string, product *TemuProduct) (*UpdateProductResponse, error)
func (c *APIClient) DeleteProduct(ctx context.Context, productID string) error
func (c *APIClient) GetProduct(ctx context.Context, productID string) (*TemuProduct, error)
func (c *APIClient) ListProducts(ctx context.Context, filter *ProductFilter) (*ListProductsResponse, error)
```

### inventory_operations.go
```go
func (c *APIClient) UpdateInventory(ctx context.Context, productID string, quantity int) error
func (c *APIClient) GetInventory(ctx context.Context, productID string) (*InventoryInfo, error)
func (c *APIClient) BatchUpdateInventory(ctx context.Context, updates []InventoryUpdate) error
```

### price_operations.go
```go
func (c *APIClient) UpdatePrice(ctx context.Context, productID string, price float64) error
func (c *APIClient) GetPrice(ctx context.Context, productID string) (*PriceInfo, error)
func (c *APIClient) BatchUpdatePrice(ctx context.Context, updates []PriceUpdate) error
```

### order_operations.go
```go
func (c *APIClient) GetOrders(ctx context.Context, filter *OrderFilter) (*GetOrdersResponse, error)
func (c *APIClient) GetOrder(ctx context.Context, orderID string) (*Order, error)
func (c *APIClient) UpdateOrderStatus(ctx context.Context, orderID string, status OrderStatus) error
func (c *APIClient) GetOrderItems(ctx context.Context, orderID string) ([]OrderItem, error)
```

## 特点说明

### TEMU 平台特点
- **认证管理复杂**：包含多层认证处理（AuthManager、RetryHandler、ErrorDetector）
- **Cookie 管理**：需要管理和刷新 Cookie
- **错误重试**：完善的错误检测和重试机制
- **暂停处理**：认证失败时的暂停机制
- **Mall ID 管理**：每个店铺有独立的 Mall ID

### 主要功能模块
1. **认证模块** - 处理 TEMU API 认证
2. **产品管理** - 创建、更新、删除产品
3. **库存管理** - 库存更新和查询
4. **价格管理** - 价格更新和查询
5. **订单管理** - 订单查询和状态更新

### 重构建议
- ✅ 认证模块已经做了很好的职责分离
- ⚠️ 可以考虑提取公共的 API 操作基类
- ⚠️ 产品映射逻辑可以与其他平台统一接口
