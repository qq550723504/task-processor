# 平台通用调度器 (Common Scheduler)

本包提供电商平台通用的调度任务基础实现，用于消除不同平台间的代码重复。

## 设计理念

采用**接口+基类**的设计模式：
- 基类提供通用的任务执行流程
- 接口定义平台特定的业务逻辑
- 各平台实现接口并组合基类

## 包含的通用任务

### 1. BaseTask - 基础任务
所有任务的基类，提供任务的基本属性和方法。

**功能：**
- 任务ID管理
- 任务状态管理
- 平台、租户、店铺信息管理
- 执行间隔管理

**使用示例：**
```go
import commonscheduler "task-processor/internal/platforms/common/scheduler"

// 直接使用通用基类
type BaseTask = commonscheduler.BaseTask

func NewBaseTask(config appscheduler.TaskConfig) *BaseTask {
    return commonscheduler.NewBaseTask(config)
}
```

### 2. ProductSyncTask - 产品同步任务
用于从平台API同步产品信息到管理系统。

**需要实现的接口：**
```go
type ProductSyncService interface {
    FetchProductList(ctx context.Context) ([]interface{}, error)
    ConvertProducts(ctx context.Context, products []interface{}, tenantID, storeID int64) ([]interface{}, error)
    SaveProducts(ctx context.Context, products []interface{}) (int, error)
}
```

**使用示例：**
```go
// 1. 实现ProductSyncService接口
type MyProductSyncService struct {
    // ... 平台特定的字段
}

func (s *MyProductSyncService) FetchProductList(ctx context.Context) ([]interface{}, error) {
    // 从平台API获取产品
}

// 2. 创建任务
task := commonscheduler.NewProductSyncTask(commonscheduler.ProductSyncTaskConfig{
    TaskConfig:       config,
    ManagementClient: managementClient,
    SyncService:      myService,
    PlatformName:     "MyPlatform",
})

// 3. 执行任务
err := task.Execute(ctx)
```

### 3. InventorySyncTask - 库存同步任务
用于监控和同步产品库存及价格变化。

**需要实现的接口：**
```go
type InventorySyncService interface {
    FetchProductsForInventorySync(ctx context.Context, tenantID, storeID int64) ([]interface{}, error)
    MonitorInventoryChanges(ctx context.Context, products []interface{}, tenantID, storeID int64) (*InventorySyncResult, error)
}
```

**返回结果：**
```go
type InventorySyncResult struct {
    TotalProducts     int // 总产品数
    ProcessedProducts int // 已处理产品数
    SkippedProducts   int // 跳过的产品数
    PriceChanges      int // 价格变化数
    StockChanges      int // 库存变化数
    AmazonFetched     int // Amazon数据获取成功数
    AmazonFailed      int // Amazon数据获取失败数
}
```

### 4. AutoPricingTask - 自动核价任务
用于半托管模式电商平台的自动核价功能。

**需要实现的接口：**
```go
type AutoPricingService interface {
    FetchPendingPriceProducts(ctx context.Context, startDate, endDate string) ([]interface{}, error)
    ApplyPricingRules(ctx context.Context, products []interface{}, storeID int64, enableRebargain bool) ([]interface{}, error)
    SubmitPricingResults(ctx context.Context, results []interface{}) (*PricingStats, error)
}
```

**核价统计：**
```go
type PricingStats struct {
    TotalProcessed int // 总处理数
    AcceptCount    int // 接受数
    RejectCount    int // 拒绝数
    ReappealCount  int // 重新议价/报价数
    SkipCount      int // 跳过数
}
```

## 迁移指南

### 从平台特定实现迁移到通用基类

#### 步骤1：保持接口兼容
```go
// 原有代码
type BaseTask struct {
    // ... 字段
}

// 迁移后（使用类型别名保持兼容）
type BaseTask = commonscheduler.BaseTask

func NewBaseTask(config appscheduler.TaskConfig) *BaseTask {
    return commonscheduler.NewBaseTask(config)
}
```

#### 步骤2：实现服务接口
将平台特定的业务逻辑提取到服务接口实现中。

#### 步骤3：组合使用
在平台特定的任务中组合使用通用基类。

## 优势

### 1. 代码复用
- 消除了约100个重复函数
- 统一的任务执行流程
- 统一的日志记录格式

### 2. 易于维护
- 修改一处，所有平台受益
- 统一的错误处理
- 统一的状态管理

### 3. 易于扩展
- 新增平台只需实现接口
- 可以轻松添加新的任务类型
- 支持平台特定的定制

### 4. 类型安全
- 使用接口定义契约
- 编译时类型检查
- 清晰的依赖关系

## 最佳实践

### 1. 接口设计
- 保持接口简单，单一职责
- 使用 `interface{}` 处理平台特定的数据类型
- 返回明确的错误信息

### 2. 日志记录
- 使用结构化日志（logrus.Fields）
- 包含关键的上下文信息
- 统一的日志级别

### 3. 错误处理
- 使用 `fmt.Errorf` 包装错误
- 提供清晰的错误上下文
- 区分可重试和不可重试错误

### 4. 测试
- 为服务接口编写单元测试
- 使用mock测试任务执行流程
- 集成测试验证端到端流程

## 示例：完整的平台集成

```go
package myplatform

import (
    "context"
    appscheduler "task-processor/internal/app/scheduler"
    "task-processor/internal/pkg/management"
    commonscheduler "task-processor/internal/platforms/common/scheduler"
)

// 1. 实现服务接口
type MyProductSyncService struct {
    apiClient APIClient
    // ... 其他依赖
}

func (s *MyProductSyncService) FetchProductList(ctx context.Context) ([]interface{}, error) {
    // 平台特定的实现
    products, err := s.apiClient.GetProducts()
    if err != nil {
        return nil, err
    }
    
    // 转换为interface{}切片
    result := make([]interface{}, len(products))
    for i, p := range products {
        result[i] = p
    }
    return result, nil
}

// 2. 创建任务工厂
func NewMyProductSyncTask(
    config appscheduler.TaskConfig,
    managementClient *management.ClientManager,
    apiClient APIClient,
) *commonscheduler.ProductSyncTask {
    
    // 创建服务实例
    syncService := &MyProductSyncService{
        apiClient: apiClient,
    }
    
    // 使用通用基类创建任务
    return commonscheduler.NewProductSyncTask(commonscheduler.ProductSyncTaskConfig{
        TaskConfig:       config,
        ManagementClient: managementClient,
        SyncService:      syncService,
        PlatformName:     "MyPlatform",
    })
}
```

## 相关文档

- [重复代码检测报告](../../../../docs/重复代码检测报告.md)
- [第二阶段平台基类抽象](../../../../docs/第二阶段平台基类抽象.md)

## 版本历史

### v1.0.0 (2026-03-06)
- 初始版本
- 添加 BaseTask 基础任务
- 添加 ProductSyncTask 产品同步任务
- 添加 InventorySyncTask 库存同步任务
- 添加 AutoPricingTask 自动核价任务
