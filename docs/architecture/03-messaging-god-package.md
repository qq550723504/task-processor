# 问题三：app/messaging 大包问题

**严重程度**：中

## 问题描述

`internal/app/messaging/` 包含 13 个文件，一个包同时承担了五种不同的职责：

1. **服务生命周期管理**（`service_manager.go`）
2. **RabbitMQ 连接与消费**（`rabbitmq_service.go`）
3. **任务处理与分发**（`task_handler.go`）
4. **HTTP 状态服务器**（`http_server_manager.go`）
5. **关闭协调**（`shutdown_coordinator.go`）
6. **结果上报**（`result_reporter.go`）

这是典型的"上帝包"（God Package）反模式。

## 代码证据

**`internal/app/messaging/service_manager.go`** — 服务管理器持有所有子服务：

```go
type ServiceManager struct {
    config           *config.RabbitMQConfig
    rabbitmqService  *RabbitMQService        // RabbitMQ 消费
    resultReporter   *ResultReporter         // 结果上报
    loadMonitor      *rabbitmq.LoadMonitor   // 负载监控
    httpServerManger *HTTPServerManager      // HTTP 服务器
    shutdownCoord    *ShutdownCoordinator    // 关闭协调
    ...
}
```

`ServiceManager` 直接持有所有子服务的具体类型，而不是接口，导致整个包内部高度耦合。

**`initializeServices` 方法** — 在一个方法中初始化所有服务：

```go
func (sm *ServiceManager) initializeServices() {
    sm.resultReporter = NewResultReporter(reporterConfig, sm.logger)
    sm.loadMonitor = rabbitmq.NewLoadMonitor(monitorConfig, sm.logger)
    sm.httpServerManger = NewHTTPServerManager(...)
    sm.shutdownCoord = NewShutdownCoordinator(
        sm.config,
        sm.rabbitmqService,
        sm.httpServerManger,
        sm.resultReporter,
        sm.loadMonitor,
        sm.logger,
    )
}
```

`ShutdownCoordinator` 的构造函数接收了 4 个具体类型参数，说明关闭逻辑与所有其他服务强耦合。

**`Start` 方法** — 按顺序启动 6 个服务：

```go
func (sm *ServiceManager) Start(ctx context.Context) error {
    sm.initializeServices()
    sm.resultReporter.Start(sm.ctx)
    sm.loadMonitor.Start(sm.ctx)
    sm.rabbitmqService.Start(sm.ctx)
    sm.httpServerManger.Start(sm.ctx)
    go sm.shutdownCoord.HandleSignals(...)
}
```

启动顺序硬编码在 `ServiceManager` 中，新增一个服务就必须修改这个方法。

## 影响分析

1. **修改风险高**：任何一个子服务的接口变化都会影响 `ServiceManager`，牵一发动全身。
2. **无法独立测试**：`RabbitMQService`、`HTTPServerManager` 等无法脱离 `ServiceManager` 单独测试。
3. **违反开闭原则**：新增服务类型（如 gRPC 服务器）必须修改 `ServiceManager` 的 `Start`、`Stop`、`initializeServices` 三个方法。
4. **包名语义模糊**：`messaging` 这个名字暗示只处理消息，但实际上还包含 HTTP 服务器和关闭协调，名不副实。

## 重构建议

引入 `lifecycle.Component` 接口，让每个服务自我管理，`ServiceManager` 只负责编排：

```go
// 定义统一的组件接口（项目中 internal/core/lifecycle 已有类似设计）
type Component interface {
    Start(ctx context.Context) error
    Stop(ctx context.Context) error
    Name() string
}
```

将 `messaging` 包按职责拆分：

```
internal/app/messaging/
    manager.go          ← 仅保留组件注册和启动顺序编排
    
internal/infra/rabbitmq/
    consumer.go         ← RabbitMQ 消费逻辑（已有 infra/rabbitmq 包，应合并）
    
internal/infra/reporter/
    result_reporter.go  ← 结果上报（与 RabbitMQ 无关，不应在 messaging 包）
```

注意：项目中 `internal/core/lifecycle` 和 `internal/app/bootstrap` 已经实现了类似的组件生命周期管理模式（`BaseComponent`、`LifecycleManager`），`messaging` 包应该复用这套机制，而不是自己再实现一套。
