# infra 目录

## 用途

基础设施层，提供技术实现细节，如数据库访问、消息队列、HTTP 客户端、认证、缓存等。实现 domain 层定义的接口。

## 重要说明

**2024年重构：** 以下模块已从 infra 层移出或优化，确保 infra 层只包含纯技术实现：
- `state/` → 已移至 `internal/application/state/` （业务状态管理）
- `crawler/` → 已移至 `internal/application/crawler/` （应用服务）
- `bootstrap/` → 已移至 `internal/app/bootstrap/` （应用启动逻辑）
- `worker/` → ✅ 已优化（引入 Job 接口，业务字段移到 domain 层）

## 目录结构

```
infra/
├── auth/        # 认证授权
├── clients/     # 外部客户端
├── di/          # 依赖注入容器
├── http/        # HTTP 相关
├── lock/        # 分布式锁
├── monitoring/  # 监控指标
├── rabbitmq/    # RabbitMQ 消息队列
├── repo/        # 数据仓储实现
└── worker/      # 工作池（✅ 已优化）
```

## 子目录说明

### auth（认证授权）
- 用户认证
- Token 管理
- 会话管理
- 权限验证

**应该放置的文件：**
- `manager.go` - 认证管理器
- `client_credentials.go` - 客户端凭证认证
- `token_store.go` - Token 存储
- `session.go` - 会话管理
- `interfaces.go` - 认证接口定义

### clients（外部客户端）
- 第三方 API 客户端
- HTTP 客户端封装
- API 调用封装

**应该放置的文件：**
- `openai/` - OpenAI 客户端
- `http_client.go` - 通用 HTTP 客户端

### di（依赖注入）
- 依赖注入容器
- 服务注册和解析
- 生命周期管理

**应该放置的文件：**
- `container.go` - DI 容器实现
- `interfaces.go` - DI 接口定义
- `registry.go` - 服务注册表
- `cache.go` - 实例缓存

### http（HTTP 相关）
- HTTP 中间件
- 请求/响应处理
- 路由管理

**应该放置的文件：**
- `middleware/` - HTTP 中间件
- `handler.go` - HTTP 处理器
- `router.go` - 路由配置

### lock（分布式锁）
- 分布式锁实现
- 内存锁实现
- 锁管理器

**应该放置的文件：**
- `distributed_lock.go` - 分布式锁
- `memory_lock.go` - 内存锁
- `interfaces.go` - 锁接口定义

### monitoring（监控指标）
- 性能指标收集
- 健康检查
- 进程信息
- 指标上报

**应该放置的文件：**
- `collector.go` - 指标收集器
- `health_checker.go` - 健康检查
- `process_info.go` - 进程信息
- `types.go` - 监控类型定义

### rabbitmq（消息队列）
- RabbitMQ 连接管理
- 消息生产者
- 消息消费者
- 任务适配器

**应该放置的文件：**
- `client.go` - RabbitMQ 客户端
- `connection.go` - 连接管理
- `consumer.go` - 消费者
- `task_submitter.go` - 任务提交器
- `task_handler.go` - 任务处理器
- `service_manager.go` - 服务管理器
- `config.go` - 配置定义

### repo（数据仓储）
- 实现 domain 层定义的仓储接口
- 数据持久化
- 数据查询

**应该放置的文件：**
- `file_repo.go` - 文件仓储
- `product_repo.go` - 产品仓储（如果需要）
- `task_repo.go` - 任务仓储（如果需要）

## 编码规范

1. 实现 domain 层定义的接口
2. 封装技术细节，不暴露给上层
3. 使用依赖注入管理依赖关系
4. 提供清晰的错误处理
5. 考虑性能和资源管理

## 示例代码

### 仓储实现示例

```go
// repo/product_repo.go
package repo

import (
    "task-processor/internal/domain/product"
)

type ProductRepository struct {
    db *sql.DB
}

func NewProductRepository(db *sql.DB) product.ProductRepository {
    return &ProductRepository{db: db}
}

func (r *ProductRepository) Save(p *product.Product) error {
    // 数据库操作
    return nil
}

func (r *ProductRepository) FindByID(id string) (*product.Product, error) {
    // 数据库查询
    return nil, nil
}
```

### 认证客户端示例

```go
// auth/client_credentials.go
package auth

type ClientCredentialsAuthClient struct {
    baseURL      string
    clientID     string
    clientSecret string
    tokenStore   TokenStore
}

func (c *ClientCredentialsAuthClient) GetAccessToken() (string, error) {
    // 获取或刷新 token
    return "", nil
}
```

## 注意事项

- 基础设施层不包含业务逻辑
- 技术实现细节不应该泄露到上层
- 使用接口隔离不同的实现
- 考虑可测试性，提供 mock 实现
- 注意资源的正确释放
