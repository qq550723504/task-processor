# 任务依赖管理系统

## 概述

任务依赖管理系统用于确保任务按照正确的顺序执行,实现任务间的联动。例如:
- 产品同步 → 库存监控 → 活动报名

## 核心组件

### 1. DependencyManager (依赖管理器)

负责管理任务间的依赖关系和执行状态。

**主要功能:**
- 注册任务依赖关系
- 跟踪任务执行状态
- 检查依赖是否满足
- 等待依赖任务完成

### 2. TaskDependency (任务依赖配置)

定义单个任务的依赖关系。

```go
type TaskDependency struct {
    TaskType     TaskType      // 当前任务类型
    DependsOn    []TaskType    // 依赖的任务类型列表
    WaitTimeout  time.Duration // 等待超时时间
    RetryOnError bool          // 依赖失败时是否重试
}
```

### 3. TaskExecutionStatus (任务执行状态)

记录任务的执行状态。

```go
type TaskExecutionStatus struct {
    TaskID      string
    TaskType    TaskType
    StoreID     int64
    LastRunTime time.Time
    LastStatus  string // "success", "failed", "running", "skipped"
    Error       error
}
```

## 默认依赖配置

系统预设了以下任务依赖关系:

```
核价任务 (pricing)
  └─ 无依赖

产品同步 (productSync)
  └─ 无依赖

库存监控 (inventory)
  └─ 依赖: 产品同步
  └─ 超时: 30分钟

活动报名 (activity)
  └─ 依赖: 产品同步 + 库存监控
  └─ 超时: 60分钟
```

## 工作流程

### 1. 任务执行前检查

```
任务开始执行
    ↓
检查是否有依赖任务
    ↓ 有依赖
等待依赖任务完成 (最多等待WaitTimeout)
    ↓
检查依赖任务状态
    ↓ 成功
执行当前任务
    ↓
更新任务状态
```

### 2. 状态更新时机

- **running**: 任务开始执行时
- **success**: 任务成功完成时
- **failed**: 任务执行失败时
- **skipped**: 依赖未满足,跳过执行时

## 使用示例

### 场景: 确保活动报名在产品同步和库存监控之后执行

**配置:**
```go
{
    TaskType:     TaskTypeActivity,
    DependsOn:    []TaskType{TaskTypeProductSync, TaskTypeInventory},
    WaitTimeout:  60 * time.Minute,
    RetryOnError: true,
}
```

**执行流程:**

1. **产品同步任务** 每小时执行一次
   - 8:00 执行成功 → 状态: success

2. **库存监控任务** 每30分钟执行一次
   - 8:05 检查依赖: 产品同步(success) ✓
   - 8:05 执行成功 → 状态: success

3. **活动报名任务** 每2小时执行一次
   - 8:10 检查依赖:
     - 产品同步(success, 10分钟前) ✓
     - 库存监控(success, 5分钟前) ✓
   - 8:10 执行活动报名 → 状态: success

## 自定义依赖配置

### 方法1: 修改默认配置

编辑 `internal/app/scheduler/task_dependency.go` 中的 `GetDefaultDependencies()` 函数。

### 方法2: 运行时注册

```go
manager := scheduler.NewManager(ctx)
depManager := manager.GetDependencyManager()

// 注册自定义依赖
depManager.RegisterDependency(&scheduler.TaskDependency{
    TaskType:     scheduler.TaskTypeActivity,
    DependsOn:    []scheduler.TaskType{scheduler.TaskTypeInventory},
    WaitTimeout:  30 * time.Minute,
    RetryOnError: false,
})
```

## 注意事项

### 1. 避免循环依赖

❌ 错误示例:
```
任务A 依赖 任务B
任务B 依赖 任务A
```

### 2. 合理设置超时时间

- 考虑依赖任务的实际执行时间
- 留有足够的缓冲时间
- 默认建议: 依赖任务执行时间 × 2

### 3. 任务执行间隔

确保依赖任务的执行间隔小于等于当前任务的执行间隔。

✓ 正确:
```
产品同步: 每1小时
活动报名: 每2小时 (依赖产品同步)
```

❌ 错误:
```
产品同步: 每2小时
活动报名: 每1小时 (依赖产品同步) - 可能等待过久
```

## 监控和调试

### 查看任务状态

```go
depManager := manager.GetDependencyManager()

// 检查任务是否可执行
canExecute, err := depManager.CanExecute(
    ctx,
    "SHEIN",
    scheduler.TaskTypeActivity,
    storeID,
)
```

### 日志输出

系统会自动记录以下日志:
- 依赖检查结果
- 等待依赖任务
- 依赖未满足时跳过执行
- 任务状态变更

## 故障排查

### 问题: 任务一直被跳过

**可能原因:**
1. 依赖任务执行失败
2. 依赖任务执行时间过久
3. 依赖任务尚未执行

**解决方法:**
1. 检查依赖任务的日志
2. 调整 WaitTimeout 时间
3. 确保依赖任务已启动

### 问题: 任务等待超时

**可能原因:**
1. 依赖任务执行时间过长
2. WaitTimeout 设置过短

**解决方法:**
1. 优化依赖任务性能
2. 增加 WaitTimeout 时间

## 最佳实践

1. **明确依赖关系**: 只添加必要的依赖
2. **合理设置超时**: 根据实际情况调整
3. **监控任务状态**: 定期检查任务执行情况
4. **错误处理**: 依赖失败时的降级策略
5. **文档记录**: 记录任务间的依赖关系

## 扩展功能

### 未来可能的增强:

1. **条件依赖**: 根据条件决定是否需要依赖
2. **部分依赖**: 允许部分依赖失败
3. **依赖版本**: 依赖特定版本的任务结果
4. **依赖数据传递**: 将依赖任务的结果传递给当前任务
5. **可视化界面**: 展示任务依赖关系图
