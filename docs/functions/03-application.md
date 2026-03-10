# 函数清单 - Application 模块

生成时间: 2026-03-10

## application/crawler 模块

### distributed_crawler_client.go ⭐ 已重构
```go
func NewDistributedCrawlerClient(rabbitmqClient *rabbitmq.Client, logger *logrus.Logger) (*DistributedCrawlerClient, error)
func (c *DistributedCrawlerClient) SubmitCrawlTask(ctx context.Context, req *CrawlRequest) (*CrawlResult, error)
func (c *DistributedCrawlerClient) SetTimeout(timeout time.Duration)
func (c *DistributedCrawlerClient) GetStats() map[string]interface{}
func (c *DistributedCrawlerClient) Close() error
func (c *DistributedCrawlerClient) ensureListenerStarted() error
func (c *DistributedCrawlerClient) buildMessageData(taskMessage interface{}, req *CrawlRequest) map[string]interface{}
func (c *DistributedCrawlerClient) createPendingTask(ctx context.Context, taskID int64) *PendingTask
func (c *DistributedCrawlerClient) publishTask(...) error
func (c *DistributedCrawlerClient) waitForResult(pendingTask *PendingTask, taskID int64) (*CrawlResult, error)
```

## application/product 模块

### distributed_fetcher.go ⭐ 已重构
```go
func NewDistributedProductFetcher(...) (*DistributedProductFetcher, error)
func (f *DistributedProductFetcher) FetchProduct(req *domainProduct.FetchRequest) (*model.Product, error)
func (f *DistributedProductFetcher) CacheProduct(req *domainProduct.FetchRequest, product *model.Product) error
func (f *DistributedProductFetcher) CacheVariants(req *domainProduct.FetchRequest, variants []*model.Product) error
func (f *DistributedProductFetcher) FetchVariants(req *domainProduct.FetchRequest, variantASINs []string) ([]*model.Product, error)
func (f *DistributedProductFetcher) GetStats() map[string]interface{}
func (f *DistributedProductFetcher) Close() error
func (f *DistributedProductFetcher) fetchFromDistributedCrawler(req *domainProduct.FetchRequest) (*model.Product, error)
func (f *DistributedProductFetcher) buildProductURL(req *domainProduct.FetchRequest) string
func (f *DistributedProductFetcher) getZipcode(region string) string
func (f *DistributedProductFetcher) calculatePriority(req *domainProduct.FetchRequest) int
func (f *DistributedProductFetcher) shouldUseCrawler(platform string) bool
```
