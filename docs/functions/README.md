# Task Processor 完整函数清单

生成时间: 2026-03-10

## 📚 文档索引

本目录包含项目所有模块的完整函数清单，按模块分类组织。

### 文档列表

1. **[01-app-messaging.md](01-app-messaging.md)** - App/Messaging 模块
   - RabbitMQ 服务管理
   - 任务处理器和提交器
   - 结果上报器
   - 平台和爬虫注册器
   - 队列配置和初始化
   - Bootstrap 启动模块

2. **[02-domain.md](02-domain.md)** - Domain 领域模块
   - Task 领域模型（已增强）
   - 领域错误处理（新增）
   - 消息类型定义（新增）
   - 队列命名服务（新增）
   - 产品领域服务

3. **[03-application.md](03-application.md)** - Application 应用服务层
   - 分布式爬虫客户端
   - 分布式产品获取器

4. **[04-infra-rabbitmq.md](04-infra-rabbitmq.md)** - Infra/RabbitMQ 基础设施
   - RabbitMQ 客户端
   - 连接管理
   - 消费者和发布者
   - 负载监控
   - 错误收集

5. **[05-infra-auth.md](05-infra-auth.md)** - Infra/Auth 认证模块
   - 客户端凭证认证
   - Token 管理
   - Session 管理
   - Token 存储

6. **[06-crawler-amazon.md](06-crawler-amazon.md)** - Crawler/Amazon 爬虫
   - 浏览器池管理
   - 页面处理器
   - 数据提取器
   - 错误检测
   - 健康检查

7. **[07-crawler-alibaba.md](07-crawler-alibaba.md)** - Crawler/Alibaba1688 爬虫
   - 验证码处理
   - 数据提取器
   - 页面处理器
   - 供应商信息提取

8. **[08-platforms-amazon.md](08-platforms-amazon.md)** - Platforms/Amazon 平台
   - Amazon API 客户端
   - 认证管理
   - Listing 操作
   - 产品映射
   - 订单管理

9. **[09-pipeline.md](09-pipeline.md)** - Pipeline 管道模式
   - 管道实现
   - 处理器（Handler）
   - 上下文管理
   - 错误处理

10. **[10-core-config.md](10-core-config.md)** - Core/Config 核心配置
    - 配置加载和验证
    - 默认值应用
    - 日志管理
    - 生命周期管理
    - 错误定义

11. **[11-pkg-utils.md](11-pkg-utils.md)** - Pkg/Utils 工具包
    - 图片下载和处理
    - 数学工具
    - 字符串工具
    - 价格计算
    - 重试和超时

12. **[12-infra-clients.md](12-infra-clients.md)** - Infra/Clients 客户端
    - OpenAI 客户端
    - HTTP 客户端
    - 分布式锁
    - 监控和健康检查
    - 仓储层
    - 依赖注入

## 📊 统计信息

### 模块统计
- **总模块数**: 12 个主要模块
- **总文件数**: 500+ Go 文件
- **总函数数**: 2000+ 函数

### 模块分类

#### 应用层 (App Layer)
- `app/messaging` - 消息处理
- `app/bootstrap` - 应用启动

#### 应用服务层 (Application Layer)
- `application/crawler` - 爬虫客户端
- `application/product` - 产品服务

#### 领域层 (Domain Layer) ⭐ 已增强
- `domain/model` - 领域模型
- `domain/errors` - 领域错误
- `domain/message` - 消息类型
- `domain/queue` - 队列服务
- `domain/product` - 产品领域

#### 基础设施层 (Infrastructure Layer)
- `infra/rabbitmq` - RabbitMQ
- `infra/auth` - 认证
- `infra/clients` - 客户端
- `infra/http` - HTTP 客户端
- `infra/lock` - 分布式锁
- `infra/monitoring` - 监控
- `infra/repo` - 仓储
- `infra/di` - 依赖注入

#### 爬虫层 (Crawler Layer)
- `crawler/amazon` - Amazon 爬虫
- `crawler/alibaba1688` - 1688 爬虫

#### 平台层 (Platform Layer)
- `platforms/amazon` - Amazon 平台
- `platforms/temu` - TEMU 平台
- `platforms/shein` - SHEIN 平台

#### 核心层 (Core Layer)
- `core/config` - 配置管理
- `core/logger` - 日志
- `core/errors` - 错误
- `core/lifecycle` - 生命周期

#### 工具层 (Utility Layer)
- `pkg/amazon` - Amazon 工具
- `pkg/downloader` - 下载器
- `pkg/mathutil` - 数学工具
- `pkg/strutil` - 字符串工具
- `pkg/pricing` - 定价工具
- `pkg/types` - 类型定义
- `pkg/utils` - 通用工具

#### 管道层 (Pipeline Layer)
- `pipeline` - 管道模式实现

## 🎯 使用指南

### 如何使用这些文档

1. **查找函数**: 根据模块名称找到对应的文档文件
2. **了解接口**: 查看函数签名和参数
3. **重构参考**: 识别重复代码和优化机会
4. **架构理解**: 了解模块间的依赖关系

### 重构建议

根据函数清单，以下是主要的重构机会：

#### 🔴 高优先级
1. **拆分 ServiceManager** (app/messaging)
   - 职责过多，建议拆分为多个专职服务
   
2. **拆分 RabbitMQService** (app/messaging)
   - 职责过多，建议使用门面模式

#### 🟡 中优先级
3. **统一平台处理器接口** (platforms)
   - 提取公共逻辑到基类
   
4. **优化爬虫模块** (crawler)
   - 提取公共数据提取逻辑

#### 🟢 低优先级
5. **添加单元测试**
   - 为重构后的代码添加测试
   
6. **性能优化**
   - 使用 pprof 分析热点

## 📝 维护说明

### 更新频率
- 每次重大重构后更新
- 添加新模块时更新
- 函数签名变更时更新

### 更新方法
```bash
# 使用 readCode 工具获取最新函数签名
# 更新对应的文档文件
# 提交到 Git
```

## 🔗 相关文档

- [项目结构文档](../project-structure.md)
- [代码清单文档](../code-inventory.md)
- [重构总结文档](../refactoring-summary.md)

## 📌 注意事项

1. 本文档仅包含函数签名，不包含实现细节
2. 标记 ⭐ 的模块表示已完成重构
3. 标记 🆕 的模块表示新增模块
4. 函数签名可能随版本更新而变化

---

**最后更新**: 2026-03-10
**维护者**: Task Processor Team
