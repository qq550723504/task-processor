# 硕米智能引擎 — AI Commerce Agent Platform

[![CI](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml/badge.svg)](https://github.com/qq550723504/task-processor/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/badge/Go-1.26+-00ADD8?style=flat&logo=go)](https://golang.org/)

`task-processor` 是硕米 AI 电商经营平台的工程仓库。产品以 **AI工作台和专业智能体** 为核心，结合供应、店铺、数据、工具和生态能力，让用户从操作零散功能转向表达业务目标、查看结果并作出必要决策。

**最终产品的 UI、信息架构、导航、页面命名和交互语义，以已批准 Figma 当前可见、非归档终稿为准。ListingKit 是需要退休的旧产品投影与混合架构，不是新平台的子产品、默认工作台或永久执行引擎。** 详见 [最终 UI / IA Authority](./docs/product/final-ui-ia-authority.md) 与下方[退休边界](#listingkit-退休边界)。

## 从 Figma 理解产品

仓库登记的产品设计权威是 Figma 页面 `31:463`「硕米官网」。下列是 [Authority 中的目标一级信息架构](./docs/product/final-ui-ia-authority.md#2-当前最终一级信息架构)，不是当前全部已实现或已开放的菜单清单：

```text
硕米智能引擎
  ├─ 运营驾驶舱
  ├─ AI工作台
  ├─ 供应市场
  ├─ 智能市场
  ├─ 工具市场
  ├─ 生态服务
  ├─ 数据服务
  ├─ 店铺中心
  ├─ 套餐与权益
  └─ 我的账户
```

新功能先确定它在 Figma 中的产品位置，再消费当前领域能力；不能由旧包名、旧路由或内部任务模型反向制造产品菜单。具体页面没有已批准设计时，在对应 Issue 记录设计缺口，不恢复旧工作台来代替。

- **AI工作台** 承载硕米Chat、用户可理解的业务任务及人工决策；任务中心是 BusinessTask，不是内部 Queue / Temporal Task Dashboard。
- **供应市场** 承载供给获取与接入；**店铺中心** 承载店铺、店铺商品和相关运营投影。SHEIN、TEMU、Amazon 是平台能力和店铺的维度，不各自复制一个工作台。
- **套餐与权益、我的账户** 分别消费当前商业、资源、身份、组织、审计和推广能力；原型示例价格、余额或成功状态不成为业务事实。

ProductSnapshot、ApprovedAsset、Platform Draft、Listing 是领域事实，不要求新增顶层 Product Center / Listing Center。产品页面映射与关键节点只在 [UI / IA Authority](./docs/product/final-ui-ia-authority.md) 维护。

## 产品目标与业务能力

典型目标是：从来源商品获取可靠资料，进行内容或图片处理，结合平台规则准备候选结果，经用户确认后执行获准的草稿或发布操作。

```text
供给获取 → 标准商品与来源证据 → 内容/图片候选
        → 平台适配与确定性校验 → 用户确认 → 获准草稿/提交
```

这是能力关系，不是新建菜单，也不表示整条链已接线或通过验收。每个动作落到哪个页面，以 Figma 和该任务已批准的产品范围为准；SDS POD 按设计生产能力处理，不冒充普通现货来源。

Agent 通过受控 Tool Contract 使用领域服务，负责确有价值的理解、候选生成和有限动态决策。商品事实、平台规则、价格公式、权限、readiness、幂等、提交与恢复仍由各自的确定性 owner 负责；不把所有固定流程改成 Agent，不复制数据库、权限或业务状态机。

长期方向见 [产品战略](./docs/product/ai-commerce-agent-platform-strategy.md)；AI Control Plane 与 Agent Runtime 的技术设计见 [AI Capability / Agent Platform](./docs/superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md)。技术文档的能力图不覆盖 Figma 的产品信息架构。

## 当前正式运行入口

当前应用装配入口是 `cmd/current-application`，当前 Console 页面位于现有前端工程的 `/workbench`。具体命令、私有配置、可选模块和实际代码落点，以 [Repository Structure 的当前入口地图](./docs/development/repository-structure.md#current-entrypoint-map) 和[新代码落点](./docs/development/repository-structure.md#current-owner-landing)为准。

前端目录 `web/listingkit-ui`、既有命令和配置中的历史名称仍可能存在；这里保留真实路径供开发者定位，**不将其解释为 ListingKit 产品仍被保留**。本 README 不重命名路径，也不授权重建其中已经符合 Figma 与当前 owner 合同的页面。

完整产品／运维 command 清单只维护在 [顶层目录约定](./docs/development/repository-structure.md#顶层目录约定)，由 `TestCmdContainsOnlyOfficialEntrypoints` 对照实际代码约束。旧执行入口仅供识别待退休调用，不是新业务的默认接入点。

需要独立本地账户环境时，阅读 [Account Compose 说明](./deployments/docker/account-compose/README.md)。其中隔离测试、基础设施准备与产品可用性分别说明，不能把容器健康、路由存在或配置开启等同于完整功能已开放。

## 当前产品现实与验收

**设计目标、代码存在、运行接线、已部署和产品验收是五件不同的事。** 本 README 不维护第二份滚动进度表，也不把旧 SHEIN / ListingKit 路径的成熟度写成新产品已经交付。

- 当前排序和任务范围：[#137](https://github.com/qq550723504/task-processor/issues/137) 及对应执行 Issue。
- 当前实现、准确 HEAD、CI 和运行证据：对应 PR 与独立验收记录。
- 历史成熟度与退休进度：[重构状态基线](./docs/refactoring/current-refactoring-status.md)，按文档所注明的日期与 SHA 阅读，不冒充今天的全量验证。

已有 SHEIN、1688、POD、图片和其他平台代码中的有效行为，按当前产品需要复用或抽取；代码存在不自动批准扩展某个平台、开放新功能或继续旧任务流程。未开放能力应真实表达 unavailable，不用原型示例、模拟成功或旧路径兜底。

新业务遵循 [全新系统产品基线（PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08）](./docs/product/greenfield-no-legacy-migration.md)：按全新安装、空业务数据和当前模型交付，不新增旧数据迁移、旧 ID 映射或旧 Service 包装。Sourcing 使用当前来源证据、Catalog 快照和资产 owner，详见 [Sourcing 指南](./docs/product/product-sourcing-handoff.md)。历史 profile 和 cutover gate 不是当前验收前置。

## ListingKit 退休边界

**退休的是旧产品与架构，不是放弃有价值的商品、平台和上架能力。** 按 [Legacy Policy](./docs/refactoring/legacy-hard-cut-policy.md)、[Register](./docs/refactoring/legacy-register.md) 和 [Mapping](./docs/refactoring/module-target-mapping.md) 处理：

| 对象 | 处理方向 |
| --- | --- |
| root `internal/listingkit` 的混合职责 | 有效行为 EXTRACT 到当前 `internal/product/*`、`internal/listing/*`、`internal/marketplace/*`、`internal/integration/*`、`internal/app/*`；调用方切换后 RETIRE 原实现。 |
| `internal/compatibility/listingkit`、旧 Task-first / Listing Workspace / Task Dashboard / 平台独立工作台 | RETIRE；不新增消费者，不保留永久 facade、fallback、双读双写或第二事实源。 |
| `web/listingkit-ui` 中符合当前 Figma / owner 的 Console、BFF 与共享组件 | 按实际职责复用，不因目录名误删；其中旧 ListingKit / Task-first 页面仍按页面和调用方退出。 |
| 历史命令、配置、路径与已提交证据中的名称 | 如实保留定位或历史用途，不据此恢复旧产品、不伪称代码已全部删除。 |

后续 Agent 不得把新功能放回旧 ListingKit、仅改名后继续旧 Task 模型，或为了复用旧 Service 再造兼容层。具体代码退休在原责任 Issue 的有界交付中完成；本次文档修正不授权全仓删除、业务迁移或操作已有数据。

## Architecture and product authority

按职责判断，而不是按旧文档的 Active 标签或旧架构图排序：

| 问题 | 依据 |
| --- | --- |
| 用户看到什么、在哪里操作、如何命名与交互 | [最终 Figma UI / IA Authority](./docs/product/final-ui-ia-authority.md)及明确批准的产品决定。 |
| 业务事实、领域归属、权限、事务、资金、幂等与外部副作用 | [架构索引](./docs/architecture/README.md)与当前领域合同；Figma 示例不替代它们。 |
| 什么旧设计要退出、有效行为归哪里 | [全新系统基线](./docs/product/greenfield-no-legacy-migration.md)、[Legacy Policy](./docs/refactoring/legacy-hard-cut-policy.md)、[Register](./docs/refactoring/legacy-register.md)、[Mapping](./docs/refactoring/module-target-mapping.md)。 |
| 从哪里启动、去哪里修改 | [Repository Structure](./docs/development/repository-structure.md#current-entrypoint-map)。 |
| 当前先做什么、已交付什么、有哪些操作权限 | [#137](https://github.com/qq550723504/task-processor/issues/137)、具体 Issue / PR 和准确证据，遵循 [AGENTS](./AGENTS.md) 与 [Issue 派工规则](./docs/engineering/issue-driven-development.md)。 |

旧 ListingKit 目标和执行计划是历史材料，不是当前子产品定义或派工依据。`next-phase-plan.md` 等历史实施记录不覆盖当前 Issue，也不恢复已退休要求。

## 身份与基础工程边界

当前平台复用 ZITADEL 与已有身份、组织和授权合同。Home Organization 表示账号归属，不能替代用户当前获授权的 Effective Organization；前端 selector 或调用方自行传入的 actor / org / role 不是权限证明。

当前 Organization 路由由 Go API 验证身份并按 route policy 解析有效组织，领域 owner 继续执行资源与动作授权。Agent / Tool 继承同一边界，不建立平行权限体系；旧 `tenantbridge` 和 legacy middleware 不成为新消费者的入口。详见 [Auth and Tenancy](./docs/architecture/auth-and-tenancy.md)。

应用层只做依赖装配；基础设施通过窄接口接入。Temporal / Queue 保留其执行与恢复职责，Agent Runtime 不成为第二套业务状态机。任何用户可见交付都必须同时有真实 owner、合法调用链和与范围匹配的验收证据。
