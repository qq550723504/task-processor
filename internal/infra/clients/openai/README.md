# OpenAI 客户端架构说明

## 📦 文件结构

```
openai/
├── README.md              # 本文档
├── types.go               # 类型定义
├── client.go              # 基础客户端
├── pool.go                # 请求池(并发+速率+负载均衡)
├── cached_client.go       # 缓存装饰器
├── resilient_client.go    # 弹性装饰器(熔断器)
└── client_manager.go      # 客户端管理器
```

## 🏗️ 分层架构

### 调用链路

```
ClientManager (管理多个客户端)
    ↓
ResilientClient (熔断保护)
    ↓
CachedClient (缓存响应)
    ↓
Client (基础客户端)
    ↓
RequestPool (并发控制 + 速率限制 + 负载均衡)
    ↓
BaseClient (单次API调用 + 基础重试)
    ↓
OpenAI API
```

### 职责分配

| 组件 | 职责 | 位置 |
|------|------|------|
| BaseClient | 单次API调用 + 网络错误重试 | pool.go |
| RequestPool | 并发控制、速率限制、负载均衡 | pool.go |
| Client | 简单封装,提供统一接口 | client.go |
| CachedClient | 缓存响应,减少API调用 | cached_client.go |
| ResilientClient | 熔断保护,防止服务雪崩 | resilient_client.go |
| ClientManager | 管理多个客户端配置 | client_manager.go |

## ⚠️ 重要说明

### 重试逻辑
- BaseClient 已实现基础重试(MaxRetries配置)
- ResilientClient 只提供熔断保护,不额外重试
- 避免双重重试导致的请求放大

### 超时控制
- 使用 Client.CreateChatCompletionWithTimeout() 设置自定义超时
- 或在context中设置超时: context.WithTimeout()

### 缓存策略
- 缓存键基于请求参数的SHA256哈希
- 相同的model、messages、temperature等参数会命中缓存

### 熔断器状态
- Closed: 正常状态,请求正常通过
- Open: 熔断状态,直接拒绝请求
- Half-Open: 尝试恢复,允许少量请求测试
