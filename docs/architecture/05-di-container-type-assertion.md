# 问题五：DI 容器形同虚设

**严重程度**：中

## 问题描述

项目引入了 DI（依赖注入）容器（`internal/app/di/`），但在实际使用中，从容器取出依赖后立即进行类型断言转回具体类型，完全绕过了接口抽象。这使得 DI 容器只起到了"全局变量注册表"的作用，没有实现真正的依赖倒置。

## 代码证据

**`internal/app/bootstrap/service_registry_simple.go`** — 注册时返回具体类型：

```go
// 注册 authClient，工厂函数返回具体类型
container.RegisterSingleton("authClient", func(c di.Container) (any, error) {
    ...
    client := auth.NewClientCredentialsAuthClient(...)
    return client, nil  // 返回 *auth.ClientCredentialsAuthClient
})

// 注册 managementClient，取出 authClient 后立即断言
container.RegisterSingleton("managementClient", func(c di.Container) (any, error) {
    authClientInstance, _ := c.Get("authClient")
    authClient := authClientInstance.(*auth.ClientCredentialsAuthClient)  // 类型断言
    accessToken, _ := authClient.GetAccessToken()
    ...
})
```

**`internal/app/bootstrap/component_adapters.go`** — 取出后立即断言具体类型：

```go
// TaskFetcherComponent.Start
func (t *TaskFetcherComponent) Start(ctx context.Context) error {
    authClient, _ := t.container.Get("authClient")
    // 直接断言为具体类型，接口形同虚设
    if err := processorSvc.StartProcessors(ctx, t.config, authClient.(*auth.ClientCredentialsAuthClient)); err != nil {
        ...
    }
}

// TemuProcessorComponent.Start
func (t *TemuProcessorComponent) Start(ctx context.Context) error {
    processor, _ := t.container.Get("temuProcessor")
    temuProcessor := processor.(*temu.TemuProcessor)  // 断言具体类型
    temuProcessor.Start(ctx)
}

// SheinProcessorComponent.Start
func (s *SheinProcessorComponent) Start(ctx context.Context) error {
    processor, _ := s.container.Get("sheinProcessor")
    sheinProcessor := processor.(*pipeline.SheinProcessor)  // 断言具体类型
    sheinProcessor.Start(ctx)
}
```

**`internal/app/bootstrap/platform_processors.go`** — getDependencies 返回具体类型：

```go
func (p *PlatformProcessorRegistry) getDependencies(c di.Container) (
    *config.Config,
    *logrus.Logger,
    *management.ClientManager,   // 具体类型
    *amazon.AmazonProcessor,     // 具体类型
    error,
) {
    managementClientInstance, _ := c.Get("managementClient")
    amazonProcessorInstance, _ := c.Get("amazonProcessor")
    
    return configInstance.(*config.Config),
        loggerInstance.(*logrus.Logger),
        managementClientInstance.(*management.ClientManager),  // 断言
        amazonProcessorInstance.(*amazon.AmazonProcessor),     // 断言
        nil
}
```

## 影响分析

1. **DI 容器价值归零**：引入 DI 容器的核心价值是"依赖接口而非实现"，但所有地方都在断言具体类型，等于没有用 DI。
2. **运行时 panic 风险**：类型断言 `x.(ConcreteType)` 在类型不匹配时会 panic，而这类错误只能在运行时发现，不能在编译期捕获。
3. **可测试性没有改善**：因为依赖的是具体类型，测试时无法注入 mock 实现，必须使用真实的 `auth.ClientCredentialsAuthClient`、`temu.TemuProcessor` 等。
4. **重构成本高**：如果要替换 `auth.ClientCredentialsAuthClient` 的实现，需要修改所有断言的地方，而不是只修改注册处。

## 重构建议

**第一步：在消费侧定义接口**

遵循"消费者定义接口"原则，在需要使用依赖的地方定义最小接口：

```go
// 在 bootstrap 或 runner 包中定义
type AuthClient interface {
    GetAccessToken() (string, error)
}

type PlatformProcessor interface {
    Start(ctx context.Context) error
    Close(ctx context.Context)
}
```

**第二步：注册时返回接口，取出时无需断言**

```go
// 注册时
container.RegisterSingleton("authClient", func(c di.Container) (any, error) {
    client := auth.NewClientCredentialsAuthClient(...)
    var _ AuthClient = client  // 编译期验证实现了接口
    return client, nil
})

// 取出时
authClientInstance, _ := c.Get("authClient")
authClient := authClientInstance.(AuthClient)  // 断言接口，而非具体类型
```

**第三步（可选）：考虑是否真的需要 DI 容器**

如果项目规模不大，Go 惯用的做法是直接在 `main.go` 或 `bootstrap` 中手动组装依赖（Wire 模式），比运行时 DI 容器更安全、更透明：

```go
// main.go 或 app.go 中直接组装
authClient := auth.NewClientCredentialsAuthClient(cfg, logger)
managementClient := management.NewClientManager(cfg, authClient)
amazonProcessor := amazon.NewAmazonProcessor(cfg)
temuProcessor := temu.NewTemuProcessor(cfg, managementClient, amazonProcessor)
```

这种方式编译期就能发现依赖缺失，不需要运行时类型断言。
