# 函数清单 - Platforms/SHEIN 模块

生成时间: 2026-03-10

## platforms/shein/api 模块

### interface.go
```go
func (fs *FlexibleString) UnmarshalJSON(data []byte) error
func (fs FlexibleString) String() string
```

## platforms/shein 模块

### processor.go
```go
func NewSheinProcessor(apiClient *client.APIClient, logger *logrus.Logger) *SheinProcessor
func (sp *SheinProcessor) Process(ctx context.Context, task *model.Task) error
func (sp *SheinProcessor) ProcessProduct(ctx context.Context, product *model.Product) error
func (sp *SheinProcessor) ValidateProduct(product *model.Product) error
func (sp *SheinProcessor) Close() error
```

### product_mapper.go
```go
func NewProductMapper(logger *logrus.Logger) *ProductMapper
func (pm *ProductMapper) MapToSheinProduct(product *model.Product) (*SheinProduct, error)
func (pm *ProductMapper) MapFromSheinProduct(sheinProduct *SheinProduct) (*model.Product, error)
func (pm *ProductMapper) MapImages(images []string) []SheinImage
func (pm *ProductMapper) MapAttributes(product *model.Product) map[string]interface{}
func (pm *ProductMapper) MapSizeChart(product *model.Product) *SizeChart
```

### listing_builder.go
```go
func NewListingBuilder() *ListingBuilder
func (lb *ListingBuilder) BuildListing(product *model.Product) (*SheinListing, error)
func (lb *ListingBuilder) BuildProductInfo(product *model.Product) *ProductInfo
func (lb *ListingBuilder) BuildPriceInfo(product *model.Product) *PriceInfo
func (lb *ListingBuilder) BuildStyleInfo(product *model.Product) *StyleInfo
func (lb *ListingBuilder) ValidateListing(listing *SheinListing) error
```

### client.go
```go
func NewAPIClient(storeID int64, config *Config) *APIClient
func (c *APIClient) SetCookies(cookies []*http.Cookie)
func (c *APIClient) ReloadCookies() error
func (c *APIClient) HasCookies() bool
func (c *APIClient) GetStoreID() int64
func (c *APIClient) SendRequest(ctx context.Context, request *Request) (*Response, error)
```

### api_operations.go
```go
func (c *APIClient) CreateProduct(ctx context.Context, product *SheinProduct) (*CreateProductResponse, error)
func (c *APIClient) UpdateProduct(ctx context.Context, productID string, product *SheinProduct) (*UpdateProductResponse, error)
func (c *APIClient) DeleteProduct(ctx context.Context, productID string) error
func (c *APIClient) GetProduct(ctx context.Context, productID string) (*SheinProduct, error)
func (c *APIClient) ListProducts(ctx context.Context, filter *ProductFilter) (*ListProductsResponse, error)
```

### inventory_operations.go
```go
func (c *APIClient) UpdateInventory(ctx context.Context, productID string, skuInventory []SKUInventory) error
func (c *APIClient) GetInventory(ctx context.Context, productID string) (*InventoryInfo, error)
func (c *APIClient) BatchUpdateInventory(ctx context.Context, updates []InventoryUpdate) error
```

### price_operations.go
```go
func (c *APIClient) UpdatePrice(ctx context.Context, productID string, priceInfo *PriceInfo) error
func (c *APIClient) GetPrice(ctx context.Context, productID string) (*PriceInfo, error)
func (c *APIClient) BatchUpdatePrice(ctx context.Context, updates []PriceUpdate) error
```

### order_operations.go
```go
func (c *APIClient) GetOrders(ctx context.Context, filter *OrderFilter) (*GetOrdersResponse, error)
func (c *APIClient) GetOrder(ctx context.Context, orderID string) (*Order, error)
func (c *APIClient) UpdateOrderStatus(ctx context.Context, orderID string, status OrderStatus) error
func (c *APIClient) GetOrderItems(ctx context.Context, orderID string) ([]OrderItem, error)
func (c *APIClient) ConfirmShipment(ctx context.Context, orderID string, shipmentInfo *ShipmentInfo) error
```

### category_operations.go
```go
func (c *APIClient) GetCategories(ctx context.Context) ([]Category, error)
func (c *APIClient) GetCategoryAttributes(ctx context.Context, categoryID string) ([]Attribute, error)
func (c *APIClient) ValidateCategory(ctx context.Context, categoryID string) (bool, error)
```

### image_operations.go
```go
func (c *APIClient) UploadImage(ctx context.Context, imageData []byte, imageName string) (*ImageUploadResponse, error)
func (c *APIClient) BatchUploadImages(ctx context.Context, images []ImageUpload) (*BatchImageUploadResponse, error)
func (c *APIClient) DeleteImage(ctx context.Context, imageID string) error
func (c *APIClient) GetImageURL(imageID string) string
```

## 特点说明

### SHEIN 平台特点
- **时尚属性**：支持尺码表、风格信息等时尚相关属性
- **SKU 管理**：支持多 SKU 的库存和价格管理
- **图片管理**：完善的图片上传和管理功能
- **分类管理**：支持分类和属性查询
- **FlexibleString**：灵活的字符串类型处理（支持字符串和数字）

### 主要功能模块
1. **产品管理** - 创建、更新、删除产品
2. **库存管理** - 多 SKU 库存管理
3. **价格管理** - 多 SKU 价格管理
4. **订单管理** - 订单查询和发货确认
5. **分类管理** - 分类和属性查询
6. **图片管理** - 图片上传和管理

### 与其他平台的区别
- **时尚导向**：更注重时尚属性（尺码、风格、颜色等）
- **SKU 复杂度**：支持更复杂的 SKU 组合
- **图片要求**：对图片质量和数量有特殊要求
- **分类体系**：有独特的分类和属性体系

### 重构建议
- ⚠️ 可以与 TEMU、Amazon 提取公共的平台接口
- ⚠️ 产品映射逻辑可以统一
- ⚠️ API 操作可以提取基类
- ✅ FlexibleString 是一个很好的类型适配器设计
