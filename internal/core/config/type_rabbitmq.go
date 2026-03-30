package config

import "time"

const (
	NodeRoleTask    = "task"
	NodeRoleCrawler = "crawler"
	NodeRoleHybrid  = "hybrid"
)

// RabbitMQConfig RabbitMQ完整配置
type RabbitMQConfig struct {
	Enabled           bool                   `yaml:"enabled"`           // 是否启用RabbitMQ分布式爬虫
	URL               string                 `yaml:"url"`               // RabbitMQ连接URL
	ReconnectInterval time.Duration          `yaml:"reconnectInterval"` // 重连间隔
	MaxReconnectTries int                    `yaml:"maxReconnectTries"` // 最大重连次数
	ConfigPath        string                 `yaml:"configPath"`        // RabbitMQ详细配置文件路径（统一管理）
	Consumer          RabbitMQConsumerConfig `yaml:"consumer"`          // 消费者配置
	ResultReporter    ResultReporterConfig   `yaml:"resultReporter"`    // 结果上报器配置
	LoadMonitor       LoadMonitorConfig      `yaml:"loadMonitor"`       // 负载监控配置
	Node              NodeConfig             `yaml:"node"`              // 节点配置
	Deduplicator      DeduplicatorConfig     `yaml:"deduplicator"`      // 去重器配置
	StoreAPI          StoreAPIConfig         `yaml:"storeAPI"`          // 店铺API配置
}

// RabbitMQConsumerConfig 消费者配置
type RabbitMQConsumerConfig struct {
	PrefetchCount int           `yaml:"prefetchCount"` // 预取数量
	PrefetchSize  int           `yaml:"prefetchSize"`  // 预取大小
	RetryDelay    time.Duration `yaml:"retryDelay"`    // 重试延迟
	MaxRetries    int           `yaml:"maxRetries"`    // 最大重试次数
	Queues        []QueueConfig `yaml:"queues"`        // 队列配置列表
}

// QueueConfig 队列配置
type QueueConfig struct {
	Name     string `yaml:"name"`     // 队列名称
	Priority int    `yaml:"priority"` // 队列优先级（10=高，5=中，1=低）
	Prefetch int    `yaml:"prefetch"` // 预取数量
}

// ResultReporterConfig 结果上报器配置
type ResultReporterConfig struct {
	ReportURL  string        `yaml:"reportURL"`  // 上报URL
	NodeID     string        `yaml:"nodeID"`     // 节点ID
	Timeout    time.Duration `yaml:"timeout"`    // 超时时间
	BufferSize int           `yaml:"bufferSize"` // 缓冲区大小
	Retry      *RetryConfig  `yaml:"retry"`      // 重试配置
}

// LoadMonitorConfig 负载监控配置
type LoadMonitorConfig struct {
	UpdateInterval time.Duration `yaml:"updateInterval"` // 更新间隔
	EnableCPU      bool          `yaml:"enableCPU"`      // 启用CPU监控
	EnableMemory   bool          `yaml:"enableMemory"`   // 启用内存监控
	EnableTasks    bool          `yaml:"enableTasks"`    // 启用任务监控
}

// NodeConfig 节点配置
type NodeConfig struct {
	NodeID          string        `yaml:"nodeID"`          // 节点ID
	Role            string        `yaml:"role"`            // 节点角色：task | crawler | hybrid
	MaxConcurrency  int           `yaml:"maxConcurrency"`  // 最大并发数
	HealthCheckPort int           `yaml:"healthCheckPort"` // 健康检查端口
	MetricsPort     int           `yaml:"metricsPort"`     // 指标端口
	LogLevel        string        `yaml:"logLevel"`        // 日志级别
	ShutdownTimeout time.Duration `yaml:"shutdownTimeout"` // 关闭超时
	OwnedStores     []int64       `yaml:"ownedStores"`     // 本节点负责的店铺ID列表，为空则处理所有店铺
	Regions         []string      `yaml:"regions"`         // 本节点负责的 region 列表（如 US、UK、JP），为空则处理所有 region
}

// NormalizedRole returns the effective node role, defaulting to hybrid for backward compatibility.
func (c NodeConfig) NormalizedRole() string {
	switch c.Role {
	case NodeRoleTask, NodeRoleCrawler, NodeRoleHybrid:
		return c.Role
	default:
		return NodeRoleHybrid
	}
}

// HasValidRole reports whether the configured role is explicitly supported.
func (c NodeConfig) HasValidRole() bool {
	switch c.Role {
	case "", NodeRoleTask, NodeRoleCrawler, NodeRoleHybrid:
		return true
	default:
		return false
	}
}

// HandlesTaskWork reports whether the node should register task processors and schedulers.
func (c NodeConfig) HandlesTaskWork() bool {
	role := c.NormalizedRole()
	return role == NodeRoleTask || role == NodeRoleHybrid
}

// HandlesCrawlerWork reports whether the node should register crawler processors.
func (c NodeConfig) HandlesCrawlerWork() bool {
	role := c.NormalizedRole()
	return role == NodeRoleCrawler || role == NodeRoleHybrid
}

// DeduplicatorConfig 去重器配置
type DeduplicatorConfig struct {
	TTL time.Duration `yaml:"ttl"` // 去重记录过期时间
}

// StoreAPIConfig 店铺API配置
type StoreAPIConfig struct {
	BaseURL  string        `yaml:"baseURL"`  // API基础URL
	CacheTTL time.Duration `yaml:"cacheTTL"` // 缓存过期时间
}

// RetryConfig 重试配置
type RetryConfig struct {
	MaxRetries    int           `yaml:"maxRetries"`
	InitialDelay  time.Duration `yaml:"initialDelay"`
	MaxDelay      time.Duration `yaml:"maxDelay"`
	BackoffFactor float64       `yaml:"backoffFactor"`
}

// DefaultRetryConfig 返回默认重试配置
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:    3,
		InitialDelay:  1 * time.Second,
		MaxDelay:      30 * time.Second,
		BackoffFactor: 2.0,
	}
}
