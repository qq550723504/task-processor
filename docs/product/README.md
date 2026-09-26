# 产品文档

本目录记录硕米智能引擎的产品语义、产品投影、业务边界和历史产品证据。

## 当前产品权威

产品 UI、信息架构、导航、页面命名和用户交互语义，以 [硕米最终 UI / IA Authority](./final-ui-ia-authority.md) 记录的已批准 Figma 当前可见、非归档终稿为准。

ListingKit 是需要退休的旧产品投影与混合架构，不是硕米的长期子产品、默认工作台或永久执行引擎。旧 ListingKit 文档继续保留为历史需求、历史验收或旧路径维护证据，但不能据此恢复旧 Workspace、Task-first 产品模型、永久 facade、fallback 或双事实源。

领域事实、安全和副作用边界不由 Figma 定义。商品、资产、Listing、Marketplace、Store、身份、组织、资金、资源、幂等、恢复等规则继续由当前领域合同和架构文档决定。

## 建议阅读顺序

1. [最终 UI / IA Authority](./final-ui-ia-authority.md) — 用户看到什么、在哪里操作、如何命名和交互。
2. [AI Commerce Agent Platform 产品战略](./ai-commerce-agent-platform-strategy.md) — 长期产品方向与 Agent / Tool 边界；技术能力图不覆盖 Figma。
3. [全新系统产品基线](./greenfield-no-legacy-migration.md) — 新业务按全新安装、空业务数据和当前模型推进，不恢复旧迁移/兼容前置。
4. [架构索引](../architecture/README.md) — 领域 owner、权限、事务、幂等、资金和外部副作用。
5. [Legacy Register](../refactoring/legacy-register.md) — 旧能力的 EXTRACT / RETIRE 去向。
6. [Repository Structure](../development/repository-structure.md#current-entrypoint-map) — 当前真实入口、命令和代码落点。
7. GitHub Roadmap #137、具体执行 Issue / PR — 当前排序、范围、准确 HEAD、验证证据和操作权限。

## 当前执行规则

- 新功能先确定在 Figma 中的产品位置，再消费当前领域能力；不能由旧包名、旧路由或旧 Task 模型反向创造产品页面。
- ListingKit 中仍有效的行为按职责抽取到当前 Product / Listing / Marketplace / Integration / App 等 owner，调用方切换后退休旧实现。
- ProductSnapshot、ApprovedAsset、Platform Draft、Listing 等是稳定领域事实，不意味着需要独立顶层 Product Center / Listing Center。
- 设计目标、代码存在、运行接线、部署和产品验收必须分别取证；旧文档中的“当前”“下一阶段”“已完成”只对其注明的历史基线有效。
- 未开放能力应真实表达 unavailable，不用旧路径兜底、原型示例或模拟成功替代。
- Agent / Tool 不直接访问数据库绕过领域规则，也不建立第二套权限、商品事实或业务状态机。

## 历史 ListingKit 文档

以下文档保留用于追溯旧需求、旧行为、旧验收和退休时需要保留的有效约束。它们均不再拥有当前产品或近期派工权威：

- [ListingKit 项目目标与范围](./listingkit-project-goals.md)
- [ListingKit 产品总览](./listingkit-product-overview.md)
- [ListingKit 操作指南](./listingkit-operating-guide.md)
- [ListingKit 产品路线图](./listingkit-product-roadmap.md)
- [ListingKit 下一阶段执行计划](./listingkit-next-execution-plan.md)
- [ListingKit 付费商业试点上线执行计划](./listingkit-paid-pilot-execution-plan.md)
- [ListingKit 付费试点产品目录与用量政策](./listingkit-paid-pilot-product-catalog.md)
- [ListingKit 错误恢复手册](./listingkit-error-recovery.md)
- [ListingKit 错误恢复 SOP](./ops/listingkit-error-recovery-sop.md)
- [ListingKit 真实接口验收报告模板](./validation/listingkit-real-api-validation-report-template.md)

读取这些历史文档时，只抽取仍被当前 owner 明确承接的业务、安全、幂等、恢复或平台规则。出现与 Figma Authority、全新系统基线、Legacy Register、当前领域合同或具体 Issue 冲突的内容时，以后者为准。

## 专项产品文档

Product Sourcing、Account、Commercial、Marketplace、Agent 等专项文档只有在其自身仍被当前 Authority / Issue 引用且未被后续决定替代时，才作为对应范围的依据。文档标题或旧 Active 标签本身不构成当前执行授权。

AI Control Plane 与 Agent Runtime 的技术设计见：

- [AI Capability / Agent Platform](../superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md)

## 面向读者

- 产品和运营：先看 Figma Authority 与当前执行 Issue，不从历史 ListingKit 路线图决定产品。
- 前后端工程师：按当前 owner、Repository Structure 与具体合同修改实现。
- AI / Agent 工程：判断需求应使用确定性能力、AI capability、Commerce Tool 还是 Agent Runtime。
- QA / 交付：按真实用户路径和当前验收条件验证，不用历史报告替代当前证据。
- 商务 / 运维 / 安全：以当前商业合同、运行文档和明确环境授权为准，不从旧 paid-pilot 文档推导生产政策。

## 文档边界

本目录记录产品级语义和历史产品证据。滚动 SHA、CI、运行状态、真实环境结果和具体操作权限留在对应 Issue / PR /验收记录；不要在多个产品文档里维护互相冲突的“当前状态”。
