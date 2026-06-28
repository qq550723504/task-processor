# ListingKit Code Wiki

## 目录
1. [项目概述](#项目概述)
2. [系统架构](#系统架构)
3. [主要模块职责](#主要模块职责)
4. [关键类与函数说明](#关键类与函数说明)
5. [依赖关系](#依赖关系)
6. [项目运行方式](#项目运行方式)

---

## 项目概述

### 项目简介
**ListingKit** 是一个 AI 驱动的跨境电商商品标准化与上架系统，其核心目标是帮助用户将来自不同来源的商品信息，整理成一套可复用、可审核、可差异化改造的标准商品资料包，并进一步上架到主流电商平台。

### 核心功能
- **多来源商品标准化**：支持从 1688、SDS POD 等来源采集商品并转换为统一标准结构
- **AI 驱动内容重构**：使用 LLM 对标题、卖点、描述、属性等进行智能优化
- **多平台资料生成**：面向 Amazon、SHEIN、TEMU 等平台生成适配的资料包
- **商品图片处理流水线**：支持图片审查、主体提取、白底图生成等
- **差异化销售支持**：同一商品可生成多个平台版本的表达
- **多租户架构**：基于 ZITADEL 提供认证、授权和租户隔离
- **分布式任务处理**：使用 RabbitMQ 进行异步任务分发，支持水平扩展

### 技术栈
- **开发语言**：Go 1.25+
- **Web 框架**：Gin
- **消息队列**：RabbitMQ
- **缓存**：Redis
- **工作流引擎**：Temporal
- **数据库**：PostgreSQL / SQLite
- **对象存储**：AWS S3 / 兼容 S3
- **浏览器自动化**：Playwright
- **LLM 集成**：OpenAI / 兼容接口
- **身份认证**：ZITADEL
- **监控指标**：Prometheus

---

## 系统架构

### 整体业务架构

```
┌─────────────────────────────────────────────────────────────┐
│                     ListingKit UI                           │
│          任务创建 / 审核修复 / 最终确认 / 平台草稿与发布      │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                  ZITADEL Auth / Tenant Scope                │
│          登录认证 / 角色授权 / 资源拥有者(Resource Owner)     │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                   ListingKit API / Workflow                 │
│  商品标准化 / AI 重写 / 图片处理 / 平台资料生成 / 校验 / 租户隔离│
└───────────────┬───────────────────────┬─────────────────────┘
                │                       │
                ▼                       ▼
      ┌──────────────────┐     ┌──────────────────┐
      │   来源商品接入    │     │   目标平台适配    │
      │ - 1688           │     │ - Amazon         │
      │ - SDS POD        │     │ - SHEIN          │
      │ - 海外仓(规划中)  │     │ - TEMU           │
      └────────┬─────────┘     └────────┬─────────┘
               │                        │
               └──────────┬─────────────┘
                          ▼
                 ┌────────────────┐
                 │    RabbitMQ    │
                 │   异步任务队列  │
                 └───────┬────────┘
                         ▼
        ┌────────────────────────────────────────────┐
        │       Consumer / Crawler / Worker 集群      │
        │  抓取 / 生成 / 审核资产 / 提交 / 重试恢复    │
        └────────────────────────────────────────────┘
```

### 项目目录结构

```
task-processor/
├── cmd/                          # 应用入口
│   ├── 1688-crawler-api/        # 1688 爬虫 API 服务
│   ├── amazon-crawler-api/      # Amazon 爬虫 API 服务
│   ├── amazon-listing/          # Amazon 上架独立入口
│   ├── listingkit-subscription/ # ListingKit 订阅服务
│   ├── product-listing-api/     # 商品增强/图片处理统一 HTTP API
│   ├── productenrich-api/       # 兼容入口，复用统一 HTTP API
│   ├── shein-address-copy/      # SHEIN 地址复制工具
│   ├── shein-listing/           # SHEIN 上架独立入口
│   ├── temu-listing/            # TEMU 上架独立入口
│   └── task/                    # 已废弃的 legacy polling 入口
├── internal/                    # 私有业务逻辑
│   ├── amazon/                  # Amazon 平台（SP-API 集成）
│   │   ├── api/                 # SP-API 客户端
│   │   ├── model/               # 数据模型
│   │   ├── pipeline/            # 处理流水线
│   │   └── processor.go         # 主处理器
│   ├── amazonlisting/           # Amazon Listing 草稿生成
│   ├── app/                     # 应用层
│   │   ├── bootstrap/           # 应用启动与依赖组装
│   │   ├── consumer/            # RabbitMQ 服务、处理器注册
│   │   ├── httpapi/             # 统一 HTTP API 装配
│   │   ├── runner/              # 处理器/调度器生命周期
│   │   ├── scheduler/           # 定时任务调度器
│   │   └── task/                # 任务分发、队列管理
│   ├── asset/                   # 资产处理（图片、设计稿等）
│   ├── catalog/                 # 目录管理
│   ├── core/                    # 核心基础设施
│   │   ├── config/              # 配置加载、校验
│   │   ├── errors/              # 统一错误类型
│   │   ├── lifecycle/           # 组件生命周期管理
│   │   └── logger/              # 日志管理
│   ├── crawler/                 # 源平台爬虫
│   ├── domain/                  # 领域模型
│   ├── infra/                   # 基础设施适配层
│   │   ├── auth/                # 认证
│   │   ├── httpx/               # HTTP 服务
│   │   ├── rabbitmq/            # RabbitMQ 客户端
│   │   ├── redis/               # Redis 客户端
│   │   └── worker/              # Worker 池实现
│   ├── listingkit/              # ListingKit 核心服务
│   ├── model/                   # 通用数据模型
│   ├── pipeline/                # 通用 Pipeline 框架
│   ├── platforms/               # 平台模块注册
│   │   ├── amazon/              # Amazon 平台模块
│   │   ├── shein/               # SHEIN 平台模块
│   │   └── temu/                # TEMU 平台模块
│   ├── product/                 # 商品领域服务
│   ├── productenrich/           # 商品信息 AI 增强
│   ├── productimage/            # 商品图片处理流水线
│   ├── prompt/                  # Prompt 管理
│   ├── publishing/              # 发布相关
│   ├── shein/                   # SHEIN 平台
│   ├── temu/                    # TEMU 平台
│   └── workspace/               # 工作区管理
├── config/                      # 配置文件
│   ├── config-dev.yaml
│   ├── config-prod.yaml
│   ├── config-test.yaml
│   └── config-task.yaml
├── data/                        # 静态数据
│   ├── prohibited_items_temu.json
│   └── sensitive_words.json
├── deployments/                 # 部署配置
│   ├── docker/
│   └── kubernetes/
├── docs/                        # 文档
├── prompts/                     # LLM Prompt 模板
├── scripts/                     # 脚本工具
├── tests/                       # 测试
├── tools/                       # 工具模块
├── web/                         # 前端应用
│   └── listingkit-ui/           # ListingKit Web 工作台
├── .env.example
├── go.mod
├── go.sum
└── README.md
```

### 核心架构设计原则

1. **分层架构**：清晰的应用层、领域层、基础设施层分离
2. **模块化**：各平台实现独立封装，通过统一接口注册
3. **异步优先**：使用 RabbitMQ 进行任务分发，支持高并发处理
4. **可扩展性**：支持横向扩展，多节点协同工作
5. **多租户隔离**：基于租户 ID 隔离数据和资源

---

## 主要模块职责

### 1. 应用层 (internal/app/)

#### bootstrap 包
**职责**：应用启动引导，负责配置加载、服务初始化、生命周期组件注册

**核心文件**：
- [app.go](file:///d:/code/task-processor/internal/app/bootstrap/app.go) - 应用启动主入口

**关键组件**：
- `ApplicationBootstrap` - 应用引导器
- `buildServices()` - 构建应用服务依赖
- `registerLifecycleComponents()` - 注册生命周期管理组件

#### consumer 包
**职责**：RabbitMQ 消费者服务，负责消息接收、处理器注册、平台运行时管理

**核心功能**：
- 平台模块注册
- 消费者运行时装配
- 任务分发与路由
- 店铺队列管理

#### httpapi 包
**职责**：统一 HTTP API 服务装配，提供 RESTful 接口

**核心文件**：
- [app.go](file:///d:/code/task-processor/internal/app/httpapi/app.go) - HTTP API 运行时

**关键功能**：
- HTTP 服务启动与优雅关闭
- 路由注册
- Worker 池管理

#### scheduler 包
**职责**：定时任务调度，负责核价、库存同步、活动报名等周期性任务

#### task 包
**职责**：任务管理，包括任务获取、分发、状态跟踪

### 2. ListingKit 核心 (internal/listingkit/)

**职责**：ListingKit 主服务，整合商品增强、图片处理、平台适配等功能

**核心文件**：
- [service.go](file:///d:/code/task-processor/internal/listingkit/service.go) - 主服务实现

**主要功能**：
- 商品资料生成与组装
- 平台草稿创建
- 审核与修复工作流
- 任务提交与状态跟踪

**关键数据结构**：
```go
type Service interface {
    // 生成商品资料
    GenerateListing(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    // 获取任务状态
    GetTaskStatus(ctx context.Context, taskID string) (*TaskStatus, error)
    // 提交到平台
    SubmitToPlatform(ctx context.Context, req *SubmitRequest) (*SubmitResponse, error)
    // ... 更多方法
}
```

### 3. 商品信息增强 (internal/productenrich/)

**职责**：使用 LLM 对商品信息进行分析、理解、重构和增强

**核心文件**：
- [service.go](file:///d:/code/task-processor/internal/productenrich/service.go) - 服务接口定义

**主要功能**：
- 商品理解与分析
- 标题、卖点、描述生成
- 变体信息处理
- 质量评分与建议

**关键组件**：
- `ProductUnderstanding` - 商品理解引擎
- `JSONGenerator` - JSON 结构生成器
- `VariantGenerator` - 变体生成器
- `QualityScorer` - 质量评分器
- `EnhancementSuggester` - 优化建议器

### 4. 图片处理 (internal/productimage/)

**职责**：商品图片的审核、处理、优化和发布

**主要功能**：
- 图片内容审查
- 主体提取与抠图
- 白底图生成
- 场景图生成
- 图片优化与压缩
- 水印检测与去除

### 5. 平台模块 (internal/platforms/)

#### Amazon 平台
**职责**：Amazon SP-API 集成，商品上架与管理

**核心组件**：
- SP-API 客户端封装
- 商品类目映射
- Listing 草稿生成
- 价格与库存同步

#### SHEIN 平台
**职责**：SHEIN 平台集成，商品上架、核价、库存同步、活动报名

**核心文件**：
- [module.go](file:///d:/code/task-processor/internal/platforms/shein/module.go) - 平台模块注册

**主要功能**：
- 商品构建与发布
- 自动核价
- 库存同步
- 活动自动报名
- 内容优化与翻译

#### TEMU 平台
**职责**：TEMU 平台集成，商品上架与管理

**主要功能**：
- 商品构建与发布
- 类目与属性映射
- 图片处理与上传
- 价格计算

### 6. 核心基础设施 (internal/core/)

#### config 包
**职责**：配置加载、验证、管理

**核心文件**：
- [config.go](file:///d:/code/task-processor/internal/core/config/config.go) - 配置结构定义

**主要功能**：
- YAML 配置文件加载
- 环境变量绑定
- 配置验证
- 热重载支持

**配置结构**：
```go
type Config struct {
    Logging      LoggingConfig
    Processor    ProcessorConfig
    Worker       WorkerConfig
    OpenAI       OpenAIConfig
    Management   ManagementConfig
    Browser      BrowserConfig
    Amazon       AmazonConfig
    RabbitMQ     *RabbitMQConfig
    Platforms    PlatformsConfig
    // ... 更多配置
}
```

#### logger 包
**职责**：结构化日志管理，支持日志分级、日志文件分割

#### lifecycle 包
**职责**：组件生命周期统一管理，支持优雅启动与关闭

### 7. 基础设施适配 (internal/infra/)

#### rabbitmq 包
**职责**：RabbitMQ 客户端封装，提供连接管理、消息发布与消费

**主要功能**：
- 连接池管理
- 消息发布
- 消费者注册
- 重试与死信队列
- 预取控制

#### redis 包
**职责**：Redis 客户端封装，用于缓存、会话、锁等

#### httpx 包
**职责**：HTTP 服务工具，提供健康检查、指标采集等

#### auth 包
**职责**：认证相关，包括 Token 获取、会话管理

### 8. 工具库 (internal/pkg/)

包含各种通用工具函数：
- `strx` - 字符串处理
- `timex` - 时间格式化
- `mathx` - 数学计算
- `jsonx` - JSON 处理
- `httpclient` - HTTP 客户端封装
- `imagex` - 图片处理工具
- `watermark` - 水印检测与去除
- `downloader` - 文件下载
- `resilience` - 熔断器与重试
- `cache` - 缓存工具
- `skugen` - SKU 生成器
- `hashx` - 哈希工具
- `fileio` - 文件 I/O
- `ptr` - 指针辅助函数
- `goroutine` - 协程安全工具
- `timeout` - 超时控制
- `perf` - 性能工具

---

## 关键类与函数说明

### 应用启动流程

#### 1. HTTP API 入口 ([main.go](file:///d:/code/task-processor/cmd/product-listing-api/main.go))

```go
func main() {
    // 解析命令行参数
    flag.Parse()
    
    // 初始化日志
    logger := appenv.SetupLoggerWithLevel(*logLevel)
    
    // 启动 HTTP API 服务
    if err := start(logger, httpapi.Options{
        ConfigPath: *configPath,
        Port:       *port,
    }); err != nil {
        logger.Fatalf("service start failed: %v", err)
    }
}
```

#### 2. 应用引导初始化

`ApplicationBootstrap.Initialize()` 流程：
1. 加载配置文件
2. 初始化共享资源
3. 构建应用服务
4. 注册生命周期组件

### 配置管理

#### Config 结构 ([config.go](file:///d:/code/task-processor/internal/core/config/config.go))

```go
type Config struct {
    Logging      LoggingConfig      // 日志配置
    Processor    ProcessorConfig    // 处理器配置
    Worker       WorkerConfig       // Worker 配置
    OpenAI       OpenAIConfig       // OpenAI 配置
    Management   ManagementConfig   // 管理 API 配置
    Browser      BrowserConfig      // 浏览器配置
    Amazon       AmazonConfig       // Amazon 配置
    RabbitMQ     *RabbitMQConfig    // RabbitMQ 配置
    Platforms    PlatformsConfig    // 平台配置
    // ... 更多配置项
}
```

#### 配置加载

```go
// 从文件加载配置
func LoadConfigFromFile(configPath string) (*Config, error)

// 构建配置
func BuildConfig(v *viper.Viper) *Config

// 验证配置
func (c *Config) ValidateWithError() error
```

### 平台模块系统

#### 平台模块接口

```go
type PlatformModule interface {
    // 模块名称
    Name() string
    
    // 是否启用
    Enabled(cfg *config.Config) bool
    
    // 注册消费者
    RegisterConsumer(ctx context.Context, rt PlatformRuntimeContext, registry ProcessorRegistrar) error
    
    // 配置运行时
    ConfigureListingRuntime(ctx context.Context, rt PlatformRuntimeContext) error
}
```

#### 模块注册 ([modules.go](file:///d:/code/task-processor/internal/platforms/modules.go))

```go
func All() []consumer.PlatformModule {
    return []consumer.PlatformModule{
        platformamazon.NewModule(),
        temu.NewModule(),
        shein.NewModule(),
    }
}
```

#### SHEIN 模块示例 ([module.go](file:///d:/code/task-processor/internal/platforms/shein/module.go))

```go
func (m Module) RegisterConsumer(ctx context.Context, rt consumer.PlatformRuntimeContext, registry consumer.ProcessorRegistrar) error {
    // 使用运行时装配好的商品获取器
    productFetcher := rt.ProductFetcher
    
    // 创建处理器
    processor, err := pipeline.NewSheinProcessor(ctx, rt.Config, rt.Logger, ...)
    
    // 注册处理器
    return registry.RegisterProcessor(m.Name(), processor)
}
```

### ListingKit 服务

#### Service 接口 ([service.go](file:///d:/code/task-processor/internal/listingkit/service.go))

```go
type Service interface {
    // 生成商品资料
    GenerateListing(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error)
    
    // 获取任务状态
    GetTaskStatus(ctx context.Context, taskID string) (*TaskStatus, error)
    
    // 提交到平台
    SubmitToPlatform(ctx context.Context, req *SubmitRequest) (*SubmitResponse, error)
    
    // 设置任务提交器
    SetTaskSubmitter(submitter TaskSubmitter)
    
    // ... 更多方法
}
```

#### 服务创建

```go
func NewService(config *ServiceConfig) (Service, error) {
    // 验证必需依赖
    if config.Repository == nil {
        return nil, fmt.Errorf("repository cannot be nil")
    }
    if config.ProductService == nil {
        return nil, fmt.Errorf("product service cannot be nil")
    }
    
    // 初始化默认依赖
    // ...
    
    return &service{
        repo:               config.Repository,
        productSvc:         config.ProductService,
        imageSvc:           config.ImageService,
        assembler:          config.Assembler,
        // ... 更多字段
    }, nil
}
```

### ProductEnrich 服务

#### ProductService 接口 ([service.go](file:///d:/code/task-processor/internal/productenrich/service.go))

```go
type ProductService interface {
    // 创建生成任务
    CreateGenerateTask(ctx context.Context, req *GenerateRequest) (*Task, error)
    
    // 获取任务结果
    GetTaskResult(ctx context.Context, taskID string) (*TaskResult, error)
    
    // 处理商品
    ProcessProduct(ctx context.Context, task *Task) (*ProductJSON, error)
    
    // 设置任务提交器
    SetTaskSubmitter(submitter TaskSubmitter)
}
```

#### 服务能力配置

```go
type ProductServiceCapabilities struct {
    Mode                           CapabilityMode
    AllowSimpleInputParsing        bool
    AllowDefaultValidationStrategy bool
    AllowSimpleAnalysis            bool
    AllowSimpleGeneration          bool
    AllowMissingResultValidator    bool
}
```

### 流水线框架

#### Pipeline 接口

```go
type Pipeline interface {
    // 添加处理器
    AddHandler(handler Handler) Pipeline
    
    // 执行流水线
    Execute(ctx context.Context, input interface{}) (interface{}, error)
}
```

#### Handler 接口

```go
type Handler interface {
    // 处理输入
    Handle(ctx context.Context, input interface{}) (interface{}, error)
    
    // 处理器名称
    Name() string
}
```

---

## 依赖关系

### 主要外部依赖

| 依赖包 | 版本 | 用途 |
|--------|------|------|
| github.com/gin-gonic/gin | v1.10.1 | Web 框架 |
| github.com/rabbitmq/amqp091-go | v1.10.0 | RabbitMQ 客户端 |
| github.com/redis/go-redis/v9 | v9.18.0 | Redis 客户端 |
| github.com/sashabaranov/go-openai | v1.41.1 | OpenAI API 客户端 |
| github.com/playwright-community/playwright-go | v0.5700.1 | 浏览器自动化 |
| go.temporal.io/sdk | v1.43.0 | Temporal 工作流 |
| gorm.io/gorm | v1.31.1 | ORM 框架 |
| gorm.io/driver/postgres | v1.6.0 | PostgreSQL 驱动 |
| github.com/aws/aws-sdk-go-v2 | v1.40.1 | AWS SDK |
| github.com/prometheus/client_golang | v1.23.2 | Prometheus 指标 |
| github.com/sirupsen/logrus | v1.9.4 | 结构化日志 |
| github.com/spf13/viper | v1.16.0 | 配置管理 |
| github.com/stretchr/testify | v1.11.1 | 测试框架 |
| github.com/sony/gobreaker | v1.0.0 | 熔断器 |
| github.com/cenkalti/backoff/v5 | v5.0.3 | 重试退避 |
| github.com/disintegration/imaging | v1.6.2 | 图片处理 |

### 内部模块依赖关系

```
cmd/
├── product-listing-api
│   └── depends on → internal/app/httpapi
│                     └── depends on → internal/app/bootstrap
│                                       ├── internal/core/config
│                                       ├── internal/core/lifecycle
│                                       ├── internal/infra/*
│                                       ├── internal/listingkit
│                                       ├── internal/productenrich
│                                       ├── internal/productimage
│                                       └── internal/platforms/*

internal/
├── listingkit
│   ├── depends on → internal/productenrich
│   ├── depends on → internal/productimage
│   ├── depends on → internal/amazonlisting
│   ├── depends on → internal/asset
│   └── depends on → internal/prompt

├── productenrich
│   ├── depends on → internal/prompt
│   └── depends on → internal/core/*

├── platforms
│   ├── amazon → internal/amazon
│   ├── shein → internal/shein
│   └── temu → internal/temu

└── core
    ├── config
    ├── errors
    ├── lifecycle
    └── logger
```

---

## 项目运行方式

### 环境要求

- Go 1.25+
- PostgreSQL 13+（可选，用于数据持久化）
- Redis 6+（用于缓存、会话、锁）
- RabbitMQ 3.8+（用于异步任务）
- Temporal Server（可选，用于长流程工作流）
- ZITADEL（可选，用于多租户认证）

### 配置文件

主要配置文件位于 `config/` 目录：
- `config-dev.yaml` - 开发环境配置
- `config-prod.yaml` - 生产环境配置
- `config-test.yaml` - 测试环境配置
- `config-task.yaml` - Legacy 任务模式配置

#### 关键配置项

```yaml
# 日志配置
logging:
  level: "DEBUG"
  format: "text"
  file: "tmp/logs/app.log"

# Worker 配置
worker:
  concurrency: 1
  bufferSize: 30
  taskInterval: 60

# OpenAI 配置
openai:
  apiKey: "your-api-key"
  model: "gemini-2.5-flash"
  baseURL: "https://api.example.com/v1"
  timeout: 300

# 平台配置
platforms:
  shein:
    enabled: true
    schedulerEnabled: false
    autoPricing:
      enabled: false
      interval: 300

  temu:
    enabled: false
    schedulerEnabled: false

# RabbitMQ 配置
rabbitmq:
  enabled: true
  url: "amqp://user:password@localhost:5672/"
  consumer:
    prefetchCount: 1

# Redis 配置
redis:
  host: "localhost"
  port: 6379
  password: ""
  db: 0

# 浏览器配置
browser:
  enabled: true
  headless: false
  poolSize: 3
```

### 启动服务

#### 1. 启动 ListingKit API（推荐）

```bash
# 开发环境
go run ./cmd/product-listing-api -config config/config-dev.yaml -port 8085

# 指定日志级别
go run ./cmd/product-listing-api -config config/config-dev.yaml -log-level debug
```

#### 2. 启动平台消费者

```bash
# SHEIN 消费者
go run ./cmd/shein-listing -config config/config-dev.yaml

# TEMU 消费者
go run ./cmd/temu-listing -config config/config-dev.yaml

# Amazon 消费者
go run ./cmd/amazon-listing -config config/config-dev.yaml
```

#### 3. 启动爬虫 API（可选）

```bash
# Amazon 爬虫 API
go run ./cmd/amazon-crawler-api -config config/config-dev.yaml

# 1688 爬虫 API
go run ./cmd/1688-crawler-api -config config/config-dev.yaml
```

### 健康检查与监控

#### 健康检查端点

```bash
# 健康检查
curl http://localhost:8081/health

# 就绪检查
curl http://localhost:8081/ready
```

#### 指标端点

```bash
# Prometheus 格式指标
curl http://localhost:8082/metrics

# 统计信息
curl http://localhost:8082/stats
```

### Docker 部署

#### 构建镜像

```bash
docker build -f deployments/docker/Dockerfile.task -t listingkit-task:latest .
```

#### Docker Compose

```yaml
version: '3.8'
services:
  rabbitmq:
    image: rabbitmq:3-management-alpine
    ports:
      - "5672:5672"
      - "15672:15672"
  
  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
  
  postgres:
    image: postgres:15-alpine
    environment:
      POSTGRES_USER: listingkit
      POSTGRES_PASSWORD: listingkit
      POSTGRES_DB: listingkit
    ports:
      - "5432:5432"
  
  product-listing-api:
    image: listingkit-task:latest
    command: ["./product-listing-api", "-config", "/app/config/config-prod.yaml"]
    ports:
      - "8085:8085"
    depends_on:
      - rabbitmq
      - redis
      - postgres
  
  shein-listing:
    image: listingkit-task:latest
    command: ["./shein-listing", "-config", "/app/config/config-prod.yaml"]
    depends_on:
      - rabbitmq
      - redis
```

### 开发流程

#### 1. 本地开发环境设置

```bash
# 克隆代码
git clone https://github.com/qq550723504/task-processor.git
cd task-processor

# 安装依赖
go mod download

# 安装 Playwright 浏览器
go run github.com/playwright-community/playwright-go/cmd/playwright install

# 复制配置
cp config/config-dev.yaml config/local.yaml
# 编辑 config/local.yaml，填入你的配置
```

#### 2. 运行测试

```bash
# 运行所有测试
go test ./...

# 运行特定包的测试
go test ./internal/app/scheduler/...

# 运行测试并显示覆盖率
go test -cover ./...
```

#### 3. 调试

使用 `hack/debug/` 目录下的调试程序：

```bash
# 测试 Amazon 爬虫
go run ./hack/debug/test-amazon

# 测试 1688 爬虫
go run ./hack/debug/test-1688

# 测试图片分析
go run ./hack/debug/test-analyzeimage
```

### 常见问题排查

#### RabbitMQ 连接失败
- 检查 RabbitMQ 服务是否启动
- 验证连接字符串配置
- 测试网络连通性

#### 浏览器启动失败
- 检查 Chrome/Chromium 是否安装
- 验证 `browserPath` 配置
- 检查系统资源是否充足

#### 任务处理失败
- 查看错误日志
- 检查外部 API 是否可用
- 验证配置参数是否正确

---

## 附录

### 相关文档

- [README.md](file:///d:/code/task-processor/README.md) - 项目主文档
- [docs/architecture/](file:///d:/code/task-processor/docs/architecture/) - 架构文档
- [docs/product/](file:///d:/code/task-processor/docs/product/) - 产品文档
- [.github/pull_request_template.md](file:///d:/code/task-processor/.github/pull_request_template.md) - PR 模板

### 代码规范

- 遵循 Go 官方代码规范
- 使用 `gofmt` 格式化代码
- 使用 `golangci-lint` 进行代码检查
- 编写单元测试
- 添加必要的注释

### 贡献指南

1. Fork 项目
2. 创建特性分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

---

*文档版本：1.0*  
*最后更新：2026-05-21*
