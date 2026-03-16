# 问题一：爬虫层双入口混乱

**严重程度**：高 | **状态**：已重构完成

## 原始问题

项目存在两套爬虫相关代码，按技术层分组，职责边界模糊：

```
internal/
  crawler/              ← 技术层：爬虫核心实现
    amazon/
    alibaba1688/
  app/
    crawler/            ← 技术层：爬虫应用编排（重复）
      amazon/
      alibaba1688/
      fetcher/
      distributed/
```

具体违规点：

1. **按技术层分组**：`crawler/` 和 `app/crawler/` 是典型的技术层划分，违反 Idiomatic Go 按功能聚合的原则
2. **无接口隔离**：`app/crawler/amazon/service.go` 直接持有 `*crawleramazon.AmazonProcessor` 具体类型，同一个包被 import 两次（`amazonPkg` 和 `crawleramazon`）
3. **重复封装**：`app/crawler/amazon/processor.go` 和 `app/crawler/alibaba1688/processor.go` 的全部逻辑只是把下层调用包了一层 Worker 接口，没有任何额外价值
4. **具体类型透传**：`fetcher_factory.go` 的参数是 `*amazon.AmazonProcessor` 具体类型，`domain/product/product_fetcher.go` 同样直接持有具体类型

## 重构内容

### 1. 接口隔离

新增 `internal/crawler/amazon/scraper.go`，定义 `Scraper` 接口：

```go
// Scraper 定义 Amazon 商品抓取能力（消费者定义接口原则）
type Scraper interface {
    Process(url string, zipcode string) (*model.Product, error)
    ProcessBatch(requests []model.ProductRequest) []model.ProductResult
}

// 编译期验证 AmazonProcessor 实现了 Scraper 接口
var _ Scraper = (*AmazonProcessor)(nil)
```

`domain/product/product_fetcher.go` 中定义 `AmazonScraper` 接口（domain 层不直接依赖 crawler 包）：

```go
// AmazonScraper 定义 Amazon 商品抓取能力（消费者定义接口原则）
type AmazonScraper interface {
    Process(url string, zipcode string) (*model.Product, error)
}
```

`fetcher_factory.go` 和 `ProductFetcher` 的参数从 `*amazon.AmazonProcessor` 改为 `AmazonScraper` 接口。

### 2. 合并双入口目录

将 `app/crawler/amazon/`（4个文件）合并进 `crawler/amazon/`：

| 原路径 | 新路径 |
|--------|--------|
| `app/crawler/amazon/service.go` | `crawler/amazon/crawler_service.go` |
| `app/crawler/amazon/processor.go` | `crawler/amazon/worker_processor.go` |
| `app/crawler/amazon/job_handler.go` | `crawler/amazon/job_handler.go` |
| `app/crawler/amazon/api_service.go` | `crawler/amazon/api_service.go` |

将 `app/crawler/alibaba1688/`（4个文件）合并进 `crawler/alibaba1688/`：

| 原路径 | 新路径 |
|--------|--------|
| `app/crawler/alibaba1688/service.go` | `crawler/alibaba1688/crawler_service.go` |
| `app/crawler/alibaba1688/processor.go` | `crawler/alibaba1688/worker_processor.go` |
| `app/crawler/alibaba1688/job_handler.go` | `crawler/alibaba1688/job_handler.go` |
| `app/crawler/alibaba1688/api_service.go` | `crawler/alibaba1688/api_service.go` |

删除空目录 `app/crawler/amazon/` 和 `app/crawler/alibaba1688/`。

### 3. 更新 cmd 入口

`cmd/amazon-crawler-api/main.go` 和 `cmd/1688-crawler-api/main.go` 的 import 路径同步更新。

## 重构后结构

```
internal/
  crawler/
    amazon/               ← Amazon 爬虫完整功能（核心实现 + 应用编排合一）
      scraper.go          ← Scraper 接口定义
      processor.go        ← 核心爬虫处理器
      crawler_service.go  ← Worker 队列 + 任务管理
      worker_processor.go ← worker.Processor 适配器
      job_handler.go      ← Worker 任务钩子
      api_service.go      ← HTTP API 服务
      browser/            ← 浏览器池
      extractor/          ← 页面解析
      variations/         ← 变体处理
    alibaba1688/          ← 1688 爬虫完整功能（同上）
    shared/               ← 跨平台共享浏览器工具
  app/
    crawler/
      fetcher/            ← 保留：跨平台产品获取器（依赖 AmazonScraper 接口）
      distributed/        ← 保留：分布式爬虫客户端
```

## 遗留事项

`app/crawler/fetcher/` 和 `app/crawler/distributed/` 是跨平台通用组件，不属于单一平台，
暂时保留在 `app/crawler/` 下。后续可考虑迁移至 `internal/infra/` 或独立的 `internal/crawler/fetcher/`。
