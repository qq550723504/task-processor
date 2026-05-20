# ListingKit - AI 驱动的跨境商品标准化与上架系统

[![CI](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml/badge.svg)](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.24+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

ListingKit 是当前项目的主程序与产品主入口，目标是帮助用户把来自不同来源的商品信息，整理成一套可复用、可审核、可差异化改造的标准商品资料包，并进一步上架到主流电商平台。

当前系统重点支持将 `1688` 商品、`SDS` 的 POD 商品整理为统一商品资料；后续会继续接入大建云仓等海外仓商品源。基于这些来源商品，ListingKit 会结合 AI 能力对商品标题、卖点、属性、图片和平台资料进行重构，让同一商品可以面向 Amazon、SHEIN、TEMU 等平台生成适配后的上架内容，用于差异化销售。

ListingKit 同时支持多租户隔离，已接入 `ZITADEL` 作为身份认证与租户边界基础设施。系统会围绕租户维度管理用户访问、任务数据、工作台会话和部分配置能力，适合多个业务主体在同一套系统中独立使用。

项目底层仍保留分布式任务处理、平台抓取、图片流水线和平台提交流程；但从产品视角看，当前核心已经不是一个通用的 `task processor`，而是一套围绕 `ListingKit` 展开的商品标准化、多租户工作台与多平台上架系统。

## 业务场景

### 核心业务流程

1. **导入来源商品**
   - 从 `1688` 商品链接导入现货型商品资料
   - 从 `SDS POD` 商品导入底版、变体、模板图、mockup 等设计生产信息
   - 后续扩展到大建云仓等海外仓商品源
2. **商品标准化**
   - 抽取标题、描述、属性、价格、规格、图片、变体等原始信息
   - 统一转换成平台无关的标准商品结构，便于后续多平台复用
3. **AI 重构与资料增强**
   - 通过 AI 重写标题、卖点、描述、属性表达和平台文案
   - 对图片、主图素材、白底图、场景图进行处理和重组
   - 结合平台规则生成更适合目标渠道的差异化资料包
4. **平台适配与审核**
   - 将标准商品映射到 Amazon、SHEIN、TEMU 等平台字段
   - 在 ListingKit 工作台中检查阻断项、修复资料并确认最终稿
5. **上架与分发**
   - 保存平台草稿或正式发布
   - 支持多店铺、多平台复用同一来源商品的差异化上架

### 平台字段约定

- `sourcePlatform` / `source_platform`: 来源抓取平台，例如 `amazon`、`1688`
- `targetPlatform` / `target_platform`: 目标上架平台，例如 `shein`、`temu`、`amazon`
- `platform`: 兼容字段，在 `task-processor` 任务模型和成功回执里默认等于目标上架平台
- RabbitMQ `TaskMessage.platform`: 历史上表示来源平台，排查消息体时需要和 `targetPlatform` 一起看

### 当前来源与目标平台

**来源商品**

- `1688`：适合现货型商品采集、标准资料抽取、多平台改写
- `SDS POD`：适合带设计图、底版、变体和 mockup 的按需定制商品
- 规划中：大建云仓等海外仓商品源

**目标平台**

- `Amazon`
- `SHEIN`
- `TEMU`
- 后续可继续扩展更多主流电商平台

### 差异化销售能力

ListingKit 的核心价值不是简单搬运商品，而是把同一个来源商品重构成多个可销售版本：

- 基于 AI 生成不同平台风格的标题、卖点和详情文案
- 对主图、场景图、白底图、设计图进行重新组织和平台适配
- 针对不同平台的类目、属性、图片规范、价格策略生成差异化资料
- 让同一来源商品可以在 Amazon、SHEIN、TEMU 等渠道形成不同表达，降低同质化

### 多租户与身份体系

- ListingKit 是多租户系统，核心任务、工作台会话、上传资产和部分配置按租户隔离
- 已接入 `ZITADEL`，用于认证、租户识别、角色授权和会话代理
- 前端 UI、代理层和 Go API 会基于 `ZITADEL` 的身份信息传递租户上下文
- 适合服务多个团队、客户或业务线，并在同一平台中保持数据边界清晰

更具体地说：

- `ZITADEL resource owner` 是当前租户边界的主要来源，ListingKit 会基于这个值识别当前用户属于哪个租户
- 前端 `ListingKit UI` 会先完成 `ZITADEL` 登录，代理层验证 session 或 bearer token 后，再把确认过的身份信息转发给 Go API
- 转发的信息不仅包含用户身份，也包含租户标识和角色语义，后端再将其注入请求上下文，用于任务、工作台、资产和配置的访问控制
- 当前 README 不展开具体角色枚举，但系统设计上已经按“认证 + 授权 + 租户隔离”三层来组织，而不是只做一个登录门禁
- 这套机制既支持同租户内多人协作，也为后续更细粒度的 owner scope、租户级配置覆盖、租户级 prompt 管理预留了基础能力

### 部署形态

- `ListingKit UI` 提供任务创建、审核、修复、确认和提交工作台
- `ZITADEL` 提供登录认证、租户识别和访问控制
- `product-listing-api` 提供统一资料生成、商品增强、图片处理等 HTTP 能力
- `shein-listing` / `temu-listing` / `amazon-listing` 负责各目标平台的消费与提交
- `1688-crawler-api` / `amazon-crawler-api` 负责来源商品抓取
- RabbitMQ 负责异步任务分发，支持分布式扩容和批量处理

## 核心特性

- **多来源商品标准化**
  - 当前重点支持 `1688` 与 `SDS POD`
  - 将不同源信息归一为统一商品结构，便于后续复用和平台适配
- **多平台 Listing 资料生成**
  - 面向 `Amazon`、`SHEIN`、`TEMU` 生成目标平台资料包
  - 平台字段映射、类目/属性适配、SKU 与图片组织可独立演进
- **AI 驱动的内容重构**
  - 标题、卖点、描述、属性表达智能优化
  - 补全缺失信息并对结果进行质量校验
- **商品图片重构**
  - 支持主图、白底图、场景图、审核图等处理流水线
  - 适合 POD 商品的设计图、mockup 和素材重组
- **差异化销售支持**
  - 同一来源商品可生成多个平台版本与不同表达方式
  - 降低简单搬运式上架带来的同质化问题
- **审核与修复工作台**
  - 运营可在 ListingKit 中查看自动化结果、修复阻断项、确认最终稿
  - 适合把 AI 结果转成可控、可复核的上架资料
- **多租户架构**
  - 基于租户维度隔离任务、会话、资产和部分配置
  - 已接入 `ZITADEL` 进行认证、授权和租户上下文传递
- **分布式处理架构**
  - RabbitMQ 消息队列驱动，支持异步生成与水平扩展
  - 拆分 crawler、listing consumer、统一 API，便于独立扩缩容
- **长流程工作流编排**
  - Temporal 作为长流程 durable workflow 底座，当前 PoC 重点覆盖 SHEIN `publish`
  - 负责阶段推进、重试恢复、跨节点续跑和运行态查询，不替代具体平台业务规则
- **平台运营能力**
  - SHEIN、TEMU 当前已接入核价、库存同步、活动报名等运营自动化能力
  - Amazon 目标平台当前以资料生成与上架提交流程为主，不默认表示已具备同等运营自动化覆盖
- **监控与容错**
  - 提供健康检查、指标监控、自动重试和故障恢复能力
- **浏览器与抓取基础设施**
  - 复用浏览器实例，提高抓取与平台交互效率

## 架构设计

补充架构文档：

- [任务状态流转说明](./docs/architecture/task-status-lifecycle.md)
- [HTTP API 装配边界](./docs/architecture/httpapi-assembly-boundaries.md)
- [中心化部署改造方案](./docs/architecture/centralized-deployment-plan.md)
- [Amazon Crawler API 说明](./cmd/amazon-crawler-api/README.md)
- [Amazon 爬虫商用化清单](./docs/architecture/amazon-crawler-commercialization-checklist.md)
- [Amazon 爬虫商用化执行计划](./docs/architecture/amazon-crawler-commercialization-execution-plan.md)
- [ListingKit 对象存储开发说明](./docs/development/listingkit-object-storage.md)
- [ListingKit 产品总览](./docs/product/listingkit-product-overview.md)
- [ListingKit 操作指南](./docs/product/listingkit-operating-guide.md)
- [ListingKit Temporal PoC Runbook](./docs/architecture/temporal-poc-runbook.md)
- [ListingKit Temporal 工作流评估](./docs/architecture/temporal-workflow-evaluation.md)

### ListingKit Temporal PoC

- 当前 Temporal 的定位不是替换 RabbitMQ，也不是重写 ListingKit 现有业务逻辑，而是承接“长流程编排”这一层。
- 现阶段 PoC 重点覆盖 `shein + publish`：
  - RabbitMQ 仍主要负责普通异步任务分发和平台消费者处理
  - Temporal 主要负责提交链路里的 durable execution、阶段状态持久化、activity 重试、worker 重启后的续跑，以及后续 signal/query 扩展能力
  - 商品 payload 组装、图片上传、SHEIN 远端接口调用、结果落库仍复用现有 ListingKit submit 逻辑
- 这意味着 Temporal 当前解决的是“流程怎么可靠推进和恢复”，不是“平台规则怎么实现”。
- API 侧启用 Temporal 提交流程：`LISTINGKIT_TEMPORAL_ENABLED=true`
- 若要把 worker 从 `product-listing-api` 进程里拆开，给 API 增加：`LISTINGKIT_TEMPORAL_START_WORKER=false`
- 独立 worker 入口：`go run ./cmd/listingkit-temporal-worker -config config/config-dev.yaml`
- 本地没有 `temporal` CLI 时，可直接用 Docker 脚本启动开发服务：`.\scripts\start-temporal-dev.ps1`
- 详细运行方式见 [docs/architecture/temporal-poc-runbook.md](./docs/architecture/temporal-poc-runbook.md)

### 推荐部署形态

当前更推荐按平台消费者拆开部署：

- `product-listing-api` 作为统一 ListingKit API，负责商品增强、图片处理和资料生成，是推荐拓扑中的基础服务
- `shein-listing` / `temu-listing` / `amazon-listing` 负责从 RabbitMQ 消费上架任务
- `amazon-crawler-api` 在 Amazon 来源抓取场景下部署；只处理 `1688` / `SDS POD` 来源时可不启用
- `1688-crawler-api` 在需要独立 1688 抓取服务时部署；若当前链路未拆分到独立服务，可暂不启用
- 平台消费者优先通过 HTTP 调用 crawler / listing API，而不是在本地直接持有所有抓取与生成能力
- `cmd/task` 已降级为 legacy emergency fallback，不作为生产主入口

对应默认配置文件：

- `product-listing-api` / `shein-listing` / `temu-listing` / `amazon-listing`: [config-dev.yaml](./config/config-dev.yaml) 或对应环境配置
- `amazon-crawler-api`: [config-amazon-crawler-api.yaml](./config/config-amazon-crawler-api.yaml)
- `task`: [config-task.yaml](./config/config-task.yaml) 仅用于 legacy fallback

启动示例：

```bash
go run ./cmd/product-listing-api -config config/config-dev.yaml
go run ./cmd/shein-listing -config config/config-dev.yaml
go run ./cmd/temu-listing -config config/config-dev.yaml
go run ./cmd/amazon-listing -config config/config-dev.yaml
# 仅在 Amazon 来源抓取场景下需要
go run ./cmd/amazon-crawler-api -config config/config-amazon-crawler-api.yaml
```

`shein-listing` / `temu-listing` / `amazon-listing` 仍兼容旧参数 `-app-config`；同时传入时 `-config` 优先生效。

### 整体业务架构

```text
┌─────────────────────────────────────────────────────────────┐
│                         ListingKit UI                       │
│      任务创建 / 审核修复 / 最终确认 / 平台草稿与发布           │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                  ZITADEL Auth / Tenant Scope                │
│        登录认证 / 角色授权 / 资源拥有者(Resource Owner)       │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                    ListingKit API / Workflow                │
│ 商品标准化 / AI 重写 / 图片处理 / 平台资料生成 / 校验 / 租户隔离 │
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

### 单节点系统架构

```text
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
│   Source / Listing Processors           │
│  ┌────────┐ ┌────────┐ ┌────────┐      │
│  │ Amazon │ │ 1688   │ │  SDS   │      │
│  │ Crawler│ │ Crawler│ │  Sync  │      │
│  └────────┘ └────────┘ └────────┘      │
│  ┌────────┐ ┌────────┐ ┌────────┐      │
│  │ SHEIN  │ │ TEMU   │ │ Amazon │      │
│  │Uploader│ │Uploader│ │Uploader│      │
│  └────────┘ └────────┘ └────────┘      │
│  ┌──────────────┐                       │
│  │ Image/AI Flow│                       │
│  └──────────────┘                       │
└─────────────────────────────────────────┘
           │              │
           ▼              ▼
┌─────────────────────────────────────────┐
│         Business Logic Layer            │
│  ┌──────────┐  ┌──────────┐            │
│  │ Standard │  │Platform  │            │
│  │ Product  │  │ Mapping  │            │
│  │ Builder  │  │ & Review │            │
│  └──────────┘  └──────────┘            │
│  ┌──────────┐  ┌──────────┐            │
│  │ Pricing  │  │Inventory │            │
│  │ Engine   │  │ Monitor  │            │
│  └──────────┘  └──────────┘            │
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

- **ListingKit UI / Workspace**: 面向运营的任务创建、审核修复、最终确认与提交工作台
- **ZITADEL Identity Layer**: 提供认证、授权、租户识别和资源拥有者上下文
- **HTTP API Runtime**: 统一 HTTP 入口，负责 `productenrich`、`productimage`、`amazonlisting` 及 ListingKit 相关能力装配
- **Standard Product Builder**: 将来源商品抽取为统一标准商品模型，作为多平台复用基座
- **AI Enrichment**: 对标题、卖点、属性、描述和平台表达进行增强与重写
- **Product Image Pipeline**: 负责图片审查、主体提取、白底图生成、审核与资产发布
- **Source Crawlers / Sync Services**: 来源商品接入能力，当前重点包含 Amazon、1688 与 SDS 相关同步
- **Platform Uploaders**: 平台上架器，将标准商品资料提交到 SHEIN、TEMU、Amazon 等目标平台
- **Review and Readiness Gate**: 在正式提交前识别阻断项、校验资料完整性、控制最终稿确认
- **Scheduler Manager**: 调度核价、库存同步、活动报名等周期性运营任务
- **RabbitMQ Consumer**: 消费异步任务，驱动抓取、生成、审核资产处理与提交
- **Worker Pool**: 管理并发任务执行
- **Browser Pool**: 复用 Chrome 实例，提高抓取与平台交互效率
- **Lifecycle Manager**: 统一管理组件启停

## 开发指南

### 项目结构

```
task-processor/
├── web/                         # 前端应用
│   └── listingkit-ui/          # ListingKit Web 工作台
├── cmd/                          # 应用入口（每个子目录一个 main.go）
│   ├── task/                    # 已废弃的 legacy polling 入口（仅应急回滚）
│   ├── 1688-crawler-api/        # 1688 爬虫 HTTP API 服务
│   ├── amazon-crawler-api/      # Amazon 爬虫 HTTP API 服务
│   ├── amazon-listing/          # Amazon 上架独立入口
│   ├── shein-listing/           # SHEIN 上架独立入口
│   ├── temu-listing/            # TEMU 上架独立入口
│   ├── product-listing-api/     # 商品增强/图片处理/Amazon Listing 统一 HTTP API
│   ├── productenrich-api/       # 兼容入口，复用统一 HTTP API 装配
│   └── shein-address-copy/      # SHEIN 地址复制工具
├── internal/                    # 私有业务逻辑（Go 编译器强制不对外暴露）
│   ├── app/                     # 应用层：启动、调度、消息、任务编排
│   │   ├── runtime/             # 服务启动生命周期（listing/crawler/httpapi）
│   │   ├── bootstrap/           # 应用启动与依赖组装（resources/fetchers/processors/schedulers）
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
│   │   ├── task/                # 任务规范化模型（source/target platform route）
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
│   ├── platforms/               # 平台模块注册（SHEIN/TEMU/Amazon）
│   ├── productimage/            # 商品图片处理流水线（domain/providers/pipeline/store/api）
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
├── hack/                        # 调试、试验、验证程序
│   ├── debug/                   # 非生产调试入口（不纳入 cmd 发布面）
│   └── makefiles/               # 本地开发辅助 makefile 片段
├── tools/                       # 长期维护的小工具模块
│   ├── go.mod                   # 工具依赖模块
│   ├── json-simplifier/         # 独立工具
├── .local/                      # 本地日志、临时文件、浏览器状态、开发产物
└── examples/                    # 示例代码
```

### 重构后边界约定

- `cmd/*-listing` 只保留入口胶水；公共启动逻辑在 `internal/app/runtime/listing`。
- `cmd/` 只保留正式服务入口；调试可执行程序统一放在 `hack/debug/`。
- `internal/app/bootstrap` 只做装配，资源、fetcher、processor、scheduler 分别放在子包里。
- `internal/app/consumer` 只依赖 `PlatformModule` 接口，不直接 import SHEIN/TEMU/Amazon 实现。
- 新增平台时优先在 `internal/platforms/{platform}` 增加模块，并加入 `platforms.All()`。
- RabbitMQ 任务消息先经过 `internal/domain/task.NormalizeTaskMessage`，业务层使用标准化后的 source/target 语义。
- `productimage` 对外仍保留原包 API，新增领域类型在 `productimage/domain`，provider 接口在 `productimage/providers`。
- 本地日志、浏览器状态、临时文件、开发期二进制产物默认进入 `.local/`，避免继续污染仓库根目录。
- `tools/` 只保留长期维护的小工具；调试/试验程序放在 `hack/`。

更多目录约定与后续演进方向见 [docs/development/repository-structure.md](./docs/development/repository-structure.md)。

### 添加新平台

1. 在 `internal/platforms/{platform}` 下创建模块目录（如 `internal/platforms/walmart/`）
2. 实现 `consumer.PlatformModule`，在模块内部装配平台 processor
3. 在 `internal/platforms/{platform}` 中实现平台模块，并加入 `internal/platforms/modules.go`
4. 在配置文件中添加平台配置

示例：

```go
type Module struct{}

func (Module) Name() string { return "walmart" }

func (Module) Enabled(cfg *config.Config) bool {
    return cfg.Platforms.Walmart.Enabled
}

func (Module) RegisterConsumer(ctx context.Context, rt consumer.PlatformRuntimeContext, registry consumer.ProcessorRegistrar) error {
    processor := walmart.NewProcessor(ctx, rt.Config, rt.Logger)
    return registry.RegisterProcessor("walmart", processor)
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

### Phase 1 基础设施说明

- 指标导出已统一走 Prometheus client registry；运维侧继续通过 `/metrics` 抓取即可，不需要兼容旧的手写文本拼装实现。
- 限流、重试退避、断路器已收敛到统一基础包；新增外部依赖调用时，优先复用这层，而不是在业务包内继续自实现重试或熔断。
- 自动更新逻辑已整理到统一接入层；后续如果替换底层自更新库，应优先在这一层内调整，避免把版本检查、下载和替换策略重新散落到业务代码。

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
- ✅ ListingKit 作为主产品入口与工作台
- ✅ `1688` 来源商品接入与标准资料抽取
- ✅ `SDS POD` 商品链路接入与图片/设计资料处理
- ✅ Amazon / SHEIN / TEMU 目标平台资料生成与提交基础能力
- ✅ 多租户租户隔离基础能力与 `ZITADEL` 接入
- ✅ RabbitMQ 分布式任务队列
- ✅ 浏览器池和并发控制
- ✅ 基础监控和健康检查
- ✅ SHEIN/TEMU 自动核价功能
- ✅ SHEIN/TEMU 自动报名营销活动
- ✅ 源平台库存自动监测

### 进行中
- 🚧 标准商品模型与多平台资料包进一步统一
- 🚧 1688 / SDS 到 Amazon / SHEIN / TEMU 的完整产品化链路持续完善
- 🚧 商品图片与文案的 AI 差异化重构能力增强
- 🚧 性能优化和稳定性提升
- 🚧 智能定价策略优化

### 规划中
- 📋 大建云仓等海外仓商品源接入
- 📋 沃尔玛平台爬虫
- 📋 速卖通平台爬虫
- 📋 更多目标平台支持
- 📋 更多海外仓和现货/POD 混合商品模式支持
- 📋 AI 智能定价
- 📋 商品质量检测
- 📋 多语言翻译优化
- 📋 销量数据分析
