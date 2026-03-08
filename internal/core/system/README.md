# system 目录

## 用途

系统级初始化模块，提供应用程序运行所需的底层系统功能，包括日志管理、Goroutine 管理、信号处理等。

## 目录结构

```
system/
└── initializer.go    # 系统初始化器
```

## 核心功能

### 1. SystemInitializer（系统初始化器）

负责初始化和管理系统级资源：

- 日志管理器初始化
- Goroutine 生命周期管理
- 系统上下文管理
- 信号处理（SIGINT、SIGTERM）
- 优雅关闭

### 2. 主要组件

**日志管理：**
- 初始化日志管理器
- 提供组件级日志记录器
- 支持日志级别动态调整

**Goroutine 管理：**
- 统一管理所有后台 Goroutine
- 提供超时等待机制
- 跟踪 Goroutine 状态

**信号处理：**
- 监听系统信号（SIGINT、SIGTERM）
- 触发优雅关闭流程
- 通知所有 Goroutine 停止

## 使用示例

### 基本使用

```go
package main

import (
    "task-processor/internal/core/system"
    "task-processor/internal/core/logger"
)

func main() {
    // 创建系统配置
    config := &system.SystemConfig{
        LogConfig: &logger.LogConfig{
            Level:      "info",
            Format:     "json",
            OutputFile: "logs/app.log",
            Console:    true,
        },
        AppName: "task-processor",
        Version: "1.0.0",
    }
    
    // 创建系统初始化器
    sysInit := system.NewSystemInitializer(config)
    
    // 初始化系统
    if err := sysInit.Initialize(); err != nil {
        panic(err)
    }
    defer sysInit.Shutdown()
    
    // 获取系统上下文
    ctx := sysInit.GetContext()
    
    // 获取日志记录器
    logger := sysInit.GetLogger("main")
    logger.Info("应用启动")
    
    // 启动后台任务
    goroutineMgr := sysInit.GetGoroutineManager()
    goroutineMgr.Start("worker", func(ctx context.Context) error {
        // 工作逻辑
        return nil
    })
    
    // 等待信号
    <-ctx.Done()
}
```

### 使用全局实例

```go
package main

import (
    "task-processor/internal/core/system"
)

func main() {
    // 初始化全局系统
    config := &system.SystemConfig{
        AppName: "task-processor",
        Version: "1.0.0",
    }
    
    if err := system.InitializeGlobalSystem(config); err != nil {
        panic(err)
    }
    defer system.ShutdownGlobalSystem()
    
    // 获取全局实例
    sysInit := system.GetGlobalSystemInitializer()
    
    // 使用系统功能
    logger := sysInit.GetLogger("worker")
    logger.Info("工作开始")
}
```

### 为处理器创建独立实例

```go
package main

import (
    "task-processor/internal/core/system"
)

func main() {
    // 为特定处理器创建初始化器
    temuInit, err := system.CreateProcessorInitializer("temu")
    if err != nil {
        panic(err)
    }
    defer temuInit.Shutdown()
    
    // 使用处理器专用的日志和上下文
    ctx := temuInit.GetContext()
    logger := temuInit.GetLogger("temu-worker")
    
    // 处理器逻辑
    logger.Info("Temu 处理器启动")
}
```

## API 说明

### SystemConfig

```go
type SystemConfig struct {
    LogConfig *logger.LogConfig  // 日志配置
    AppName   string              // 应用名称
    Version   string              // 应用版本
}
```

### SystemInitializer 方法

```go
// 创建系统初始化器
func NewSystemInitializer(config *SystemConfig) *SystemInitializer

// 初始化系统
func (si *SystemInitializer) Initialize() error

// 获取系统上下文
func (si *SystemInitializer) GetContext() context.Context

// 获取 Goroutine 管理器
func (si *SystemInitializer) GetGoroutineManager() *utils.GoroutineManager

// 获取日志记录器
func (si *SystemInitializer) GetLogger(component string) *logrus.Entry

// 获取系统状态
func (si *SystemInitializer) GetSystemStatus() map[string]interface{}

// 优雅关闭系统
func (si *SystemInitializer) Shutdown() error
```

### 全局函数

```go
// 初始化全局系统
func InitializeGlobalSystem(config *SystemConfig) error

// 获取全局系统初始化器
func GetGlobalSystemInitializer() *SystemInitializer

// 关闭全局系统
func ShutdownGlobalSystem() error

// 从文件加载系统配置
func LoadSystemConfigFromFile(configPath string) (*SystemConfig, error)

// 为处理器创建初始化器
func CreateProcessorInitializer(processorName string) (*SystemInitializer, error)
```

## 与 infra/bootstrap 的区别

| 特性 | core/system | infra/bootstrap |
|------|-------------|-----------------|
| 职责 | 系统级基础设施 | 应用级组装启动 |
| 依赖 | 只依赖 core 和 pkg | 依赖所有层 |
| 功能 | 日志、Goroutine、信号 | DI容器、服务注册、组件生命周期 |
| 使用时机 | 应用最开始 | 系统初始化之后 |
| 层级 | 核心层 | 基础设施层 |

## 典型启动流程

```
1. core/system 初始化
   ├── 初始化日志管理器
   ├── 创建系统上下文
   ├── 启动 Goroutine 管理器
   └── 注册信号处理

2. infra/bootstrap 初始化
   ├── 加载配置文件
   ├── 创建 DI 容器
   ├── 注册所有服务
   └── 注册组件到生命周期管理器

3. infra/bootstrap 启动
   ├── 启动更新服务
   ├── 启动平台处理器
   ├── 启动任务获取器
   └── 启动调度服务
```

## 注意事项

1. SystemInitializer 应该在应用最开始创建
2. 确保调用 Shutdown() 以优雅关闭
3. 使用 GetContext() 获取的上下文会在收到信号时取消
4. Goroutine 应该通过 GoroutineManager 启动以便统一管理
5. 不要在 system 包中引入业务逻辑

## 最佳实践

1. 在 main 函数开始时初始化系统
2. 使用 defer 确保系统正确关闭
3. 所有后台任务通过 GoroutineManager 启动
4. 使用结构化日志记录关键信息
5. 监听系统上下文的 Done 信号
