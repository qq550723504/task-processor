# 项目文档索引

## 📚 文档概览

本目录包含项目的所有技术文档，包括架构设计、API 文档、开发指南等。

## 🏗️ 架构文档

### 重构相关

- **[重构状态](./REFACTORING_STATUS.md)** - 项目重构进度和状态总览
- **[RabbitMQ 重构完成报告](./RABBITMQ_REFACTORING_COMPLETED.md)** - RabbitMQ 包重构的详细报告
- **[RabbitMQ 重构建议](./REFACTORING_RABBITMQ.md)** - 原始重构建议和分析
- **[新架构快速入门](./QUICK_START_NEW_STRUCTURE.md)** - 如何使用重构后的新架构

### 迁移指南

- **[迁移指南](./MIGRATION_GUIDE.md)** - Bootstrap 初始化功能迁移指南

## 📖 API 文档

- **[API 文档目录](./api/README.md)** - REST API 文档和使用指南
  - OpenAPI 规范
  - Postman 集合
  - 代码示例

## 🏛️ 架构设计

- **[架构文档目录](./architecture/README.md)** - 系统架构设计文档
  - 架构概览
  - 数据流程图
  - 组件设计
  - 消息队列设计

## 💻 开发文档

- **[开发文档目录](./development/README.md)** - 开发指南和规范
  - 环境搭建
  - 编码规范
  - Git 工作流
  - 测试指南

## 🎯 快速导航

### 我想了解...

#### 新架构如何使用？
👉 [新架构快速入门](./QUICK_START_NEW_STRUCTURE.md)

#### 项目重构进度？
👉 [重构状态](./REFACTORING_STATUS.md)

#### RabbitMQ 如何使用？
👉 [RabbitMQ 文档](../internal/infra/rabbitmq/README.md)

#### 如何提交任务？
👉 [任务提交服务文档](../internal/app/messaging/README.md)

#### 业务规则在哪里？
👉 [领域层文档](../internal/domain/task/README.md)

#### 如何搭建开发环境？
👉 [开发文档](./development/README.md)

#### API 如何调用？
👉 [API 文档](./api/README.md)

## 📂 项目结构

```
task-processor/
├── cmd/                    # 应用入口
├── internal/               # 内部代码
│   ├── app/               # 应用层
│   │   ├── messaging/     # 消息处理服务
│   │   └── worker/        # 工作器
│   ├── domain/            # 领域层
│   │   ├── model/         # 领域模型
│   │   └── task/          # 任务领域逻辑
│   ├── infra/             # 基础设施层
│   │   ├── rabbitmq/      # RabbitMQ 客户端
│   │   ├── database/      # 数据库访问
│   │   └── http/          # HTTP 客户端
│   └── core/              # 核心组件
│       ├── config/        # 配置管理
│       ├── logger/        # 日志管理
│       └── system/        # 系统初始化
├── docs/                  # 文档（当前目录）
├── tests/                 # 测试
└── scripts/               # 脚本
```

## 🔄 架构分层

```
┌─────────────────────────────────────┐
│         应用层 (app/)                │
│  - 流程编排                          │
│  - 服务协调                          │
│  - 事务管理                          │
└─────────────────────────────────────┘
           ↓ 使用
┌─────────────────────────────────────┐
│         领域层 (domain/)             │
│  - 业务规则                          │
│  - 领域逻辑                          │
│  - 领域模型                          │
└─────────────────────────────────────┘
           ↓ 使用
┌─────────────────────────────────────┐
│      基础设施层 (infra/)             │
│  - 数据库访问                        │
│  - 消息队列                          │
│  - 外部服务                          │
└─────────────────────────────────────┘
```

## 📝 最近更新

### 2024-01 RabbitMQ 包重构完成

- ✅ 创建领域层 `domain/task`
- ✅ 创建应用层 `app/messaging`
- ✅ 精简基础设施层 `infra/rabbitmq`
- ✅ 提供完整文档和迁移指南

详见：[RabbitMQ 重构完成报告](./RABBITMQ_REFACTORING_COMPLETED.md)

### 2024-01 Bootstrap 初始化重构完成

- ✅ 解决初始化功能重复问题
- ✅ 明确职责划分
- ✅ 提供迁移指南

详见：[迁移指南](./MIGRATION_GUIDE.md)

### 2024-01 文档体系建立

- ✅ API 文档模板
- ✅ 架构文档模板
- ✅ 开发文档模板
- ✅ 测试模板

## 🎓 学习路径

### 新手入门

1. 阅读 [项目 README](../README.md)
2. 阅读 [开发环境搭建](./development/README.md)
3. 阅读 [新架构快速入门](./QUICK_START_NEW_STRUCTURE.md)
4. 查看代码示例

### 深入理解

1. 阅读 [架构文档](./architecture/README.md)
2. 阅读 [重构状态](./REFACTORING_STATUS.md)
3. 阅读各层的 README 文档
4. 查看测试代码

### 贡献代码

1. 阅读 [编码规范](./development/README.md)
2. 阅读 [Git 工作流](./development/README.md)
3. 阅读 [测试指南](./development/README.md)
4. 提交 Pull Request

## 🔗 相关链接

- [项目主页](../README.md)
- [领域层文档](../internal/domain/task/README.md)
- [应用层文档](../internal/app/messaging/README.md)
- [基础设施层文档](../internal/infra/rabbitmq/README.md)

## 📮 反馈

如果你发现文档有任何问题或建议，请：

1. 提交 Issue
2. 提交 Pull Request
3. 联系团队成员

---

**最后更新：** 2024-01  
**维护者：** 开发团队
