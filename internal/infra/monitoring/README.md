# Monitoring 包

## 概述

`monitoring` 包是基础设施层的核心监控组件,提供通用的指标收集、健康检查和性能监控功能。作为通用的技术基础设施,它与具体业务逻辑解耦,可以被任何应用层组件复用。

## 位置

```
internal/infra/monitoring/  # 基础设施层
```

**为什么放在 infra 层?**
- 提供通用的监控能力,与业务无关
- 类似于 `rabbitmq`、`worker` 等基础设施组件
- 可以被多个应用层组件复用
- 是技术实现细节,不是业务逻辑

## 核心功能

- **指标收集**: 自动收集系统和应用指标
- **健康检查**: 定期检查组件健康状态
- **进程信息**: 跟踪进程启动时间和运行时长
- **生命周期管理**: 管理监控组件的启动和优雅关闭

## 核心组件

### 1. 指标收集器 (collector.go)

`MetricsCollector` 提供自动化的指标收集功能。

#### 功能特性

- 自动收集系统指标(内存、CPU、Goroutine等)
- 支持自定义应用指标
- 支持多种指标类型(Counter、Gauge、Histogram、Summary)
- 支持指标标签(Labels)
- 定期更新和输出指标

#### 使用示例

```go
// 创建指标收集器
collector := monitoring.NewMetricsCollector(logger, 30*time.Second)

// 启动收集器
if err := collector.Start(ctx); err != nil {
    log.Fatal(err)
}
defer collector.Stop(ctx)

// 设置自定义指标
collector.SetCounter("requests_total", 100, nil, "总请求数")
collector.SetGauge("queue_size", 50, map[string]string{"queue": "tasks"}, "队列大小")

// 增加计数器
collector.IncrementCounter("errors_total", map[string]string{"type": "timeout"}, "错误总数")

// 获取所有指标
metrics := collector.GetMetrics()
for name, metric := range metrics {
    fmt.Printf("%s: %.2f\n", name, metric.Value)
}
```

#### 自动收集的系统指标

| 指标名称 | 类型 | 描述 |
|---------|------|------|
| `system_memory_heap_bytes` | Gauge | 堆内存使用量 |
| `system_memory_sys_bytes` | Gauge | 系统内存使用量 |
| `system_gc_runs_total` | Gauge | GC运行次数 |
| `system_gc_pause_ns` | Gauge | 上次GC暂停时间 |
| `system_goroutines_count` | Gauge | Goroutine数量 |
| `system_cpu_cores` | Gauge | CPU核心数 |
| `system_process_id` | Gauge | 进程ID |
| `system_process_start_time` | Gauge | 进程启动时间 |
| `system_process_uptime_seconds` | Gauge | 进程运行时间 |

### 2. 健康检查器 (health_checker.go)

`HealthChecker` 提供组件健康状态检查功能。

#### 功能特性

- 支持注册多个健康检查
- 定期自动执行健康检查
- 记录健康状态和错误信息
- 支持超时控制

#### 使用示例

```go
// 创建健康检查器
checker := monitoring.NewHealthChecker(logger, 60*time.Second)

// 启动检查器
if err := checker.Start(ctx); err != nil {
    log.Fatal(err)
}
defer checker.Stop(ctx)

// 实现自定义健康检查
type DatabaseHealthCheck struct{}

func (d *DatabaseHealthCheck) Name() string {
    return "database"
}

func (d *DatabaseHealthCheck) Check(ctx context.Context) error {
    // 检查数据库连接
    return db.Ping(ctx)
}

// 注册健康检查
checker.RegisterCheck(&DatabaseHealthCheck{})
```

#### 健康检查接口

```go
type HealthCheck interface {
    Name() string
    Check(ctx context.Context) error
}
```

实现此接口即可创建自定义健康检查。

### 3. 进程信息 (process_info.go)

提供进程启动时间和运行时长的跟踪。

#### 使用示例

```go
// 在main函数开始时记录启动时间
func main() {
    monitoring.RecordProcessStartTime()
    
    // 获取进程启动时间
    startTime := monitoring.GetProcessStartTime()
    fmt.Printf("进程启动时间: %d\n", startTime)
    
    // 获取进程运行时长
    uptime := monitoring.GetProcessUptime()
    fmt.Printf("进程运行时长: %d秒\n", uptime)
}
```

### 4. 类型定义 (types.go)

定义了监控相关的数据结构。

#### Metric 指标结构

```go
type Metric struct {
    Name        string            // 指标名称
    Type        MetricType        // 指标类型
    Value       float64           // 指标值
    Labels      map[string]string // 标签
    Timestamp   time.Time         // 时间戳
    Description string            // 描述
}
```

#### MetricType 指标类型

- `MetricTypeCounter`: 计数器,只增不减
- `MetricTypeGauge`: 仪表盘,可增可减
- `MetricTypeHistogram`: 直方图
- `MetricTypeSummary`: 摘要

#### HealthStatus 健康状态

```go
type HealthStatus struct {
    Name      string    // 检查名称
    Status    string    // 状态: healthy/unhealthy
    Error     string    // 错误信息
    Timestamp time.Time // 时间戳
}
```

### 5. 指标操作 (metric_operations.go)

提供便捷的指标操作方法。

#### 方法列表

```go
// 设置计数器
SetCounter(name string, value float64, labels map[string]string, description string)

// 设置仪表盘
SetGauge(name string, value float64, labels map[string]string, description string)

// 增加计数器
IncrementCounter(name string, labels map[string]string, description string)
```

## 架构设计

```
┌─────────────────────────────────────────────────────────┐
│                 Monitoring Package                       │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────────┐         ┌──────────────────┐     │
│  │ MetricsCollector │         │  HealthChecker   │     │
│  │                  │         │                  │     │
│  │ - System Metrics │         │ - Health Checks  │     │
│  │ - App Metrics    │         │ - Status Monitor │     │
│  │ - Auto Collect   │         │ - Auto Check     │     │
│  └──────────────────┘         └──────────────────┘     │
│         │                              │                │
│         │                              │                │
│         ▼                              ▼                │
│  ┌──────────────────┐         ┌──────────────────┐     │
│  │  Process Info    │         │      Types       │     │
│  │                  │         │                  │     │
│  │ - Start Time     │         │ - Metric         │     │
│  │ - Uptime         │         │ - HealthStatus   │     │
│  └──────────────────┘         └──────────────────┘     │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

## 与其他包的关系

### 依赖关系
- `internal/core/lifecycle` - 生命周期管理
- `internal/core/errors` - 错误处理
- `github.com/sirupsen/logrus` - 日志记录

### 被依赖关系
- `internal/infra/rabbitmq` - RabbitMQ负载监控
- `internal/infra/worker` - Worker池指标收集
- `internal/crawler/amazon/browser` - 浏览器池健康检查
- 其他需要监控的组件

## 集成示例

### 在 RabbitMQ 中使用

```go
// rabbitmq/load_monitor.go
type LoadMonitor struct {
    metricsCollector *monitoring.MetricsCollector
    // ...
}

func NewLoadMonitor(cfg config.LoadMonitorConfig, logger *logrus.Logger) *LoadMonitor {
    collector := monitoring.NewMetricsCollector(logger, cfg.UpdateInterval)
    
    return &LoadMonitor{
        metricsCollector: collector,
        // ...
    }
}

func (lm *LoadMonitor) updateStats() {
    // 更新RabbitMQ特定指标
    lm.metricsCollector.SetCounter("rabbitmq_tasks_processed_total", 
        float64(lm.stats.TasksProcessed), nil, "处理的任务总数")
}
```

### 在 Worker 中使用

```go
// worker/pool.go
type Pool struct {
    metrics *monitoring.MetricsCollector
    // ...
}

func (p *Pool) recordMetrics() {
    p.metrics.SetGauge("worker_pool_queue_size", 
        float64(len(p.jobQueue)), nil, "队列大小")
    p.metrics.SetGauge("worker_pool_active_workers", 
        float64(p.activeWorkers), nil, "活跃Worker数")
}
```

### 实现自定义健康检查

```go
// 数据库健康检查
type DBHealthCheck struct {
    db *sql.DB
}

func (d *DBHealthCheck) Name() string {
    return "database"
}

func (d *DBHealthCheck) Check(ctx context.Context) error {
    return d.db.PingContext(ctx)
}

// Redis健康检查
type RedisHealthCheck struct {
    client *redis.Client
}

func (r *RedisHealthCheck) Name() string {
    return "redis"
}

func (r *RedisHealthCheck) Check(ctx context.Context) error {
    return r.client.Ping(ctx).Err()
}

// 注册健康检查
checker.RegisterCheck(&DBHealthCheck{db: db})
checker.RegisterCheck(&RedisHealthCheck{client: redisClient})
```

## 最佳实践

### 1. 合理设置收集间隔

```go
// 开发环境: 更频繁的收集
collector := monitoring.NewMetricsCollector(logger, 10*time.Second)

// 生产环境: 适中的收集间隔
collector := monitoring.NewMetricsCollector(logger, 30*time.Second)

// 低频监控: 较长的收集间隔
collector := monitoring.NewMetricsCollector(logger, 5*time.Minute)
```

### 2. 使用标签区分指标

```go
// 为不同队列设置标签
collector.SetGauge("queue_size", 50, 
    map[string]string{"queue": "tasks", "priority": "high"}, 
    "高优先级任务队列大小")

collector.SetGauge("queue_size", 30, 
    map[string]string{"queue": "tasks", "priority": "low"}, 
    "低优先级任务队列大小")
```

### 3. 及时清理资源

```go
// 使用defer确保资源清理
collector := monitoring.NewMetricsCollector(logger, 30*time.Second)
if err := collector.Start(ctx); err != nil {
    return err
}
defer func() {
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    collector.Stop(shutdownCtx)
}()
```

### 4. 健康检查超时控制

健康检查默认超时为10秒,确保检查逻辑不会阻塞过久:

```go
func (d *DBHealthCheck) Check(ctx context.Context) error {
    // 使用传入的context,它已经设置了超时
    return d.db.PingContext(ctx)
}
```

### 5. 指标命名规范

遵循 Prometheus 命名规范:

```go
// 好的命名
collector.SetCounter("http_requests_total", ...)
collector.SetGauge("queue_size_bytes", ...)
collector.SetGauge("worker_pool_active_count", ...)

// 不好的命名
collector.SetCounter("RequestCount", ...)  // 使用驼峰
collector.SetGauge("size", ...)            // 太模糊
```

## 注意事项

1. **性能影响**: 指标收集会有一定性能开销,合理设置收集间隔
2. **内存占用**: 指标数据会占用内存,避免创建过多指标
3. **并发安全**: 所有方法都是并发安全的,可以在多个goroutine中使用
4. **生命周期**: 确保在应用关闭时正确停止监控组件

## 未来改进

- [ ] 支持导出到 Prometheus
- [ ] 支持导出到 Grafana
- [ ] 支持更多指标类型(Histogram、Summary)
- [ ] 支持指标聚合和计算
- [ ] 支持告警功能
- [ ] 支持分布式追踪集成

## 相关文档

- [Worker 包文档](../worker/README.md)
- [RabbitMQ 包文档](../rabbitmq/README.md)
- [Lifecycle 包文档](../../core/lifecycle/README.md)
