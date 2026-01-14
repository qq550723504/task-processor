# 调度器架构重构总结

## 重构目标

将原本分散在各平台的调度器代码统一到 `internal/app/scheduler` 目录下，实现跨平台的统一调度管理。

## 新架构设计

### 目录结构

```
internal/
  app/
    scheduler/                    # 统一调度器核心
      ├── manager.go              # 调度器管理器
      ├── types.go                # 通用类型定义
      ├── registry.go             # 任务工厂注册表
      ├── task_executor.go        # 任务执行器
      └── README.md               # 使用文档
      
  platforms/
    shein/
      scheduler/                  # SHEIN平台任务实现
        ├── factory.go            # SHEIN任务工厂
        ├── base_task.go          # 基础任务实现
        ├── pricing_task.go       # 核价任务
        ├── sync_task.go          # 同步任务
        ├── inventory_task.go     # 库存任务
        └── activity_task.go      # 活动任务
        
    temu/
      scheduler/                  # TEMU平台任务实现
        ├── factory.go            # TEMU任务工厂
        ├── base_task.go          # 基础任务实现
        ├── pricing_task.go       # 核价任务
        ├── sync_task.go          # 同步任务
        ├── inventory_task.go     # 库存任务
        └── activity_task.go      # 活动任务
```

## 核心组件

### 1. 统一调度器 (internal/app/scheduler)

#### Manager (调度器管理器)
- 管理所有任务的生命周期
- 提供任务的创建、启动、停止、移除功能
- 维护任务工厂注册表

#### Registry (注册表)
- 管理所有平台的任务工厂
- 提供工厂的注册和查询功能

#### TaskExecutor (任务执行器)
- 负责定时执行任务
- 处理任务的启动、停止、panic恢复
- 支持优雅关闭

#### Types (类型定义)
- Task接口：定义任务的基本行为
- TaskFactory接口：定义任务工厂
- TaskConfig：任务配置
- TaskType：任务类型枚举

### 2. 平台任务实现

每个平台实现自己的：
- **TaskFactory**：创建该平台的各种任务
- **BaseTask**：提供基础任务实现
- **具体任务**：实现 Execute 方法执行业务逻辑

## 任务类型

### TaskTypePricing (核价任务)
- 自动处理待核价商品
- 根据规则自动接受或拒绝报价

### TaskTypeSync (同步任务)
- 同步平台产品数据到本地数据库
- 包括产品信息、价格、库存等

### TaskTypeInventory (库存任务)
- 定时同步库存信息
- 更新产品库存状态

### TaskTypeActivity (活动任务)
- 同步活动产品信息
- 自动报名符合条件的活动

## 使用示例

### 初始化调度器

```go
package main

import (
    "context"
    "time"
    
    "task-processor/internal/app/scheduler"
    "task-processor/internal/pkg/management"
    sheinscheduler "task-processor/internal/platforms/shein/scheduler"
    temuscheduler "task-processor/internal/platforms/temu/scheduler"
)

func main() {
    ctx := context.Background()
    
    // 创建管理客户端
    managementClient := management.NewClientManager(...)
    
    // 创建调度器管理器
    schedulerManager := scheduler.NewManager(ctx)
    
    // 注册SHEIN平台任务工厂
    sheinFactory := sheinscheduler.NewSheinTaskFactory(managementClient)
    schedulerManager.RegisterFactory(sheinFactory)
    
    // 注册TEMU平台任务工厂
    temuFactory := temuscheduler.NewTemuTaskFactory(managementClient, configProvider)
    schedulerManager.RegisterFactory(temuFactory)
    
    // 创建并启动SHEIN核价任务
    sheinConfig := scheduler.TaskConfig{
        TaskType:  scheduler.TaskTypePricing,
        Platform:  "SHEIN",
        TenantID:  1001,
        StoreID:   2001,
        Interval:  30 * time.Minute,
        Enabled:   true,
        AutoStart: true,
    }
    schedulerManager.CreateAndStartTask(sheinConfig)
    
    // 创建并启动TEMU核价任务
    temuConfig := scheduler.TaskConfig{
        TaskType:  scheduler.TaskTypePricing,
        Platform:  "TEMU",
        TenantID:  1001,
        StoreID:   3001,
        Interval:  20 * time.Minute,
        Enabled:   true,
        AutoStart: true,
    }
    schedulerManager.CreateAndStartTask(temuConfig)
    
    // 等待...
    
    // 优雅关闭
    schedulerManager.StopAll()
}
```

## 已完成的工作

### ✅ 核心框架
- [x] 创建统一调度器核心组件
- [x] 实现任务管理器 (Manager)
- [x] 实现任务注册表 (Registry)
- [x] 实现任务执行器 (TaskExecutor)
- [x] 定义通用类型和接口 (Types)

### ✅ SHEIN平台
- [x] 创建SHEIN任务工厂
- [x] 实现核价任务 (PricingTask)
- [x] 实现同步任务 (SyncTask)
- [x] 实现库存任务 (InventoryTask)
- [x] 实现活动任务 (ActivityTask)
- [x] 删除旧的调度器代码

### ✅ TEMU平台
- [x] 创建TEMU任务工厂
- [x] 实现核价任务 (PricingTask)
- [x] 实现同步任务 (SyncTask)
- [x] 实现库存任务 (InventoryTask)
- [x] 实现活动任务 (ActivityTask)
- [x] 迁移旧的调度器代码

### ✅ 文档
- [x] 创建使用文档 (README.md)
- [x] 创建重构总结文档

## 待完善的工作

### 🔧 代码修复
- [ ] 修复SHEIN同步服务的编译错误
- [ ] 修复SHEIN定价服务的类型引用
- [ ] 完善TEMU同步任务的具体实现
- [ ] 完善TEMU库存任务的具体实现
- [ ] 完善TEMU活动任务的具体实现

### 📝 功能完善
- [ ] 添加任务执行结果记录
- [ ] 添加任务执行统计
- [ ] 添加任务失败重试机制
- [ ] 添加任务执行日志持久化
- [ ] 添加任务监控和告警

### 🧪 测试
- [ ] 添加单元测试
- [ ] 添加集成测试
- [ ] 添加性能测试

## 架构优势

### 1. 统一管理
- 所有平台的定时任务统一在一个地方管理
- 便于监控和维护

### 2. 易于扩展
- 添加新平台只需实现TaskFactory接口
- 添加新任务类型只需实现Task接口

### 3. 职责清晰
- 核心调度逻辑与平台业务逻辑分离
- 每个组件职责单一

### 4. 可配置
- 通过TaskConfig灵活配置任务参数
- 支持动态创建和管理任务

### 5. 健壮性
- 任务执行失败不影响其他任务
- 自动panic恢复
- 支持优雅关闭

## 迁移指南

### 从旧架构迁移

#### 旧代码 (TEMU示例)
```go
// 旧方式
AddTemuPricingTask(scheduler, clientManager, storeID, interval, configProvider)
```

#### 新代码
```go
// 新方式
config := scheduler.TaskConfig{
    TaskType:  scheduler.TaskTypePricing,
    Platform:  "TEMU",
    TenantID:  tenantID,
    StoreID:   storeID,
    Interval:  interval,
    Enabled:   true,
    AutoStart: true,
}
schedulerManager.CreateAndStartTask(config)
```

## 注意事项

1. **任务ID唯一性**：任务ID格式为 `{Platform}:{TaskType}:{TenantID}:{StoreID}`
2. **并发控制**：同一店铺的同类型任务不应重复创建
3. **资源清理**：停止任务时确保释放相关资源
4. **错误处理**：任务执行失败应记录日志但不影响其他任务
5. **配置管理**：任务配置应从配置文件或数据库读取

## 后续计划

1. 完成所有编译错误修复
2. 完善各平台任务的具体实现
3. 添加任务执行监控和告警
4. 添加任务执行历史记录
5. 支持任务执行结果通知
6. 支持任务依赖关系
7. 支持任务优先级
8. 支持任务并发控制
