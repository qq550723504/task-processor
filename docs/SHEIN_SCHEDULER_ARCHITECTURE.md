# SHEIN 定时任务调度架构

## 概述

SHEIN 平台定时任务调度架构已经搭建完成,包含 4 个核心定时任务,每个任务都有清晰的调用链路和职责划分。架构采用三层设计:

- **Scheduler 层**: 任务调度和执行控制
- **Service 层**: 业务逻辑处理
- **Repo 层**: 数据访问和API调用

## 任务列表

### 1. 产品同步任务 (SyncTask)

**功能**: 定时从 SHEIN 平台同步产品信息到管理系统

**执行流程**:
```
Scheduler: sync_task.go
    ↓
Service: ProductSyncService
    ├─ FetchProductList()      - 从SHEIN API获取产品列表
    ├─ ConvertProducts()       - 转换为后端格式
    └─ SaveProducts()          - 批量保存到管理系统
```

**文件**:
- Scheduler: `internal/platforms/shein/scheduler/sync_task.go`
- Service: `internal/platforms/shein/service/product_sync_service.go`

---

### 2. 自动核价任务 (PricingTask)

**功能**: 定时处理待核价产品,根据规则自动接受/拒绝/重新报价

**执行流程**:
```
Scheduler: pricing_task.go
    ↓
Service: AutoPricingService
    ├─ FetchPendingPriceProducts()  - 获取待核价产品列表
    ├─ ApplyPricingRules()          - 应用核价规则
    └─ SubmitPricingResults()       - 提交核价结果
```

**文件**:
- Scheduler: `internal/platforms/shein/scheduler/pricing_task.go`
- Service: `internal/platforms/shein/service/auto_pricing_service.go`

---

### 3. 库存同步任务 (InventoryTask)

**功能**: 定时同步产品库存信息

**执行流程**:
```
Scheduler: inventory_task.go
    ↓
Service: InventorySyncService
    ├─ FetchProductsForInventorySync()  - 获取需要同步库存的产品列表
    ├─ FetchInventoryFromShein()        - 从SHEIN API获取最新库存信息
    └─ UpdateInventoryToManagement()    - 更新到管理系统
```

**文件**:
- Scheduler: `internal/platforms/shein/scheduler/inventory_task.go`
- Service: `internal/platforms/shein/service/inventory_sync_service.go`

---

### 4. 活动报名任务 (ActivityTask)

**功能**: 定时自动报名符合条件的产品到平台活动

**执行流程**:
```
Scheduler: activity_task.go
    ↓
Service: ActivityRegistrationService
    ├─ FetchAvailableActivities()           - 获取可报名的活动列表
    ├─ FetchEligibleProducts()              - 获取符合条件的产品
    ├─ RegisterActivities()                 - 自动报名活动
    └─ SyncActivityProductsToManagement()   - 同步活动产品信息到管理系统
```

**文件**:
- Scheduler: `internal/platforms/shein/scheduler/activity_task.go`
- Service: `internal/platforms/shein/service/activity_registration_service.go`

---

## 架构设计

### 三层架构

```
┌─────────────────────────────────────────┐
│         Scheduler 层                     │
│  - 任务调度和执行控制                    │
│  - 状态管理                              │
│  - 错误处理和日志                        │
└─────────────────┬───────────────────────┘
                  │ 调用
┌─────────────────▼───────────────────────┐
│         Service 层                       │
│  - 业务逻辑处理                          │
│  - 数据转换                              │
│  - 规则应用                              │
└─────────────────┬───────────────────────┘
                  │ 调用
┌─────────────────▼───────────────────────┐
│         Repo 层                          │
│  - API 调用                              │
│  - 数据库操作                            │
│  - 缓存管理                              │
└─────────────────────────────────────────┘
```

### 目录结构

```
internal/platforms/shein/
├── scheduler/              # Scheduler 层
│   ├── factory.go          # 任务工厂,负责创建各类任务
│   ├── base_task.go        # 基础任务实现,提供通用功能
│   ├── pricing_task.go     # 核价任务
│   ├── sync_task.go        # 同步任务
│   ├── inventory_task.go   # 库存任务
│   └── activity_task.go    # 活动任务
│
├── service/                # Service 层
│   ├── product_sync_service.go           # 产品同步服务
│   ├── auto_pricing_service.go           # 自动核价服务
│   ├── inventory_sync_service.go         # 库存同步服务
│   └── activity_registration_service.go  # 活动报名服务
│
└── repo/                   # Repo 层
    └── client/             # SHEIN API 客户端
```

### 任务工厂

**文件**: `factory.go`

```go
type SheinTaskFactory struct {
    managementClient *management.ClientManager
    cookieManager    *memory.CookieManager
    clientManager    *client.ClientManager
}

// CreateTask 创建任务时会:
// 1. 初始化对应的 Service
// 2. 注入到 Task 中
// 3. 返回完整的任务实例

// 支持的任务类型
- TaskTypePricing   // 核价
- TaskTypeSync      // 同步
- TaskTypeInventory // 库存
- TaskTypeActivity  // 活动
```

### 基础任务

**文件**: `base_task.go`

提供所有任务的通用功能:
- 任务ID管理
- 状态管理 (Running/Stopped/Error)
- 租户ID/店铺ID获取
- 执行间隔管理

### 任务配置

每个任务通过 `TaskConfig` 配置:

```go
type TaskConfig struct {
    TaskType  TaskType      // 任务类型
    Platform  string        // 平台名称 (SHEIN)
    TenantID  int64         // 租户ID
    StoreID   int64         // 店铺ID
    Interval  time.Duration // 执行间隔
    Enabled   bool          // 是否启用
    AutoStart bool          // 是否自动启动
}
```

## 使用示例

### 创建并启动任务

```go
// 1. 创建任务工厂
factory := scheduler.NewSheinTaskFactory(managementClient)

// 2. 注册到调度器
manager := scheduler.NewManager()
manager.RegisterFactory(factory)

// 3. 创建任务配置
config := scheduler.TaskConfig{
    TaskType:  scheduler.TaskTypePricing,
    Platform:  "SHEIN",
    TenantID:  1,
    StoreID:   100,
    Interval:  30 * time.Minute,
    Enabled:   true,
    AutoStart: true,
}

// 4. 添加任务
err := manager.AddTask(ctx, config)

// 5. 启动调度器
err = manager.Start(ctx)
```

### 停止任务

```go
// 停止特定任务
taskID := "SHEIN:pricing:1:100"
err := manager.StopTask(taskID)

// 停止所有任务
err = manager.Stop(ctx)
```

## 当前状态

### ✅ 已完成

1. **架构搭建** - 三层架构已建立
2. **Scheduler 层** - 所有任务的调度逻辑已完成
3. **Service 层** - 所有服务接口和实现已创建
4. **调用链路** - Scheduler → Service 的调用链路已打通
5. **依赖注入** - Factory 中正确初始化和注入 Service
6. **编译验证** - 所有代码可以正常编译
7. **日志记录** - 关键步骤都有日志输出

### 🔧 待实现 (标记为 TODO)

每个 Service 的具体实现逻辑:

**ProductSyncService**:
- `FetchProductList()` - 调用 SHEIN API 获取产品
- `ConvertProducts()` - 数据格式转换
- `SaveProducts()` - 保存到管理系统

**AutoPricingService**:
- `FetchPendingPriceProducts()` - 获取待核价产品
- `ApplyPricingRules()` - 应用核价规则
- `SubmitPricingResults()` - 提交核价结果

**InventorySyncService**:
- `FetchProductsForInventorySync()` - 获取需要同步的产品
- `FetchInventoryFromShein()` - 获取库存信息
- `UpdateInventoryToManagement()` - 更新库存

**ActivityRegistrationService**:
- `FetchAvailableActivities()` - 获取活动列表
- `FetchEligibleProducts()` - 筛选符合条件的产品
- `RegisterActivities()` - 报名活动
- `SyncActivityProductsToManagement()` - 同步活动产品

## 下一步

1. **实现 Service 层业务逻辑** - 逐步实现每个 TODO 标记的方法
2. **创建 Repo 层** - 封装 SHEIN API 调用和数据访问
3. **添加错误处理** - 完善异常情况的处理
4. **添加重试机制** - 对于网络请求失败的情况
5. **添加监控指标** - 记录任务执行时间、成功率等
6. **添加单元测试** - 确保每个层的正确性

## 相关文档

- [调度器架构重构](./SCHEDULER_REFACTORING.md)
- [核价服务重构](./PRICING_SERVICE_REFACTORING.md)
- [编译问题汇总](./COMPILATION_ISSUES.md)
