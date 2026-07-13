# ListingKit - AI 驱动的跨境商品标准化与上架系统

[![CI](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml/badge.svg)](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

ListingKit 是当前项目的主程序与产品主入口，目标是帮助用户把来自不同来源的商品信息，整理成一套可复用、可审核、可差异化改造的标准商品资料包，并进一步上架到主流电商平台。

当前系统重点支持 `SHEIN` 主链路稳定化，并将 `1688` 商品、`SDS POD` 商品整理为统一商品资料。系统已经具备 Product Sourcing 的中立建模、Amazon/1688 source envelope、catalog/asset facts handoff，以及到 ListingKit 任务创建边界的窄桥接；下一步重点是完成当前基线的验证和受控真实链路闭环，而不是同时扩多个新来源。

ListingKit 会结合 AI 能力对商品标题、卖点、属性、图片和平台资料进行重构，让同一来源商品可以面向 SHEIN 等目标平台生成适配后的上架内容，用于差异化销售。Amazon、TEMU 等目标平台相关代码和 runtime 资产仍保留，但完整新平台工作台扩张目前应视为 deferred，不能按 SHEIN 主链路成熟度来理解。

ListingKit 同时支持多租户隔离，已接入 `ZITADEL` 作为身份认证与租户边界基础设施。系统会围绕租户维度管理用户访问、任务数据、工作台会话和部分配置能力，适合多个业务主体在同一套系统中独立使用。

项目底层仍保留分布式任务处理、平台抓取、图片流水线和平台提交流程；但从产品视角看，当前核心已经不是一个通用的 `task processor`，而是一套围绕 `ListingKit` 展开的商品标准化、多租户工作台与多平台上架系统。

## Architecture and refactoring authority

Current project status should start from [`docs/refactoring/current-refactoring-status.md`](./docs/refactoring/current-refactoring-status.md), then [`docs/refactoring/next-phase-plan.md`](./docs/refactoring/next-phase-plan.md), [`docs/product/product-sourcing-mvp-plan.md`](./docs/product/product-sourcing-mvp-plan.md), and [`docs/refactoring/project-wide-refactoring-plan.md`](./docs/refactoring/project-wide-refactoring-plan.md).

When later refactoring work conflicts with older local plans or ad-hoc package decisions, use the current status document as the Now / Next / Later source of truth, and use the project-wide plan for long-term architecture direction unless a newer ADR explicitly supersedes it.

In particular, future changes should preserve these rules:

- `app` packages assemble runtime dependencies and should not own business rules.
- ListingKit should remain an orchestration and compatibility facade, not absorb new marketplace-specific rules.
- Marketplace-specific rules should live under marketplace-specific packages.
- Product facts, source identity, and reusable assets should stay outside root ListingKit.
- Infrastructure and external clients should be hidden behind small interfaces.
- Implemented code paths are not considered release-ready until the exact baseline has visible validation evidence.

## 当前正式运行入口

当前受维护的正式 `cmd/` 入口以仓库结构测试和 CI 构建为准：

- `cmd/product-listing-api`：统一 ListingKit HTTP API。
- `cmd/listing-control-plane`：Listing Control Plane 运行时。
- `cmd/shein-listing`：SHEIN listing worker/runtime。
- `cmd/temu-listing`：TEMU listing worker/runtime。

“正式运行入口”表示该 command 当前受维护并受结构测试约束，不表示所有目标平台产品体验都与 SHEIN 主链路同等成熟。历史爬虫、订阅、调试或一次性迁移入口不应继续放在 `cmd/` 下；需要保留时应放入 `hack/`、`tools/` 或对应业务模块，并同步更新 `docs/development/repository-structure.md` 与结构测试。

## 当前能力成熟度

| 能力 | 当前状态 | 说明 |
| --- | --- | --- |
| SHEIN 目标上架 | 生产主路径，稳定化中 | 当前产品与发布重点。价格、促销、readiness、缓存、保存草稿、发布、恢复和浏览器启动需要保持当前基线验证证据。 |
| SDS POD | 活跃能力，稳定化中 | 作为 POD/design 能力处理，不作为普通 product-source 接入目标。 |
| 1688 来源商品 | 已实现中立化和 ListingKit 任务桥接，受控验证待闭环 | 下一步应跑一条 import → envelope → facts → task → preview/readiness 的真实或受控链路。 |
| Amazon 来源商品 | 已实现 source-envelope 边界验证路径 | 用于 source modeling 和测试，不代表完整 Amazon 目标工作台已开启。 |
| TEMU 目标上架 | runtime / 部署资产保留，完整工作台 deferred | 维护已有 runtime 正确性，但不在当前阶段扩成 SHEIN 等级的完整工作台。 |
| Amazon 目标上架 | 历史/目标代码保留，完整工作台 deferred | 不应作为当前扩张主线。 |
| 大建云仓等仓库来源 | 下一来源候选 | 需要在当前 Product Sourcing MVP 验证闭环后再选择一个来源推进。 |

## 业务场景

### 核心业务流程

1. **导入来源商品**
   - 从 `1688` 商品链接导入现货型商品资料
   - 从 `SDS POD` 商品导入底版、变体、模板图、mockup 等设计生产信息
   - 后续在当前 source loop 验证闭环后，选择一个仓库或 catalog 来源扩展
2. **商品标准化**
   - 抽取标题、描述、属性、价格、规格、图片、变体等原始信息
   - 统一转换成平台无关的标准商品结构，便于后续多平台复用
3. **AI 重构与资料增强**
   - 通过 AI 重写标题、卖点、描述、属性表达和平台文案
   - 对图片、主图素材、白底图、场景图进行处理和重组
   - 结合平台规则生成更适合目标渠道的差异化资料包
4. **平台适配与审核**
   - 优先围绕 SHEIN 主链路进行平台字段映射和审核
   - 在 ListingKit 工作台中检查阻断项、修复资料并确认最终稿
5. **上架与分发**
   - 保存平台草稿或正式发布
   - 支持多店铺、多平台复用同一来源商品的差异化上架，但新目标平台工作台扩张应等 SHEIN 模板和 source loop 稳定后再推进

### 平台字段约定

- `sourcePlatform` / `source_platform`: 来源抓取平台，例如 `amazon`、`1688`
- `targetPlatform` / `target_platform`: 目标上架平台，例如 `shein`、`temu`、`amazon`
- `platform`: 兼容字段，在 `task-processor` 任务模型和成功回执里默认等于目标上架平台
- RabbitMQ `TaskMessage.platform`: 历史上表示来源平台，排查消息体时需要和 `targetPlatform` 一起看

### 当前来源与目标平台

**来源商品**

- `1688`：现货型商品采集、标准资料抽取、多平台改写；当前是下一条业务-source 验证闭环重点。
- `SDS POD`：带设计图、底版、变体和 mockup 的按需定制商品；按 POD/design 能力处理。
- `Amazon`：source-envelope 边界验证路径已实现；不代表 Amazon 目标上架工作台已进入主线。
- 规划中：大建云仓等海外仓商品源。

**目标平台**

- `SHEIN`：当前生产主链路。
- `TEMU`：runtime 资产保留，完整工作台 deferred。
- `Amazon`：历史/目标代码保留，完整工作台 deferred。
- 后续可继续扩展更多主流电商平台，但需要先完成当前 source loop 与 SHEIN 模板稳定化。

### 差异化销售能力

ListingKit 的核心价值不是简单搬运商品，而是把同一个来源商品重构成多个可销售版本：

- 基于 AI 生成不同平台风格的标题、卖点和详情文案
- 对主图、场景图、白底图、设计图进行重新组织和平台适配
- 针对不同平台的类目、属性、图片规范、价格策略生成差异化资料
- 让同一来源商品可以在 SHEIN 等渠道形成不同表达，降低同质化

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
