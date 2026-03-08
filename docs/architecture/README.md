# 架构文档目录

## 用途

存放项目的架构设计文档，包括系统架构、模块设计、数据流程等。

## 目录结构

```
architecture/
├── README.md           # 本文件
├── overview.md         # 架构概览
├── data-flow.md        # 数据流程图
├── component-design.md # 组件设计
├── database-schema.md  # 数据库设计（如果有）
├── message-queue.md    # 消息队列设计
└── diagrams/           # 架构图
    ├── system-architecture.png
    ├── data-flow.png
    └── component-diagram.png
```

## 应该放置的文件

### 1. 架构概览（overview.md）

系统的整体架构说明。

**模板：**
```markdown
# 系统架构概览

## 1. 系统简介

task-processor 是一个多平台电商任务处理系统，支持 Amazon、Temu、Shein 等平台的产品数据采集和处理。

## 2. 技术栈

### 后端
- Go 1.21+
- RabbitMQ（消息队列）
- Chrome/Chromium（网页爬取）

### 日志和监控
- Logrus（结构化日志）
- 自定义监控指标

## 3. 架构风格

采用分层架构 + DDD（领域驱动设计）：

```
┌─────────────────────────────────────┐
│         应用层 (app)                 │
│  业务编排、服务组合、任务调度        │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│        平台层 (platforms)            │
│  Temu、Shein、Amazon 业务逻辑       │
└─────────────────────────────────────┘
              ↓
┌──────────────┬──────────────────────┐
│ 领域层       │  基础设施层           │
│ (domain)     │  (infra)             │
│ 业务规则     │  技术实现             │
└──────────────┴──────────────────────┘
              ↓
┌─────────────────────────────────────┐
│         核心层 (core)                │
│  配置、日志、生命周期、错误处理      │
└─────────────────────────────────────┘
```

## 4. 核心模块

### 4.1 任务处理流程

1. RabbitMQ 接收任务消息
2. 任务分发到对应平台处理器
3. 爬虫获取产品数据
4. 数据验证和转换
5. 结果上报到管理系统

### 4.2 调度系统

三层调度架构：
- app/scheduler - 通用任务调度框架
- platforms/common/scheduler - 平台通用调度基类
- platforms/{platform}/scheduler - 平台特定实现

### 4.3 爬虫系统

- 浏览器池管理
- 反爬虫处理
- 验证码识别
- 数据提取

## 5. 部署架构

```
┌──────────────┐
│  管理系统     │
│  (HTTP API)  │
└──────┬───────┘
       │
       ↓
┌──────────────┐     ┌──────────────┐
│  RabbitMQ    │────→│ Task         │
│  消息队列     │     │ Processor    │
└──────────────┘     └──────┬───────┘
                            │
                            ↓
                     ┌──────────────┐
                     │  Chrome      │
                     │  浏览器池     │
                     └──────────────┘
```

## 6. 设计原则

1. 单一职责原则
2. 依赖倒置原则
3. 接口隔离原则
4. 开闭原则
5. 里氏替换原则

## 7. 关键设计决策

### 7.1 为什么使用分层架构？

- 清晰的职责边界
- 易于测试和维护
- 支持独立演进

### 7.2 为什么使用 DDD？

- 业务逻辑集中在领域层
- 技术细节隔离在基础设施层
- 便于理解和沟通

### 7.3 为什么使用 RabbitMQ？

- 解耦任务生产和消费
- 支持任务持久化
- 提供重试机制

## 8. 性能考虑

- 浏览器池复用
- 并发任务处理
- 缓存机制
- 批量操作

## 9. 安全考虑

- 认证和授权
- 敏感信息加密
- 输入验证
- 错误信息脱敏
```

### 2. 数据流程图（data-flow.md）

描述数据在系统中的流转过程。

**模板：**
```markdown
# 数据流程

## 1. 任务处理流程

```
┌─────────────┐
│ 管理系统     │
│ 创建任务     │
└──────┬──────┘
       │ 1. 发送任务消息
       ↓
┌─────────────┐
│ RabbitMQ    │
│ 任务队列     │
└──────┬──────┘
       │ 2. 消费任务
       ↓
┌─────────────┐
│ TaskFetcher │
│ 任务获取器   │
└──────┬──────┘
       │ 3. 分发任务
       ↓
┌─────────────┐
│ Platform    │
│ Processor   │
└──────┬──────┘
       │ 4. 爬取数据
       ↓
┌─────────────┐
│ Crawler     │
│ 爬虫引擎     │
└──────┬──────┘
       │ 5. 返回数据
       ↓
┌─────────────┐
│ Data        │
│ Validator   │
└──────┬──────┘
       │ 6. 验证通过
       ↓
┌─────────────┐
│ Management  │
│ API         │
└─────────────┘
```

## 2. 产品同步流程

详细描述产品数据同步的完整流程...

## 3. 库存监控流程

详细描述库存监控的数据流...

## 4. 自动核价流程

详细描述自动核价的数据流...
```

### 3. 组件设计（component-design.md）

详细说明各个组件的设计。

**模板：**
```markdown
# 组件设计

## 1. 任务调度器（Scheduler）

### 1.1 职责

- 管理定时任务
- 处理任务依赖
- 监控任务执行

### 1.2 接口设计

```go
type Scheduler interface {
    RegisterTask(task Task) error
    StartTask(taskID string) error
    StopTask(taskID string) error
    GetTaskStatus(taskID string) TaskStatus
}
```

### 1.3 实现细节

- 使用 Goroutine 池执行任务
- 基于优先级队列调度
- 支持任务依赖图

### 1.4 配置示例

```yaml
scheduler:
  max_workers: 10
  task_timeout: 300s
  retry_times: 3
```

## 2. 爬虫引擎（Crawler）

### 2.1 职责

- 管理浏览器实例
- 执行页面爬取
- 处理反爬虫

### 2.2 架构

```
Crawler
├── BrowserPool
│   ├── Browser Instance 1
│   ├── Browser Instance 2
│   └── Browser Instance N
├── PageOperator
│   ├── Navigation
│   ├── Interaction
│   └── Extraction
└── AntiBot
    ├── CaptchaHandler
    └── HumanBehavior
```

## 3. 数据验证器（Validator）

### 3.1 验证规则

- 敏感词检查
- 禁止项检查
- 数据完整性验证
- 格式验证

### 3.2 实现

```go
type Validator interface {
    Validate(data interface{}) error
}

type ProductValidator struct {
    sensitiveWordChecker *SensitiveWordChecker
    prohibitedItemChecker *ProhibitedItemChecker
}
```
```

### 4. 消息队列设计（message-queue.md）

RabbitMQ 的使用设计。

**模板：**
```markdown
# 消息队列设计

## 1. 队列结构

```
Exchange: task.exchange (topic)
├── Queue: task.temu
│   └── Routing Key: task.temu.#
├── Queue: task.shein
│   └── Routing Key: task.shein.#
└── Queue: task.dlq
    └── Routing Key: task.dlq.#
```

## 2. 消息格式

```json
{
  "task_id": 12345,
  "platform": "temu",
  "action": "product_sync",
  "payload": {
    "product_id": "abc123",
    "store_id": 1
  },
  "priority": 5,
  "created_at": "2024-01-01T00:00:00Z"
}
```

## 3. 消费策略

- 预取数量：10
- 确认模式：手动确认
- 重试次数：3
- 死信队列：task.dlq

## 4. 性能优化

- 连接池复用
- 批量确认
- 消息持久化
- 优先级队列
```

## 文档编写规范

### 1. 使用图表

- 使用 Mermaid 或 PlantUML 绘制图表
- 提供 PNG/SVG 格式的图片
- 保持图表简洁清晰

### 2. 代码示例

- 提供关键接口的代码示例
- 使用实际的代码片段
- 添加必要的注释

### 3. 版本管理

- 记录架构变更历史
- 说明变更原因
- 提供迁移指南

## 工具推荐

### 1. 绘图工具

- **Draw.io** - 在线绘图工具
- **PlantUML** - 代码生成 UML 图
- **Mermaid** - Markdown 中的图表

### 2. 文档生成

- **Swagger** - API 文档
- **GoDoc** - Go 代码文档
- **MkDocs** - 文档网站生成

## 维护指南

1. 架构变更时及时更新文档
2. 定期审查文档准确性
3. 收集团队反馈
4. 保持文档简洁实用

## 注意事项

1. 架构文档应该面向开发者
2. 使用清晰的图表和示例
3. 说明设计决策的原因
4. 记录已知的限制和权衡
5. 提供相关资源的链接
