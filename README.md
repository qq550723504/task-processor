# Task Processor - 跨境电商智能上架系统

[![CI](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml/badge.svg)](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

一个高性能、分布式的跨境电商产品智能上架系统，支持从多个源平台（Amazon、1688 等）爬取商品信息，并自动上架到目标平台（SHEIN、TEMU、Amazon 等）。系统采用 RabbitMQ 消息队列架构，支持大规模分布式部署，可管理 300+ 店铺的商品上架任务。

## 业务场景

### 核心业务流程

1. **任务获取**: 通过 Java 后端管理系统经 RabbitMQ 提交上架任务
2. **商品爬取**:
   - 从源平台（Amazon、1688、沃尔玛、速卖通等）爬取商品详情
   - 包括标题、描述、价格、图片、规格等完整信息
3. **数据处理**:
   - 图片优化处理（压缩、裁剪、水印等）
   - 商品信息格式转换和适配
4. **商品信息增强**:
   - 通过 LLM 对商品标题、描述、属性进行智能优化
   - 质量评分与增强建议，提升商品上架通过率
   - 变体信息补全与结果校验
5. **商品上架**:
   - 自动上架到目标平台（SHEIN、TEMU、Amazon 等）
   - 支持批量上架和多店铺管理

### 平台字段约定

- `sourcePlatform` / `source_platform`: 来源抓取平台，例如 `amazon`、`1688`
- `targetPlatform` / `target_platform`: 目标上架平台，例如 `shein`、`temu`、`amazon`
- `platform`: 兼容字段，在 `task-processor` 任务模型和成功回执里默认等于目标上架平台
- RabbitMQ `TaskMessage.platform`: 历史上表示来源平台，排查消息体时需要和 `targetPlatform` 一起看

### 智能运营功能

1. **自动核价**:
   - 定时监测 SHEIN、TEMU 平台商品价格
   - 根据竞品价格、成本、利润率自动调整售价
   - 支持价格策略配置（最低价、跟随价、溢价等）
   - 保护最低利润率，避免亏本销售

2. **自动营销活动报名**:
   - 自动检测 SHEIN、TEMU 平台营销活动
   - 智能报名符合条件的营销活动
   - 提升商品曝光和转化率
   - 支持活动效果跟踪和分析

3. **源平台库存监测**:
   - 实时监测源平台（Amazon、1688 等）产品库存
   - 库存变化自动同步到目标平台
   - 缺货自动下架，补货自动上架
   - 库存预警通知，避免超卖

### 部署架构

- **中央管理**: 1 台服务器部署 Java 后端，负责任务分发和管理
- **分布式节点**: 约 100 台本地计算机（8核16G），每台运行本 Go 程序
- **店铺规模**: 当前管理约 300 个目标店铺，支持动态扩展
- **消息队列**: RabbitMQ 实现任务分发和负载均衡

## 核心特性

- **多源平台支持**:
  - 当前支持: Amazon、1688
  - 规划中: 沃尔玛、速卖通等
- **多目标平台**: SHEIN、TEMU、Amazon 等主流跨境电商平台
- **智能运营功能**:
  - 自动核价: SHEIN、TEMU 平台商品价格自动调整
  - 自动营销: 自动报名平台营销活动，提升商品曝光
  - 库存监测: 实时监测源平台产品库存变化
- **分布式架构**:
  - RabbitMQ 消息队列驱动，支持水平扩展
  - 定时调度模式，支持核价、库存同步等周期性任务
- **高并发处理**: Worker Pool 模式，单机可配置并发数
- **智能容错**: 自动重试、死信队列、优雅降级
- **图片处理**:
  - `productimage` 图片处理流水线（主图、白底图、审核、资产发布）
  - `amazonlisting` Listing 草稿生成、审核工作台与提交流程
- **监控运维**: 健康检查、指标监控、负载统计
- **浏览器池**: 复用浏览器实例，提升爬取效率

## 架构设计

补充架构文档：

- [任务状态流转说明](./docs/architecture/task-status-lifecycle.md)
- [中心化部署改造方案](./docs/architecture/centralized-deployment-plan.md)
- [Amazon Crawler API 说明](./cmd/amazon-crawler-api/README.md)
- [Amazon 爬虫商用化清单](./docs/architecture/amazon-crawler-commercialization-checklist.md)
- [Amazon 爬虫商用化执行计划](./docs/architecture/amazon-crawler-commercialization-execution-plan.md)
- [ListingKit 对象存储开发说明](./docs/development/listingkit-object-storage.md)

### 推荐部署形态

当前更推荐按平台消费者拆开部署：

- `shein-listing` / `temu-listing` / `amazon-listing` 负责从 RabbitMQ 消费上架任务
- `amazon-crawler-api` 负责 Amazon 商品抓取
- 平台消费者通过 HTTP 调用 crawler API，而不是在本地直接持有浏览器爬虫
- `cmd/task` 已降级为 legacy emergency fallback，不作为生产主入口

对应默认配置文件：

- `shein-listing` / `temu-listing` / `amazon-listing`: [config-dev.yaml](./config/config-dev.yaml) 或对应环境配置
- `amazon-crawler-api`: [config-amazon-crawler-api.yaml](./config/config-amazon-crawler-api.yaml)
- `task`: [config-task.yaml](./config/config-task.yaml) 仅用于 legacy fallback

启动示例：

```bash
go run ./cmd/shein-listing -app-config config/config-dev.yaml
go run ./cmd/temu-listing -app-config config/config-dev.yaml
go run ./cmd/amazon-listing -app-config config/config-dev.yaml
go run ./cmd/amazon-crawler-api -config config/config-amazon-crawler-api.yaml
```

### 整体业务架构

```
┌──────────────────────────────────────────────────────────────┐
│                    Java 后端管理系统                          │
│         (任务导入、店铺管理、数据统计、策略配置)               │
└──────────────────────────┬───────────────────────────────────┘
                           │
                           ▼
                    ┌─────────────┐
                    │  RabbitMQ   │ (任务队列)
                    └──────┬──────┘
                           │
        ┌──────────────────┼──────────────────┐
        │                  │                  │
        ▼                  ▼                  ▼
┌──────────────┐   ┌──────────────┐   ┌──────────────┐
│ Go Worker 1  │   │ Go Worker 2  │...│ Go Worker N  │
│  (8C/16G)    │   │  (8C/16G)    │   │  (8C/16G)    │
└──────┬───────┘   └──────┬───────┘   └──────┬───────┘
       │                  │                  │
       └──────────────────┼──────────────────┘
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
        ▼                 ▼                 ▼
┌──────────────┐  ┌──────────────┐  ┌──────────────┐
│源平台爬取     │  │数据处理       │  │目标平台上架   │
│- Amazon      │  │- 图片优化     │  │- SHEIN       │
│- 1688        │  │- 格式转换     │  │- TEMU        │
│- 沃尔玛(计划) │  │- 数据清洗     │  │- Amazon      │
│- 速卖通(计划) │  │              │  │              │
└──────────────┘  └──────────────┘  └──────────────┘
        │                                   │
        └───────────────┬───────────────────┘
                        ▼
        ┌───────────────────────────────────┐
        │         智能运营功能               │
        │  ┌──────────┐  ┌──────────┐      │
        │  │自动核价   │  │库存监测   │      │
        │  └──────────┘  └──────────┘      │
        │  ┌──────────┐                    │
        │  │营销活动   │                    │
        │  │自动报名   │                    │
        │  └──────────┘                    │
        └───────────────────────────────────┘
```

### 单节点系统架构

```
┌─────────────────────────────────────────┐
│         Application Layer               │
│  ┌──────────┐  ┌──────────┐            │
│  │ RabbitMQ │  │   Task   │            │
│  │ Consumer │  │Scheduler │            │
│  │          │  │(核价/库存)│            │
│  └──────────┘  └──────────┘            │
└─────────────────────────────────────────┘
           │              │
           ▼              ▼
┌─────────────────────────────────────────┐
│      Platform Processors (爬取+上架)     │
│  ┌────────┐ ┌────────┐ ┌────────┐     │
│  │ Amazon │ │ 1688   │ │  TEMU  │     │
│  │ Crawler│ │ Crawler│ │Uploader│     │
│  └────────┘ └────────┘ └────────┘     │
│  ┌────────┐ ┌────────┐                │
│  │ SHEIN  │ │ Image  │                │
│  │Uploader│ │Optimizer│               │
│  └────────┘ └────────┘                │
└─────────────────────────────────────────┘
           │              │
           ▼              ▼
┌─────────────────────────────────────────┐
│         Business Logic Layer            │
│  ┌──────────┐  ┌──────────┐            │
│  │ Pricing  │  │Inventory │            │
│  │  Engine  │  │ Monitor  │            │
│  │(自动核价) │  │(库存监测) │            │
│  └──────────┘  └──────────┘            │
│  ┌──────────┐                          │
│  │Marketing │                          │
│  │ Manager  │                          │
│  │(活动报名) │                          │
│  └──────────┘                          │
└─────────────────────────────────────────┘
           │              │
           ▼              ▼
┌─────────────────────────────────────────┐
│         Infrastructure Layer            │
│  ┌──────────┐  ┌──────────┐            │
│  │ Browser  │  │   HTTP   │            │
│  │   Pool   │  │  Client  │            │
│  │ (Chrome) │  │          │            │
│  └──────────┘  └──────────┘            │
└─────────────────────────────────────────┘
```

### 核心组件

- **RabbitMQ Consumer**: 消息队列消费者，从队列获取上架任务
- **Scheduler Manager**: 任务调度管理器，负责定时任务
  - 自动核价任务: 定期检查并调整商品价格
  - 库存同步任务: 监测源平台库存变化
  - 营销活动任务: 自动报名平台活动
- **HTTP API Runtime**: 统一 HTTP 入口，负责 `productenrich` / `productimage` / `amazonlisting` 三条异步流水线
- **Platform Crawlers**: 平台爬虫，从源平台抓取商品信息
  - Amazon Crawler: 爬取 Amazon 商品和库存
  - 1688 Crawler: 爬取 1688 商品和库存
  - 扩展中: 沃尔玛、速卖通等
- **Platform Uploaders**: 平台上架器，将商品上架到目标平台
  - SHEIN Uploader: 上架到 SHEIN，支持核价和活动报名
  - TEMU Uploader: 上架到 TEMU，支持核价和活动报名
  - Amazon Uploader: 上架到 Amazon
- **Pricing Engine**: 智能核价引擎
  - 竞品价格分析
  - 动态定价策略
  - 利润率保护
- **Inventory Monitor**: 库存监测器
  - 源平台库存实时监控
  - 库存变化自动同步
  - 缺货预警和自动下架
- **Marketing Manager**: 营销管理器
  - 活动信息抓取
  - 自动报名符合条件的活动
  - 活动效果跟踪
- **Product Image Pipeline**: 商品图片处理流水线，负责图片审查、主体提取、白底图生成、审核与发布
- **Amazon Listing Pipeline**: Amazon Listing 草稿生成、校验、审核工作台与提交
- **Worker Pool**: 工作池，管理并发任务执行
- **Browser Pool**: 浏览器池，复用 Chrome 实例提高爬取效率
- **Lifecycle Manager**: 生命周期管理器，统一管理组件启停

## 开发指南

### 项目结构

```
task-processor/
├── cmd/                          # 应用入口（每个子目录一个 main.go）
│   ├── task/                    # 已废弃的 legacy polling 入口（仅应急回滚）
│   ├── rabbitmq-consumer/       # RabbitMQ 消费者模式（上架任务）
│   ├── crawler-consumer/        # 爬虫专用消费者
│   ├── 1688-crawler/            # 1688 爬虫独立入口
│   ├── 1688-crawler-api/        # 1688 爬虫 HTTP API 服务
│   ├── amazon-crawler/          # Amazon 爬虫独立入口
│   ├── amazon-crawler-api/      # Amazon 爬虫 HTTP API 服务
│   ├── shein-listing/           # SHEIN 上架独立入口
│   ├── temu-listing/            # TEMU 上架独立入口
│   ├── product-listing-api/     # 商品增强/图片处理/Amazon Listing 统一 HTTP API
│   └── productenrich-api/       # 兼容入口，复用统一 HTTP API 装配
├── internal/                    # 私有业务逻辑（Go 编译器强制不对外暴露）
│   ├── app/                     # 应用层：启动、调度、消息、任务编排
│   │   ├── bootstrap/           # 应用启动与依赖组装
│   │   ├── consumer/            # RabbitMQ 服务、处理器注册、节点运行时
│   │   ├── httpapi/             # 统一 HTTP API 装配（productenrich/productimage/amazonlisting）
│   │   ├── ports/               # 应用层抽象端口
│   │   ├── processor/           # 通用处理器基类与接口
│   │   ├── runner/              # 处理器/调度器生命周期管理
│   │   ├── scheduler/           # 定时任务调度器（含分布式锁）
│   │   ├── state/               # 运行时状态管理（Cookie、计数、暂停）
│   │   ├── task/                # 任务分发、去重、队列管理
│   │   ├── updater/             # 自动更新管理
│   │   ├── crawler/             # 爬虫分布式调度
│   │   └── worker/              # Worker 任务处理
│   ├── core/                    # 核心基础设施（无业务依赖）
│   │   ├── config/              # 配置加载、校验、类型定义
│   │   ├── errors/              # 统一错误类型与辅助函数
│   │   ├── lifecycle/           # 组件生命周期接口与管理器
│   │   ├── logger/              # 日志管理（含滚动写入）
│   │   ├── metrics/             # 任务指标采集
│   │   └── system/              # 系统初始化
│   ├── domain/                  # 领域模型与核心业务规则
│   │   ├── model/               # 通用领域模型（Task、AmazonProduct 等）
│   │   ├── task/                # 任务领域（Job、去重、消息适配）
│   │   ├── product/             # 商品领域（获取、缓存、校验）
│   │   ├── message/             # 消息类型定义
│   │   ├── queue/               # 队列命名规范
│   │   └── validation/          # 通用过滤规则
│   ├── pipeline/                # 通用 Pipeline 框架（Handler 链式处理）
│   │   └── handlers/            # 内置 Handler（初始化、日志、校验）
│   ├── taskbase/                # 跨平台任务基类（核价、库存同步、商品同步）
│   ├── platformbase/            # 平台处理器公共基类与任务类型定义
│   ├── pricing/                 # 通用成本与利润计算
│   ├── productenrich/           # 商品信息 AI 增强（标题优化、属性补全）
│   ├── productimage/            # 商品图片处理流水线（审核、白底图、资产发布）
│   ├── amazonlisting/           # Amazon Listing 草稿生成、工作台、提交
│   ├── crawler/                 # 源平台爬虫实现
│   │   ├── amazon/              # Amazon 爬虫（浏览器+API 双模式）
│   │   ├── alibaba1688/         # 1688 爬虫（含验证码处理）
│   │   └── shared/              # 爬虫共享组件（浏览器池）
│   ├── amazon/                  # Amazon 目标平台（SP-API 上架）
│   │   ├── api/                 # SP-API 客户端（Listings、Catalog、Pricing 等）
│   │   ├── attribute/           # 属性映射与校验
│   │   └── core/                # 转换器、变体提取、标识符生成
│   ├── shein/                   # SHEIN 平台（上架、核价、活动、库存同步）
│   │   ├── api/                 # SHEIN API 客户端（分模块）
│   │   ├── client/              # HTTP 客户端与 Cookie 管理
│   │   ├── category/            # 类目管理（AI 选类、限制检查）
│   │   ├── content/             # 内容处理（文本清洗、敏感词、翻译）
│   │   ├── operation/           # 运营功能（核价、库存同步、活动报名）
│   │   ├── pipeline/            # SHEIN 上架 Pipeline
│   │   ├── pricing/             # 定价计算
│   │   ├── product/             # 商品构建（属性、图片、SKU、变体）
│   │   ├── productdata/         # 商品数据提交
│   │   ├── publish/             # 发布流程（校验、保存、结果处理）
│   │   ├── store/               # 店铺与仓库管理
│   │   ├── taskexecutor/        # 调度任务执行器（核价、库存、活动）
│   │   ├── translate/           # 翻译服务
│   │   └── validation/          # 上架校验（数量、每日限额、过滤规则）
│   ├── temu/                    # TEMU 平台（上架、核价、活动、库存同步）
│   │   ├── api/                 # TEMU API 客户端（分模块）
│   │   ├── handlers/            # Pipeline Handler（AI、类目、图片、SKU 等）
│   │   ├── pricingsvc/          # 自动核价服务
│   │   ├── scheduler/           # 调度任务执行器
│   │   ├── syncsvc/             # 库存与商品同步服务
│   │   ├── bulkrelist/          # 批量重新上架
│   │   ├── context/             # TEMU 上架上下文
│   │   └── format/              # 数据格式化工具
│   ├── infra/                   # 基础设施适配层
│   │   ├── auth/                # 认证（Token 获取、Session 管理）
│   │   ├── clients/             # 外部服务客户端（管理后台、OpenAI）
│   │   ├── database/            # 数据库访问
│   │   ├── httpx/               # HTTP 服务（健康检查、爬虫 API Handler）
│   │   ├── lock/                # 分布式锁与内存锁
│   │   ├── monitoring/          # 监控采集（健康检查、指标、进程信息）
│   │   ├── productcrawler/      # 爬虫仓储实现
│   │   ├── rabbitmq/            # RabbitMQ 客户端（连接、消费、重试）
│   │   ├── repository/          # 数据仓储实现
│   │   └── worker/              # Worker Pool 实现
│   └── pkg/                     # 内部通用工具库（按职责命名，无业务逻辑）
│       ├── strx/                # 字符串处理
│       ├── timex/               # 时间格式化
│       ├── mathx/               # 数学计算
│       ├── jsonx/               # JSON 序列化/反序列化
│       ├── httpclient/          # HTTP 客户端封装
│       ├── imagex/              # 图片处理工具
│       ├── watermark/           # 水印检测与去除
│       ├── downloader/          # 图片下载与处理
│       ├── resilience/          # 熔断器与重试
│       ├── cache/               # 缓存工具
│       ├── skugen/              # SKU 生成器
│       ├── hashx/               # 哈希工具
│       ├── fileio/              # 文件 I/O
│       ├── ptr/                 # 指针辅助函数
│       ├── goroutine/           # Goroutine 安全工具
│       ├── recovery/            # Panic 恢复
│       ├── timeout/             # 超时控制
│       ├── perf/                # 性能工具
│       ├── appenv/              # 应用环境工具
│       ├── apperr/              # 应用错误定义
│       └── types/               # 通用类型（FlexibleValue 等）
├── api/                         # API 定义（Protobuf、OpenAPI）
├── config/                      # 配置文件（dev/prod/test）
├── data/                        # 静态数据（敏感词、禁售品列表）
├── deployments/                 # 部署配置（Docker、Kubernetes）
├── docs/                        # 文档
└── examples/                    # 示例代码
```

### 添加新平台

1. 在 `internal/` 下创建新平台目录（如 `internal/walmart/`）
2. 实现 `internal/platformbase` 中定义的 `Processor` 接口
3. 在 `internal/app/bootstrap/platform_modules.go` 中注册平台模块
4. 在配置文件中添加平台配置

示例：

```go
type Processor struct {
    *platformbase.BaseFactory
}

func (p *Processor) ProcessTask(ctx context.Context, task *model.Task) error {
    // 实现任务处理逻辑
    return nil
}
```

### 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/app/scheduler/...

# 运行测试并显示覆盖率
go test -cover ./...
```

## 监控和运维

### 部署建议

**单节点配置**:
- CPU: 8 核
- 内存: 16GB
- 并发数: 建议 3-5 个浏览器实例
- 适合处理: 约 3-5 个店铺的上架任务

**集群配置**:
- 节点数量: 100 台（当前）
- 总处理能力: 约 300-500 个店铺
- RabbitMQ: 单独部署，建议集群模式
- Java 后端: 单独服务器，负责任务管理

### 健康检查

```bash
# 健康检查
curl http://localhost:8081/health

# 就绪检查
curl http://localhost:8081/ready
```

### 指标监控

```bash
# Prometheus 格式指标
curl http://localhost:8082/metrics

# 统计信息
curl http://localhost:8082/stats
```

### 日志级别

支持的日志级别：`debug`, `info`, `warn`, `error`, `fatal`

```bash
# 启动时指定日志级别
./task-processor --log-level=debug
```

### 性能调优

**浏览器池大小**:
```yaml
browser:
  poolSize: 3  # 根据 CPU 核心数调整，建议 CPU核心数 / 2
```

**Worker 并发数**:
```yaml
worker:
  concurrency: 5  # 根据任务类型和资源调整
```

**RabbitMQ 预取数量**:
```yaml
rabbitmq:
  prefetchCount: 1  # 避免单节点积压过多任务
```

## 故障排查

### 常见问题

**问题 1: 连接 RabbitMQ 失败**
- 检查 RabbitMQ 服务是否启动
- 验证连接字符串配置
- 测试网络连通性

**问题 2: 浏览器启动失败**
- 检查 Chrome/Chromium 是否安装
- 验证 `browserPath` 配置
- 检查系统资源是否充足

**问题 3: 任务处理失败**
- 查看错误日志
- 检查外部 API 是否可用
- 验证配置参数是否正确

## 发展路线

### 已完成
- ✅ Amazon → SHEIN/TEMU 商品上架
- ✅ RabbitMQ 分布式任务队列
- ✅ 浏览器池和并发控制
- ✅ 基础监控和健康检查
- ✅ 1688 平台爬虫集成
- ✅ SHEIN/TEMU 自动核价功能
- ✅ SHEIN/TEMU 自动报名营销活动
- ✅ 源平台库存自动监测

### 进行中
- 🚧 1688 → Amazon/SHEIN/TEMU 上架流程
- 🚧 性能优化和稳定性提升
- 🚧 智能定价策略优化

### 规划中
- 📋 沃尔玛平台爬虫
- 📋 速卖通平台爬虫
- 📋 更多目标平台支持
- 📋 AI 智能定价
- 📋 商品质量检测
- 📋 多语言翻译优化
- 📋 销量数据分析
