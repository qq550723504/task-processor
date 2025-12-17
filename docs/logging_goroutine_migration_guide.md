# 日志系统和并发安全迁移指南

## 📋 概述

本指南帮助将现有代码迁移到新的统一日志系统和goroutine管理系统，提升代码的可维护性和并发安全性。

## 🎯 迁移目标

### 日志系统优化
- ✅ 替换所有 `fmt.Println` 为结构化日志
- ✅ 统一日志格式和级别管理
- ✅ 支持动态日志级别调整
- ✅ 文件和控制台双输出

### 并发安全优化
- ✅ 消除"野生goroutine"
- ✅ 统一goroutine生命周期管理
- ✅ 添加panic recovery机制
- ✅ 支持context控制和优雅退出

## 📁 新增文件结构

```
internal/
├── logger/
│   └── manager.go              # 统一日志管理器
├── goroutine/
│   └── manager.go              # 统一goroutine管理器
├── scheduler/
│   └── safe_scheduler.go       # 安全调度器
└── bootstrap/
    └── system_init.go          # 系统初始化器

examples/
└── logger_goroutine_example.go # 使用示例

docs/
└── logging_goroutine_migration_guide.md # 本迁移指南
```

## 🔄 迁移步骤

### 第1步: 替换fmt.Println日志

#### ❌ 迁移前
```go
import "fmt"

func ExampleFunction() {
    fmt.Println("开始处理")
    fmt.Printf("处理用户: %s\n", username)
}
```

#### ✅ 迁移后
```go
import (
    "task-processor/internal/logger"
    "github.com/sirupsen/logrus"
)

func ExampleFunction() {
    log := logger.GetGlobalLogger("example")
    log.Info("开始处理")
    log.WithField("username", username).Info("处理用户")
}
```

### 第2步: 优化goroutine使用

#### ❌ 迁移前 - 野生goroutine
```go
func StartBackgroundTask() {
    go func() {
        // 没有panic recovery
        // 没有退出条件
        for {
            doWork()
            time.Sleep(5 * time.Second)
        }
    }()
}
```

#### ✅ 迁移后 - 管理的goroutine
```go
import (
    "context"
    "task-processor/internal/goroutine"
    "task-processor/internal/logger"
)

func StartBackgroundTask(ctx context.Context) {
    logger := logger.GetGlobalLogger("background_task")
    goroutineManager := goroutine.NewGoroutineManager(ctx, logger)
    
    goroutineManager.StartPeriodic("work_task", 5*time.Second, func(ctx context.Context) error {
        return doWork(ctx)
    })
}
```

### 第3步: 迁移调度器

#### ❌ 迁移前 - 不安全的调度
```go
func StartScheduler() {
    go func() {
        ticker := time.NewTicker(10 * time.Second)
        for {
            select {
            case <-ticker.C:
                processData() // 可能panic
            }
        }
    }()
}
```

#### ✅ 迁移后 - 安全调度器
```go
import "task-processor/internal/scheduler"

func StartScheduler(ctx context.Context) {
    safeScheduler := scheduler.NewSafeScheduler(ctx)
    
    task := &scheduler.ScheduledTask{
        ID:       "data_processor",
        Name:     "数据处理任务",
        Interval: 10 * time.Second,
        Enabled:  true,
        Fn: func(ctx context.Context) error {
            return processData(ctx)
        },
    }
    
    safeScheduler.AddTask(task)
    safeScheduler.Start()
}
```

### 第4步: 系统初始化集成

#### ✅ 在main.go中集成
```go
package main

import (
    "task-processor/internal/bootstrap"
    "task-processor/internal/logger"
)

func main() {
    // 初始化系统
    config := &bootstrap.SystemConfig{
        LogConfig: &logger.LogConfig{
            Level:      "info",
            Format:     "json",
            OutputFile: "logs/app.log",
            Console:    true,
        },
        AppName: "task-processor",
        Version: "1.0.0",
    }
    
    if err := bootstrap.InitializeGlobalSystem(config); err != nil {
        panic(err)
    }
    defer bootstrap.ShutdownGlobalSystem()
    
    // 获取系统组件
    initializer := bootstrap.GetGlobalSystemInitializer()
    ctx := initializer.GetContext()
    logger := initializer.GetLogger("main")
    
    logger.Info("应用启动")
    
    // 启动业务逻辑
    startApplication(ctx)
    
    logger.Info("应用结束")
}
```

## 📝 具体文件迁移示例

### 已迁移文件

#### 1. platforms/temu/utils/format_example.go
- ✅ 替换 `fmt.Println` 为结构化日志
- ✅ 使用 `logger.GetGlobalLogger()`
- ✅ 添加字段化日志记录

#### 2. internal/utils/help_utils.go
- ✅ 替换 `fmt.Println` 为结构化日志
- ✅ 使用结构化字段记录参数信息
- ✅ 改进日志可读性

#### 3. cmd/amazon-upload-test/main.go
- ✅ 替换 `fmt.Printf` 为结构化日志
- ✅ 使用现有的日志系统

#### 4. platforms/shein/modules/enhanced_pricing_handler.go
- ✅ 使用 `goroutine.GoroutineManager`
- ✅ 替换原始goroutine为管理的周期性任务
- ✅ 添加context控制

#### 5. platforms/shein/modules/integrated_pricing_handler.go
- ✅ 使用统一goroutine管理器
- ✅ 添加panic recovery和context控制

#### 6. platforms/shein/modules/daily_reset_handler.go
- ✅ 使用安全的goroutine管理
- ✅ 改进错误处理和退出条件

## 🔍 需要迁移的其他文件

### 高优先级迁移文件

1. **platforms/scheduler/sync_scheduler.go**
   ```go
   // 当前问题: 直接启动goroutine，缺乏管理
   for storeID, apiClient := range s.sheinAPIClients {
       go func(sid int64, client *shops.ShopAPIClient) {
           // 缺乏panic recovery和context控制
       }(storeID, apiClient)
   }
   ```

2. **platforms/scheduler/monitor_scheduler.go**
   ```go
   // 需要使用安全调度器替换
   ```

3. **common/amazon/browser/browser_pool.go**
   ```go
   // 异步重新创建实例的goroutine需要管理
   go func() {
       // 需要添加panic recovery
   }()
   ```

### 迁移模板

#### Goroutine迁移模板
```go
// ❌ 原始代码
go func() {
    // 业务逻辑
}()

// ✅ 迁移后
goroutineManager.Start("task_name", func(ctx context.Context) error {
    // 业务逻辑
    return nil
})
```

#### 周期性任务迁移模板
```go
// ❌ 原始代码
go func() {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    for {
        select {
        case <-ticker.C:
            doWork()
        }
    }
}()

// ✅ 迁移后
goroutineManager.StartPeriodic("periodic_task", interval, func(ctx context.Context) error {
    return doWork(ctx)
})
```

## 🧪 测试迁移结果

### 1. 编译检查
```bash
go build ./...
```

### 2. 运行示例
```bash
go run examples/logger_goroutine_example.go
```

### 3. 日志验证
检查生成的日志文件格式是否正确：
```bash
tail -f logs/app.log
```

### 4. 并发安全验证
使用race detector检查：
```bash
go run -race ./...
```

## 📊 迁移收益

### 日志系统改进
- **标准化**: 统一的JSON格式日志
- **可观测性**: 结构化字段便于分析
- **可配置性**: 动态调整日志级别
- **可靠性**: 文件和控制台双输出

### 并发安全改进
- **可控性**: 统一的goroutine生命周期管理
- **可观测性**: 实时监控goroutine状态
- **稳定性**: panic recovery防止程序崩溃
- **优雅退出**: context控制确保资源清理

### 性能改进
- **资源管理**: 避免goroutine泄露
- **错误恢复**: 自动重试机制
- **监控指标**: 实时性能数据

## ⚠️ 注意事项

### 1. 向后兼容性
- 保持现有API接口不变
- 逐步迁移，避免大规模重构

### 2. 性能考虑
- goroutine管理器有轻微性能开销
- 日志系统I/O操作需要考虑性能影响

### 3. 错误处理
- 确保所有错误都有适当的处理
- 使用context.Context传递取消信号

### 4. 测试覆盖
- 为新的管理器添加单元测试
- 验证并发安全性

## 🚀 下一步计划

1. **完成剩余文件迁移** (本周)
2. **添加单元测试** (下周)
3. **性能基准测试** (下周)
4. **生产环境部署** (下下周)

## 📞 支持

如果在迁移过程中遇到问题，请参考：
- `examples/logger_goroutine_example.go` - 完整使用示例
- 现有已迁移文件作为参考
- 本迁移指南的具体示例