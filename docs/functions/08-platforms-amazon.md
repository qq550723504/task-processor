# 函数清单 - Platforms/Amazon 模块

生成时间: 2026-03-10

## platforms/amazon/api 模块

### auth.go
```go
func NewAuthManager(clientID, clientSecret, refreshToken string) *AuthManager
func (a *AuthManager) GetAccessToken(ctx context.Context) (string, error)
func (a *AuthManager) refreshAccessToken(ctx context.Context) (string, error)
func (a *AuthManager) SetAccessToken(token string, expiresIn int)
func (a *AuthManager) IsTokenValid() bool
```

### aws_signer.go
```go
func NewAWSSigner(accessKeyID, secretAccessKey, region string) *AWSSigner
func (s *AWSSigner) SignRequest(req *http.Request, payload []byte) error
func (s *AWSSigner) calculatePayloadHash(payload []byte) string
func (s *AWSSigner) createSignature(req *http.Request, payloadHash string, timestamp time.Time) string
func (s *AWSSigner) createCanonicalRequest(req *http.Request, payloadHash string) string
func (s *AWSSigner) createStringToSign(canonicalRequest string, timestamp time.Time) string
```

### catalog.go
```go
func (c *Client) SearchCatalog(ctx context.Context, req *SearchCatalogRequest) (*SearchCatalogResponse, error)
func (c *Client) GetSellerListings(ctx context.Context, req *GetSellerListingsRequest) (*GetSellerListingsResponse, error)
func (c *Client) GetCatalogItem(ctx context.Context, asin string, marketplaceID string) (*CatalogItem, error)
func (c *Client) SearchCatalogByKeyword(ctx context.Context, keyword string) (*SearchCatalogResponse, error)
```

### client.go
```go
func NewClient(cfg *Config) *Client
func getRegionEndpoint(region string) string
func (c *Client) GetAccessToken(ctx context.Context) (string, error)
func (c *Client) SetAccessToken(token string, expiresIn int)
func (c *Client) GetMarketplaceID() string
func (c *Client) GetRegion() string
func (c *Client) doRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error)
```

### inventory.go
```go
func (c *Client) UpdateInventory(ctx context.Context, req *InventoryRequest) (*InventoryResponse, error)
func (c *Client) buildInventoryFeedXML(req *InventoryRequest) string
func (c *Client) createInventoryFeed(ctx context.Context, feedContent string) (string, error)
func (c *Client) createFeedDocument(ctx context.Context, content, contentType string) (string, error)
func (c *Client) uploadFeedDocument(ctx context.Context, url, content, contentType string) error
func (c *Client) GetFeedStatus(ctx context.Context, feedID string) (*FeedStatus, error)
```

### listing_operations.go
```go
func (c *Client) CreateListing(ctx context.Context, req *ListingRequest) (*ListingResponse, error)
func (c *Client) ValidateListing(ctx context.Context, req *ListingRequest) (*ListingResponse, error)
func (c *Client) createListingWithMode(ctx context.Context, req *ListingRequest, mode string) (*ListingResponse, error)
func (c *Client) UpdateListing(ctx context.Context, req *ListingRequest) (*ListingResponse, error)
func (c *Client) DeleteListing(ctx context.Context, sku string) error
func (c *Client) PatchListing(ctx context.Context, sku string, patches []ListingPatch) (*ListingResponse, error)
```

### listing_details.go
```go
func (c *Client) GetDetailedListing(ctx context.Context, sku, marketplaceID string) (*ListingResponse, error)
func (c *Client) printProductDetails(data map[string]interface{})
func (c *Client) GetListingByASIN(ctx context.Context, asin, marketplaceID string) (*ListingResponse, error)
```

### listing_postman_test.go
```go
func (c *Client) TestPostmanGetListing(ctx context.Context, sku, marketplaceID string) error
func (c *Client) TestPostmanPutListing(ctx context.Context, req *PostmanListingRequest) (*ListingResponse, error)
```

### orders.go
```go
func (c *Client) GetOrders(ctx context.Context, req *GetOrdersRequest) (*GetOrdersResponse, error)
func (c *Client) GetOrder(ctx context.Context, orderID string) (*Order, error)
func (c *Client) GetOrderItems(ctx context.Context, orderID string) (*GetOrderItemsResponse, error)
```

### products.go
```go
func (c *Client) GetProductPricing(ctx context.Context, asin, marketplaceID string) (*PricingResponse, error)
func (c *Client) GetCompetitivePricing(ctx context.Context, asins []string, marketplaceID string) (*CompetitivePricingResponse, error)
func (c *Client) GetMyFeesEstimate(ctx context.Context, req *FeesEstimateRequest) (*FeesEstimateResponse, error)
```

## platforms/amazon 模块

### processor.go
```go
func NewAmazonPlatformProcessor(apiClient *api.Client, logger *logrus.Logger) *AmazonPlatformProcessor
func (app *AmazonPlatformProcessor) Process(ctx context.Context, task *model.Task) error
func (app *AmazonPlatformProcessor) ProcessProduct(ctx context.Context, product *model.Product) error
func (app *AmazonPlatformProcessor) ValidateProduct(product *model.Product) error
func (app *AmazonPlatformProcessor) Close() error
```

### product_mapper.go
```go
func NewProductMapper(logger *logrus.Logger) *ProductMapper
func (pm *ProductMapper) MapToAmazonListing(product *model.Product) (*api.ListingRequest, error)
func (pm *ProductMapper) MapFromAmazonListing(listing *api.ListingResponse) (*model.Product, error)
func (pm *ProductMapper) MapImages(images []string) []api.ImageInfo
func (pm *ProductMapper) MapAttributes(product *model.Product) map[string]interface{}
```

### listing_builder.go
```go
func NewListingBuilder() *ListingBuilder
func (lb *ListingBuilder) BuildListing(product *model.Product) (*api.ListingRequest, error)
func (lb *ListingBuilder) BuildProductType(product *model.Product) string
func (lb *ListingBuilder) BuildAttributes(product *model.Product) map[string]interface{}
func (lb *ListingBuilder) ValidateListing(listing *api.ListingRequest) error
```
