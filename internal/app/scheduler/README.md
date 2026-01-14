# 统一任务调度器

## 架构说明

统一任务调度器提供了跨平台的任务调度能力，支持多种任务类型和多个电商平台。

### 目录结构

```
internal/
  app/
    scheduler/                    # 统一调度器核心
      manager.go                  # 调度器管理器
      types.go                    # 通用类型定义
      registry.go                 # 任务工厂注册表
      task_executor.go            # 任务执行器
      
  platforms/
    shein/
      scheduler/                  # SHEIN平台任务实现
        factory.go                # SHEIN任务工厂
        base_task.go              # 基础任务实现
        pricing_task.go           # 核价任务
        sync_task.go              # 同步任务
        inventory_task.go         # 库存任务
        activity_task.go          # 活动任务
```

## 核心概念

### 1. Task (任务)
- 定义了任务的基本接口
- 每个任务有唯一ID、类型、平台、执行间隔等属性
- 实现 `Execute(ctx context.Context) error` 方法执行具体逻辑

### 2. TaskFactory (任务工厂)
- 负责创建特定平台的任务
- 每个平台实现自己的工厂
- 支持多种任务类型

### 3. TaskExecutor (任务执行器)
- 负责定时执行任务
- 处理任务的启动、停止、panic恢复
- 支持优雅关闭

### 4. Manager (调度器管理器)
- 管理所有任务的生命周期
- 提供任务的创建、启动、停止、移除功能
- 维护任务工厂注册表

### 5. Registry (注册表)
- 管理所有平台的任务工厂
- 提供工厂的注册和查询功能

## 使用示例

### 1. 初始化调度器

```go
package main

import (
    "context"
    "time"
    
    "task-processor/internal/app/scheduler"
    "task-processor/internal/pkg/management"
    sheinscheduler "task-processor/internal/platforms/shein/scheduler"
)

func main() {
    ctx := context.Background()
    
    // 创建管理客户端
    managementClient := management.NewClientManager(...)
    
    // 创建调度器管理器
    schedulerManager := scheduler.NewManager(ctx)
    
    // 注册SHEIN平台任务工厂
    sheinFactory := sheinscheduler.NewSheinTaskFactory(managementClient)
    if err := schedulerManager.RegisterFactory(sheinFactory); err != nil {
        panic(err)
    }
    
    // 创建并启动核价任务
    config := scheduler.TaskConfig{
        TaskType:  scheduler.TaskTypePricing,
        Platform:  "SHEIN",
        TenantID:  1001,
        StoreID:   2001,
        Interval:  30 * time.Minute,
        Enabled:   true,
        AutoStart: true,
    }
    
    if err := schedulerManager.CreateAndStartTask(config); err != nil {
        panic(err)
    }
    
    // 等待...
    
    // 优雅关闭
    schedulerManager.StopAll()
}
```

### 2. 添加新平台支持

要添加新平台（如TEMU），需要：

1. 创建平台目录：`internal/platforms/temu/scheduler/`

2. 实现任务工厂：
```go
type TemuTaskFactory struct {
    // ...
}

func (f *TemuTaskFactory) CreateTask(ctx context.Context, config scheduler.TaskConfig) (scheduler.Task, error) {
    // 根据任务类型创建对应的任务
}

func (f *TemuTaskFactory) SupportedPlatform() string {
    return "TEMU"
}

func (f *TemuTaskFactory) SupportedTaskTypes() []scheduler.TaskType {
    return []scheduler.TaskType{
        scheduler.TaskTypePricing,
        scheduler.TaskTypeSync,
    }
}
```

3. 实现具体任务：
```go
type TemuPricingTask struct {
    *BaseTask
    // ...
}

func (t *TemuPricingTask) Execute(ctx context.Context) error {
    // 实现TEMU平台的核价逻辑
}
```

4. 注册工厂：
```go
temuFactory := temuscheduler.NewTemuTaskFactory(managementClient)
schedulerManager.RegisterFactory(temuFactory)
```

### 3. 任务管理

```go
// 列出所有任务
tasks := schedulerManager.ListTasks()
for _, task := range tasks {
    fmt.Printf("任务: %s, 类型: %s, 平台: %s, 状态: %s\n",
        task.GetID(),
        task.GetType(),
        task.GetPlatform(),
        task.GetStatus(),
    )
}

// 停止特定任务
taskID := "SHEIN:pricing:1001:2001"
if err := schedulerManager.StopTask(taskID); err != nil {
    log.Error(err)
}

// 重新启动任务
if err := schedulerManager.StartTask(taskID); err != nil {
    log.Error(err)
}

// 移除任务
if err := schedulerManager.RemoveTask(taskID); err != nil {
    log.Error(err)
}
```

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

## 最佳实践

1. **任务隔离**：每个任务独立运行，互不影响
2. **错误处理**：任务执行失败不影响其他任务
3. **优雅关闭**：支持context取消，确保任务正常退出
4. **日志记录**：使用结构化日志记录任务执行情况
5. **panic恢复**：任务执行器自动捕获panic，防止程序崩溃
6. **配置驱动**：通过TaskConfig配置任务参数
7. **工厂模式**：使用工厂模式创建任务，便于扩展

## 注意事项

1. 任务ID格式：`{Platform}:{TaskType}:{TenantID}:{StoreID}`
2. 任务执行超时：默认30分钟，可根据需要调整
3. 任务间隔：建议不小于5分钟，避免频繁调用API
4. 并发控制：同一店铺的同类型任务不应重复创建
5. 资源清理：停止任务时确保释放相关资源
