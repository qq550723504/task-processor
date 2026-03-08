# bootstrap 目录

## 用途

应用启动器，负责应用级的组装和启动，包括依赖注入容器管理、服务注册、组件生命周期管理等。

## 目录结构

```
bootstrap/
├── app.go                        # 应用启动器主逻辑
├── component_adapters.go         # 组件适配器
├── platform_processors.go        # 平台处理器注册
└── service_registry_simple.go    # 服务注册表
```

## 核心功能

### 1. ApplicationBootstrap（应用启动器）

负责应用的完整启动流程：

- 加载配置文件
- 创建和管理 DI 容器
- 注册所有服务
- 管理组件生命周期
- 协调启动和停止

### 2. ComponentAdapters（组件适配器）

将各个组件适配到生命周期管理器：

- UpdaterServiceComponent - 更新服务组件
- TemuProcessorComponent - Temu 处理器组件
- SheinProcessorComponent - Shein 处理器组件
- TaskFetcherComponent - 任务获取器组件
- SchedulerServiceComponent - 调度服务组件

### 3. ServiceRegistrySimple（服务注册表）

负责注册所有业务服务到 DI 容器：

- 基础服务（配置、更新）
- 认证服务
- 共享资源（Amazon 处理器、管理客户端）
- 应用服务（处理器、调度器）
- 平台处理器

## 使用示例

### 基本使用

```go
package main

import (
    "context"
    "task-processor/internal/app/bootstrap"
    "github.com/sirupsen/logrus"
)

func main() {
    logger := logrus.New()
    
    // 创建应用启动器
    app := bootstrap.NewApplicationBootstrap(logger)
    
    // 初始化应用
    if err := app.Initialize("config/config.yaml", "1.0.0"); err != nil {
        logger.Fatal(err)
    }
    
    // 启动应用
    ctx := context.Background()
    if err := app.Start(ctx, "1.0.0"); err != nil {
        logger.Fatal(err)
    }
    defer app.Stop(ctx)
    
    // 应用运行中...
    <-ctx.Done()
}
```

### 获取服务

```go
// 从 DI 容器获取服务
container := app.GetContainer()

// 获取配置
config, err := container.Get("config")
if err != nil {
    logger.Fatal(err)
}

// 获取管理客户端
managementClient, err := container.Get("managementClient")
if err != nil {
    logger.Fatal(err)
}
```

## 启动流程

### 1. Initialize 阶段

```
Initialize(configPath, appVersion)
├── loadConfiguration()           # 加载配置文件
├── registerCoreServices()        # 注册核心服务
│   ├── configManager
│   ├── config
│   ├── lifecycleManager
│   └── logger
├── registerBusinessServices()    # 注册业务服务
│   ├── configService
│   ├── updaterService
│   ├── authService
│   ├── authClient
│   ├── amazonProcessor
│   ├── managementClient
│   ├── processorService
│   ├── temuProcessor
│   ├── sheinProcessor
│   └── schedulerService
└── registerComponents()          # 注册组件
    ├── UpdaterServiceComponent
    ├── TemuProcessorComponent (如果启用)
    ├── SheinProcessorComponent (如果启用)
    ├── TaskFetcherComponent
    └── SchedulerServiceComponent
```

### 2. Start 阶段

```
Start(ctx, appVersion)
└── lifecycleManager.StartAll()   # 按依赖顺序启动所有组件
    ├── updater (优先级 10)
    ├── temu-processor (优先级 20)
    ├── shein-processor (优先级 20)
    ├── task-fetcher (优先级 25)
    └── scheduler (优先级 30)
```

### 3. Stop 阶段

```
Stop(ctx)
├── lifecycleManager.StopAll()    # 按反向顺序停止所有组件
└── container.Close()             # 关闭 DI 容器
```

## 组件依赖关系

```
scheduler (30)
  ↓ 依赖
task-fetcher (25)
  ↓ 依赖
temu-processor (20) + shein-processor (20)
  ↓ 依赖
updater (10)
```

数字表示启动优先级，数字越小越先启动。

## 与 core/system 的区别

| 特性 | core/system | infra/bootstrap |
|------|-------------|-----------------|
| **职责** | 系统级基础设施 | 应用级组装启动 |
| **层级** | 核心层 | 基础设施层 |
| **依赖** | 只依赖 core 和 pkg | 依赖所有层 |
| **功能** | 日志、Goroutine、信号 | DI容器、服务注册、组件生命周期 |
| **使用时机** | 应用最开始 | 系统初始化之后 |
| **管理对象** | 系统资源 | 业务组件 |

## 完整启动示例

```go
package main

import (
    "context"
    "os"
    "os/signal"
    "syscall"
    
    "task-processor/internal/core/system"
    "task-processor/internal/app/bootstrap"
    "github.com/sirupsen/logrus"
)

func main() {
    // 第一步：初始化系统级资源
    sysConfig := &system.SystemConfig{
        AppName: "task-processor",
        Version: "1.0.0",
    }
    
    sysInit := system.NewSystemInitializer(sysConfig)
    if err := sysInit.Initialize(); err != nil {
        panic(err)
    }
    defer sysInit.Shutdown()
    
    // 获取系统上下文和日志
    ctx := sysInit.GetContext()
    logger := sysInit.GetLogger("main").Logger
    
    // 第二步：初始化应用
    app := bootstrap.NewApplicationBootstrap(logger)
    if err := app.Initialize("config/config.yaml", "1.0.0"); err != nil {
        logger.Fatal(err)
    }
    
    // 第三步：启动应用
    if err := app.Start(ctx, "1.0.0"); err != nil {
        logger.Fatal(err)
    }
    defer app.Stop(ctx)
    
    logger.Info("应用启动完成")
    
    // 等待信号
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    
    select {
    case sig := <-sigChan:
        logger.Infof("收到信号: %v", sig)
    case <-ctx.Done():
        logger.Info("上下文取消")
    }
    
    logger.Info("开始优雅关闭...")
}
```

## API 说明

### ApplicationBootstrap 方法

```go
// 创建应用启动器
func NewApplicationBootstrap(logger *logrus.Logger) *ApplicationBootstrap

// 初始化应用
func (a *ApplicationBootstrap) Initialize(configPath, appVersion string) error

// 启动应用
func (a *ApplicationBootstrap) Start(ctx context.Context, appVersion string) error

// 停止应用
func (a *ApplicationBootstrap) Stop(ctx context.Context) error

// 获取 DI 容器
func (a *ApplicationBootstrap) GetContainer() di.Container

// 获取配置管理器
func (a *ApplicationBootstrap) GetConfigManager() config.ConfigManager

// 获取生命周期管理器
func (a *ApplicationBootstrap) GetLifecycleManager() lifecycle.LifecycleManager
```

## 扩展指南

### 添加新的服务

1. 在 `service_registry_simple.go` 中注册服务：

```go
func (s *ServiceRegistrySimple) registerApplicationServices(container di.Container) error {
    // 注册新服务
    if err := container.RegisterSingleton("myService", func(c di.Container) (any, error) {
        // 创建服务实例
        return NewMyService(), nil
    }); err != nil {
        return err
    }
    
    return nil
}
```

### 添加新的组件

1. 在 `component_adapters.go` 中创建组件适配器：

```go
type MyComponent struct {
    *lifecycle.BaseComponent
    container di.Container
    logger    *logrus.Logger
}

func (m *MyComponent) Start(ctx context.Context) error {
    // 启动逻辑
    return nil
}

func (m *MyComponent) Stop(ctx context.Context) error {
    // 停止逻辑
    return nil
}
```

2. 在 `RegisterAllComponents` 中注册：

```go
func (c *ComponentAdapters) RegisterAllComponents(...) error {
    component := &MyComponent{
        BaseComponent: lifecycle.NewBaseComponent("my-component", []string{}, 40),
        container:     c.container,
        logger:        c.logger,
    }
    
    return lifecycleManager.Register(component)
}
```

## 注意事项

1. 确保在 core/system 初始化之后再使用 bootstrap
2. 服务注册顺序很重要，注意依赖关系
3. 组件的启动优先级决定了启动顺序
4. 使用 DI 容器管理所有服务依赖
5. 组件应该实现 lifecycle.Component 接口

## 最佳实践

1. 所有服务通过 DI 容器管理
2. 使用单例模式注册共享资源
3. 组件之间通过依赖声明关系
4. 提供清晰的错误信息
5. 记录关键的启动步骤
