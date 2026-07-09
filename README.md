# ListingKit - AI 驱动的跨境商品标准化与上架系统

[![CI](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml/badge.svg)](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

ListingKit 是当前项目的主程序与产品主入口，目标是帮助用户把来自不同来源的商品信息，整理成一套可复用、可审核、可差异化改造的标准商品资料包，并进一步上架到主流电商平台。

当前系统重点支持将 `1688` 商品、`SDS` 的 POD 商品整理为统一商品资料；后续会继续接入大建云仓等海外仓商品源。基于这些来源商品，ListingKit 会结合 AI 能力对商品标题、卖点、属性、图片和平台资料进行重构，让同一商品可以面向 Amazon、SHEIN、TEMU 等平台生成适配后的上架内容，用于差异化销售。

ListingKit 同时支持多租户隔离，已接入 `ZITADEL` 作为身份认证与租户边界基础设施。系统会围绕租户维度管理用户访问、任务数据、工作台会话和部分配置能力，适合多个业务主体在同一套系统中独立使用。

项目底层仍保留分布式任务处理、平台抓取、图片流水线和平台提交流程；但从产品视角看，当前核心已经不是一个通用的 `task processor`，而是一套围绕 `ListingKit` 展开的商品标准化、多租户工作台与多平台上架系统。

## Architecture and refactoring authority

Project-wide restructuring should follow [`docs/refactoring/project-wide-refactoring-plan.md`](./docs/refactoring/project-wide-refactoring-plan.md).

When later refactoring work conflicts with older local plans or ad-hoc package decisions, use the project-wide plan as the default source of truth unless a newer ADR or refactoring document explicitly supersedes it.

In particular, future changes should preserve these rules:

- `app` packages assemble runtime dependencies and should not own business rules.
- ListingKit should move toward an orchestration and compatibility facade, not absorb new marketplace-specific rules.
- Marketplace-specific rules should live under marketplace-specific packages.
- Product facts and reusable assets should stay outside ListingKit.
- Infrastructure and external clients should be hidden behind small interfaces.

## 当前正式运行入口

当前受维护的正式 `cmd/` 入口以仓库结构测试和 CI 构建为准：

- `cmd/product-listing-api`：统一 ListingKit HTTP API。
- `cmd/listing-control-plane`：Listing Control Plane 运行时。
- `cmd/shein-listing`：SHEIN listing worker/runtime。
- `cmd/temu-listing`：TEMU listing worker/runtime。

历史爬虫、订阅、调试或一次性迁移入口不应继续放在 `cmd/` 下；需要保留时应放入 `hack/`、`tools/` 或对应业务模块，并同步更新 `docs/development/repository-structure.md` 与结构测试。

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
