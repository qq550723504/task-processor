# 产品文档

这个目录记录 `task-processor` / AI Commerce Agent Platform 的产品级语义：平台解决什么问题、用户如何完成任务、ListingKit 与 Agent 能力如何协作，以及当前商业化和执行门禁。

## 产品方向

长期产品北极星：

> **AI 驱动的新一代跨境电商智能体平台，让用户从“操作软件功能”逐步转向“给 AI 一个电商目标，由 Agent 调用受控电商能力完成工作”。**

当前最成熟的产品执行面仍然是 ListingKit。新的 Agent 战略不会绕过现有 SHEIN 稳定化、Product Sourcing 闭环、付费试点、租户隔离、readiness、幂等和发布安全门禁。

## 建议阅读顺序

1. [AI Commerce Agent Platform 产品战略](./ai-commerce-agent-platform-strategy.md)
2. [ListingKit 项目目标与范围](./listingkit-project-goals.md)
3. [ListingKit 产品总览](./listingkit-product-overview.md)
4. [ListingKit 操作指南](./listingkit-operating-guide.md)
5. [ListingKit 产品路线图](./listingkit-product-roadmap.md)
6. [ListingKit 下一阶段执行计划](./listingkit-next-execution-plan.md)
7. [ListingKit 付费商业试点上线执行计划](./listingkit-paid-pilot-execution-plan.md)
8. [ListingKit 付费试点产品目录与用量政策](./listingkit-paid-pilot-product-catalog.md)
9. [ListingKit 错误恢复手册](./listingkit-error-recovery.md)
10. [ListingKit 错误恢复 SOP](./ops/listingkit-error-recovery-sop.md)
11. [ListingKit 真实接口验收报告模板](./validation/listingkit-real-api-validation-report-template.md)

AI Control Plane 与 Agent Runtime 的详细技术设计位于：

- [`docs/superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md`](../superpowers/specs/2026-08-06-ai-capability-agent-platform-design.md)

## 文档权威关系

- `ai-commerce-agent-platform-strategy.md` 决定长期产品方向、产品边界与 Agent 路线图。
- `../refactoring/current-refactoring-status.md` 决定当前代码现实以及现在允许推进什么。
- ListingKit、Product Sourcing、SHEIN 和 paid pilot 专项文档在各自范围内继续作为执行 source of truth。

原则：**战略决定最终往哪里去，current status 决定现在允许做什么。**

## 当前执行入口

- 日常产品能力、运营效率和平台扩展按 `listingkit-next-execution-plan.md` 理解。
- 准备邀请制付费使用时，安全、租户隔离、提交幂等、订阅计量、数据保护、发布门禁和试点放行标准以 `listingkit-paid-pilot-execution-plan.md` 为准。
- `listingkit-paid-pilot-product-catalog.md` 定义唯一 `paid_pilot` 套餐、正式发布 entitlement 和首发用量政策。
- 付费试点计划不代表已经达到公开注册或公众自助 SaaS 的 General Availability 标准。
- Agent PoC、Commerce Tools 与 Agent Workspace 的推进必须遵守 AI Commerce Agent Platform 战略及 AI Capability & Agent Platform 设计中的 feature flag、tenant allowlist、预算、评测与人工审核要求。

## 面向读者

- 产品和运营负责人：理解平台长期方向、ListingKit 当前业务闭环和未来 Agent 用户路径。
- 前后端工程师：在改实现前对齐产品词汇、领域 owner、Tool 边界、页面责任和状态语义。
- AI/Agent 工程负责人：判断需求应使用固定 AI capability、Commerce Tool 还是 Agent Runtime。
- QA 和交付负责人：按真实用户流程和 Agent eval 做验证，而不是只验证接口是否返回。
- 商务、运维、安全和支持负责人：按付费试点计划确认收费、上线、恢复和客户支持门禁。

## 文档边界

本目录只记录产品级语义。除非字段会直接影响用户操作、Agent 权限、商业计量、安全隔离或上线门禁，否则不展开底层字段清单。
