# Worker 包

## 概述

`worker` 包是基础设施层的核心组件，提供**通用的**任务调度和并发管理功能。作为纯技术基础设施，它与具体业务逻辑完全解耦，可以被任何应用层组件复用。

## 位置

```
internal/infra/worker/  # 基础设施层
```

**为什么放在 infra 层？**
- ✅ 提供通用的并发处理能力，与业务无关
- ✅ 类似于 `rabbitmq`、`lock` 等基础设施组件
- ✅ 可以被多个应用层组件复用
- ✅ 是技术实现细节，不是业务逻辑
- ✅ 使用接口抽象，不包含业务字段

## 核心特性

- **任务调度**：管理任务的分发和执行
- **并发控制**：通过工作池控制并发数量
- **队列管理**：管理任务队列，防止过载
- **生命周期管理**：管理 worker 的启动和优雅关闭
- **接口抽象**：使用 `Job` 接口，支持任意类型的任务

## 核心组件

### 1. 接口定义 (interfaces.go)

#### Processor
任务处理器接口，定义了处理器的基本行为：
```go
type Processor interface {
    Start(ctx context.Context) error
    ProcessTask(ctx context.Context, job WorkerJob) error
    Close(ctx context.Context)
}
```

#### JobHandler
任务处理钩子接口，用于在任务处理的各个阶段接收通知和处理事件：
```go
type JobHandler interface {
    OnJobStart(job WorkerJob)
    OnJobSuccess(job WorkerJob)
    OnJobFailure(job WorkerJob, err error)
    OnJobPanic(job WorkerJob, panicValue any, stackTrace string)
    OnJobCompleted(job WorkerJob)
}
```

#### WorkerPool
工作池接口，管理并发任务执行：
```go
type WorkerPool interface {
    Start(ctx context.Context)
    Stop(ctx context.Context)
    Submit(job WorkerJob) error
    AvailableSlots() int
    GetQueueStats() QueueStats
    SetJobHandler(handler JobHandler)
}
```

### 2. 工作池实现 (pool.go)

`Pool` 是 `WorkerPool` 接口的具体实现，提供：

- **并发控制**：通过配置的 worker 数量控制并发
- **任务队列**：带缓冲的任务队列，防止过载
- **优雅关闭**：等待所有任务完成后再关闭
- **Panic 恢复**：自动恢复 worker 中的 panic，保证稳定性
- **指标收集**：自动收集任务处理的各项指标（提交数、成功数、失败数、panic数等）
- **任务钩子**：支持在任务处理的各个阶段接收通知

#### 使用示例

```go
// 创建工作池
pool := worker.NewPool(processor, config.WorkerConfig{
    Concurrency: 5,      // 5个并发worker
    BufferSize: 100,     // 队列容量100
})

// 启动工作池
pool.Start(ctx)

// 提交任务
err := pool.Submit(worker.WorkerJob{
    TaskID:   12345,
    TenantID: "1",
    ShopID:   "100",
    TaskData: taskJSON,
})

// 获取队列统计
stats := pool.GetQueueStats()
fmt.Printf("队列使用率: %.1f%%\n", stats.UsagePercent)

// 优雅关闭
pool.Stop(ctx)
```

### 2. 数据类型 (types.go)

#### Job 接口（推荐）
工作任务接口，业务层应该实现这个接口：
```go
type Job interface {
    GetID() int64  // 获取任务ID，用于追踪和指标收集
}
```

**示例：在业务层定义自己的 Job**
```go
// internal/domain/task/job.go
package task

type TaskJob struct {
    TaskID   int64
    TenantID string
    ShopID   string
    TaskData string
}

func (j TaskJob) GetID() int64 {
    return j.TaskID
}
```

#### WorkerJob（已废弃）
为了向后兼容保留的具体实现：
```go
// Deprecated: 建议在业务层定义自己的 Job 类型
type WorkerJob struct {
    TaskID   int64   // 任务ID
    TenantID string  // 租户ID（业务字段）
    ShopID   string  // 店铺ID（业务字段）
    TaskData string  // 任务数据（业务字段）
}
```

**注意：** `WorkerJob` 包含业务字段，不符合 infra 层的职责。建议使用 `Job` 接口，在业务层定义具体的 Job 类型。

#### QueueStats
队列统计信息：
```go
type QueueStats struct {
    QueueSize      int     // 当前队列中的任务数
    BufferSize     int     // 队列总容量
    AvailableSlots int     // 可用槽位数
    UsagePercent   float64 // 使用率（%）
}
```

## 架构设计

```
┌─────────────────────────────────────────────────────────┐
│                    Worker Package                        │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────┐         ┌──────────────┐             │
│  │  Interfaces  │────────▶│     Pool     │             │
│  │              │         │              │             │
│  │ - Processor  │         │ - Workers[]  │             │
│  │ - WorkerPool │         │ - JobQueue   │             │
│  │ - Submitter  │         │ - Lifecycle  │             │
│  └──────────────┘         └──────────────┘             │
│         │                        │                      │
│         │                        │                      │
│         ▼                        ▼                      │
│  ┌──────────────┐         ┌──────────────┐             │
│  │    Types     │         │   Worker     │             │
│  │              │         │              │             │
│  │ - WorkerJob  │◀────────│ - Run()      │             │
│  │ - QueueStats │         │ - Process()  │             │
│  └──────────────┘         └──────────────┘             │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

## 与其他包的关系

### 依赖关系
- `internal/domain/model` - 任务模型定义
- `internal/core/config` - 配置管理
- `internal/core/logger` - 日志记录

### 被依赖关系
- `internal/app/processor` - 具体的任务处理器实现
- `internal/app/task` - 任务管理服务
- `internal/app/messaging` - 消息队列服务
- `internal/platforms/*` - 各平台的处理器

## 最佳实践

### 1. 在业务层定义自己的 Job 类型（推荐）
```go
// internal/domain/task/job.go
package task

import "task-processor/internal/infra/worker"

type TaskJob struct {
    TaskID   int64
    TenantID string
    ShopID   string
    TaskData string
}

// 实现 worker.Job 接口
func (j TaskJob) GetID() int64 {
    return j.TaskID
}
```

### 2. 合理设置并发数
```go
// 根据任务类型设置并发数
// CPU密集型：并发数 = CPU核心数
// IO密集型：并发数 = CPU核心数 * 2-4
// 浏览器任务：并发数 = 浏览器池大小
workerConfig := config.WorkerConfig{
    Concurrency: runtime.NumCPU() * 2,
    BufferSize:  100,
}
```

### 2. 监控队列使用率
```go
stats := pool.GetQueueStats()
if stats.UsagePercent > 80 {
    log.Warn("队列使用率过高，考虑增加worker数量或减少任务提交速率")
}
```

### 3. 实现任务处理钩子
```go
type MyJobHandler struct{}

func (h *MyJobHandler) OnJobStart(job worker.WorkerJob) {
    log.Infof("任务 %d 开始处理", job.TaskID)
}

func (h *MyJobHandler) OnJobSuccess(job worker.WorkerJob) {
    log.Infof("任务 %d 处理成功", job.TaskID)
}

func (h *MyJobHandler) OnJobFailure(job worker.WorkerJob, err error) {
    log.Errorf("任务 %d 处理失败: %v", job.TaskID, err)
}

func (h *MyJobHandler) OnJobPanic(job worker.WorkerJob, panicValue any, stackTrace string) {
    log.Errorf("任务 %d 发生panic: %v\n%s", job.TaskID, panicValue, stackTrace)
}

func (h *MyJobHandler) OnJobCompleted(job worker.WorkerJob) {
    log.Infof("任务 %d 已完成", job.TaskID)
}

pool.SetJobHandler(&MyJobHandler{})
```

### 4. 获取指标统计
```go
// 获取指标收集器
metrics := pool.GetMetrics()
if metrics != nil {
    snapshot := metrics.GetSnapshot()
    log.Infof("总提交: %d, 总处理: %d, 成功率: %.2f%%", 
        snapshot.TotalSubmitted, 
        snapshot.TotalProcessed,
        snapshot.SuccessRate())
}
```

### 5. 优雅关闭
```go
// 设置关闭超时
shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

// 停止接收新任务
pool.Stop(shutdownCtx)

// 等待所有任务完成
<-shutdownCtx.Done()
```

## 注意事项

1. **避免阻塞**：ProcessTask 中不要执行长时间阻塞操作，使用 context 控制超时
2. **错误处理**：Worker 会自动恢复 panic，但应该在 ProcessTask 中妥善处理错误
3. **资源清理**：使用 defer 确保资源被正确释放
4. **并发安全**：Pool 内部已实现并发安全，外部使用时无需额外加锁

## 新增功能

### 指标收集 (metrics.go)
工作池现在支持自动收集以下指标：
- 总提交数、总处理数
- 成功数、失败数、Panic数
- 队列满次数
- 运行时间
- 成功率、失败率、Panic率

### 任务钩子 (JobHandler)
支持在任务处理的各个阶段接收通知：
- `OnJobStart` - 任务开始处理
- `OnJobSuccess` - 任务处理成功
- `OnJobFailure` - 任务处理失败
- `OnJobPanic` - 任务发生panic
- `OnJobCompleted` - 任务处理完成（无论成功或失败）

### 配置增强 (config.go)
新增 `PoolConfig` 配置结构，支持：
- 任务超时时间配置
- 优雅关闭超时配置
- 指标收集开关
- 配置验证

## 未来改进

- [ ] 支持任务优先级队列
- [ ] 支持动态调整 worker 数量
- [ ] 支持任务重试策略配置
- [ ] 支持更详细的性能分析（处理时间分布、P99延迟等）
