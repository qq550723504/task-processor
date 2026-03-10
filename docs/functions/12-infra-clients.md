# 函数清单 - Infra/Clients 模块

生成时间: 2026-03-10

## infra/clients/openai 模块

### client.go
```go
func NewClient(config *ClientConfig) *Client
func (c *Client) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
func (c *Client) CreateChatCompletionWithTimeout(ctx context.Context, req *ChatCompletionRequest, timeout time.Duration) (*ChatCompletionResponse, error)
func (c *Client) GetDefaultModel() string
func (c *Client) GetStats() map[string]any
func (c *Client) Close() error
```

### context_manager.go
```go
func NewContextManager(config *ContextConfig) *ContextManager
func (cm *ContextManager) CreateContext(parent context.Context, taskType string, timeout time.Duration) (context.Context, string, error)
func (cm *ContextManager) ReleaseContext(contextID string)
func (cm *ContextManager) monitorContext(contextID string)
func (cm *ContextManager) GetActiveContexts() []ContextInfo
func (cm *ContextManager) GetContextInfo(contextID string) (*ContextInfo, error)
func (cm *ContextManager) CancelContext(contextID string) error
```

### pool.go
```go
func NewRequestPool(config *PoolConfig) (*RequestPool, error)
func newBaseClient(config *ClientConfig) *BaseClient
func (p *RequestPool) CreateChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
func (bc *BaseClient) createChatCompletion(ctx context.Context, req *ChatCompletionRequest) (*ChatCompletionResponse, error)
func (p *RequestPool) waitForRateLimit(ctx context.Context) error
func (p *RequestPool) GetPoolStats() PoolStats
func (p *RequestPool) Close() error
```

### rate_limiter.go
```go
func NewRateLimiter(requestsPerMinute int) *RateLimiter
func (rl *RateLimiter) Wait(ctx context.Context) error
func (rl *RateLimiter) TryAcquire() bool
func (rl *RateLimiter) GetStats() RateLimiterStats
func (rl *RateLimiter) UpdateRate(requestsPerMinute int)
```

### retry.go
```go
func NewRetryPolicy(maxRetries int, initialDelay time.Duration) *RetryPolicy
func (rp *RetryPolicy) Execute(ctx context.Context, fn func() error) error
func (rp *RetryPolicy) ShouldRetry(err error, attempt int) bool
func (rp *RetryPolicy) GetBackoffDuration(attempt int) time.Duration
```

## infra/http 模块

### client.go
```go
func NewHTTPClient(config *HTTPClientConfig) *HTTPClient
func (c *HTTPClient) Get(ctx context.Context, url string) (*http.Response, error)
func (c *HTTPClient) Post(ctx context.Context, url string, body interface{}) (*http.Response, error)
func (c *HTTPClient) Put(ctx context.Context, url string, body interface{}) (*http.Response, error)
func (c *HTTPClient) Delete(ctx context.Context, url string) (*http.Response, error)
func (c *HTTPClient) Do(req *http.Request) (*http.Response, error)
func (c *HTTPClient) Close() error
```

### middleware.go
```go
func NewLoggingMiddleware(logger *logrus.Logger) Middleware
func NewRetryMiddleware(maxRetries int) Middleware
func NewTimeoutMiddleware(timeout time.Duration) Middleware
func NewAuthMiddleware(token string) Middleware
func ChainMiddleware(middlewares ...Middleware) Middleware
```

## infra/lock 模块

### distributed_lock.go
```go
func NewDistributedLock(client *redis.Client, key string, ttl time.Duration) *DistributedLock
func (dl *DistributedLock) Lock(ctx context.Context) error
func (dl *DistributedLock) Unlock(ctx context.Context) error
func (dl *DistributedLock) TryLock(ctx context.Context) (bool, error)
func (dl *DistributedLock) Extend(ctx context.Context, duration time.Duration) error
func (dl *DistributedLock) IsLocked() bool
```

## infra/monitoring 模块

### metrics.go
```go
func NewMetricsCollector() *MetricsCollector
func (mc *MetricsCollector) RecordRequest(method, path string, duration time.Duration, statusCode int)
func (mc *MetricsCollector) RecordError(errorType string)
func (mc *MetricsCollector) IncrementCounter(name string, value int64)
func (mc *MetricsCollector) RecordGauge(name string, value float64)
func (mc *MetricsCollector) RecordHistogram(name string, value float64)
func (mc *MetricsCollector) GetMetrics() map[string]interface{}
```

### health.go
```go
func NewHealthChecker() *HealthChecker
func (hc *HealthChecker) RegisterCheck(name string, check HealthCheck)
func (hc *HealthChecker) Check(ctx context.Context) HealthStatus
func (hc *HealthChecker) CheckComponent(ctx context.Context, name string) ComponentHealth
func (hc *HealthChecker) GetStatus() HealthStatus
```

## infra/repo 模块

### base_repository.go
```go
func NewBaseRepository(db *sql.DB, logger *logrus.Logger) *BaseRepository
func (br *BaseRepository) GetDB() *sql.DB
func (br *BaseRepository) BeginTx(ctx context.Context) (*sql.Tx, error)
func (br *BaseRepository) CommitTx(tx *sql.Tx) error
func (br *BaseRepository) RollbackTx(tx *sql.Tx) error
func (br *BaseRepository) QueryRow(ctx context.Context, query string, args ...interface{}) *sql.Row
func (br *BaseRepository) Query(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
func (br *BaseRepository) Exec(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
```

### task_repository.go
```go
func NewTaskRepository(db *sql.DB, logger *logrus.Logger) *TaskRepository
func (tr *TaskRepository) Create(ctx context.Context, task *model.Task) error
func (tr *TaskRepository) Update(ctx context.Context, task *model.Task) error
func (tr *TaskRepository) Delete(ctx context.Context, id int64) error
func (tr *TaskRepository) GetByID(ctx context.Context, id int64) (*model.Task, error)
func (tr *TaskRepository) List(ctx context.Context, filter *TaskFilter) ([]*model.Task, error)
func (tr *TaskRepository) UpdateStatus(ctx context.Context, id int64, status model.TaskStatus) error
```

## infra/di 模块

### container.go
```go
func NewContainer() *Container
func (c *Container) Register(name string, factory interface{}) error
func (c *Container) RegisterSingleton(name string, factory interface{}) error
func (c *Container) Resolve(name string) (interface{}, error)
func (c *Container) Has(name string) bool
func (c *Container) Remove(name string)
func (c *Container) Clear()
```

### injector.go
```go
func NewInjector(container *Container) *Injector
func (i *Injector) Inject(target interface{}) error
func (i *Injector) InjectField(target interface{}, fieldName string) error
func (i *Injector) InjectMethod(target interface{}, methodName string) error
```
