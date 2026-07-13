# 产品文档

这个目录放 `task-processor` 和 ListingKit 的产品视角文档。这里不按代码包解释系统，而是说明产品解决什么问题、用户如何完成任务、关键能力如何映射到后端与前端模块。

## 建议阅读顺序

1. [ListingKit 产品总览](./listingkit-product-overview.md)
2. [ListingKit 操作指南](./listingkit-operating-guide.md)
3. [ListingKit 产品路线图](./listingkit-product-roadmap.md)
4. [ListingKit 下一阶段执行计划](./listingkit-next-execution-plan.md)
5. [ListingKit 付费商业试点上线执行计划](./listingkit-paid-pilot-execution-plan.md)
6. [ListingKit 错误恢复手册](./listingkit-error-recovery.md)
7. [ListingKit 错误恢复 SOP](./ops/listingkit-error-recovery-sop.md)
8. [ListingKit 真实接口验收报告模板](./validation/listingkit-real-api-validation-report-template.md)

## 当前执行入口

- 日常产品能力、运营效率和平台扩展按 `listingkit-next-execution-plan.md` 理解。
- 准备邀请制付费使用时，安全、租户隔离、提交幂等、订阅计量、数据保护、发布门禁和试点放行标准以 `listingkit-paid-pilot-execution-plan.md` 为准。
- 付费试点计划不代表已经达到公开注册或公众自助 SaaS 的 General Availability 标准。

## 面向读者

- 产品和运营负责人：理解 ListingKit 的业务闭环和用户路径。
- 前后端工程师：在改实现前对齐产品词汇、页面责任和状态语义。
- QA 和交付负责人：按真实用户流程做验证，而不是只验证接口是否返回。
- 商务、运维、安全和支持负责人：按付费试点计划确认收费、上线、恢复和客户支持门禁。

## 文档边界

本目录只记录产品级语义。除非字段会直接影响用户操作、商业计量、安全隔离或上线门禁，否则不展开底层字段清单。
