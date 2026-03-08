# App 层重构说明

## 重构日期
2026-03-08

## 重构目标
厘清代码职责，将混乱的代码结构重新组织，实现清晰的分层架构。

## 重构内容

### 1. 创建 `processor` 包（应用层）

将业务处理器相关代码从 `worker` 包移动到新的 `processor` 包：

```
internal/app/worker/                    internal/app/processor/
├── base_processor.go          →        ├── base_processor.go
├── base_task_handler.go       →        ├── base_task_handler.go
└── crawler_processor.go       →        └── crawler_processor.go
```

**原因**：
- 业务处理器逻辑应该在应用层，便于扩展和维护
- 与具体业务相关的处理器不应该与基础设施混在一起

### 2. 移动 `worker` 包到基础设施层

将 `worker` 包从应用层移动到基础设施层：

```
internal/app/worker/            →       internal/infra/worker/
├── README.md                           ├── README.md
├── interfaces.go                       ├── interfaces.go
├── pool.go                             ├── pool.go
└── types.go                            └── types.go
```

**原因**：
- `worker` 包提供的是通用的并发处理能力，与业务无关
- 类似于 `rabbitmq`、`memory` 等基础设施组件
- 可以被多个应用层组件复用
- 是技术实现细节，不是业务逻辑

### 3. 更新引用关系

所有引用 `internal/app/worker` 的代码已更新为 `internal/infra/worker`：

**受影响的文件**：
- `internal/platforms/temu/processor.go`
- `internal/platforms/temu/task_submitter.go`
- `internal/platforms/shein/service/pipeline/processor_service.go`
- `internal/platforms/shein/service/pipeline/submitter_service.go`
- `internal/app/processor/base_processor.go`
- `internal/app/processor/base_task_handler.go`
- `internal/app/processor/crawler_processor.go`
- `internal/app/messaging/rabbitmq_service.go`
- `internal/app/messaging/service_manager.go`
- `internal/app/messaging/task_handler.go`
- `internal/app/task/models.go`
- `internal/app/task/processor_submitter_adapter.go`
- `internal/app/task/interfaces.go`
- `internal/app/task/monitor_service.go`
- `internal/app/task/deduplication_manager.go`

### 4. 接口定义优化

在 `worker/interfaces.go` 中添加了 `VariantTaskSubmitter` 接口：

```go
// VariantTaskSubmitter 变体任务提交器接口
type VariantTaskSubmitter interface {
    SubmitVariantTasks(ctx context.Context, parentTask *model.Task, 
        variations []model.Variation, parentAsin string) (successCount, failCount int)
}
```

这样避免了在多个地方重复定义相同的接口。

## 架构改进

### 重构前
```
internal/app/worker/
├── interfaces.go           # 接口定义
├── pool.go                # 工作池
├── types.go               # 数据类型
├── base_processor.go      # ❌ 业务处理器（层次混乱）
├── base_task_handler.go   # ❌ 业务处理器（层次混乱）
└── crawler_processor.go   # ❌ 业务处理器（层次混乱）
```

### 重构后
```
internal/
├── infra/                  # 基础设施层
│   └── worker/            # ✅ 任务调度基础设施
│       ├── README.md      # 📖 包文档
│       ├── interfaces.go  # ✅ 接口定义
│       ├── pool.go       # ✅ 工作池实现
│       └── types.go      # ✅ 数据类型
│
└── app/                   # 应用层
    └── processor/         # ✅ 业务处理层
        ├── README.md          # 📖 包文档
        ├── base_processor.go      # ✅ 基础处理器
        ├── base_task_handler.go   # ✅ 任务处理器基类
        └── crawler_processor.go   # ✅ 爬虫处理器
```

## 分层架构

```
┌─────────────────────────────────────────────────┐
│         platforms/* (平台层)                     │
│      (TEMU, SHEIN, Amazon)                      │
└────────────────┬────────────────────────────────┘
                 │ 使用
                 ▼
┌─────────────────────────────────────────────────┐
│      app/processor (应用层)                      │
│   (BaseProcessor, CrawlerProcessor)             │
└────────────────┬────────────────────────────────┘
                 │ 实现接口
                 ▼
┌─────────────────────────────────────────────────┐
│      infra/worker (基础设施层)                   │
│   (Processor接口, WorkerPool)                   │
└─────────────────────────────────────────────────┘
```

## 优势

1. **分层清晰**：
   - 基础设施层：通用技术组件（worker）
   - 应用层：业务逻辑处理（processor）
   - 平台层：具体平台实现

2. **职责明确**：
   - `infra/worker` 专注于任务调度和并发管理
   - `app/processor` 专注于业务逻辑处理

3. **易于复用**：
   - worker 作为基础设施，可以被任何应用层组件使用
   - 不同的业务场景可以共享同一套并发管理机制

4. **便于测试**：
   - 基础设施层和应用层分离，可以独立测试
   - 接口定义清晰，易于 mock

## 文档

- [Worker 包文档](../infra/worker/README.md)
- [Processor 包文档](processor/README.md)

## 注意事项

1. **导入路径变更**：
   - 旧：`task-processor/internal/app/worker`
   - 新：`task-processor/internal/infra/worker`

2. **类型引用**：
   - 基础处理器：`processor.BaseProcessor`（应用层）
   - 工作池接口：`worker.WorkerPool`（基础设施层）
   - 处理器接口：`worker.Processor`（基础设施层）

3. **向后兼容**：
   - 所有接口保持不变
   - 只是移动了包的位置

## 后续优化建议

1. **考虑将 `BaseProcessor` 改为组合而非继承**：
   ```go
   type TemuProcessor struct {
       base *processor.BaseProcessor  // 组合
       // ...
   }
   ```

2. **考虑引入依赖注入框架**：
   - 简化组件创建和依赖管理
   - 提高代码可测试性

3. **考虑添加处理器注册机制**：
   - 自动发现和注册处理器
   - 支持插件化扩展

4. **考虑添加处理器生命周期钩子**：
   - OnStart、OnStop、OnError 等
   - 统一的错误处理和资源清理
