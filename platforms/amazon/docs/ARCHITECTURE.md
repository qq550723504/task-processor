# Amazon 平台模块架构设计

## 架构概览

Amazon 平台模块采用分层架构设计，遵循单一职责原则和依赖倒置原则。

```
┌─────────────────────────────────────────────────────────┐
│                    Processor Layer                       │
│  (processor.go - 主处理器，协调各组件)                    │
└────────────────────┬────────────────────────────────────┘
                     │
        ┌────────────┼────────────┐
        │            │            │
        ▼            ▼            ▼
┌──────────┐  ┌──────────┐  ┌──────────┐
│ Pipeline │  │  Worker  │  │ Management│
│  处理管道  │  │   Pool   │  │  Client   │
└─────┬────┘  └──────────┘  └──────────┘
      │
      ▼
┌─────────────────────────────────────────┐
│          Handler Layer                   │
│  (handlers/ - 处理步骤)                  │
│  ├─ StoreInfoHandler                    │
│  ├─ ProductDataHandler                  │
│  ├─ ValidationHandler                   │
│  ├─ ListingHandler                      │
│  ├─ InventoryHandler                    │
│  └─ PricingHandler                      │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│          Service Layer                   │
│  (service/ - 业务逻辑)                   │
│  ├─ ListingService                      │
│  ├─ InventoryService                    │
│  └─ PricingService                      │
└────────────┬────────────────────────────┘
             │
             ▼
┌─────────────────────────────────────────┐
│           API Layer                      │
│  (api/ - Amazon SP-API 客户端)          │
│  ├─ Client (基础客户端)                  │
│  ├─ Listings API                        │
│  ├─ Inventory API                       │
│  └─ Pricing API                         │
└─────────────────────────────────────────┘
```

## 核心组件

### 1. Processor (处理器)

**文件**: `processor.go`

**职责**:
- 初始化和管理所有子组件
- 协调任务处理流程
- 管理生命周期（启动/关闭）

**关键方法**:
- `NewAmazonProcessor()` - 创建处理器
- `Start()` - 启动处理器
- `ProcessTask()` - 处理任务
- `Close()` - 关闭处理器

### 2. Pipeline (处理管道)

**文件**: `pipeline.go`

**职责**:
- 定义任务处理流程
- 按顺序执行处理步骤
- 传递上下文数据

**处理流程**:
```
获取店铺信息 → 获取产品数据 → 验证数据 → 
创建Listing → 设置库存 → 设置价格 → 保存结果
```

### 3. TaskHandler (任务处理器)

**文件**: `task_handler.go`

**职责**:
- 处理单个任务
- 错误处理和重试逻辑
- 更新任务状态

### 4. Handlers (处理步骤)

**目录**: `handlers/`

每个 Handler 负责一个具体的处理步骤：

- **StoreInfoHandler**: 获取和验证店铺信息
- **ProductDataHandler**: 获取产品原始数据
- **ValidationHandler**: 验证产品数据完整性
- **ListingHandler**: 创建 Amazon listing
- **InventoryHandler**: 设置产品库存
- **PricingHandler**: 设置产品价格

### 5. Services (业务服务)

**目录**: `service/`

封装业务逻辑，提供高层次的操作接口：

- **ListingService**: Listing 管理逻辑
- **InventoryService**: 库存管理逻辑
- **PricingService**: 价格管理逻辑

### 6. API Client (API 客户端)

**目录**: `api/`

封装 Amazon SP-API 调用：

- **Client**: 基础 HTTP 客户端，处理认证
- **Listings API**: 产品 listing 相关操作
- **Inventory API**: 库存相关操作
- **Pricing API**: 价格相关操作

### 7. Utils (工具类)

**目录**: `utils/`

提供通用工具方法：

- **Converter**: 数据格式转换
- **Validator**: 数据验证

## 数据流

```
Task (任务)
  ↓
TaskContext (任务上下文)
  ↓
Pipeline (处理管道)
  ↓
Handler 1 → Handler 2 → ... → Handler N
  ↓           ↓                  ↓
Service Layer (业务逻辑)
  ↓
API Layer (Amazon SP-API)
  ↓
Amazon Seller Central
```

## 错误处理策略

### 可重试错误
- API 速率限制 (Throttled)
- 服务不可用 (ServiceUnavailable)
- 内部错误 (InternalError)

### 不可重试错误
- 认证失败 (AuthenticationFailed)
- 无效参数 (InvalidInput)
- 产品不存在 (ProductNotFound)
- 分类受限 (CategoryRestricted)

## 并发模型

```
┌─────────────────────────────────────┐
│         Worker Pool                  │
│  ┌─────────┐  ┌─────────┐           │
│  │ Worker 1│  │ Worker 2│  ...      │
│  └────┬────┘  └────┬────┘           │
└───────┼────────────┼────────────────┘
        │            │
        ▼            ▼
    Task Queue (任务队列)
        ↑
        │
   Task Fetcher (任务获取器)
```

## 配置管理

配置采用分层结构：

1. **全局配置** (`config/config-dev.yaml`)
2. **平台配置** (Amazon 特定配置)
3. **运行时配置** (动态调整)

## 扩展性设计

### 添加新的处理步骤

1. 在 `handlers/` 创建新的 handler
2. 实现 `StepHandler` 接口
3. 在 `buildPipeline()` 中添加到管道

```go
type MyHandler struct{}

func (h *MyHandler) Name() string {
    return "我的处理步骤"
}

func (h *MyHandler) Handle(ctx *TaskContext) error {
    // 实现处理逻辑
    return nil
}
```

### 添加新的 API 功能

1. 在 `api/` 创建新的 API 文件
2. 在 `Client` 中添加方法
3. 在 `service/` 中封装业务逻辑

## 性能优化

1. **连接池**: HTTP 客户端使用连接池
2. **批量操作**: 支持批量更新库存和价格
3. **并发控制**: Worker Pool 控制并发数
4. **缓存**: 店铺信息等静态数据缓存
5. **限流**: API 调用速率限制

## 监控指标

- 任务处理成功率
- 任务处理耗时
- API 调用成功率
- API 调用延迟
- 错误类型分布
- Worker 池使用率

## 测试策略

1. **单元测试**: 每个组件独立测试
2. **集成测试**: API 客户端集成测试
3. **Mock 测试**: Mock Amazon API 响应
4. **端到端测试**: 完整流程测试

## 安全考虑

1. **凭证管理**: 敏感信息不记录日志
2. **访问令牌**: 定期刷新 LWA 令牌
3. **HTTPS**: 所有 API 调用使用 HTTPS
4. **输入验证**: 严格验证所有输入数据
5. **错误信息**: 不暴露敏感的错误详情
