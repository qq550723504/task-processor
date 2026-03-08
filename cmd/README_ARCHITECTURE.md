# Task Processor - 架构说明

## 🏗️ 架构概览

本项目采用清晰的分层架构，遵循领域驱动设计（DDD）和 SOLID 原则。

### 架构图

```
┌─────────────────────────────────────────────────────────┐
│                    应用层 (app/)                         │
│  - 流程编排                                              │
│  - 服务协调                                              │
│  - 事务管理                                              │
│                                                          │
│  app/messaging/     - 消息处理服务                       │
│  app/worker/        - 工作器                             │
│  app/scheduler/     - 调度器                             │
└─────────────────────────────────────────────────────────┘
                          ↓ 使用
┌─────────────────────────────────────────────────────────┐
│                    领域层 (domain/)                      │
│  - 业务规则                                              │
│  - 领域逻辑                                              │
│  - 领域模型                                              │
│                                                          │
│  domain/model/      - 领域模型                           │
│  domain/task/       - 任务领域逻辑                       │
│  domain/product/    - 产品领域逻辑                       │
└─────────────────────────────────────────────────────────┘
                          ↓ 使用
┌─────────────────────────────────────────────────────────┐
│                 基础设施层 (infra/)                      │
│  - 数据库访问                                            │
│  - 消息队列                                              │
│  - 外部服务                                              │
│                                                          │
│  infra/rabbitmq/    - RabbitMQ 客户端                    │
│  infra/database/    - 数据库访问                         │
│  infra/http/        - HTTP 客户端                        │
│  infra/crawler/     - 爬虫客户端                         │
└─────────────────────────────────────────────────────────┘
```

## 📁 目录结构

```
task-processor/
├── cmd/                    # 应用入口
│   ├── task/              # 主程序
│   └── crawler-consumer/  # 爬虫消费者
│
├── internal/              # 内部代码
│   ├── app/              # 应用层
│   │   ├── messaging/    # 消息处理服务 ✨ 新增
│   │   ├── worker/       # 工作器
│   │   └── scheduler/    # 调度器
│   │
│   ├── domain/           # 领域层
│   │   ├── model/        # 领域模型
│   │   ├── task/         # 任务领域逻辑 ✨ 新增
│   │   └── product/      # 产品领域逻辑
│   │
│   ├── infra/            # 基础设施层
│   │   ├── rabbitmq/     # RabbitMQ 客户端
│   │   ├── database/     # 数据库访问
│   │   ├── http/         # HTTP 客户端
│   │   └── crawler/      # 爬虫客户端
│   │
│   ├── core/             # 核心组件
│   │   ├── config/       # 配置管理
│   │   ├── logger/       # 日志管理
│   │   └── system/       # 系统初始化
│   │
│   ├── crawler/          # 爬虫实现
│   │   ├── amazon/       # Amazon 爬虫
│   │   ├── temu/         # TEMU 爬虫
│   │   └── shein/        # SHEIN 爬虫
│   │
│   └── pkg/              # 公共包
│       ├── management/   # 管理客户端
│       └── utils/        # 工具函数
│
├── config/               # 配置文件
├── docs/                 # 文档
├── examples/             # 示例程序 ✨ 新增
├── tests/                # 测试
└── scripts/              # 脚本
```

## 🎯 设计原则

### 1. 单一职责原则（SRP）

每个模块只有一个变化的原因。

**示例：**
- `domain/task/deduplicator.go` - 只负责任务去重
- `app/messaging/task_submitter.go` - 只负责任务提交流程
- `infra/rabbitmq/client.go` - 只负责 RabbitMQ 技术实现

### 2. 依赖倒置原则（DIP）

高层模块不依赖低层模块，都依赖抽象。

**示例：**
```go
// 应用层依赖接口，不依赖具体实现
type MessageQueue interface {
    Publish(ctx context.Context, msg *Message) error
}

type TaskSubmitter struct {
    mq MessageQueue  // 依赖接口
}
```

### 3. 开闭原则（OCP）

对扩展开放，对修改关闭。

**示例：**
```go
// 新增平台只需添加配置，无需修改代码
func (a *MessageAdapter) GetQueueName(platform string) string {
    if queue, ok := a.queueMapping[platform]; ok {
        return queue
    }
    return "default.queue"
}
```

## 🔄 最近重构

### RabbitMQ 包重构（2024-01）

**问题：** `internal/infra/rabbitmq` 包职责过重，混合了基础设施、业务逻辑、应用编排。

**解决方案：** 按照分层架构原则拆分

#### 重构前
```
infra/rabbitmq/
├── client.go              # RabbitMQ 客户端
├── task_adapter.go        # ❌ 业务逻辑（不应在这里）
├── task_submitter.go      # ❌ 应用逻辑（不应在这里）
├── deduplicator.go        # ❌ 业务规则（不应在这里）
└── ...
```

#### 重构后
```
domain/task/               # ✅ 领域层（业务规则）
├── deduplicator.go        # 任务去重器
└── message_adapter.go     # 消息适配器

app/messaging/             # ✅ 应用层（流程编排）
└── task_submitter.go      # 任务提交服务

infra/rabbitmq/            # ✅ 基础设施层（技术实现）
├── client.go              # RabbitMQ 客户端
├── connection.go          # 连接管理
└── ...
```

#### 改进效果

| 方面 | 重构前 | 重构后 | 改进 |
|------|--------|--------|------|
| 职责清晰度 | ⭐⭐ | ⭐⭐⭐⭐⭐ | +150% |
| 可测试性 | ⭐⭐ | ⭐⭐⭐⭐⭐ | +150% |
| 可维护性 | ⭐⭐⭐ | ⭐⭐⭐⭐⭐ | +67% |
| 可复用性 | ⭐⭐ | ⭐⭐⭐⭐⭐ | +150% |

**详细文档：**
- [重构总结](./docs/REFACTORING_SUMMARY.md)
- [迁移完成报告](./docs/MIGRATION_COMPLETED.md)
- [新架构快速入门](./docs/QUICK_START_NEW_STRUCTURE.md)

## 💻 使用新架构

### 基本用法

```go
import (
    "task-processor/internal/app/messaging"
    "task-processor/internal/domain/task"
    "task-processor/internal/infra/rabbitmq"
)

// 1. 创建基础设施层
connManager := rabbitmq.NewConnectionManager(config, logger)
mqClient := rabbitmq.NewClient(connManager, logger)

// 2. 创建应用层服务
submitter := messaging.NewTaskSubmitter(mqClient, logger)

// 3. 提交任务
err := submitter.SubmitTask(ctx, task)
```

### 使用领域层

```go
// 消息适配器（业务规则）
adapter := task.NewMessageAdapter()
queueName := adapter.GetQueueName("amazon")
priority := adapter.CalculatePriority(1)

// 去重器（业务规则）
dedup := task.NewDeduplicator(5*time.Minute, logger)
defer dedup.Stop()

if !dedup.IsDuplicate(taskID) {
    processTask(taskID)
    dedup.MarkProcessed(taskID)
}
```

## 🧪 测试

### 单元测试

```bash
# 测试领域层（无需外部依赖）
go test -v ./internal/domain/task/

# 查看测试覆盖率
go test -cover ./internal/domain/task/

# 运行所有测试
go test ./...
```

### 示例程序

```bash
# 运行任务提交示例
go run examples/task_submission_example.go
```

## 📚 文档导航

### 架构文档
- [架构概览](./docs/architecture/README.md)
- [重构状态](./docs/REFACTORING_STATUS.md)
- [重构总结](./docs/REFACTORING_SUMMARY.md)

### 使用指南
- [新架构快速入门](./docs/QUICK_START_NEW_STRUCTURE.md)
- [领域层文档](./internal/domain/task/README.md)
- [应用层文档](./internal/app/messaging/README.md)
- [基础设施层文档](./internal/infra/rabbitmq/README.md)

### 开发文档
- [开发指南](./docs/development/README.md)
- [API 文档](./docs/api/README.md)
- [示例程序](./examples/README.md)

## 🎓 最佳实践

### 1. 依赖注入

```go
// ✅ 推荐：通过构造函数注入依赖
func NewService(submitter *messaging.TaskSubmitter) *Service {
    return &Service{submitter: submitter}
}

// ❌ 不推荐：在内部创建依赖
func NewService() *Service {
    submitter := messaging.NewTaskSubmitter(...)
    return &Service{submitter: submitter}
}
```

### 2. 错误处理

```go
err := submitter.SubmitTask(ctx, task)
if err != nil {
    logger.WithFields(logrus.Fields{
        "task_id": task.ID,
        "error":   err,
    }).Error("提交任务失败")
    return fmt.Errorf("提交任务失败: %w", err)
}
```

### 3. 上下文传递

```go
// 使用带超时的上下文
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()

err := submitter.SubmitTask(ctx, task)
```

### 4. 资源清理

```go
dedup := task.NewDeduplicator(ttl, logger)
defer dedup.Stop()  // 确保清理资源
```

## 🔧 扩展指南

### 添加新平台

1. 在 `domain/task/message_adapter.go` 中添加队列映射
2. 在 `crawler/` 下创建新平台的爬虫实现
3. 注册到应用层服务

### 添加新的业务规则

1. 在 `domain/task/` 中创建新的领域逻辑
2. 编写单元测试
3. 在应用层中使用

### 添加新的应用服务

1. 在 `app/` 下创建新的服务
2. 使用领域层和基础设施层
3. 提供清晰的接口

## 📞 获取帮助

- 查看 [文档索引](./docs/README.md)
- 查看 [示例程序](./examples/README.md)
- 提交 Issue
- 联系团队成员

---

**最后更新：** 2024-01  
**维护者：** 开发团队
