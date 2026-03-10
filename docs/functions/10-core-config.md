# 函数清单 - Core/Config 模块

生成时间: 2026-03-10

## core/config 模块

### config.go
```go
func LoadConfig() *Config
func LoadConfigFromFile(configFile string) *Config
func (c *Config) Validate() error
func (c *Config) SetDefaults()
func (c *Config) GetBrowserConfig() *BrowserConfig
func (c *Config) GetAmazonConfig() *AmazonConfig
func (c *Config) GetRabbitMQConfig() *RabbitMQConfig
```

### builder.go
```go
func buildConfig() *Config
func NewConfigBuilder() *ConfigBuilder
func (cb *ConfigBuilder) WithBrowser(browser *BrowserConfig) *ConfigBuilder
func (cb *ConfigBuilder) WithAmazon(amazon *AmazonConfig) *ConfigBuilder
func (cb *ConfigBuilder) WithRabbitMQ(rabbitmq *RabbitMQConfig) *ConfigBuilder
func (cb *ConfigBuilder) Build() *Config
```

### common_types.go
```go
func DefaultRetryConfig() *RetryConfig
func (rc *RetryConfig) Validate() error
func DefaultTimeoutConfig() *TimeoutConfig
func (tc *TimeoutConfig) Validate() error
func DefaultHTTPClientConfig() *HTTPClientConfig
func (hc *HTTPClientConfig) Validate() error
func DefaultCacheConfig() *CacheConfig
func (cc *CacheConfig) Validate() error
func DefaultRateLimitConfig() *RateLimitConfig
func (rlc *RateLimitConfig) Validate() error
```

### defaults.go
```go
func setDefaults()
func setBrowserDefaults()
func setAmazonDefaults()
func setUpdaterDefaults()
func setPlatformDefaults()
func setRabbitMQDefaults()
```

### defaults_applier.go
```go
func NewDefaultsApplier() *DefaultsApplier
func (da *DefaultsApplier) Apply(cfg *Config)
func (da *DefaultsApplier) applyStructDefaults(target, source reflect.Value)
func (da *DefaultsApplier) applyFieldDefaults(targetField, sourceField reflect.Value)
```

### helpers.go
```go
func LoadJSONConfig(path string, config any) error
func LoadYAMLConfig(path string, config any) error
func SaveJSONConfig(path string, config any) error
func SaveYAMLConfig(path string, config any) error
func ResolveConfigPath(basePath, configPath string) string
func MergeConfigs(base, override *Config) *Config
```

### loader.go
```go
func LoadFromBytes(data []byte) (*Config, error)
func LoadConfigWithFallback(configPath string, logger *logrus.Logger) *Config
func NewDefaultConfig() *Config
func applyDefaults(cfg *Config)
func validateConfig(cfg *Config) error
```

### validator.go
```go
func NewConfigValidator() *ConfigValidator
func (cv *ConfigValidator) Validate(cfg *Config) error
func (cv *ConfigValidator) ValidateBrowser(browser *BrowserConfig) error
func (cv *ConfigValidator) ValidateAmazon(amazon *AmazonConfig) error
func (cv *ConfigValidator) ValidateRabbitMQ(rabbitmq *RabbitMQConfig) error
```

## core/logger 模块

### logger.go
```go
func NewLogger(level string) *logrus.Logger
func NewLoggerWithConfig(cfg *LoggerConfig) *logrus.Logger
func SetupLogger(cfg *LoggerConfig) *logrus.Logger
func (l *Logger) WithField(key string, value interface{}) *logrus.Entry
func (l *Logger) WithFields(fields logrus.Fields) *logrus.Entry
func (l *Logger) WithError(err error) *logrus.Entry
```

### formatter.go
```go
func NewJSONFormatter() *logrus.JSONFormatter
func NewTextFormatter() *logrus.TextFormatter
func NewCustomFormatter(cfg *FormatterConfig) logrus.Formatter
```

## core/errors 模块

### errors.go
```go
func NewError(code ErrorCode, message string) *Error
func NewErrorWithCause(code ErrorCode, message string, cause error) *Error
func (e *Error) Error() string
func (e *Error) Unwrap() error
func (e *Error) Is(target error) bool
func Wrap(err error, message string) error
func Unwrap(err error) error
func Is(err, target error) bool
```

## core/lifecycle 模块

### lifecycle.go
```go
func NewLifecycleManager(logger *logrus.Logger) *LifecycleManager
func (lm *LifecycleManager) Register(component Component) error
func (lm *LifecycleManager) Start(ctx context.Context) error
func (lm *LifecycleManager) Stop(ctx context.Context) error
func (lm *LifecycleManager) GetComponent(name string) (Component, error)
func (lm *LifecycleManager) GetAllComponents() []Component
```

### component.go
```go
func NewBaseComponent(name string) *BaseComponent
func (bc *BaseComponent) Name() string
func (bc *BaseComponent) Start(ctx context.Context) error
func (bc *BaseComponent) Stop(ctx context.Context) error
func (bc *BaseComponent) IsRunning() bool
func (bc *BaseComponent) GetStatus() ComponentStatus
```
