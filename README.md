# AI Commerce Agent Platform - AI 驱动的新一代跨境电商智能体平台

[![CI](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml/badge.svg)](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org/)

`task-processor` 的长期产品目标是构建 **AI Commerce Agent Platform**：让用户从“操作一组电商软件功能”逐步转向“给 AI 一个跨境电商目标，由 Agent 调用受控的商品、内容、图片和平台能力完成工作”。

平台不会把现有确定性业务流程全部改写成 Agent。商品事实、平台规则、权限、价格公式、readiness、提交状态、幂等和恢复仍由现有领域服务与工作流负责；Agent 主要承担商品理解、动态工具选择、有限自修复和人工介入等真正需要非确定性推理的环节。

完整长期产品战略见 [`docs/product/ai-commerce-agent-platform-strategy.md`](./docs/product/ai-commerce-agent-platform-strategy.md)。AI 模型治理与 Agent Runtime 的技术边界见 [`docs/superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md`](./docs/superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md)。

## 当前产品现实

CURRENT STATE：以下实现说明核对于 `main @ cae67730c5c0e645d708cb2f6814f14781962bb1`，
不代表该提交已获生产验收。最终 UI / IA 以 [硕米最终产品 Authority](./docs/product/final-ui-ia-authority.md)
为准；现存 ListingKit 壳层与内部 Task 不决定最终导航。

**ListingKit 仍然是当前最成熟的主程序与产品执行入口。** 它负责把来自不同来源的商品信息整理成可复用、可审核、可差异化改造的标准商品资料包，并进一步完成平台适配、审核、草稿和发布。

当前系统重点支持 `SHEIN` 主链路稳定化，并将 `1688` 商品、`SDS POD` 商品整理为统一商品资料。系统已有 `SourceEnvelope`、`sourcing.Publisher` → `catalog.Publisher` / `ProductSnapshot` 和 `ApprovedAsset` 合同。
现存 1688 → ListingKit task handoff 是待抽取、退休的旧路径；新 import HTTP 合同已合入但未注册接线。
当前 #30/#307 验收目标是新受控导入 → 新 Catalog snapshot → 显式资产批准 → readiness；
详见 [Sourcing 指南](./docs/product/product-sourcing-handoff.md) 和 [clean-slate 决策](./docs/product/issue30-clean-slate-cutover.md)。

ListingKit 会结合 AI 能力对商品标题、卖点、属性、图片和平台资料进行重构，让同一来源商品可以面向 SHEIN 等目标平台生成适配后的上架内容。Amazon、TEMU 等目标平台相关代码和 runtime 资产仍保留，但完整新平台工作台扩张目前应视为 deferred，不能按 SHEIN 主链路成熟度来理解。

系统支持多租户隔离，已接入 `ZITADEL` 作为身份认证与租户边界基础设施。底层同时保留分布式任务处理、平台抓取、图片流水线、平台提交流程、RabbitMQ、Temporal，以及正在演进的 AI Capability Control Plane。

长期上，ListingKit 中有效能力抽取到当前领域与执行 owner。下图是能力分解，不是最终一级菜单，也不批准保留 root ListingKit 为永久 facade：

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

按职责读取权威，而不是按文档日期或旧 Active 标签排序：

1. [最终 UI / IA Authority](./docs/product/final-ui-ia-authority.md)：用户可见导航、命名与产品投影；AI工作台的任务中心是 #298 BusinessTask，领域事实不等于顶层 Product/Listing Center。
2. [产品战略](./docs/product/ai-commerce-agent-platform-strategy.md)与批准的领域合同：Product/Identity/Store/Resource、Tool/Agent 的事实、权限与副作用边界；从[架构索引](./docs/architecture/README.md)进入。
3. [Legacy Policy](./docs/refactoring/legacy-hard-cut-policy.md)、[Register](./docs/refactoring/legacy-register.md) 与 [Mapping](./docs/refactoring/module-target-mapping.md)：EXTRACT / RETIRE，旧 owner 不是新功能落点。
4. [当前状态](./docs/refactoring/current-refactoring-status.md)：绑定基线的实现现实与发布门禁。
5. [#137](https://github.com/qq550723504/task-processor/issues/137) 和具体执行 Issue：工程顺序、依赖、范围与权限；遵循 [Issue 派工规则](./docs/engineering/issue-driven-development.md)。
6. [Product Sourcing closeout](./docs/product/product-sourcing-mvp-plan.md)：专项验收指南；[next-phase-plan.md](./docs/refactoring/next-phase-plan.md) 仅为 HISTORICAL implementation record，不能作为近期执行队列。

最终原型不等于当前 release capability；已实现、已验证、已接线和生产验收分别取证。

未来变化应继续保留这些规则：

- `app` packages 只组装 runtime dependencies，不拥有业务规则。
- root ListingKit 与 compatibility 路径按 EXTRACT → RETIRE 收口，不作为新依赖或永久 facade；有效行为归当前 Product/Listing/Marketplace/App owner。
- Marketplace-specific 规则属于对应 marketplace package。
- Product facts、source identity 和 reusable assets 保持在 root ListingKit 之外。
- Infrastructure 与外部 clients 通过小接口隐藏。
- Agent 通过受控 Tool Contract 使用现有领域能力，不直接访问数据库绕过领域规则。
- Agent Runtime 不成为第二套业务状态机，也不替代 Temporal。
- Implemented code path 在没有当前基线验证证据前不能视为 release-ready。

## 当前正式运行入口

完整的产品／运维 command 清单只维护在 [Repository Structure](./docs/development/repository-structure.md#顶层目录约定)，
由 `TestCmdContainsOnlyOfficialEntrypoints` 对照实际代码约束，构建／脚本维护归属也在该处说明。
受维护不表示已部署、平台体验同等成熟或已开放独立 Agent Runtime；不得把未合 PR 的入口当成 main 现实。

## 当前能力成熟度

| 能力 | 当前状态 | 说明 |
| --- | --- | --- |
| SHEIN 目标上架 | 生产主路径，稳定化中 | 当前产品与发布重点。价格、促销、readiness、缓存、保存草稿、发布、恢复和浏览器启动需要保持当前基线验证证据。 |
| SDS POD | 活跃能力，稳定化中 | 作为 POD/design 能力处理，不作为普通 product-source 接入目标。 |
| 1688 来源商品 | 中立 Publisher 已实现；新 HTTP prepared-only；旧 handoff 待退休 | #30/#307：新导入 → Catalog → 显式资产批准 → readiness；环境范围、旧执行隔离及接线批准待验收。 |
| Amazon 来源商品 | 已实现 source-envelope 边界验证路径 | 用于 source modeling 和测试，不代表完整 Amazon 目标工作台已开启。 |
| TEMU 目标上架 | runtime / 部署资产保留，完整工作台 deferred | 维护已有 runtime 正确性，但不在当前阶段扩成 SHEIN 等级的完整工作台。 |
| Amazon 目标上架 | 历史/目标代码保留，完整工作台 deferred | 不应作为当前扩张主线。 |
| AI Capability Control Plane | 基础能力已落地并继续治理 | 用于 provider-neutral routing、策略、调用记录和后续 Agent 能力，不表示全部 AI 调用已迁移。 |
| Product Agent | 规划中的首个 Agent PoC | 首个场景为商品资料诊断与补全；受 feature flag、tenant allowlist、预算和人工审核约束。 |
| 大建云仓等仓库来源 | 下一来源候选 | 需要在当前 Product Sourcing MVP 验证闭环后再选择一个来源推进。 |

## 业务场景

### 业务能力流程（不是整条新路径已接线的声明）

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

- `resourceowner:id` / Home Organization 表示账号归属，不能当作当前 Effective Organization。
- 当前 Organization 路由由 Go API 验证 bearer token，再按 route policy 解析获授权的 effective organization 和组织角色；请求的 Organization selector 本身不是权限证明。
- `internal/app/httpapi/server_auth.go` 丢弃调用方 identity/tenant/role headers，解析成功后才设置 effective Organization 的上下文与下游 headers；Store/资源访问继续受领域权限和资源归属约束。
- 部分旧路由仍使用 legacy auth middleware，`internal/tenantbridge` 仍有待 drain caller；它们不是当前 Organization 合同，也不允许新增 consumer。详见 [Auth and Tenancy](./docs/architecture/auth-and-tenancy.md)。

- 系统设计按“认证 + 授权 + 租户隔离”三层组织，而不是只做一个登录门禁
