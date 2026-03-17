# 问题三：app/messaging 大包问题

**严重程度**：中

## 问题描述

`internal/app/messaging/` 包含 12 个文件，一个包同时承担了六种不同的职责：

1. **服务生命周期编排**（`service_manager.go`）
2. **RabbitMQ 连接与消费**（`rabbitmq_service.go`）
3. **任务处理与分发**（`task_handler.go`，含 `TaskProcessorRegistry`）
4. **HTTP 状态服务器**（`http_servers.go`）
5. **关闭协调**（`shutdown_coordinator.go`）
6. **结果上报**（`result_reporter.go`）
7. **平台/爬虫处理器注册**（`platform_registry.go`、`crawler_registry.go`）
8. **队列声明与绑定**（`queue_config.go`、`queue_initializer.go`）
9. **任务发布**（`task_submitter.go`、`rabbitmq_publisher_adapter.go`）

这是典型的"上帝包"（God Package）反模式，违反了 Go 编码规范中"按功能分组"和"一个文件只做一类事"的原则。


## 代码证据

**`service_manager.go`** — 持有所有子服务的具体类型，而非接口：

```go
type ServiceManager struct {
    config           *config.RabbitMQConfig
    rabbitmqService  *RabbitMQService        // 具体类型
    resultReporter   *ResultReporter         // 具体类型
    loadMonitor      *rabbitmq.LoadMonitor   // 具体类型
    httpServerManger *HTTPServerManager      // 具体类型（注：字段名拼写错误）
    shutdownCoord    *ShutdownCoordinator    // 具体类型
}
```

违反了 Go 规范中"消费者定义接口"的原则——`ServiceManager` 作为编排者，应依赖接口而非具体实现。

**`initializeServices` 方法** — 在一个方法中硬编码所有服务的构造与装配：

```go
func (sm *ServiceManager) initializeServices() {
    sm.resultReporter = NewResultReporter(reporterConfig, sm.logger)
    sm.loadMonitor = rabbitmq.NewLoadMonitor(monitorConfig, sm.logger)
    sm.httpServerManger = NewHTTPServerManager(...)
    sm.shutdownCoord = NewShutdownCoordinator(
        sm.config,
        sm.rabbitmqService,
        sm.httpServerManger,  // 4 个具体类型参数
        sm.resultReporter,
        sm.loadMonitor,
        sm.logger,
    )
}
```

`ShutdownCoordinator` 构造函数接收 4 个具体类型参数，说明关闭逻辑与所有子服务强耦合，新增服务必须同时修改 `initializeServices`、`Start`、`Stop` 三处。

**`shutdown_coordinator.go`** — 直接持有所有子服务的具体类型：

```go
type ShutdownCoordinator struct {
    rabbitmqService  *RabbitMQService
    httpServerManger *HTTPServerManager
    resultReporter   *ResultReporter
    loadMonitor      *rabbitmq.LoadMonitor
}
```

**`rabbitmq_service.go`** — 混合了基础设施职责与业务辅助依赖：

```go
type RabbitMQService struct {
    // 基础设施
    connManager *rabbitmq.ConnectionManager
    client      *rabbitmq.Client
    consumer    *rabbitmq.MessageConsumer
    initializer *QueueInitializer
    // 业务辅助（不属于消息基础设施）
    resultReporter *ResultReporter
    storeAPI       api.StoreAPI
    deduplicator   *task.Deduplicator
}
```

**`task_handler.go`** — 同一文件中定义了两个独立职责的类型：`TaskHandler`（消息处理）和 `TaskProcessorRegistry`（处理器注册表），违反"一个文件只做一类事"的原则。

**`platform_registry.go`** — 包含大量 emoji 注释（`📦`、`✅`、`⚠️`），违反 Go 注释规范；`initializeSharedResources` 与 `initializeManagementClient` 存在明显的代码重复。


## 命名规范违规

当前代码存在多处违反 Go 命名规范的问题：

- `httpServerManger` — 字段名拼写错误（`Manger` 应为 `Manager`），且缩写词应全大写：`httpServerManager` → `httpSrvMgr` 或直接 `httpServer`
- `HTTPServerManager` — 符合规范，但对应字段名 `httpServerManger` 不一致
- `platform_registry.go` 中大量 emoji 注释不符合 godoc 规范，注释应解释"为什么"而非用图标装饰

## 影响分析

1. **修改风险高**：任何子服务的签名变化都会波及 `ServiceManager`、`ShutdownCoordinator`，牵一发动全身。
2. **无法独立测试**：`RabbitMQService`、`HTTPServerManager` 等无法脱离 `ServiceManager` 单独测试，因为它们依赖具体类型而非接口。
3. **违反开闭原则**：新增服务类型（如 gRPC 服务器）必须同时修改 `initializeServices`、`Start`、`Stop` 三个方法。
4. **包名语义模糊**：`messaging` 暗示只处理消息，但实际上还包含 HTTP 服务器、平台注册、任务发布、关闭协调，名不副实。
5. **代码量超标**：`task_handler.go` 约 350 行但承担两个独立职责；`platform_registry.go` 约 280 行且存在重复逻辑。


## 重构建议

### 核心思路：复用已有的 lifecycle 机制

项目中 `internal/core/lifecycle` 已经定义了完整的组件生命周期接口：

```go
// internal/core/lifecycle/interfaces.go（已有）
type Component interface {
    Name()         string
    Dependencies() []string
    Priority()     int
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    IsRunning()    bool
}
```

`messaging` 包应直接复用这套机制，而不是在 `ServiceManager` 中自己维护一套硬编码的启动/停止顺序。

### 第一步：让 ShutdownCoordinator 依赖接口而非具体类型

`ShutdownCoordinator` 目前持有 4 个具体类型，应改为依赖 `lifecycle.Component` 接口：

```go
// 重构后：ShutdownCoordinator 只依赖接口
type ShutdownCoordinator struct {
    components []lifecycle.Component  // 统一接口，顺序即关闭顺序
    timeout    time.Duration
    logger     *logrus.Logger
}

func NewShutdownCoordinator(
    components []lifecycle.Component,
    timeout time.Duration,
    logger *logrus.Logger,
) *ShutdownCoordinator
```

### 第二步：ServiceManager 改为基于接口编排

```go
// 重构后：ServiceManager 只持有接口切片
type ServiceManager struct {
    config    *config.RabbitMQConfig
    logger    *logrus.Logger
    lifecycle lifecycle.LifecycleManager  // 复用已有的 LifecycleManager
}
```

启动顺序通过各组件的 `Priority()` 返回值声明，而非硬编码在 `Start` 方法中。新增服务只需实现 `lifecycle.Component` 接口并注册，无需修改 `ServiceManager`。

### 第三步：按职责拆分包结构

```
internal/app/messaging/
    service_manager.go      ← 仅保留组件注册与 lifecycle.LifecycleManager 编排
    shutdown_coordinator.go ← 信号监听 + 调用 LifecycleManager.StopAll

internal/infra/rabbitmq/
    （已有）                 ← RabbitMQService 的连接/消费逻辑应合并至此

internal/infra/reporter/
    result_reporter.go      ← 结果上报与 RabbitMQ 无关，不应在 messaging 包

internal/app/registry/
    platform_registry.go    ← 平台处理器注册（业务编排，非消息基础设施）
    crawler_registry.go     ← 爬虫处理器注册
```

### 第四步：修复命名规范问题

- 将字段 `httpServerManger` 统一修正为 `httpServerManager`（拼写错误）
- 清理 `platform_registry.go` 和 `crawler_registry.go` 中的 emoji 注释，改为符合 godoc 规范的文字注释
- `TaskHandler` 与 `TaskProcessorRegistry` 拆分到独立文件：`task_handler.go` 和 `processor_registry.go`
- `initializeSharedResources` 与 `initializeManagementClient` 的重复逻辑提取为一个私有方法

### 注意事项

- `internal/app/bootstrap` 中可能已有类似的组件注册逻辑，重构前应先确认，避免引入新的重复。
- `RabbitMQService.SetComponents` 是一个可变状态注入方法，重构时应改为构造函数注入，消除两阶段初始化。
- `TaskSubmitter` 中的 `publishChannel` 延迟创建逻辑存在并发风险（`channelMutex` 仅保护通道创建，但 `cleanExpiredCache` goroutine 没有退出机制），重构时一并处理。
