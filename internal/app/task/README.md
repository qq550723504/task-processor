# Task 应用层

## 概述

`internal/app/task` 包含任务相关的应用层服务，负责协调领域层和基础设施层，实现完整的业务流程。

## 设计原则

1. **应用编排** - 协调多个领域服务和基础设施服务
2. **流程控制** - 实现完整的业务流程
3. **生命周期管理** - 管理服务的启动、停止和状态
4. **依赖管理** - 管理对领域层和基础设施层的依赖

## 文件说明

### cleanup_service.go
任务清理服务，负责清理过期、超时、失败的任务。

**核心功能：**
- 定期清理任务
- 多种清理策略
- 可配置的清理规则
- 生命周期管理

**使用示例：**
```go
cleanupService := task.NewCleanupService(fetcher, config)
cleanupService.Start(ctx)
defer cleanupService.Stop()
```

### monitor_service.go
任务监控服务，负责监控任务队列的健康状态。

**核心功能：**
- 队列健康检查
- 性能监控
- 告警通知
- 统计报告

**使用示例：**
```go
monitorService := task.NewMonitorService(fetcher)
monitorService.StartMonitoring(ctx)
```

### queue_manager.go
队列管理器，负责队列的管理和命令执行。

**核心功能：**
- 队列状态查询
- 问题诊断
- 强制清理
- 命令执行

**使用示例：**
```go
queueManager := task.NewQueueManager(fetcher, cleanupService, monitorService)
err := queueManager.ExecuteCommand(cmd)
```

### fetcher.go
任务获取器，负责从数据源获取任务并调度。

**核心功能：**
- 任务获取
- 任务调度
- 负载均衡
- 生命周期管理

**使用示例：**
```go
fetcher := task.NewUnifiedTaskFetcher(config, logger)
err := fetcher.Start(ctx)
defer fetcher.Stop(ctx)
```

### dispatcher.go
任务分发器，负责将任务分发到不同的处理器。

**核心功能：**
- 任务路由
- 负载均衡
- 优先级处理
- 错误处理

### deduplication_manager.go
去重管理器，负责复杂的任务去重和状态管理。

**核心功能：**
- 任务状态跟踪
- 重复检测
- 重试管理
- 超时处理

**使用示例：**
```go
dedupManager := task.NewDeduplicationManager(logger, maxAge)
dedupManager.Start(ctx)
defer dedupManager.Stop()

canSubmit, err := dedupManager.CanSubmitTask(taskID)
if canSubmit {
    // 提交任务
}
```

## 依赖关系

```
app/task (应用层)
    ↓ 使用
domain/task (领域层) - 业务规则
domain/model (领域模型) - 数据模型
infra/rabbitmq (基础设施层) - 消息队列
infra/database (基础设施层) - 数据库
```

## 架构分层

```
┌─────────────────────────────────────┐
│      app/task (应用层)               │
│  - 流程编排                          │
│  - 服务协调                          │
│  - 生命周期管理                      │
└─────────────────────────────────────┘
           ↓ 使用
┌─────────────────────────────────────┐
│      domain/task (领域层)            │
│  - 业务规则                          │
│  - 领域逻辑                          │
│  - 去重规则                          │
└─────────────────────────────────────┘
           ↓ 使用
┌─────────────────────────────────────┐
│      infra/* (基础设施层)            │
│  - RabbitMQ                          │
│  - 数据库                            │
│  - HTTP 客户端                       │
└─────────────────────────────────────┘
```

## 服务协调示例

### 完整的任务处理流程

```go
// 1. 创建基础服务
fetcher := task.NewUnifiedTaskFetcher(config, logger)
cleanupService := task.NewCleanupService(fetcher, config)
monitorService := task.NewMonitorService(fetcher)

// 2. 创建管理器
queueManager := task.NewQueueManager(fetcher, cleanupService, monitorService)

// 3. 启动服务
ctx := context.Background()
fetcher.Start(ctx)
cleanupService.Start(ctx)
monitorService.StartMonitoring(ctx)

// 4. 执行管理命令
queueManager.ExecuteCommand(ShowStatusCommand)

// 5. 停止服务
defer fetcher.Stop(ctx)
```

## 最佳实践

### 1. 依赖注入

```go
// ✅ 推荐：通过构造函数注入依赖
func NewCleanupService(fetcher *TaskFetcher, cfg *config.Config) *CleanupService {
    return &CleanupService{
        fetcher: fetcher,
        config:  cfg,
    }
}

// ❌ 不推荐：在内部创建依赖
func NewCleanupService() *CleanupService {
    fetcher := NewTaskFetcher(...)  // ❌
    return &CleanupService{fetcher: fetcher}
}
```

### 2. 生命周期管理

```go
service := task.NewCleanupService(fetcher, config)

// 启动服务
err := service.Start(ctx)
if err != nil {
    return err
}

// 确保停止
defer service.Stop()
```

### 3. 错误处理

```go
err := queueManager.ExecuteCommand(cmd)
if err != nil {
    logger.WithFields(logrus.Fields{
        "command": cmd,
        "error":   err,
    }).Error("执行命令失败")
    return fmt.Errorf("执行命令失败: %w", err)
}
```

### 4. 上下文传递

```go
// 使用带超时的上下文
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := service.Start(ctx)
```

## 测试示例

### 单元测试（Mock 依赖）

```go
func TestCleanupService(t *testing.T) {
    // 创建 Mock
    mockFetcher := &MockTaskFetcher{}
    mockConfig := &config.Config{}
    
    // 创建服务
    service := task.NewCleanupService(mockFetcher, mockConfig)
    
    // 测试
    err := service.Start(context.Background())
    assert.NoError(t, err)
}
```

### 集成测试（真实依赖）

```go
func TestCleanupService_Integration(t *testing.T) {
    // 使用真实的依赖
    fetcher := task.NewUnifiedTaskFetcher(config, logger)
    service := task.NewCleanupService(fetcher, config)
    
    // 测试完整流程
    err := service.Start(context.Background())
    assert.NoError(t, err)
    
    // 验证清理效果
    // ...
}
```

## 扩展点

### 添加新的清理策略

```go
// 在 cleanup_service.go 中
func (s *CleanupService) registerStrategies() {
    s.strategies = append(s.strategies, 
        NewTimeoutCleanupStrategy(),
        NewFailedCleanupStrategy(),
        NewCustomCleanupStrategy(),  // 新增策略
    )
}
```

### 添加新的监控指标

```go
// 在 monitor_service.go 中
func (s *MonitorService) registerStrategies() {
    s.strategies = append(s.strategies,
        NewQueueHealthStrategy(),
        NewPerformanceStrategy(),
        NewCustomMetricStrategy(),  // 新增指标
    )
}
```

## 注意事项

1. 应用层不应该包含业务规则，业务规则应该在领域层
2. 应用层负责协调，不负责具体实现
3. 保持应用服务的薄层特性，避免过度膨胀
4. 使用依赖注入，便于测试和替换实现
5. 合理使用上下文传递超时和取消信号
6. 确保资源正确清理（使用 defer）

## 相关文档

- [领域层文档](../../domain/task/README.md)
- [消息处理服务](../messaging/README.md)
- [架构说明](../../../README_ARCHITECTURE.md)

---

**最后更新：** 2024-01  
**维护者：** 开发团队
