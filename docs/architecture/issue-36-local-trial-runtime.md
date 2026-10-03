# #36 可保留的本机隔离试用运行边界

> 状态：**DRAFT / NOT IMPLEMENTATION_READY**。本文件是 2026-10-03 用户决定后的设计候选，不批准正式业务代码、schema 或部署变更。适用的独立 Architecture Review 和 Issue 准入记录完成前保持 Blocked。

## 用户结果、权威与范围

试用者在一套全新、仅监听本机的实例，用该实例自己的 ZITADEL / Auth.js 正常登录，从任务中心「待确认」读取一条合成 Product 标题提案，作人工接受、编辑或拒绝，再单独确认 Apply 并读回 Catalog 新版本和 receipt；从「已完成」读取一条合成 SHEIN 本地资料准备记录的历史 Store ID、本地动作和原离线诊断。stop/restart 后，这些事实和登录所依赖的身份数据库仍在。交接给试用者的是可访问地址、私密凭据位置、操作步骤和限制，不是结束即销毁的测试 fixture。

产品决定为 [#36 的 2026-10-03 新阶段](https://github.com/qq550723504/task-processor/issues/36)。当前页面与任务中心的 Figma Authority 沿用 file `tg48P46SSXl6TBy9lZwg63` 的 `31:463`、`427:3005`；Figma 的示例名称、数量与进度不成为事实。现有 [标题审核 UI 合同](../engineering/product-title-review-ui.md)、[#36 Store 投影合同](issue-36-completed-work-store-scope.md)、[Store Center 当前 owner](store-center-current-application-v1.md)、[当前模型/无旧数据迁移决定](../product/greenfield-no-legacy-migration.md) 和 [Hard-Cut](../refactoring/legacy-hard-cut-policy.md) 继续适用。

本片是**本机、全新、空业务数据**的内部试用装配，范围内仅有当前 Product Review/Catalog、ApprovedAsset、Store Center、Listing Record、现有 Console/BFF 和正常本地身份。不得据此默认开放生产/共享实例。真实 provider、付费模型、1688/SHEIN 远端调用或发布、真实用户/业务数据、旧数据迁移、Agent/BusinessTask 新流程、通用试用或验收平台、Store 名称由 Listing 旁读，均不在本片。合成提案只用于审核/Apply；合成 Listing record 只用于已完成/诊断，两条业务链相互独立。

本片 Must 是正常本地登录和当前企业授权、原 Review 精确版本/独立 Apply 与 receipt、原 Listing 历史 Store/动作/诊断、独立实例的数据保存及安全停止/重启。Should 是明确的失败提示和可操作的试用交接；不要求新增 UI 或全功能菜单。沿用 #36 当前 Threat Model：不能跨组织/owner 读取或 Apply、不能把响应丢失当成未提交、不能把本地 `publish` 当成平台发布，不能泄露本地凭据或把另一试用项目的数据当成本项目事实。本片没有新增 Accepted Risk；不以本机范围取消身份、授权、事务、幂等或副作用底线。

## 现状与必须作出的装配决定

PR #579 只完成严格 v2 投影并已合并；它不提供试用运行源。现有 `account-compose` 的 current-application manifest 没有当前 Product DB，UI 没有 `PRODUCT_REVIEW_API_ORIGIN`。`NewProductReviewApplication` 与 `NewSheinRecordApplication` 是独立、已有的当前 owner 组合；[模块映射](../refactoring/module-target-mapping.md)把两者的默认装配标为未交付。特别是 DRAFT-S1 构造器明确不允许靠一个默认布尔开关推断 Product/Asset/Store/Listing 的共同事实边界。现有 #344 fixture 的合成 Auth.js 身份与 29 分钟生命周期只证明受控代码路径，不交付正常登录/可保留实例。

本片决定：**显式 opt-in 的本机试用组合**复用 `cmd/current-application`、现有 ZITADEL/Login V2、Auth.js/BFF、Product Review/Catalog 与 Listing Record 路由/服务；不得包裹旧 Task-first Service 或新建第二套事实源。试用配置缺失时，Review/Listing 路由一律不装配。试用配置只允许全新项目、loopback HTTPS、独立 Compose project/volumes；运行时不得执行 DDL、隐式授权或用超级用户连接。实现时从现有独立构造器抽取可复用的当前模块装配，保持原中间件、授权、deadline 和服务逻辑。当前应用中的 Review 准入须与 `productAgent.enabled` 解耦，单独批准本片的 Review-only 路由集合；不因打开审核而启用 Agent、模型、预算或提案生成入口。

这不是把 `productAcquisitionDatabase` 指向任意现有库，也不是修改全局默认应用。#590 正在修改同一 current-application/任务中心路径；本片 Writer 等其稳定后只同步一次实际 main，核实最终路由组合，不在其活跃共享写路径并行落生产代码。

## 事实、数据库和调用链

| Owner | 权威事实 | 试用装配与不变量 |
| --- | --- | --- |
| ZITADEL 与 Workbench context | 身份、组织成员/角色和生效企业；Auth.js 管会话 | 复用现有本地 issuer、项目角色、官方 Login V2；服务端重新验证 bearer、组织和读/写权限，不能由浏览器 cookie 单独授予权限。私密凭据不入仓库或日志。 |
| Catalog/Sourcing 与 Product Review | 当前商品版本、证据、提案/决定、Apply receipt | 同一个试用 Product 数据库中使用现有 Repository/UoW；GET 采用原授权，decision/Apply 用原 LiveWrite、精确 revision/base 与幂等身份；只有显式 Apply 可写 Catalog。试用不开放提案生成 POST，预置样本由一次性本地安装步骤写入当前 owner。 |
| ApprovedAsset、Store Center、Listing Record | 已批准素材、当前 Store、不可变本地 Listing record | 这四类事实与 Catalog/Review **共用一个全新试用 PostgreSQL 数据库边界**，满足 `NewSheinRecordApplication` 的既有共同边界假设；Store Center 是此实例唯一 Store owner。Listing 写入仍先验证 Store/Asset/Product，读集合仍按组织和 owner 范围。历史 Store ID 不推导当前 Store read。 |
| Console/BFF | 无 canonical 业务事实 | 既有 `/workbench/ai/tasks/pending`、`/workbench/ai/tasks/completed`，分别经严格、服务器配置的 Review/Listing API origin 到同一获准应用。保留原同源写校验、组织断言、取消/超时、大小上限、非法响应显错及固定诊断链接；不加兜底 DTO、假成功或前端本地保存。 |

最小流程：本机浏览器 → HTTPS Console/Auth.js → ZITADEL 登录及当前企业选择 → BFF → 当前应用的已批准 Review/Listing HTTP descriptor → 原领域服务/UoW → 同一试用 PostgreSQL；试用者看见的是 owner 回读的版本、receipt、record 和诊断。`action=publish` 仍仅表示本地发布准备，不是远端发布。

试用 Product 数据库可在新的 Compose project 内复用一个现有、**未写入业务样本**的 Store Center PostgreSQL 数据库作为共同数据库；只安装当前 Product/Asset/Review/Listing 所需 schema，并给独立运行角色授予所需的精确表/序列权限。这样 Store Center 仍只保有一份 Store 表，Review/Listing 不访问其他项目的数据库。实现前必须用现有 installer/当前 schema 检查落实上述共同边界；若现有 installer 不能在这一全新数据库上安全组合，应退回设计，不通过双数据库读写、复制 Store 或临时 fallback 绕过。

## 状态、故障、副作用与生命周期

- 不新增业务状态机。Proposal/decision/Apply 的版本、事务、stale、UNKNOWN、重放和撤权语义沿用 REV-1；Listing record 创建/集合/诊断沿用 DRAFT-S1。两个 owner 不互相驱动。相同幂等键不同载荷按原合同拒绝；超时/响应丢失不得生成新键推断未提交。
- 一次性样本准备使用已有 Catalog/Asset/Store/Review/Listing 当前 owner 的安装与写入路径，显式固定合成组织、用户、Store、商品、提案和本地 record。先安装 schema/角色，再以受限身份装配，最后准备样本并从 API 回读；失败时标记实例未就绪，不宣称部分 PASS。重复执行必须只复核同一身份/载荷，不能覆盖已试用修改或重复增加事实。样本不触发 provider、平台、支付、邮件外发；本地邮件只经 Mailpit。
- 服务启动 fail closed：缺少 opt-in、共同数据库身份不一致、schema/角色/issuer 或 API origin 不满足合同，均不挂载试用路由或保持明确不可用；不得在请求路径迁表/补 GRANT。请求继续遵守原 deadline、资源上限和权限错误语义。对其它可用账户能力不宣称 Review/Listing 可用。
- Compose project 名、外部端口、数据库/身份卷均唯一且在启动前验证无占用；只监听 loopback。stop/restart 保留所有卷；destroy/删除卷另需用户决定。交接仅输出公共 CA、地址、合成账号的私密凭据位置和合成样本 ID；不打印密码、PAT、cookie、原始 trace。不会对当前运行的其他项目执行 `down`、升级或改配置。
- 任何浏览器实际写入只作用于此新试用数据库；客户端取消不撤销已经提交的 Apply。试用实例若不完整，明确呈现 unavailable，而不造 Store 名、全局任务数、进度或平台发布状态。

## 实施切片与验证准入

同一个 Delivery Batch/主要 PR：① 冻结独立架构与受影响模块/角色合同；② 当前运行装配与严格配置、相关 TDD/Go 集成测试；③ 复用现有 Compose 创建全新项目、精确 schema/权限和合成样本，接上真实本地身份/Console；④ 在同一实例经浏览器完成 Review→独立 Apply 与 completed→诊断，stop/restart 后回读，并做撤权/跨企业负例；⑤ 最终 main 组合、集中独立复核、准确 HEAD 的必需 CI。仅当组合确有独立 main Blocker 时再讨论辅助 PR，不为了层数机械拆分。

验收分别记录：源码/类型/测试、准确 PR HEAD CI、正常本机实例运行健康、用户操作与独立验证、merge/main CI；无实际执行保持 `NOT_RUN`。作者和 AI 评审均不自行签发产品验收。真实 IAM 指**本机 ZITADEL 的正常登录**，仍是合成用户，不等于生产 IAM、真实业务数据或用户对外上线验收。

## Legacy decision

```text
Legacy decision: EXTRACT existing current Product Review and Listing Record assembly behavior; RETIRE any temptation to route through Task-first Product/Listing Workspace.
Reusable behavior: current owner repositories/UoW, HTTP descriptors, Workbench authorization, ZITADEL, Auth.js/BFF, Compose bootstrap and exact schema installers.
Current owner: Product Review/Catalog, Store Center, ApprovedAsset and Listing Record remain their respective sole fact owners; this trial composition owns no business fact.
Cutover/deletion condition: new opt-in trial points directly at current owners; no legacy adapter, dual read/write, migration or fallback is added. Nothing is cut over in default/shared/production runtime.
```

## Architecture Review 待核点

重点核对共同数据库是否能复用现有 Store Center schema 安装/权限而保持唯一事实 owner；Review-only 模块能否排除生成 POST 而完整保留决策/Apply 的能力绑定；当前应用路由准入及 #590 组合；一次性样本准备是否复用已有 owner 且不覆盖试用状态。任何需要第二 Store/Product 事实源、变更原事务/授权/recovery、打开 provider 或修改默认生产装配的方案，均超出本文件，须返回 #36 决定。
