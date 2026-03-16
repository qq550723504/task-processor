# 问题六：Amazon 双重角色混乱

**严重程度**：中

## 问题描述

项目中存在两个 `amazon` 包，分别承担完全不同的职责，但命名和结构上没有清晰区分：

- `internal/crawler/amazon/` — Amazon **爬虫**：用浏览器抓取 Amazon 商品页面，获取价格、库存等数据
- `internal/platforms/amazon/` — Amazon **上架**：通过 SP-API 将商品上架到 Amazon 平台

这两个包的核心类型都叫 `Processor`，都有 `ProcessTask` 方法，但做的是完全不同的事情。

## 代码证据

**`internal/crawler/amazon/processor.go`** — 爬虫处理器：

```go
// AmazonProcessor Amazon爬虫处理器
type AmazonProcessor struct {
    browserPool     *browser.BrowserPool  // 浏览器池，用于抓取网页
    poolManager     *browser.PoolManager
    config          *config.Config
    ...
}

// Process 处理Amazon产品页面 — 本质是"抓取"
func (ap *AmazonProcessor) Process(url string, zipcode string) (*model.Product, error) {
    // 用浏览器访问 Amazon 页面，解析 HTML，返回产品数据
}
```

**`internal/platforms/amazon/processor.go`** — 上架处理器：

```go
// Processor Amazon平台处理器
type Processor struct {
    *processor.BaseProcessor
    services  *amazonModel.Services  // SP-API 服务容器
    apiClient *api.Client            // Amazon SP-API 客户端
}

// ProcessTask 处理任务 — 本质是"上架"
func (p *Processor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
    // 解析任务，调用 SP-API，将商品上架到 Amazon
}
```

**`bootstrap/service_registry_simple.go`** 和 **`bootstrap/platform_processors.go`** 中的混用：

```go
// service_registry_simple.go
import "task-processor/internal/crawler/amazon"  // 爬虫包

// 注册的是爬虫处理器，但名字叫 "amazonProcessor"
container.RegisterSingleton("amazonProcessor", func(c di.Container) (any, error) {
    return amazon.NewAmazonProcessor(config), nil  // crawler/amazon
})

// platform_processors.go
// TEMU 和 SHEIN 处理器都依赖 amazonProcessor（爬虫），用于获取 Amazon 价格参考
func (p *PlatformProcessorRegistry) getDependencies(c di.Container) (
    ...
    *amazon.AmazonProcessor,  // 这里的 amazon 是 crawler/amazon，不是 platforms/amazon
    error,
) {
```

而 `internal/platforms/amazon/processor.go` 中的上架处理器（`Processor`）在 bootstrap 中完全没有被注册，说明两套 Amazon 代码的使用路径是分离的，但没有文档说明。

## 影响分析

1. **命名歧义**：`amazon.Processor` 在不同包中指代完全不同的东西，阅读代码时必须时刻注意 import 路径。
2. **职责混淆**：`bootstrap` 中注册的 `"amazonProcessor"` 实际上是爬虫处理器，但 TEMU/SHEIN 平台处理器依赖它来获取 Amazon 价格，这个依赖关系没有任何文档说明。
3. **两套启动路径**：`crawler/amazon` 通过 `bootstrap` 注册并被 TEMU/SHEIN 使用；`platforms/amazon` 有独立的 `Processor` 但在 bootstrap 中未注册，两套代码的激活方式不同，容易造成混乱。
4. **测试困难**：测试 TEMU 上架逻辑时，必须同时 mock Amazon 爬虫处理器，因为两者在 bootstrap 层耦合在一起。

## 重构建议

**第一步：重命名，消除歧义**

```
internal/crawler/amazon/
    processor.go  → AmazonCrawler（或 AmazonScraper）

internal/platforms/amazon/
    processor.go  → AmazonLister（或 AmazonPublisher）
```

或者在包级别区分：

```
internal/crawler/amazon/   → internal/scraper/amazon/   （爬虫/抓取）
internal/platforms/amazon/ → internal/listing/amazon/   （上架/发布）
```

**第二步：用接口隔离爬虫依赖**

TEMU/SHEIN 处理器依赖 Amazon 爬虫获取价格，这个依赖应该通过接口表达：

```go
// 在 temu 或 shein 包中定义
type PriceReference interface {
    FetchPrice(url string, zipcode string) (float64, error)
}
```

这样 TEMU/SHEIN 不再直接依赖 `*amazon.AmazonProcessor`，可以注入任何实现了 `PriceReference` 的对象。

**第三步：补充架构文档**

在 `docs/architecture/` 中说明两个 Amazon 包的职责边界和使用场景，避免后续开发者混淆。这是成本最低、收益最快的改进。
