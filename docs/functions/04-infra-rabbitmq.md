# 函数清单 - Infra/RabbitMQ 模块

生成时间: 2026-03-10

## infra/rabbitmq 模块

### client.go
```go
func NewClient(connManager *ConnectionManager, logger *logrus.Logger) *Client
func (c *Client) DeclareQueue(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) error
func (c *Client) DeclareExchange(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
func (c *Client) DeleteQueue(name string, ifUnused, ifEmpty, noWait bool) error
func (c *Client) BindQueue(queueName, routingKey, exchangeName string, noWait bool, args amqp.Table) error
func (c *Client) Publish(ctx context.Context, exchange, routingKey string, msg *Message) error
func (c *Client) PublishWithRetry(ctx context.Context, exchange, routingKey string, msg *Message, maxRetries int) error
func (c *Client) GetChannel() *amqp.Channel
func (c *Client) Close() error
```

### connection.go
```go
func (cs ConnectionState) String() string
func NewConnectionManager(config ConnectionConfig, logger *logrus.Logger) *ConnectionManager
func (cm *ConnectionManager) Connect() error
func (cm *ConnectionManager) Close() error
func (cm *ConnectionManager) GetConnection() *amqp.Connection
func (cm *ConnectionManager) GetChannel() (*amqp.Channel, error)
func (cm *ConnectionManager) IsConnected() bool
func (cm *ConnectionManager) AddStateListener(listener ConnectionStateListener)
func (cm *ConnectionManager) notifyStateChange(oldState, newState ConnectionState)
func (cm *ConnectionManager) GetErrorCollector() *ErrorCollector
func (cm *ConnectionManager) reconnect() error
func (cm *ConnectionManager) handleConnectionClose()
```

### consumer.go
```go
func NewMessageConsumer(client *Client, config ConsumerConfig, logger *logrus.Logger) *MessageConsumer
func (mc *MessageConsumer) SetQueueConfigs(configs []QueueConfig)
func (mc *MessageConsumer) RegisterHandler(queueName string, handler MessageHandler)
func (mc *MessageConsumer) Start(ctx context.Context) error
func (mc *MessageConsumer) Stop(ctx context.Context) error
func (mc *MessageConsumer) Restart() error
func (mc *MessageConsumer) GetState() ConsumerState
func (mc *MessageConsumer) GetStats() ConsumerStats
func (mc *MessageConsumer) consumeQueue(ctx context.Context, queueConfig QueueConfig) error
func (mc *MessageConsumer) processMessage(ctx context.Context, delivery amqp.Delivery, handler MessageHandler, queueName string)
```

### consumer_state.go
```go
func (s ConsumerState) String() string
func NewConsumerStateManager() *ConsumerStateManager
func (csm *ConsumerStateManager) GetState() ConsumerState
func (csm *ConsumerStateManager) GetStateInfo() ConsumerStateInfo
func (csm *ConsumerStateManager) SetState(newState ConsumerState, queueName string)
func (csm *ConsumerStateManager) RecordSuccess(queueName string)
func (csm *ConsumerStateManager) RecordFailure(queueName string, err error)
func (csm *ConsumerStateManager) GetQueueStats(queueName string) QueueStats
```

### error_collector.go
```go
func (et ErrorType) String() string
func NewErrorCollector(maxSize int) *ErrorCollector
func (ec *ErrorCollector) Collect(errorType ErrorType, queueName, messageID string, err error, context string)
func (ec *ErrorCollector) GetErrors() []ErrorRecord
func (ec *ErrorCollector) GetRecentErrors(n int) []ErrorRecord
func (ec *ErrorCollector) GetErrorsByType(errorType ErrorType) []ErrorRecord
func (ec *ErrorCollector) GetErrorsByQueue(queueName string) []ErrorRecord
func (ec *ErrorCollector) Clear()
func (ec *ErrorCollector) GetStats() ErrorStats
```

### load_monitor.go
```go
func NewLoadMonitor(cfg config.LoadMonitorConfig, logger *logrus.Logger) *LoadMonitor
func (lm *LoadMonitor) Start(ctx context.Context) error
func (lm *LoadMonitor) Stop(ctx context.Context) error
func (lm *LoadMonitor) RecordTaskProcessed(queueName string, success bool, duration time.Duration)
func (lm *LoadMonitor) GetQueueStats(queueName string) QueueLoadStats
func (lm *LoadMonitor) GetAllStats() map[string]QueueLoadStats
func (lm *LoadMonitor) monitorLoop()
func (lm *LoadMonitor) updateStats()
func (lm *LoadMonitor) calculateMetrics(stats *QueueLoadStats)
```

### publisher.go
```go
func NewPublisher(client *Client, logger *logrus.Logger) *Publisher
func (p *Publisher) Publish(ctx context.Context, exchange, routingKey string, msg *Message) error
func (p *Publisher) PublishBatch(ctx context.Context, exchange, routingKey string, messages []*Message) error
func (p *Publisher) PublishWithConfirm(ctx context.Context, exchange, routingKey string, msg *Message) error
func (p *Publisher) Close() error
```

### config.go
```go
func (c *Config) Validate() error
func (c *ConnectionConfig) Validate() error
func (c *ConsumerConfig) Validate() error
func (c *QueueConfig) Validate() error
func (c *Config) SetDefaults()
func (c *ConnectionConfig) SetDefaults()
func (c *ConsumerConfig) SetDefaults()
```

### message.go
```go
func NewMessage(data []byte) *Message
func (m *Message) SetHeader(key string, value interface{})
func (m *Message) GetHeader(key string) (interface{}, bool)
func (m *Message) SetPriority(priority uint8)
func (m *Message) SetExpiration(expiration time.Duration)
func (m *Message) ToAMQPPublishing() amqp.Publishing
func ParseMessage(delivery amqp.Delivery) (*Message, error)
```
