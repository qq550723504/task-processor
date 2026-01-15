# 调度服务重构说明

## 📋 重构概述

将原来的 `PricingService`（核价服务）重构为 `SchedulerService`（调度服务），统一管理所有周期性调度任务。

## 🎯 重构目标

1. **架构统一** - 所有调度任务（核价、产品同步、库存同步、活动报名）统一管理
2. **配置灵活** - 每种任务类型独立配置，支持启用/禁用和自定义间隔
3. **易于扩展** - 新增任务类型只需添加配置和工厂支持
4. **代码清晰** - 职责明确，模块化设计

## 🔄 架构变化

### **重构前**
```
PricingService (只管理核价任务)
    ↓
为每个店铺创建核价任务
```

### **重构后**
```
SchedulerService (统一管理所有调度任务)
    ↓
为每个店铺创建多种类型的任务
    ├── 核价任务 (Pricing)
    ├── 产品同步任务 (ProductSync)
    ├── 库存同步任务 (InventorySync)
    └── 活动报名任务 (ActivityRegistration)
```

## 📁 文件变化

### **删除的文件**
- `internal/app/service/pricing_service.go`
- `internal/app/service/pricing_service_impl.go`
- `internal/app/service/pricing_task_starter.go`
- `internal/app/service/pricing_platform_config.go`
- `internal/app/service/pricing_factory_creator.go`

### **新增的文件**
- `internal/app/service/scheduler_service.go` - 调度服务接口定义
- `internal/app/service/scheduler_service_impl.go` - 调度服务实现
- `internal/app/service/scheduler_task_starter.go` - 任务启动器
- `internal/app/service/scheduler_platform_config.go` - 平台配置管理
- `internal/app/service/scheduler_factory_creator.go` - 工厂创建器

### **修改的文件**
- `internal/app/service/processor_service_impl.go` - 更新服务引用
- `internal/app/service/processor_lifecycle.go` - 更新启动/停止逻辑
- `internal/core/config/types.go` - 添加调度任务配置结构
- `config/config-dev.yaml` - 添加新的任务配置项

## ⚙️ 配置变化

### **TEMU 平台配置示例**
```yaml
platforms:
  temu:
    enabled: true
    # 自动核价配置
    autoPricing:
      enabled: true
      interval: 300  # 5分钟
    # 产品同步配置
    productSync:
      enabled: true
      interval: 3600  # 1小时
    # 库存同步配置
    inventorySync:
      enabled: true
      interval: 1800  # 30分钟
    # 活动报名配置
    activityRegistration:
      enabled: true
      interval: 7200  # 2小时
```

### **SHEIN 平台配置示例**
```yaml
platforms:
  shein:
    enabled: true
    # 自动核价配置
    autoPricing:
      enabled: true
      interval: 300
    # 产品同步配置
    productSync:
      enabled: true
      interval: 3600
    # 库存同步配置
    inventorySync:
      enabled: true
      interval: 1800
    # 活动报名配置
    activityRegistration:
      enabled: true
      interval: 7200
```

## 🔧 使用方式

### **启用/禁用任务**
在配置文件中设置 `enabled: true/false`：

```yaml
platforms:
  temu:
    productSync:
      enabled: true   # 启用产品同步
    inventorySync:
      enabled: false  # 禁用库存同步
```

### **调整执行间隔**
修改 `interval` 值（单位：秒）：

```yaml
platforms:
  temu:
    autoPricing:
      interval: 600  # 改为10分钟执行一次
```

### **查看任务状态**
调度服务会在日志中输出任务启动信息：

```
[INFO] 启动TEMU平台调度任务...
[INFO] ✅ 成功启动 2 个TEMU核价任务
[INFO] ✅ 成功启动 2 个TEMU产品同步任务
[INFO] ✅ TEMU平台共启动 4 个调度任务
```

## 🎨 设计亮点

### **1. 模块化设计**
每个功能独立文件，职责清晰：
- `scheduler_service.go` - 接口定义
- `scheduler_service_impl.go` - 核心实现
- `scheduler_task_starter.go` - 任务启动逻辑
- `scheduler_platform_config.go` - 配置管理
- `scheduler_factory_creator.go` - 工厂创建

### **2. 统一的任务管理**
所有任务类型使用相同的配置结构：
```go
type taskTypeConfig struct {
    Enabled  bool
    Interval int
}
```

### **3. 灵活的平台配置**
每个平台独立配置，互不影响：
```go
type platformTaskConfig struct {
    PlatformName         string
    AutoPricing          taskTypeConfig
    ProductSync          taskTypeConfig
    InventorySync        taskTypeConfig
    ActivityRegistration taskTypeConfig
    FactoryCreator       func() scheduler.TaskFactory
}
```

### **4. 优雅的错误处理**
单个任务启动失败不影响其他任务：
```go
if err := s.startTasksByType(...); err != nil {
    s.logger.Errorf("启动任务失败: %v", err)
    // 继续启动其他任务
}
```

## 🚀 扩展指南

### **添加新的任务类型**

1. **在 `scheduler/types.go` 中定义任务类型**
```go
const (
    TaskTypeNewTask TaskType = "newTask"
)
```

2. **在配置中添加任务配置**
```go
type PlatformConfig struct {
    NewTask ScheduledTaskConfig `yaml:"newTask"`
}
```

3. **在工厂中实现任务创建**
```go
case scheduler.TaskTypeNewTask:
    return NewNewTask(ctx, config, ...), nil
```

4. **在配置文件中启用**
```yaml
platforms:
  temu:
    newTask:
      enabled: true
      interval: 1800
```

## ✅ 测试验证

### **编译测试**
```bash
go build -o task-processor.exe ./cmd/task
```

### **运行测试**
```bash
./task-processor.exe
```

### **验证日志**
查看日志输出，确认任务正常启动：
```
[INFO] 🚀 开始启动调度服务...
[INFO] 启动TEMU平台调度任务...
[INFO] ✅ 成功启动 X 个TEMU核价任务
[INFO] ✅ 成功启动 X 个TEMU产品同步任务
[INFO] ✅ 调度服务启动完成
```

## 📝 注意事项

1. **向后兼容** - 保留了旧版 `sync` 配置，确保现有配置不受影响
2. **默认值** - 未配置的任务默认禁用，不会自动启动
3. **间隔单位** - 所有间隔配置统一使用秒（seconds）
4. **店铺过滤** - 只为匹配平台的店铺创建任务

## 🔗 相关文档

- [调度器架构文档](./SCHEDULER_REFACTORING.md)
- [SHEIN调度器架构](./SHEIN_SCHEDULER_ARCHITECTURE.md)
- [配置说明](../config/config-dev.yaml)

## 📅 更新日期

2025-01-14
