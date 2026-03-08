# Processor 包

## 概述

`processor` 包提供任务处理器的基础实现和通用组件，为各平台的具体处理器提供基础设施。

## 职责

- **基础设施**：提供所有处理器共用的基础组件
- **通用逻辑**：实现处理器的通用行为和生命周期管理
- **资源管理**：管理处理器所需的共享资源（配置、客户端、内存管理器等）

## 核心组件

### 1. 基础处理器 (base_processor.go)

`BaseProcessor` 是所有平台处理器的基类，提供通用的字段和方法。

#### 主要功能
- 配置管理
- 管理系统客户端
- 内存管理器
- 工作池管理
- 日志记录

#### 使用示例
```go
// 创建基础处理器
baseProcessor := processor.NewBaseProcessor(ctx, &processor.BaseProcessorConfig{
    Config:           appConfig,
    ManagementClient: managementClient,
    Logger:           logger,
    Platform:         "amazon",
})

// 获取共享资源
config := baseProcessor.GetConfig()
memoryMgr := baseProcessor.GetMemoryManager()
workerPool := baseProcessor.GetWorkerPool()

// 启动基础组件
err := baseProcessor.StartBase(ctx)

// 关闭基础组件
baseProcessor.CloseBase(ctx)
```

### 2. 基础任务处理器 (base_task_handler.go)

`BaseTaskHandler` 提供统一的任务处理逻辑，包装具体的处理器实现。

#### 主要功能
- 任务处理流程控制
- 性能监控（处理时间）
- 统一的日志记录
- 错误处理

#### 使用示例
```go
// 创建任务处理器
handler := processor.NewBaseTaskHandler(concreteProcessor, "amazon")

// 处理任务
err := handler.ProcessTask(ctx, task, pipeline)
```

### 3. 爬虫处理器 (crawler_processor.go)

`CrawlerProcessor` 是 Amazon 爬虫的具体实现，负责产品数据爬取。

#### 主要功能
- 产品数据爬取
- 变体任务提交
- 数据缓存
- 浏览器资源管理

#### 使用示例
```go
// 创建爬虫处理器
crawlerProcessor := processor.NewCrawlerProcessor(
    logger,
    amazonProcessor,
    productFetcher,
    taskSubmitter,
)

// 启动处理器
err := crawlerProcessor.Start(ctx)

// 处理任务
err = crawlerProcessor.ProcessTask(ctx, task)

// 关闭处理器
crawlerProcessor.Close(ctx)
```

## 架构设计

```
┌─────────────────────────────────────────────────────────┐
│                  Processor Package                       │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────────────────────────────────┐           │
│  │         BaseProcessor                    │           │
│  │  ┌────────────────────────────────────┐  │           │
│  │  │ - Config                           │  │           │
│  │  │ - ManagementClient                 │  │           │
│  │  │ - MemoryManager                    │  │           │
│  │  │ - WorkerPool                       │  │           │
│  │  │ - Logger                           │  │           │
│  │  └────────────────────────────────────┘  │           │
│  └──────────────────────────────────────────┘           │
│                    │                                     │
│                    │ 继承/组合                            │
│                    ▼                                     │
│  ┌──────────────────────────────────────────┐           │
│  │      BaseTaskHandler                     │           │
│  │  ┌────────────────────────────────────┐  │           │
│  │  │ - Processor (interface)            │  │           │
│  │  │ - Logger                           │  │           │
│  │  │                                    │  │           │
│  │  │ + ProcessTask()                    │  │           │
│  │  └────────────────────────────────────┘  │           │
│  └──────────────────────────────────────────┘           │
│                    │                                     │
│                    │ 使用                                 │
│                    ▼                                     │
│  ┌──────────────────────────────────────────┐           │
│  │      CrawlerProcessor                    │           │
│  │  ┌────────────────────────────────────┐  │           │
│  │  │ - AmazonProcessor                  │  │           │
│  │  │ - ProductFetcher                   │  │           │
│  │  │ - TaskSubmitter                    │  │           │
│  │  │                                    │  │           │
│  │  │ + Start()                          │  │           │
│  │  │ + ProcessTask()                    │  │           │
│  │  │ + Close()                          │  │           │
│  │  └────────────────────────────────────┘  │           │
│  └──────────────────────────────────────────┘           │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

## 与其他包的关系

### 依赖关系
- `internal/app/worker` - 工作池和接口定义
- `internal/core/config` - 配置管理
- `internal/core/logger` - 日志记录
- `internal/infra/memory` - 内存管理
- `internal/pkg/management` - 管理系统客户端
- `internal/crawler/amazon` - Amazon 爬虫实现
- `internal/domain/product` - 产品领域模型

### 被依赖关系
- `internal/platforms/*` - 各平台的具体处理器
- `internal/app/messaging` - 消息队列服务

## 处理器实现指南

### 1. 创建新的平台处理器

```go
package myplatform

import (
    "context"
    "task-processor/internal/app/processor"
    "task-processor/internal/app/worker"
    "task-processor/internal/domain/model"
)

// MyPlatformProcessor 自定义平台处理器
type MyPlatformProcessor struct {
    *processor.BaseProcessor  // 继承基础处理器
    // 添加平台特定的字段
    apiClient *MyPlatformAPI
}

// NewMyPlatformProcessor 创建处理器
func NewMyPlatformProcessor(ctx context.Context, cfg *processor.BaseProcessorConfig) *MyPlatformProcessor {
    base := processor.NewBaseProcessor(ctx, cfg)
    
    return &MyPlatformProcessor{
        BaseProcessor: base,
        apiClient:     NewMyPlatformAPI(cfg.Config),
    }
}

// Start 启动处理器
func (p *MyPlatformProcessor) Start(ctx context.Context) error {
    // 启动基础组件
    if err := p.StartBase(ctx); err != nil {
        return err
    }
    
    // 启动平台特定组件
    // ...
    
    return nil
}

// ProcessTask 处理任务
func (p *MyPlatformProcessor) ProcessTask(ctx context.Context, task *model.Task) error {
    // 实现具体的任务处理逻辑
    // ...
    
    return nil
}

// Close 关闭处理器
func (p *MyPlatformProcessor) Close(ctx context.Context) {
    // 关闭平台特定组件
    // ...
    
    // 关闭基础组件
    p.CloseBase(ctx)
}
```

### 2. 使用 BaseTaskHandler

```go
// 创建任务处理器
processor := NewMyPlatformProcessor(ctx, config)
handler := processor.NewBaseTaskHandler(processor, "myplatform")

// 在消息处理器中使用
func (h *MessageHandler) HandleMessage(ctx context.Context, msg *Message) error {
    var task model.Task
    if err := json.Unmarshal(msg.Body, &task); err != nil {
        return err
    }
    
    return handler.ProcessTask(ctx, task, nil)
}
```

## 最佳实践

### 1. 资源管理
```go
// 使用 BaseProcessor 管理共享资源
func (p *MyProcessor) ProcessTask(ctx context.Context, task *model.Task) error {
    // 获取共享资源
    memoryMgr := p.GetMemoryManager()
    config := p.GetConfig()
    
    // 使用资源
    // ...
    
    return nil
}
```

### 2. 错误处理
```go
func (p *MyProcessor) ProcessTask(ctx context.Context, task *model.Task) error {
    // 使用 defer 确保资源清理
    defer func() {
        if r := recover(); r != nil {
            p.GetLogger().Errorf("处理任务时发生panic: %v", r)
        }
    }()
    
    // 处理任务
    if err := p.doSomething(); err != nil {
        return fmt.Errorf("处理失败: %w", err)
    }
    
    return nil
}
```

### 3. 生命周期管理
```go
func (p *MyProcessor) Start(ctx context.Context) error {
    // 1. 启动基础组件
    if err := p.StartBase(ctx); err != nil {
        return err
    }
    
    // 2. 初始化平台特定资源
    if err := p.initResources(); err != nil {
        p.CloseBase(ctx)  // 失败时清理
        return err
    }
    
    // 3. 启动后台任务
    go p.backgroundTask(ctx)
    
    return nil
}

func (p *MyProcessor) Close(ctx context.Context) {
    // 1. 停止后台任务
    p.stopBackgroundTasks()
    
    // 2. 清理平台特定资源
    p.cleanupResources()
    
    // 3. 关闭基础组件
    p.CloseBase(ctx)
}
```

## 注意事项

1. **继承 vs 组合**：优先使用组合而不是继承，BaseProcessor 提供了 getter 方法
2. **资源共享**：通过 BaseProcessor 共享资源，避免重复创建
3. **生命周期**：正确实现 Start/Close 方法，确保资源被正确管理
4. **错误传播**：使用 `fmt.Errorf` 和 `%w` 保留错误链
5. **上下文传递**：始终传递 context，支持超时和取消

## 目录结构

```
processor/
├── README.md                    # 本文档
├── base_processor.go            # 基础处理器实现
├── base_task_handler.go         # 基础任务处理器
└── crawler_processor.go         # 爬虫处理器实现
```

## 相关文档

- [Worker 包文档](../worker/README.md)
- [消息队列文档](../messaging/README.md)
- [平台处理器文档](../../platforms/README.md)
