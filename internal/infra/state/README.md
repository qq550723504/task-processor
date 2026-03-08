# State 包

## 概述

`state` 包是基础设施层的状态管理组件,提供应用运行时状态的内存存储和管理功能。这些管理器负责维护临时状态、缓存数据和队列管理,与具体业务逻辑解耦。

## 位置

```
internal/infra/state/  # 基础设施层
```

**为什么叫 state 而不是 memory?**
- 这些组件管理的是**应用状态**,而不是内存本身
- 包含 Cookie、暂停状态、计数、队列等业务状态
- 虽然当前实现使用内存存储,但未来可能扩展到 Redis 等持久化存储
- 命名更准确地反映了包的职责

**为什么放在 infra 层?**
- 提供通用的状态管理能力
- 可以被多个应用层组件复用
- 是技术实现细节,不是业务逻辑
- 类似于 `rabbitmq`、`worker` 等基础设施组件

## 核心功能

- **Cookie 管理**: 存储和管理店铺的 Cookie 信息
- **店铺暂停管理**: 管理店铺的暂停状态和恢复时间
- **每日计数管理**: 跟踪每日上架数量
- **重新上架队列**: 管理需要重新上架的任务队列
- **统一管理**: 通过 MemoryManager 统一管理所有状态

## 核心组件

### 1. 统一管理器 (manager.go)

`MemoryManager` 是所有状态管理器的统一入口。

#### 功能特性

- 统一初始化所有子管理器
- 提供统一的访问接口
- 收集统计信息

#### 使用示例

```go
// 创建统一管理器
memoryManager := state.NewMemoryManager(ctx, managementClientMgr)

// 访问各个子管理器
cookieManager := memoryManager.CookieManager
shopPauseManager := memoryManager.ShopPauseManager
dailyCountManager := memoryManager.DailyCountManager
reListingQueue := memoryManager.ReListingQueue

// 获取统计信息
stats := memoryManager.GetStats()
fmt.Printf("Cookies数量: %d\n", stats["cookies_count"])
fmt.Printf("暂停店铺数量: %d\n", stats["paused_shops_count"])
```

### 2. Cookie 管理器 (cookie_manager.go)

`CookieManager` 管理店铺的 Cookie 信息。

#### 功能特性

- 存储店铺 Cookie
- 自动记录更新时间
- 线程安全操作
- 支持批量查询

#### 使用示例

```go
cookieManager := state.NewCookieManager()

// 设置 Cookie
cookieManager.SetCookie(12345, "session_id=abc123; token=xyz789")

// 获取 Cookie
cookie, err := cookieManager.GetCookie(12345)
if err != nil {
    log.Printf("Cookie不存在: %v", err)
}

// 删除 Cookie
cookieManager.DeleteCookie(12345)

// 获取所有 Cookie (调试用)
allCookies := cookieManager.GetAllCookies()
for key, info := range allCookies {
    fmt.Printf("店铺: %s, 更新时间: %s\n", key, info.UpdateTime)
}
```

#### 数据结构

```go
type CookieInfo struct {
    Cookie     string    // Cookie 字符串
    UpdateTime time.Time // 更新时间
}
```

### 3. 店铺暂停管理器 (shop_pause_manager.go)

`ShopPauseManager` 管理店铺的暂停状态。

#### 功能特性

- 支持多种暂停类型(认证过期、配额限制)
- 自动过期清理
- 与后台 API 同步状态
- 定时清理任务

#### 暂停类型

- `auth_expired`: 认证过期暂停,需要重新登录
- `quota_limit`: 配额限制暂停,到期自动恢复

#### 使用示例

```go
shopPauseManager := state.NewShopPauseManager()

// 设置店铺 API 客户端(用于同步状态到后台)
shopPauseManager.SetStoreClient(storeClient)

// 启动自动清理任务
shopPauseManager.StartCleanupTask(ctx)

// 暂停店铺指定时长
shopPauseManager.PauseShopForDuration(tenantID, shopID, "Cookie过期", 10*time.Minute)

// 暂停到当日结束
shopPauseManager.PauseShopUntilEndOfDay(tenantID, shopID, "达到每日上架限制")

// 因认证过期暂停(需要手动恢复)
shopPauseManager.PauseShopForAuthExpired(tenantID, shopID, "登录失效")

// 检查店铺是否暂停
if shopPauseManager.IsShopPaused(tenantID, shopID) {
    log.Println("店铺已暂停")
}

// 获取暂停信息
if info, exists := shopPauseManager.GetPauseInfo(tenantID, shopID); exists {
    fmt.Printf("暂停原因: %s, 恢复时间: %s\n", info.Reason, info.ResumeAt)
}

// 手动恢复店铺
shopPauseManager.ResumeShop(tenantID, shopID)
```

#### 数据结构

```go
type ShopPauseInfo struct {
    Reason    string        // 暂停原因
    PausedAt  time.Time     // 暂停时间
    ResumeAt  time.Time     // 恢复时间
    IsPaused  bool          // 是否暂停
    PauseType string        // 暂停类型
}
```

#### 自动清理机制

- 每分钟检查一次过期的暂停记录
- 只自动清理 `quota_limit` 类型的暂停
- `auth_expired` 类型需要等待登录成功后手动恢复
- 清理时会同步更新后台 API 状态

### 4. 每日计数管理器 (daily_count_manager.go)

`DailyCountManager` 管理每日上架数量统计。

#### 功能特性

- 基于后台 API 存储
- 支持增量更新
- 支持重置计数
- 线程安全

#### 使用示例

```go
dailyCountManager := state.NewDailyCountManager(managementClientMgr)

// 增加计数
date := time.Now().Format("2006-01-02")
newCount := dailyCountManager.IncrementCount(tenantID, shopID, date, 1)
fmt.Printf("今日已上架: %d\n", newCount)

// 获取当前计数
count := dailyCountManager.GetCount(tenantID, shopID, date)
fmt.Printf("当前计数: %d\n", count)

// 重置计数(通常在新的一天开始时)
dailyCountManager.ResetCount(tenantID, shopID, date)
```

#### 注意事项

- 数据存储在后台 API,不是本地内存
- 需要确保 ManagementClient 已正确初始化
- 如果 API 调用失败,会返回默认值并记录错误日志

### 5. 重新上架队列管理器 (relisting_queue_manager.go)

`ReListingQueueManager` 管理需要重新上架的任务队列。

#### 功能特性

- FIFO 队列(先进先出)
- 按店铺分组
- 线程安全
- 支持批量操作

#### 使用示例

```go
reListingQueue := state.NewReListingQueueManager()

// 添加任务到队列
taskData := `{"product_id": 123, "action": "relist"}`
reListingQueue.PushTask(tenantID, shopID, taskData)

// 从队列取出任务
taskData, err := reListingQueue.PopTask(tenantID, shopID)
if err != nil {
    log.Printf("队列为空: %v", err)
}

// 获取队列长度
length := reListingQueue.GetQueueLength(tenantID, shopID)
fmt.Printf("队列长度: %d\n", length)

// 清空队列
reListingQueue.ClearQueue(tenantID, shopID)

// 获取所有队列键
keys := reListingQueue.GetAllKeys()
for _, key := range keys {
    fmt.Printf("队列: %s\n", key)
}
```

#### 队列特性

- 从头部添加(`PushTask`),从尾部取出(`PopTask`)
- 队列为空时自动删除键,节省内存
- 支持多租户、多店铺隔离

## 架构设计

```
┌─────────────────────────────────────────────────────────┐
│                    State Package                         │
├─────────────────────────────────────────────────────────┤
│                                                          │
│  ┌──────────────────────────────────────────────────┐  │
│  │            MemoryManager (统一入口)               │  │
│  └──────────────────────────────────────────────────┘  │
│         │           │           │           │           │
│         ▼           ▼           ▼           ▼           │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐  │
│  │  Cookie  │ │   Shop   │ │  Daily   │ │ ReListing│  │
│  │ Manager  │ │  Pause   │ │  Count   │ │  Queue   │  │
│  │          │ │ Manager  │ │ Manager  │ │ Manager  │  │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘  │
│       │             │             │             │       │
│       ▼             ▼             ▼             ▼       │
│  ┌────────────────────────────────────────────────┐    │
│  │           内存存储 / API 存储                   │    │
│  └────────────────────────────────────────────────┘    │
│                                                          │
└─────────────────────────────────────────────────────────┘
```

## 与其他包的关系

### 依赖关系
- `internal/pkg/management` - 后台 API 客户端
- `github.com/sirupsen/logrus` - 日志记录

### 被依赖关系
- `internal/app/processor` - 任务处理器
- `internal/pipeline` - 任务管道
- `internal/platforms/temu` - TEMU 平台
- `internal/platforms/shein` - SHEIN 平台
- `internal/platforms/amazon` - Amazon 平台

## 使用场景

### 场景 1: 店铺 Cookie 管理

```go
// 登录成功后保存 Cookie
cookieManager.SetCookie(shopID, newCookie)

// 发起 API 请求前获取 Cookie
cookie, err := cookieManager.GetCookie(shopID)
if err != nil {
    // Cookie 不存在,需要重新登录
    return errors.New("需要重新登录")
}

// 登出时删除 Cookie
cookieManager.DeleteCookie(shopID)
```

### 场景 2: 配额限制处理

```go
// 检查每日上架数量
date := time.Now().Format("2006-01-02")
currentCount := dailyCountManager.GetCount(tenantID, shopID, date)

if currentCount >= dailyLimit {
    // 达到限制,暂停到当日结束
    shopPauseManager.PauseShopUntilEndOfDay(tenantID, shopID, "达到每日上架限制")
    return errors.New("已达到每日上架限制")
}

// 上架成功后增加计数
dailyCountManager.IncrementCount(tenantID, shopID, date, 1)
```

### 场景 3: 认证失效处理

```go
// API 返回认证失效错误
if isAuthError(err) {
    // 暂停店铺
    shopPauseManager.PauseShopForAuthExpired(tenantID, shopID, "登录失效")
    
    // 删除旧 Cookie
    cookieManager.DeleteCookie(shopID)
    
    // 通知用户重新登录
    notifyUserToLogin(shopID)
}

// 重新登录成功后
if loginSuccess {
    // 保存新 Cookie
    cookieManager.SetCookie(shopID, newCookie)
    
    // 恢复店铺
    shopPauseManager.ResumeShop(tenantID, shopID)
}
```

### 场景 4: 任务重试队列

```go
// 任务失败时加入重试队列
if err := processTask(task); err != nil {
    taskData := serializeTask(task)
    reListingQueue.PushTask(tenantID, shopID, taskData)
    log.Printf("任务已加入重试队列: %s", taskData)
}

// 定期处理重试队列
func processRetryQueue() {
    for _, key := range reListingQueue.GetAllKeys() {
        tenantID, shopID := parseKey(key)
        
        // 检查店铺是否暂停
        if shopPauseManager.IsShopPaused(tenantID, shopID) {
            continue
        }
        
        // 取出任务重试
        if taskData, err := reListingQueue.PopTask(tenantID, shopID); err == nil {
            retryTask(taskData)
        }
    }
}
```

## 最佳实践

### 1. 统一使用 MemoryManager

```go
// 推荐: 通过统一管理器访问
memoryManager := state.NewMemoryManager(ctx, managementClient)
cookieManager := memoryManager.CookieManager

// 不推荐: 直接创建各个管理器
cookieManager := state.NewCookieManager()  // 缺少统一管理
```

### 2. 设置 StoreClient

```go
// 创建后立即设置 StoreClient,确保状态同步
shopPauseManager := memoryManager.ShopPauseManager
shopPauseManager.SetStoreClient(storeClient)
```

### 3. 启动清理任务

```go
// 在应用启动时启动清理任务
memoryManager := state.NewMemoryManager(ctx, managementClient)
// 清理任务已在 NewMemoryManager 中自动启动
```

### 4. 错误处理

```go
// 获取 Cookie 时处理错误
cookie, err := cookieManager.GetCookie(shopID)
if err != nil {
    // 记录日志
    log.Warnf("获取Cookie失败: %v", err)
    
    // 触发重新登录流程
    triggerRelogin(shopID)
    return
}
```

### 5. 并发安全

所有管理器都是并发安全的,可以在多个 goroutine 中使用:

```go
// 多个 goroutine 同时操作是安全的
go func() {
    cookieManager.SetCookie(shopID1, cookie1)
}()

go func() {
    cookieManager.SetCookie(shopID2, cookie2)
}()
```

## 注意事项

1. **内存占用**: 所有数据存储在内存中(除 DailyCountManager),应用重启后数据丢失
2. **数据持久化**: 如需持久化,考虑扩展为 Redis 实现
3. **清理机制**: ShopPauseManager 会自动清理过期记录,无需手动清理
4. **API 依赖**: DailyCountManager 依赖后台 API,确保 API 可用
5. **暂停类型**: 注意区分 `auth_expired` 和 `quota_limit` 的不同处理方式

## 未来改进

- [ ] 支持 Redis 作为存储后端
- [ ] 添加数据持久化选项
- [ ] 支持分布式部署
- [ ] 添加数据过期策略
- [ ] 支持更多队列类型(优先级队列、延迟队列)
- [ ] 添加监控指标导出
- [ ] 支持数据备份和恢复

## 迁移说明

### 从 `memory` 包迁移到 `state` 包

如果你的代码还在使用旧的 `internal/infra/memory` 包,请按以下步骤迁移:

1. 更新 import 语句:
```go
// 旧的
import "task-processor/internal/infra/memory"

// 新的
import "task-processor/internal/infra/state"
```

2. 更新类型引用:
```go
// 旧的
var manager *memory.MemoryManager

// 新的
var manager *state.MemoryManager
```

3. 功能完全兼容,无需修改业务逻辑

## 相关文档

- [Worker 包文档](../worker/README.md)
- [Monitoring 包文档](../monitoring/README.md)
- [Management API 文档](../../pkg/management/README.md)
