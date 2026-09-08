# 全新系统开发基线：不兼容、不迁移历史数据

Product Decision: **PD-GREENFIELD-NO-LEGACY-MIGRATION-2026-09-08**<br>
Status: **APPROVED / ACTIVE**<br>
Authority: [Issue #365](https://github.com/qq550723504/task-processor/issues/365)

## 当前产品基线

后续业务任务一律按**全新安装、空业务数据、当前模型和当前产品需求**设计。历史 A/B1/B2、#364/PR #366、旧 C/D 分期和旧设计/测试仅保留为历史证据；它们不是未来迁移授权、实现前置或验收要求。

新系统不得承接旧业务历史：不开发旧数据迁移、导入或 backfill，旧 ID 映射，旧任务/结果续跑或恢复，旧 profile/登录态/目录复用，旧套餐、账务或权益承接。新用户、Organization、源账号、店铺连接、profile/登录态和权益都必须按当前合法创建、认证和初始化流程建立。

不得为了旧设计继续存在而新增旧 Service 包装、compatibility adapter、fallback、双读、双写、同步或第二事实源，也不得新增 tenantbridge consumer。发现旧代码时仍只按 Hard-Cut 的 `EXTRACT | RETIRE` 处理：可复用行为抽取到当前 owner，否则退休；没有 Compatibility 类别。

合格的现有能力和成熟开源组件可以复用，条件是它们符合当前职责、依赖和正确性，而不是它们曾属于旧设计。新系统自身正常的 Schema 演进不等于承接旧系统历史数据，也不能据此恢复旧数据迁移项目。

## 例外与真实环境边界

若新任务声称存在兼容义务，必须先提供**具体、当前的外部可观察契约证据**，并建立显式、可评审的 Exception；历史 Issue、PR、设计、测试或既有代码不能单独构成该证据，不能默认预建兼容路径。

本决定不授权访问、迁移、删除或清理任何既有客户数据库、IAM、profile、账务、订单、对象存储或外部平台状态；环境隔离、归档或删除仍需独立范围和授权。新系统仍必须满足自身的授权、租户隔离、幂等、事务、审计、资源限制和凭据安全要求。

## 直接历史入口

- [#30 / #307 clean-slate cutover](issue30-clean-slate-cutover.md)、[Source Account ownership cutover](../superpowers/specs/2026-09-05-source-account-organization-cutover.md)、[#30 sourcing cutover](../superpowers/specs/2026-09-05-issue-30-sourcing-cutover.md) 与 [source-account preflight](../operations/source-account-ownership-preflight.md) 仅保留历史证据，必须标记为 `SUPERSEDED`。
- 后续派工以本决定、当前 Issue 的产品需求及 [Legacy Hard-Cut Policy](../refactoring/legacy-hard-cut-policy.md) 为准；不得把历史迁移 gate 重新变成新任务依赖。
