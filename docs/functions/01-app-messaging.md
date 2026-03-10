# 函数清单 - App/Messaging 模块

生成时间: 2026-03-10

## app/messaging 模块

### rabbitmq_service.go
```go
func NewRabbitMQService(cfg *config.RabbitMQConfig, logger *logrus.Logger) *RabbitMQService
func (s *RabbitMQService) RegisterProcessor(platform string, processor worker.Processor) error
func (s *RabbitMQService) SetComponents(...)
func (s *RabbitMQService) SetQueueConfigs(configs []config.QueueConfig)
func (s *RabbitMQService) Start(ctx context.Context) error
func (s *RabbitMQService) Stop(ctx context.Context) error
func (s *RabbitMQService) IsConnected() bool
func (s *RabbitMQService) GetStats() map[string]interface{}
func (s *RabbitMQService) GetClient() *rabbitmq.Client
func (s *RabbitMQService) GetConsumer() *rabbitmq.MessageConsumer
func (s *RabbitMQService) registerMessageHandlers() error
```

### service_manager.go
```go
func NewServiceManager(rabbitmqConfig *config.RabbitMQConfig, logger *logrus.Logger) (*ServiceManager, error)
func (sm *ServiceManager) RegisterProcessor(platform string, processor worker.Processor) error
func (sm *ServiceManager) Start(ctx context.Context) error
func (sm *ServiceManager) Stop(ctx context.Context) error
func (sm *ServiceManager) IsStarted() bool
func (sm *ServiceManager) GetConfig() *config.RabbitMQConfig
func (sm *ServiceManager) GetStats() map[string]interface{}
func (sm *ServiceManager) Wait()
func (sm *ServiceManager) GetClient() *rabbitmq.Client
func (sm *ServiceManager) initializeServices() error
func (sm *ServiceManager) startHTTPServers() error
func (sm *ServiceManager) startHealthServer()
func (sm *ServiceManager) startMetricsServer()
func (sm *ServiceManager) handleHealth(w http.ResponseWriter, r *http.Request)
func (sm *ServiceManager) handleReady(w http.ResponseWriter, r *http.Request)
func (sm *ServiceManager) handleMetrics(w http.ResponseWriter, r *http.Request)
func (sm *ServiceManager) handleStats(w http.ResponseWriter, r *http.Request)
func (sm *ServiceManager) handleSignals()
func (sm *ServiceManager) gracefulShutdown()
```

### task_handler.go ⭐ 已重构
```go
// 构造函数
func NewTaskHandler(cfg TaskHandlerConfig) *TaskHandler

// 主要方法
func (eth *TaskHandler) HandleMessage(ctx context.Context, msg *rabbitmq.Message) error

// 私有方法（重构后拆分）
func (eth *TaskHandler) convertAndValidateMessage(msg *rabbitmq.Message) (*model.Task, map[string]any, error)
func (eth *TaskHandler) extractNestedPayload(domainMsg *task.Message) map[string]any
func (eth *TaskHandler) shouldSkipDuplicate(task *model.Task) bool
func (eth *TaskHandler) validateStoreAccess(task *model.Task) (bool, error)
func (eth *TaskHandler) validatePlatform(task *model.Task) error
func (eth *TaskHandler) getBasePlatform() string
func (eth *TaskHandler) processTaskWithReporting(...) error
func (eth *TaskHandler) shouldRetry(task *model.Task, err error) bool
func (eth *TaskHandler) isOwnedStore(storeID int64) bool
```

### task_submitter.go ⭐ 已重构
```go
func NewTaskSubmitter(client *rabbitmq.Client, logger *logrus.Logger) *TaskSubmitter
func (ts *TaskSubmitter) SubmitTask(ctx context.Context, t *model.Task) error
func (ts *TaskSubmitter) SubmitVariantTasks(...) (int, int)
func (ts *TaskSubmitter) cleanExpiredCache()
func (ts *TaskSubmitter) getCacheKey(tenantID int64, region, asin string) string
func (ts *TaskSubmitter) isRecentlySubmitted(tenantID int64, region, asin string) bool
func (ts *TaskSubmitter) markAsSubmitted(tenantID int64, region, asin string)
```

### result_reporter.go
```go
func NewResultReporter(cfg ReporterConfig, logger *logrus.Logger) *ResultReporter
func (rr *ResultReporter) Start(ctx context.Context) error
func (rr *ResultReporter) Stop(ctx context.Context) error
func (rr *ResultReporter) ReportSuccess(task *model.Task, data map[string]any, processTime time.Duration) error
func (rr *ResultReporter) ReportFailure(task *model.Task, err error, processTime time.Duration) error
func (rr *ResultReporter) ReportRetry(task *model.Task, err error, processTime time.Duration) error
func (rr *ResultReporter) GetStats() ReporterStats
func (rr *ResultReporter) GetNodeID() string
func (rr *ResultReporter) reportAsync(result *TaskResult) error
func (rr *ResultReporter) reportWorker()
func (rr *ResultReporter) processResult(result *TaskResult)
func (rr *ResultReporter) reportWithRetry(result *TaskResult) error
func (rr *ResultReporter) doReport(result *TaskResult) error
func (rr *ResultReporter) updateStats(statType string)
```

### platform_registry.go
```go
func NewPlatformRegistry(...) *PlatformRegistry
func (r *PlatformRegistry) RegisterAllProcessors(ctx context.Context, serviceManager *ServiceManager) error
func (r *PlatformRegistry) initializeSharedResources() error
func (r *PlatformRegistry) needsAmazonProcessor() bool
func (r *PlatformRegistry) registerAmazonPlatform(ctx context.Context, serviceManager *ServiceManager) error
```

### crawler_registry.go
```go
func NewCrawlerRegistry(...) *CrawlerRegistry
func (r *CrawlerRegistry) RegisterCrawlerProcessor(serviceManager *ServiceManager, sharedAmazonProcessor *amazon.AmazonProcessor) error
func (r *CrawlerRegistry) createProductFetcher(amazonProcessor *amazon.AmazonProcessor) *product.ProductFetcher
```

### queue_config.go
```go
func GetExchangeConfigs() []ExchangeConfig
func GetQueueConfigs() []QueueConfig
func getTaskQueues() []QueueConfig
func getCrawlerQueues() []QueueConfig
func getSystemQueues() []QueueConfig
```

### queue_initializer.go
```go
func NewQueueInitializer(client *rabbitmq.Client, logger *logrus.Logger) *QueueInitializer
func (qi *QueueInitializer) InitializeAll() error
func (qi *QueueInitializer) initializeExchanges() error
func (qi *QueueInitializer) initializeQueues() error
func (qi *QueueInitializer) declareQueueWithRetry(queue QueueConfig) error
```

### rabbitmq_publisher_adapter.go
```go
func NewRabbitMQPublisherAdapter(client *rabbitmq.Client, logger *logrus.Logger) *RabbitMQPublisherAdapter
func (a *RabbitMQPublisherAdapter) Publish(ctx context.Context, queueName string, data []byte) error
```

## app/bootstrap 模块

### app.go
```go
func NewApplicationBootstrap(logger *logrus.Logger) *ApplicationBootstrap
func (a *ApplicationBootstrap) Initialize(configPath, appVersion string) error
func (a *ApplicationBootstrap) Start(ctx context.Context, appVersion string) error
func (a *ApplicationBootstrap) Stop(ctx context.Context) error
func (a *ApplicationBootstrap) GetContainer() di.Container
```

### component_adapters.go
```go
func NewComponentAdapters(container di.Container, logger *logrus.Logger) *ComponentAdapters
func (c *ComponentAdapters) RegisterAllComponents(lifecycleManager lifecycle.LifecycleManager, cfg *config.Config, appVersion string) error
func (c *ComponentAdapters) registerUpdaterComponent(lifecycleManager lifecycle.LifecycleManager, cfg *config.Config, appVersion string) error
func (c *ComponentAdapters) registerProcessorComponents(lifecycleManager lifecycle.LifecycleManager, cfg *config.Config) error
func (c *ComponentAdapters) registerTaskFetcherComponent(lifecycleManager lifecycle.LifecycleManager, cfg *config.Config) error
```

### platform_processors.go
```go
func NewPlatformProcessorRegistry(logger *logrus.Logger) *PlatformProcessorRegistry
func (p *PlatformProcessorRegistry) RegisterPlatformProcessors(container di.Container) error
func (p *PlatformProcessorRegistry) getDependencies(c di.Container) (...)
func (p *PlatformProcessorRegistry) registerTemuProcessor(container di.Container) error
func (p *PlatformProcessorRegistry) registerSheinProcessor(container di.Container) error
```

### service_registry_simple.go
```go
func NewServiceRegistrySimple(logger *logrus.Logger) *ServiceRegistrySimple
func (s *ServiceRegistrySimple) RegisterAllServices(container di.Container, cfg *config.Config) error
func (s *ServiceRegistrySimple) registerBaseServices(container di.Container) error
func (s *ServiceRegistrySimple) registerAuthServices(container di.Container) error
func (s *ServiceRegistrySimple) registerSharedResources(container di.Container) error
```
