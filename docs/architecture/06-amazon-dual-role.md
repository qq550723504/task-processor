# 问题六：Amazon 双重角色混乱

**严重程度**：中

## 问题描述

项目中存在两个 `amazon` 包，分别承担完全不同的职责，但命名和结构上没有清晰区分：

- `internal/crawler/amazon/` — Amazon **爬虫**：用浏览器抓取 Amazon 商品页面，获取价格、库存等数据
- `internal/amazon/` — Amazon **上架**：通过 SP-API 将商品上架到 Amazon 平台

两个包的核心类型都叫 `Processor`，都有 `ProcessTask` 方法，但做的是完全不同的事情。这违反了 Go 编码规范中"按功能分组"的包组织原则——包名应准确反映其业务职责，而非技术层级。

## 代码证据

**`internal/crawler/amazon/processor.go`** — 爬虫处理器：

```go
// AmazonProcessor Amazon爬虫处理器
// 违规：类型名与包名重复（amazon.AmazonProcessor），应简化为 Crawler 或 Scraper
type AmazonProcessor struct {
    browserPool *browser.BrowserPool // 浏览器池，用于抓取网页
    poolManager *browser.PoolManager
    config      *config.Config
}

// Process 处理Amazon产品页面 — 本质是"抓取"
func (ap *AmazonProcessor) Process(url string, zipcode string) (*model.Product, error) {
    // 用浏览器访问 Amazon 页面，解析 HTML，返回产品数据
}
```

**`internal/amazon/processor.go`** — 上架处理器：

```go
// Processor Amazon平台处理器
// 违规：与 crawler/amazon 中的 Processor 同名，阅读代码时必须时刻核对 import 路径
type Processor struct {
    *processor.BaseProcessor
    services  *amazonModel.Services // SP-API 服务容器
    apiClient *api.Client           // Amazon SP-API 客户端
}

// ProcessTask 处理任务 — 本质是"上架"
func (p *Processor) ProcessTask(ctx context.Context, job worker.WorkerJob) error {
    // 解析任务，调用 SP-API，将商品上架到 Amazon
}
```

**`bootstrap/service_registry_simple.go`** 和 **`bootstrap/platform_processors.go`** 中的混用：

```go
// service_registry_simple.go
import "task-processor/internal/crawler/amazon" // 爬虫包

// 注册的是爬虫处理器，但名字叫 "amazonProcessor"，语义不清
container.RegisterSingleton("amazonProcessor", func(c di.Container) (any, error) {
    return amazon.NewAmazonProcessor(config), nil // crawler/amazon
})

// platform_processors.go
// TEMU 和 SHEIN 处理器直接依赖具体类型 *amazon.AmazonProcessor
// 违规：消费者应依赖接口，而非具体实现（"消费者定义接口"原则）
func (p *PlatformProcessorRegistry) getDependencies(c di.Container) (
    ...
    *amazon.AmazonProcessor, // 这里的 amazon 是 crawler/amazon，不是 platforms/amazon
    error,
) {
```

`internal/amazon/processor.go` 中的上架处理器在 bootstrap 中完全没有被注册，两套 Amazon 代码的激活路径完全分离，但没有任何文档说明。

## 违反的编码规范

| 规范条目 | 违规描述 |
|---|---|
| 按功能分组，而非按层分组 | 两个 `amazon` 包按技术角色（爬虫 vs 上架）区分，但包名相同，职责边界不清 |
| 包名应准确反映业务职责 | `crawler/amazon` 和 `platforms/amazon` 的包名都是 `amazon`，无法从包名判断职责 |
| 小接口设计 / 消费者定义接口 | TEMU/SHEIN 直接依赖 `*amazon.AmazonProcessor` 具体类型，而非接口 |
| 类型名不应与包名重复 | `amazon.AmazonProcessor` 冗余，Go 惯例应为 `amazon.Processor` 或 `amazon.Crawler` |
| 禁止导出不必要的成员 | 爬虫处理器的内部字段通过具体类型暴露给 bootstrap 层 |

## 影响分析

1. **命名歧义**：`amazon.Processor` 在不同包中指代完全不同的东西，阅读代码时必须时刻注意 import 路径。
2. **职责混淆**：bootstrap 中注册的 `"amazonProcessor"` 实际上是爬虫处理器，但 TEMU/SHEIN 平台处理器依赖它获取 Amazon 价格参考，这个依赖关系没有任何文档说明。
3. **两套启动路径**：`crawler/amazon` 通过 bootstrap 注册并被 TEMU/SHEIN 使用；`platforms/amazon` 有独立的 `Processor` 但在 bootstrap 中未注册，容易造成混乱。
4. **测试困难**：测试 TEMU 上架逻辑时，必须同时 mock Amazon 爬虫处理器，因为两者在 bootstrap 层通过具体类型耦合在一起。

## 重构进度

| 步骤 | 状态 | 说明 |
|---|---|---|
| 消除类型断言 | ✅ 已完成 | `parallel_variant_handler.go`、`shein/productdata/` 三个文件参数从 `any` 改为 `domainProduct.AmazonScraper`，移除所有 `.(*amazon.AmazonProcessor)` 断言 |
| 移除 SHEIN 内部直接 new 爬虫 | ✅ 已完成 | `raw_json_service.go` 不再自行创建 `AmazonProcessor`，改为由调用方注入 |
| temu/context 字段改为接口 | ✅ 已完成 | `TemuTaskContext.AmazonProcessor` 从 `*amazon.AmazonProcessor` 改为 `product.AmazonScraper` |
| bootstrap 注册名语义化 | ✅ 已完成 | `"amazonProcessor"` → `"amazonCrawler"`，明确表达"爬虫"职责 |
| 消费者定义接口（全量） | ✅ 已完成 | temu/shein 所有 handler、processor、scheduler、syncsvc、pricingsvc、platformbase、runner、bootstrap 包均已改为各自包内定义的接口，彻底移除对 `*amazon.AmazonProcessor` 具体类型的跨包依赖 |
| 重命名包 | ⏳ 待执行 | 影响所有 import，建议配合下次大重构。当前仍有 8 个外部文件引用 `crawler/amazon`，均为合理的基础设施层/DI 装配层使用点（`infra/productcrawler`、`app/processor`、`app/messaging`、`app/bootstrap`、`cmd/`） |

## 重构建议

### 第一步：重命名包，消除歧义

按照"包名应准确反映业务职责"的原则，在包级别区分两个 Amazon 包：

```
internal/crawler/amazon/   → internal/scraper/amazon/   （职责：抓取/爬虫）
internal/platforms/amazon/ → internal/listing/amazon/   （职责：上架/发布）
```

同时修正类型名，避免与包名重复（Go 惯例：调用方写 `scraper.Crawler`，而非 `scraper.AmazonCrawler`）：

```go
// internal/scraper/amazon/crawler.go
// Crawler 使用浏览器抓取 Amazon 商品页面。
type Crawler struct {
    browserPool *browser.BrowserPool
    poolManager *browser.PoolManager
    config      *config.Config
}

// internal/listing/amazon/lister.go
// Lister 通过 SP-API 将商品上架到 Amazon 平台。
type Lister struct {
    base      *processor.BaseProcessor
    services  *model.Services
    apiClient *api.Client
}
```

### 第二步：用接口隔离爬虫依赖

TEMU/SHEIN 处理器依赖 Amazon 爬虫获取价格，这个依赖应遵循"消费者定义接口"原则，在消费方包内声明接口：

```go
// internal/platforms/temu/processor.go（或 shein）
// priceFetcher 从外部平台获取商品参考价格。
// 遵循小接口设计原则，仅暴露消费方实际需要的方法。
type priceFetcher interface {
    FetchPrice(url string, zipcode string) (float64, error)
}
```

这样 TEMU/SHEIN 不再直接依赖 `*amazon.AmazonProcessor` 具体类型，可以注入任何实现了 `priceFetcher` 的对象，测试时直接 mock 接口即可。

### 第三步：更新 bootstrap 注册名称

注册名称应与实际职责对应，避免语义误导：

```go
// 修改前：名称 "amazonProcessor" 语义不清
container.RegisterSingleton("amazonProcessor", ...)

// 修改后：名称明确反映职责
container.RegisterSingleton("amazonPriceCrawler", func(c di.Container) (any, error) {
    return scraper.NewCrawler(config), nil
})
```

### 优先级建议

| 步骤 | 成本 | 收益 | 建议优先级 |
|---|---|---|---|
| 补充本文档（已完成） | 低 | 消除认知混乱 | 立即执行 |
| 第二步：接口隔离 | 中 | 解耦测试，提升可维护性 | 高 |
| 第一步：重命名包 | 高（影响所有 import） | 根本消除命名歧义 | 中（配合下次大重构） |
| 第三步：更新注册名 | 低 | 提升 bootstrap 可读性 | 高 |
