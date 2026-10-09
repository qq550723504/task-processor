# 指定企业私有智能体交付 V1

Status: DRAFT / NOT_READY (2026-10-09). Design Basis: Independent Architecture. Execution: #611, primary PR #617.

## 产品结果与权威

用户本会话确认继续推进实际私有智能体交付，并在首个业务选择中授权“你想一个出来吧”。首个业务为商品资料质检：客户手动输入名称、材质、尺寸、规格与描述，生成缺失项、重复/矛盾值及修改建议的报告。首版是确定性资料检查，UI 明示检查规则与限制；没有模型生成、语义推断、行业合规判定或质量保证。报告不是商品正式事实，不修改商品，不发布，不调用外部系统，不收费。后续模型能力须另行按现有 governed AI 准入，不能伪造 Product Agent binding 或借用 ChatPlan operation。

Must：平台专员在定制单上交付代码注册的精确版本；目标企业从已保存需求取得；仅该企业有 agent.read/use 权限的当前成员能读取/执行；我的智能体出现真实私有入口；输入、规则版本、报告、执行 actor/time 真实保存，刷新/重启可读；同键同意图只有一个报告，同键不同意图冲突；人工交付说明与实际可执行交付分开。

Out of Scope：通用插件/Agent runtime、第三方代码/任意 prompt/tools、模型调用和凭据、付费/点数、自动联系、客户知识库、图片/文件输入、商品事实修改、版本升级/转移/单独撤销交付授权。V1 授权持续绑定原需求企业，成员撤权/企业访问撤销立即由既有 live authorization 生效；暂停整个本功能可移除 runtime 模块。不是个人账户私有，UI 明确当前企业。指定真实客户处理与用户验收未授权，使用独立测试企业。

UI Authority：docs/product/final-ui-ia-authority.md；当前 Figma tg48P46SSXl6TBy9lZwg63 页面31:463。2026-10-09 已读取 4703:429 我的智能体企业语义页面（含截图），复用已有 Console/Card/Button/Input/tokens；详情沿用详情页面结构、由本产品决定定义输入与报告，不引入新的顶层页面。定制进度沿用原批准设计，增加真实交付链接。

## Owner 与最小调用链

`agentcustomization` 拥有定制需求与交付聚合：人工事件、immutable delivery（固定定义/版本/目标企业）以及该交付产生的 immutable execution report。它不是 Product Catalog/AgentRun/agentconfig/AI ledger 的第二事实源。报告是交付执行结果，不是 canonical Product，不写已有商品库。

`internal/product/quality` 是无 IO 的当前 Product 资料检查能力，负责 typed input、规则与报告计算。复用标准库校验/集合；仅检查确定性缺失、重复规格键（相同/不同值）、名称或描述相同重复信息、尺寸未说明单位，不猜测事实。不存在值得引入通用规则引擎的当前复杂度，不再建报告平台。

contract → implementation → injection → consumer：quality.Check → agentcustomization.Service/Repository → 同库 SQL adapter → 当前 customization runtime module/正常 HTTP authorization → 已有同源 BFF → 我的智能体私有卡片/详情/报告；平台 progress 命令 → 单库事务发布并关联交付。

现有 `internal/agent` 的 Binding 与 Candidate 强绑定 Product publication/catalog/title proposal，不能用虚构 Product ID 运行人工资料报告。本批保留此边界，质检直接调用有界 read/compute 领域能力；不新增第二个通用 Agent runtime，也不扩展已有 Product title 模板。

## 交付与持久化

复用原独立 agent_customization DB/pool/受限 role/installer。新增 `deliveries(id uuid,request_id uuid UNIQUE FK,organization_id text,payload jsonb)`、`quality_runs(id uuid,delivery_id uuid FK,organization_id text,actor_id text,key uuid,fingerprint text,payload jsonb, UNIQUE(organization_id,actor_id,key))`，两表仅 SELECT/INSERT；索引支持按组织/交付 UUID cursor 有界分页。schema 安装明确执行，serving 构造只 VerifySchema，不 DDL，不初始化示例。

平台进度 Update 增加可选 `deliverQualityAgent=true`。只在目标阶段 DELIVERED 可用（从 DEVELOPING 或 DELIVERED 同阶段记录）；仍要求交付说明、原报价/线下确认和强 If-Match。固定代码定义 product.quality.check / 1.0.0，不接受客户端定义/版本/企业。发布 ID 绑定原请求及精确版本；每单至多一交付。需求锁事务同时写 delivery、更新阶段/版本、事件和原命令回执，全部成功或回滚。相同命令重放仍先取原回执；后续同阶段记录不会重复创建交付。

Request 只存 deliveryId 作为关联，详情读取 delivery owner；不会把旧人工 DELIVERED 自动转换成可用智能体。空安装没有交付。My Agents 私有列表只读 delivery 表的当前企业，独立于 Product title 依赖的加载/错误状态，避免 title runtime 未安装阻断本路径。详情与执行拒绝不同企业的 deliveryId；专员权限不产生企业执行权限。

执行没有 pending/running/unknown 新状态机：同步纯 CPU 规则在一数据库事务内算出结果并持久化 immutable run。输入上限 16KiB，字段字符上限固定（名称120、材质240、尺寸240、描述4000；规格最多20项，键80/值240），输出最多固定数量检查项。客户可保留空字段接受缺失诊断，整个空输入拒绝。每页20，UUID cursor；HTTP/BFF deadline 沿用30/35秒，GET 拒绝未读 body/未知 query，写严格 JSON/同源/UUID key。

完整执行幂等身份 `(organization, actor, key)`，fingerprint 包含 delivery、typed input、固定定义/版本。事务先 advisory lock 完整身份，再检查当前企业 delivery 与原报告。同意图返回原报告，不同资源/输入冲突。不存在外部发送或资金预留；失败回滚可重试同一 key，响应丢失读取/重放原报告。页面保留冻结原 org/user/输入/key 的小型 sessionStorage 命令，撤权/切企业卸载并 abort；刷新可核实原操作，不以新 key 自动重跑。任意输入仅 React 文本渲染，无 HTML 执行、网络或日志正文。

## 权限与验证

私有列表/详情/报告复用 CurrentIdentity + EffectiveOrganization + LiveWrite（含读取）及 agent.read，运行用 agent.use。不接收 org/actor/platform；HTTP route descriptor 明确仅 enterprise。平台发布仍通过 VerifiedRoles + platform-admin + OrgNone，无需推导企业上下文。BFF 延续 identity/org 期望头、正常企业 cookie、same-origin mutation、严格响应大小及身份一致性检查。

TDD：纯能力缺失/重复/矛盾/单位规则；真实隔离 PG 原子发布、并发同键、冲突/CAS、跨企业猜链接、报告读回与不可变权限；正常 HTTP middleware/BFF 路由/输入 bounds；前端读取及冻结命令恢复。沿用现有必要测试，不建 runner/通用验收平台。独立高风险准入检查后实现，最终完整路径检查一次。正常本地入口供用户操作，开发自检不等于用户验收。

Legacy decision: EXTRACT 合格 Console/当前 auth/BFF/SQL transaction/installer；RETIRE 对所有定制交付都展示可用的占位语义。无 legacy wrapper、旧数据迁移、双读双写；旧手工记录保持原人工事实。

## 准入

待独立 Architecture Review。仅设计文档与 Issue 范围先行；IMPLEMENTATION_READY 前不改正式业务代码。原人工设计仍作为其范围的冻结基线，本文件仅覆盖上述新边界。
