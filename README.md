# AI Commerce Agent Platform - AI 驱动的新一代跨境电商智能体平台

[![CI](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml/badge.svg)](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

`task-processor` 的长期产品目标是构建 **AI Commerce Agent Platform**：让用户从“操作一组电商软件功能”逐步转向“给 AI 一个跨境电商目标，由 Agent 调用受控的商品、内容、图片和平台能力完成工作”。

平台不会把现有确定性业务流程全部改写成 Agent。商品事实、平台规则、权限、价格公式、readiness、提交状态、幂等和恢复仍由现有领域服务与工作流负责；Agent 主要承担商品理解、动态工具选择、有限自修复和人工介入等真正需要非确定性推理的环节。

完整长期产品战略见 [`docs/product/ai-commerce-agent-platform-strategy.md`](./docs/product/ai-commerce-agent-platform-strategy.md)。AI 模型治理与 Agent Runtime 的技术边界见 [`docs/superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md`](./docs/superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md)。

## 当前产品现实

**ListingKit 仍然是当前最成熟的主程序与产品执行入口。** 它负责把来自不同来源的商品信息整理成可复用、可审核、可差异化改造的标准商品资料包，并进一步完成平台适配、审核、草稿和发布。

当前系统重点支持 `SHEIN` 主链路稳定化，并将 `1688` 商品、`SDS POD` 商品整理为统一商品资料。系统已经具备 Product Sourcing 的中立建模、Amazon/1688 source envelope、catalog/asset facts handoff，以及到 ListingKit 任务创建边界的窄桥接；近期重点仍是完成当前基线验证和受控真实链路闭环，而不是因为 Agent 战略同时扩多个来源或多个平台工作台。

ListingKit 会结合 AI 能力对商品标题、卖点、属性、图片和平台资料进行重构，让同一来源商品可以面向 SHEIN 等目标平台生成适配后的上架内容。Amazon、TEMU 等目标平台相关代码和 runtime 资产仍保留，但完整新平台工作台扩张目前应视为 deferred，不能按 SHEIN 主链路成熟度来理解。

系统支持多租户隔离，已接入 `ZITADEL` 作为身份认证与租户边界基础设施。底层同时保留分布式任务处理、平台抓取、图片流水线、平台提交流程、RabbitMQ、Temporal，以及正在演进的 AI Capability Control Plane。

长期上，ListingKit 将作为 AI Commerce Agent Platform 中的核心执行面之一，而不是整个产品边界的唯一名称：

```text
AI Commerce Agent Platform
  ├─ Agent Workspace
  ├─ Commerce Agents
  ├─ Commerce Tools
  ├─ Product Intelligence
  ├─ Product Sourcing
  ├─ ListingKit
  ├─ Image / Content Studio
  ├─ Marketplace Connectors
  └─ AI Control Plane
```

## Architecture and product authority

长期产品方向和当前执行状态应按以下层级理解：

1. [`docs/product/ai-commerce-agent-platform-strategy.md`](./docs/product/ai-commerce-agent-platform-strategy.md)：长期产品战略、产品边界与 Agent 路线图。
2. [`docs/superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md`](./docs/superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md)：AI Control Plane 与 Agent Runtime 技术设计。
3. [`docs/refactoring/current-refactoring-status.md`](./docs/refactoring/current-refactoring-status.md)：当前代码现实、Now / Next / Later 和重构门禁。
4. [`docs/refactoring/next-phase-plan.md`](./docs/refactoring/next-phase-plan.md)：近期工程执行顺序。
5. [`docs/product/product-sourcing-mvp-plan.md`](./docs/product/product-sourcing-mvp-plan.md)：Product Sourcing 专项计划。
6. [`docs/refactoring/project-wide-refactoring-plan.md`](./docs/refactoring/project-wide-refactoring-plan.md)：长期代码结构方向。

当长期战略和当前成熟度出现表面冲突时：**战略决定最终往哪里去，current status 决定现在允许做什么。** 不得使用长期 Agent 目标绕过当前稳定性、验证和发布门禁。

未来变化应继续保留这些规则：

- `app` packages 只组装 runtime dependencies，不拥有业务规则。
- ListingKit 保持 orchestration / compatibility / execution facade，不吸收新的 marketplace-specific 规则。
- Marketplace-specific 规则属于对应 marketplace package。
- Product facts、source identity 和 reusable assets 保持在 root ListingKit 之外。
- Infrastructure 与外部 clients 通过小接口隐藏。
- Agent 通过受控 Tool Contract 使用现有领域能力，不直接访问数据库绕过领域规则。
- Agent Runtime 不成为第二套业务状态机，也不替代 Temporal。
- Implemented code path 在没有当前基线验证证据前不能视为 release-ready。

## 当前正式运行入口

当前受维护的正式 `cmd/` 入口以仓库结构测试和 CI 构建为准：

- `cmd/product-listing-api`：统一 ListingKit HTTP API。
- `cmd/listing-control-plane`：Listing Control Plane 运行时。
- `cmd/shein-listing`：SHEIN listing worker/runtime。
- `cmd/temu-listing`：TEMU listing worker/runtime。

“正式运行入口”表示该 command 当前受维护并受结构测试约束，不表示所有目标平台产品体验都与 SHEIN 主链路同等成熟，也不表示独立 Agent Runtime 已经成为正式生产入口。历史爬虫、订阅、调试或一次性迁移入口不应继续放在 `cmd/` 下；需要保留时应放入 `hack/`、`tools/` 或对应业务模块，并同步更新 `docs/development/repository-structure.md` 与结构测试。

## 当前能力成熟度

| 能力 | 当前状态 | 说明 |
| --- | --- | --- |
| SHEIN 目标上架 | 生产主路径，稳定化中 | 当前产品与发布重点。价格、促销、readiness、缓存、保存草稿、发布、恢复和浏览器启动需要保持当前基线验证证据。 |
| SDS POD | 活跃能力，稳定化中 | 作为 POD/design 能力处理，不作为普通 product-source 接入目标。 |
| 1688 来源商品 | 已实现中立化和 ListingKit 任务桥接，受控验证待闭环 | 下一步应跑一条 import → envelope → facts → task → preview/readiness 的真实或受控链路。 |
| Amazon 来源商品 | 已实现 source-envelope 边界验证路径 | 用于 source modeling 和测试，不代表完整 Amazon 目标工作台已开启。 |
| TEMU 目标上架 | runtime / 部署资产保留，完整工作台 deferred | 维护已有 runtime 正确性，但不在当前阶段扩成 SHEIN 等级的完整工作台。 |
| Amazon 目标上架 | 历史/目标代码保留，完整工作台 deferred | 不应作为当前扩张主线。 |
| AI Capability Control Plane | 基础能力已落地并继续治理 | 用于 provider-neutral routing、策略、调用记录和后续 Agent 能力，不表示全部 AI 调用已迁移。 |
| Product Agent | 规划中的首个 Agent PoC | 首个场景为商品资料诊断与补全；受 feature flag、tenant allowlist、预算和人工审核约束。 |
| 大建云仓等仓库来源 | 下一来源候选 | 需要在当前 Product Sourcing MVP 验证闭环后再选择一个来源推进。 |

## 业务场景

### 当前核心业务流程

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

### 长期 Agent 交互目标

长期用户不必手动理解并依次操作所有模块，而可以提交业务目标，例如：

```text
把这个 1688 商品准备成适合 SHEIN 美国站销售的商品，
利润率至少 25%，图片做差异化处理，先生成草稿，不要直接发布。
```

Agent 在权限、成本和工具 allowlist 内完成：

```text
理解目标
 -> 读取来源证据
 -> 商品诊断与补全
 -> 平台类目/属性适配
 -> 文案与图片候选
 -> 定价计算
 -> deterministic readiness
 -> 有限修复
 -> 人工审核
 -> 经授权后保存草稿/发布
```

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

这些能力后续优先通过小而明确的领域合同逐步变成 `Commerce Tools`，而不是复制到 Agent 内部。

### 多租户与身份体系

- ListingKit 是多租户系统，核心任务、工作台会话、上传资产和部分配置按租户隔离
- 已接入 `ZITADEL`，用于认证、租户识别、角色授权和会话代理
- 前端 UI、代理层和 Go API 会基于 `ZITADEL` 的身份信息传递租户上下文
- Agent Runtime 和 Commerce Tools 必须继承同一 tenant/user identity，不建立平行权限体系
- 适合服务多个团队、客户或业务线，并在同一平台中保持数据边界清晰

更具体地说：

- `ZITADEL resource owner` 是当前租户边界的主要来源，ListingKit 会基于这个值识别当前用户属于哪个租户
- 前端 `ListingKit UI` 会先完成 `ZITADEL` 登录，代理层验证 session 或 bearer token 后，再把确认过的身份信息转发给 Go API
- 转发的信息不仅包含用户身份，也包含租户标识和角色语义，后端再将其注入请求上下文，用于任务、工作台、资产和配置的访问控制
- 系统设计按“认证 + 授权 + 租户隔离”三层组织，而不是只做一个登录门禁
