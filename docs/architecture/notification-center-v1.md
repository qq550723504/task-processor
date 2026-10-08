# 通知中心 V1：站内通知与业务状态投影

> Status: DESIGNING / NOT_READY
> Design Basis: Independent Architecture
> Execution Issue: [#608](https://github.com/qq550723504/task-processor/issues/608)
> Delivery Batch: 当前登录用户从顶部通知入口查看官方消息、自己有权看到的业务提醒，并回到原业务完成操作。
> Product decisions: 用户于 2026-10-08 要求通知中心开工，选择 Figma 完整通知中心范围，并明确通知类型应按系统业务判断，不能局限于 Figma 示例。
> Investigation baseline: main `01cdce76040f56fc4eb7cdef301f5a4850893190`。
> 正式业务代码必须等当前执行 Issue 完成独立 Architecture Review 并记录 IMPLEMENTATION_READY 后才能修改。本文不是已交付能力声明。

## 1. 用户结果、范围与依据

使用者为已登录个人及当前企业成员。当前顶部「通知」按钮不可用，用户必须分别打开任务、审核、店铺、资源、付款和账户页面才能发现需要处理的事项。本批将这些当前事实投影到一个站内入口：查看列表与详情、按未读或待处理筛选、标为已读、跳转回原业务；刷新、重新登录和换设备后阅读状态仍保存。

依据：

- [最终 UI / IA Authority](../product/final-ui-ia-authority.md)。
- [全新系统基线](../product/greenfield-no-legacy-migration.md)。
- [项目边界](project-boundaries.md)、[Legacy Policy](../refactoring/legacy-hard-cut-policy.md)、[Legacy Register](../refactoring/legacy-register.md)。
- [AI Workbench](ai-workbench-chat-business-task-v1.md)、[标题审核](../engineering/product-title-review-ui.md)、[Store Center](store-center-current-application-v1.md)。
- [统一商业合同](unified-base-prepaid-resources-v1.md)、[钱包计费](commercial-wallet-billing-contract.md)、[充值](alipay-wallet-topup-design.md)。
- [成员邀请](account-facts-invitations-v1.md)、[推广结算](referral-settlement-and-conduct.md)、[主体认证](subject-verification-tencent-esign-design.md)。

本轮实际读取 Figma file `tg48P46SSXl6TBy9lZwg63` / page `31:463` 中以下可见节点的 design context 与截图，并只读核对点击 reactions：

| 产品投影 | 节点 |
| --- | --- |
| 顶部通知弹层 | `409:372` |
| 硕米官方通知 / 商家经营触发通知 | `422:323` / `422:523` |
| 通知详情 | `422:723` |
| 仅未读 / 仅待处理 | `498:359` / `498:621` |
| 浅色列表 / 详情 | `425:323` / `425:781` |

深浅色经营列表及筛选对应节点为 `425:551` / `498:490` / `498:750`。实现前按需读取这些精确节点，不从归档设计推导交互。Figma 的 3 条未读、2 条待处理、库存、广告和活动样例不进入生产数据。设计原稿未修改。

### Must

- 两类通知、顶部入口、列表、详情、未读/待处理筛选、全部已读、正常业务跳转、深浅色和移动适配。
- 当前业务有可靠事实的通知类型按 §2 覆盖；同一业务在多处投影不能重复报同一事项。
- 官方消息为真实发布内容；经营通知为当前授权的原 owner 投影，不能用示例、日志猜测、浏览器 toast 或短期缓存生成业务事实。
- 阅读回执持久化且按身份/企业隔离；已读不等于待处理解除。
- 跨企业切换、撤权、数据源不可用和 UNKNOWN 均如实呈现；不以总数 0 或「全部正常」掩盖读取失败。

### Should

- 新的同一事项版本成为未读，已处理后的版本显示最新结果。
- 详情保留消息类型、来源、范围、事实时间、阅读状态和建议入口。
- 来源以明确的可用状态说明，减少用户逐页面查找。

### Out of Scope

本批仅交付站内通知。短信、邮件、微信、浏览器推送、通知偏好/免打扰、模板编辑器、营销自动化、新的通用消息总线/调度器/Outbox 平台、任意插件通知、全企业审计历史以及替用户自动执行业务动作不在范围内。现有邀请 SMTP 与身份 OTP 的发送、retry/UNKNOWN owner 不变。

库存预测、广告诊断、选品机会、订单履约、站内商家聊天及报告生成的业务引擎没有因为 Figma 出现示例而获得新建授权。其通知类型保留为依赖缺失，不能记为 PASS，不能伪造或静默降为已完成。接入这些类型须有对应当前领域能力与可消费事实。

### Threat Model / 风险边界

必须保护当前授权数据：跨企业混淆、同企业私有任务泄露、个人提现/认证误归企业、通过摘要/计数/详情绕过原业务权限、把通知已读当业务成功、对 UNKNOWN 提供重复付款/运行等危险操作、官方发布入口错误授权，以及消息正文/链接注入。

没有另行批准的 Accepted Risk。全套安全告警平台、全部审计日志事件转通知和理论侧信道增强不从本批自动扩展。

## 2. 按当前业务确定通知类型

大分类沿 Figma 两类；下表是类型目录与准入映射，不是已经实现或已接线的声明。`CURRENT_FACT` 表示准确基线有当前 owner/事实，不表示已存在合格的通知读取接口；所有源仍须满足 §3 的授权、版本、分页、详情和故障合同。

| 类型 / 用户需要知道什么 | 权威事实 / 代码证据 | 可见范围与业务入口 | 当前判断 |
| --- | --- | --- | --- |
| 官方产品更新、系统/维护公告、活动、规则更新 | 新的官方公告 owner；Figma `422:323` 四个类型 | 已登录本人；发布由既有 verified platform-admin 权限保护 | NEW_OWNER，须本批完成真实发布、撤回与阅读回执 |
| AI 规划结果待核实、输出无效、方案待本人确认 | `internal/aiworkbench/contracts.go` 的 PlanningCommand / ExecutionProposal；尚未创建 BusinessTask | 当前企业 + 原 Conversation ActorID + Chat read；原 Chat 对话 | CURRENT_FACT；不因 Task 尚未建立而漏掉跨页面规划结果 |
| 业务任务待确认、完成、异常、暂停、结果待核实 | `internal/aiworkbench/contracts.go`、`projection.go`；Agent/Review 当前事实；`internal/app/httpapi/ai_workbench_tasks.go` | 当前企业 + task ActorID；任务详情，不扩大为企业全员任务列表 | CURRENT_FACT；需抽出可复用的授权投影 reader |
| 人工审核待决策、已接受待 Apply、拒绝/应用结果 | `internal/product/review/service.go` 与当前 Review owner | 继承 Review 原权限和可见范围；既有审核/Apply 页面 | CURRENT_FACT；有 BusinessTask 绑定时合并其通知，独立 Review 保持独立来源 |
| 1688 获取结果、采集受阻、可安全恢复/结果 UNKNOWN | `internal/product/sourcing/acquisition_operation.go`、`acquisition_charge.go` 与当前 Acquisition 应用 | 当前企业 + 原操作可见范围；采集结果/恢复入口 | CURRENT_FACT；浏览器本地文件不是服务端通知事实，无发布证据不报已保存 |
| 店铺连接失效/断开、服务到期/暂停、原操作结果待核实 | `internal/storecenter/store.go`、`connection_status.go`、`official_connection.go` | 继承 store.read 与 StoreAccess；店铺详情/连接/服务页面 | CURRENT_FACT；依赖 unavailable 不伪装成 expired，首次主动未连接不每次提醒 |
| 企业 AI/数据/续费资源用尽、本人分配或月度额度阻塞 | `internal/ledger/orgresource/balance_read.go`、`member_allocation.go`、`member_limit.go`；`internal/app/httpapi/unified_commercial_read.go` | 企业余额按 commercial.read；个人额度按本人原读取合同；资源页面 | CURRENT_FACT；不把基础方案解释成订阅即将到期，不发明低余额阈值或赠送权益 |
| 充值待支付、付款待核对、已付款待入账、入账结果、资源购买待核对 | `internal/commercial/billing/topup_contracts.go`、`contracts.go` | 原订单/企业商业权限；账单订单详情；UNKNOWN 仅查看核对 | CURRENT_FACT；不返回付款链接、二维码、商户凭据或把通知当资金证明 |
| 邀请投递失败/结果未知、待处理 Consent、接受/拒绝/过期结果 | `internal/organization/membership/inviteflow/domain.go`、`service.go` | 邀请管理者或可靠 RecipientID/当前验证联系绑定的本人；原邀请入口 | CURRENT_FACT；没有可靠已绑定收件主体时不把邮箱直接猜成 UserID，不复制 IAM grant |
| 成员/授权操作待核对或失败 | 当前 `internal/organization/membership` 操作回执；不是旧 memberinvite owner | 本人或已有成员管理权限；原 operation 核对入口 | CURRENT_FACT；不枚举其他成员的私有动作；权限撤销先隐藏受限经营数据 |
| 知识文件处理完成/失败、需重新处理 | `internal/knowledge/types.go`、`processor.go` | 原 Knowledge read/manage 可见范围；知识文件页面 | CURRENT_FACT；不从解析日志取私有内容，首次上传的临时状态不报错误 |
| 认证待本人继续/链接到期/结果待核实/已验证，个人认证拒绝结果 | `internal/subjectverification/service.go`、`personal.go` | 原个人/企业认证可见范围；本人继续认证仍检查 ActorID | CURRENT_FACT；个人原 owner 的 REJECTED 可消费，企业认证不另造失败状态；不复制证件、人脸、手机号、签名 URL，不重新发起 UNKNOWN |
| 推广收益调整与提现申请结果 | `internal/referraleconomics/types.go`、`internal/app/httpapi/account_referral_economics.go` | 个人 canonical Subject；推广/提现页面，独立于当前企业 | CURRENT_FACT；只有 canonical ledger/Withdrawal 状态，PAID 由原 owner 决定 |
| 库存预警、广告异常、市场机会、报告、店铺消息、订单履约提醒 | Figma 示例；当前没有本批可直接消费的完整新系统事实合同 | 未来对应 owner / 权限 / 正常入口 | DEPENDENCY_MISSING；逐类记录接入条件，不通过旧 ListingKit / marketplace 内部运行态补齐 |

普通成功操作不是全部都产生通知：只有跨页面/异步结果或当前需要用户处理的阻碍形成事项；按钮刚完成的同步保存沿用原页面反馈，避免把通知中心变成全量审计。当前可自动判断的是原 owner 已确认的状态及资源用尽，不擅自加入「7 天到期」「低于 20%」「连续 3 天」等新告警规则。若原 owner 提供已批准的预警事实，直接消费。

独立 Review 与 BusinessTask 的关联去重以 exact AgentRun/Review binding 为据；没有可靠关联时不可用标题、时间邻近或 ProductKey 猜测合并。付款、权益发放、收益调整是不同 owner 事实，只在 canonical 原操作关系已明确时组合展示，不互相替代。

## 3. 方案：授权的源投影 + 通知自有阅读回执

经营通知首版为 **源事实驱动的当前事项/动态列表**，不是承诺保存每个瞬时状态变更的历史事件仓库。完成结果来自 source owner 保留的 durable records；当前阻碍随原业务解除。用户未打开通知中心也不会丢失 source owner 保留的待处理/完成事实。源数据的保留/删除规则不由通知中心改变。

不采取「业务成功后顺手调用通知写库」：跨库写失败会制造消息丢失、业务重试或第二恢复责任方。读取已保存 owner facts + 写阅读回执不存在跨 owner 原子提交义务，也不需要另造 Saga。未来若产品明确要求保留全部瞬时转换，须先由对应 source owner 给出 durable event/outbox 合同，不能把本方案的 current projection 宣称为该能力。

### 3.1 Owner 与依赖方向

拟新增 `internal/notificationcenter`，仅拥有：

- 官方公告内容、发布版本、发布/撤回命令回执。
- source reference 的规范身份、类型映射、站内展示规则。
- 当前用户的已读回执与全部已读命令回执。
- 有界、短期的全部已读 reference 快照（不保存业务正文）。

经营事实、权限、业务已处理状态及外部副作用继续归原 owner。新 domain 不 import app/httpapi、GORM、provider SDK、旧 ListingKit、compatibility 或 tenantbridge。

```text
当前 owner 的已授权 NotificationSource reader
  -> notificationcenter.Service（映射、合并、阅读状态）
  -> notificationcenter/httpapi（固定路由和 protocol）
integration/persistence/notificationcenter -> notificationcenter.Repository
app/runtime + app/httpapi -> 构造现有 owner readers、注入 feature module
Console / dedicated BFF -> 当前身份、期望企业、严格 schema -> feature HTTP
```

现有 Task 投影大量逻辑位于 app/httpapi 的 private `taskView`，不能把新 domain 反向依赖该私有方法，也不能复制其中的权限/Review/UNKNOWN 判定。最小处理是把当前授权投影窄合同抽至 `aiworkbench` 适当 owner，app 只注入其已有 Agent/Review ports；同一 reader 同时服务 Task Center 与通知。抽取不改变 Task 状态、执行身份或权限。其他 source 同样从当前授权 service 取得数据，不能因 repository 可读而跳过服务授权。

### 3.2 Source 合同

每个静态登记来源实现如下语义；具体 Go 类型在准入后按本合同落地：

- `ListVisible(ctx, verifiedScope, sourceCursor, limit)`：在原权限内给出 items、next、完整性、source availability；无当前权限与依赖失败分开。
- `ReadVisible(ctx, verifiedScope, sourceRef)`：重读并验证 exact owner/resource/audience，返回当前同一事项或 NotFound/Denied/Unavailable。
- Item：source namespace、canonical entity ID、明确 notification type、语义 revision、真实发生/生效时间、最小安全标题/摘要、可见范围、当前 attention、typed business target。
- 不接受 browser 传入 ActorID、RecipientID、角色、状态、正文、owner names、任意 URL 或任意 provider payload 来创建经营消息。

语义 revision 是下表冻结的 source-owned canonical tuple 的有界编码，而不是通知库增加的一套业务版本。数值 version 存在时用原 version；其他 tuple 按固定 schema JSON 数组编码后 SHA256，禁止加入读取时间、租约心跳、原始正文或任意未定义字段。相同 owner facts 得到相同 ref；它是当前事项身份，不承诺区分源未保存的每个历史瞬时转换。列表/read-all 均消费同一生成方法。非法或缺少必需 identity 时该源 UNAVAILABLE，不能自动降为零。

排序用 source 的真实 `occurredAt`，同时间以稳定 source ref 排序。未知发生时间明确 unknown，不能拿任务创建时间冒充完成时间。source cursor 必须绑定当前 principal/企业/filters，按稳定原 source key 分页，不能把 count 当分页终止条件。

### 3.3 逐源可执行映射

下表定义本批要实现的窄合同，不声称这些方法已经存在。新增 reader 放在原 owner，persistence 只实现原 owner 的可选读 port；app 提供既有授权/能力 binder。通过授权后才读 owner repository；通知 domain 不接收 DB，不借其他 owner 的 serving role。所有枚举按原 durable key 做 keyset，固定 size <=100，limit+1 判断 next；纯 offset 或 limit(100) 截断接口不能用于 complete feed。新读 port 不改变原公开列表的 actionable 范围、状态机或源保留策略。

| source / 最小读取 seam | 授权与完整枚举 | entity / semantic tuple / 事实时间 | attention 与合法 target |
| --- | --- | --- | --- |
| workbench-plan：aiworkbench 的 PlanningNoticeReader | 与 Chat bind 相同 fresh identity + WorkbenchChatRead；org/Actor exact；按原 PlanningCommand key 枚举，连原 proposal 与 LookupTask | command invocation ID；`(State, TerminalDigest, ProposalID, 是否已确认, 是否对话归档)`；terminal CommittedAt，没有则 unknown | 未确认 READY proposal，UNKNOWN、INVALID_OUTPUT、FAILED_BEFORE_DISPATCH 为需查看；归档或已确认 proposal 不提醒，CLARIFY 留 Chat 页面；`/workbench/ai/chat/{conversationId}` |
| workbench-task：抽出 aiworkbench AuthorizedTaskProjectionReader | 与 Task bind 相同 WorkbenchTaskRead + org/Actor；沿 ListTasks keyset； exact request/run binding，复用 ProjectTask；缺失 run 与依赖不可用严格区分 | TaskID；`(executionRequestKey, RunID, projected state, reason, authorized review state/revision)`；Review Receipt.At 或最后 History.At 有权且可用时使用；其他终态时间 unknown | WAITING_CONFIRMATION/ERROR/PAUSED attention，RUNNING 不生成完成消息；`/workbench/ai/tasks/{taskId}`。无 Review read 权限只返回本 Task 状态，不暴露 Review ID/正文/target |
| product-review：review.Service 的 NotificationFacts | 复用 authorize 和原 resource check；新只读 port 枚举当前 scope 的 pending/accepted/rejected/applied，不能复用只含 actionable 的 List 冒充全量 | ReviewID + 原 Revision；applied 用 Receipt.At，accepted/rejected 用 History.At，pending 无发生时间则 unknown | pending/accepted attention。有原 exact AgentRun binding 且当前 Task 可读时只保留 Task 事项，否则独立 Review；`/workbench/ai/tasks/pending?proposal_id={reviewId}` 复用正常详情 selector，包括合法可读终态；不能拼旧 task/workspace 路径 |
| acquisition：当前 ProductSourcing AuthorizedAcquisitionFacts | 复用 Acquire/Read 的 fresh scope、源权限、actor/StoreAccess；新 operation keyset reader，仅本 org/actor，ReadPublished 必须当前可读 | OperationID；`(State, FailureCode, CommandHash, published publication identity)`，Fence/LeaseUntil 不作语义 revision；published 原 published_at，其他时间 unknown | failed/unknown/原 owner 的受阻结果 attention，published 仅在正常 ReadPublished 证据完整后为完成；`/workbench/supply/acquisition/operation/{operationId}` |
| store：当前 store 应用的 AuthorizedStoredNoticeFacts | fresh StoreRead + 原 StoreAccess；按 StoreID keyset；只读 StoreSnapshot/已保存 connection receipt，不调用 Status/QueryStore/Observe、解密凭据或后台查询 | StoreID + service/connection 子类型；service `(Version, effective service status, ExpiresAt)`；connection `(AttemptID, stored State, Status, ConnectionVersion)`；真实 ExpiresAt / owner durable observation time，缺失则 unknown | 曾有连接尝试的断开/过期/UNKNOWN、服务 expired/suspended attention；首次未连接不报失效；provider 当前不可查是来源状态，不能推断过期；`/workbench/stores/{storeId}` |
| org-resource：Commercial 授权投影 + orgresource EventReader | fresh CommercialRead；余额不自行授权，必须 app binder；三种资源固定集合。取当前 Balance + 最近匹配 durable EventID，不能只读 observedAt | org/resource；`(last matching durable EventID, 当前可用/债务状态)`；Event.OccurredAt。没有事件/使用事实的新企业零余额不生成“耗尽”；事件与余额无法核实时 unavailable | 原 available 已用尽或债务阻碍；不发明低余额阈值；`/workbench/plans/entitlements` |
| member-resource / member-limit：原 authorized services | canonical EffectiveMemberID，复用 ReadPosition/MemberLimitRead 的本人/管理员规则；本人资源固定集合与当前 UTC month，不枚举其他成员 | org/member/resource 的 Position.Version；月度 `(month, Version, 原 consumed/reserved/limit 阻碍 tuple)`；原 UpdatedAt，有效耗尽时间缺失则 unknown | 非默认/已使用的额度被原 owner 判阻碍才提醒；不把 member ID 当 UserID；`/workbench/account/organization/resources` |
| billing：当前 commercial authorized NoticeFacts | fresh CommercialRead + 原 order visibility；新 `(CreatedAt,OrderID)` keyset reader；只读订单与原 topup attempt，不调用 checkout、reconcile 或 provider | OrderID；`(Order.Version, TopUpAttempt.Version/Phase)`；owner 对应状态时间存在时用，否则 unknown；不把 CreatedAt 当到账时间 | awaiting-payment/reconciliation-required/paid-pending-credit attention；completed 原 committed order/ledger 为结果；`/workbench/plans/orders/{orderId}` |
| invitation-admin / invitation-recipient：inviteflow.NotificationFacts | admin 用 Manager，完整 org/ProjectID keyset。recipient 用当前 freshly verified email exact Contact + RecipientID（若已绑定必须同人），按同验证 contact 的当前 ProjectID keyset，不只按已绑定 ID 枚举；每次详情再次验证 | InvitationID；`(Revision, State, DeliveryState, DeliveryAttempt, effective expiry)`；delivery 用 DeliveryUpdatedAt，expired 用 ExpiresAt，其他没有转换时间则 unknown | pending Consent/accepting/delivery failed或unknown attention；accepted/declined/expired 为结果。新纯读 seam 复用 recipient/Manager/visible，禁止 ReadAdmin/ReadRecipient/reconcile；`/invitations/{id}` 或现有 members 邀请面板 |
| member-operation：membership.Commands 的 NoticeFacts | 复用 current() 前后检查及 MemberManage；org/project/Actor exact；新可选 receipt keyset port 枚举 pending + completed/rejected，不把当前 ListPending 声称为全量 | OperationKey + Revision；只采用对应当前 step 的有效 Acknowledgment.At，否则 unknown；pending 不擅自改名成失败或 UNKNOWN | pending 为“待核对”，rejected 为原拒绝结果，不猜 provider 成败；原 members 操作核对入口，GET 不调用 Resume |
| knowledge：授权的 Knowledge NoticeFacts | 与 knowledge HTTP 相同 fresh KnowledgeRead + org scope；按 BaseID/SourceID keyset 枚举当前 latest revision，拒绝 disabled/base 不可见项；不用不带授权的 Scope 参数绕过 route gate | SourceID；`(LatestRevision.ID, State, terminal Attempts)`；terminal AVAILABLE/PARTIAL/FAILED 的原完成 UpdatedAt，PROCESSING 的 lease/UpdatedAt 不进入版本或产生新消息 | FAILED/PARTIAL attention，AVAILABLE 完成；不拷贝解析正文、Failure 原始日志、object key；`/workbench/ai/knowledge/{baseId}` |
| personal / subject-verification：各原 owner AuthorizedNoticeFact | 个人只本人 canonical subject；企业按原 verification 权限/current org，继续动作仅原 Actor；单条当前 application 自然 complete，不枚举他人 | ApplicationID；`(State, effective expiry, VerifiedAt)`；verified 用 VerifiedAt、expired 用 ExpiresAt，UNKNOWN 时间 unknown；personal REJECTED 仅个人原状态 | pending/expired/unknown attention，verified/rejected 为结果；不调 provider/refresh/observe，不输出 URL/证件/手机号；本人或企业现有 verification 页面 |
| referral：个人 authorized Earnings/Withdrawal NoticeFacts | current subject + 既有 referralRequest 身份规则，不依赖企业；新 owner `(OccurredAt,EntryID)` 与 `(CreatedAt,WithdrawalID)` keyset，不复用最近100截断方法 | adjustment EntryID（不可变 revision 1）；WithdrawalID + Version；ledger OccurredAt/withdrawal UpdatedAt | 仅 canonical 收益调整与申请跨页面结果；无收益或未申请不提醒；`/workbench/account/referrals/earnings` / `withdrawals`；不返回付款目的地或 payment reference |

Task/Review 合并仅在同一个被授权集合内进行：Review 的 exact AgentRun 与 Task exact run 关联；选择 Task ref 作为阅读身份，Review 不再产生第二条同义通知。只拥有 TaskRead 的人读取状态-only seam（补齐 state+semantic revision，无 ReviewID），不能借去重取得受限 Review 的标识或 CTA；只拥有 Review 权限的人仍可读独立 Review。所有关联失败返回明确 unavailable/stale，不按 ProductKey/标题/邻近时间猜测。

原 source writer 仍负责恢复、核对及状态提交；通知 reader 禁止调用含这些动作的既有“GET”方法。读取授权允许既有 live directory/profile 只读查询，但不增加 provider 请求、任何数据写入、付费查询、SMTP、模型调用或新的后台任务。新窄查询可在同一 owner DB 的短只读 repeatable-read 事务内固定该次枚举；不同 owner 没有共同全局 snapshot。

### 3.4 聚合、分页与计数

V1 不做跨 source k-way merge 的错误优化。一次源收集先完整枚举所有当前可见且已接入的 source，去重后才按真实 occurredAt 降序（unknown 排末尾），再用 canonical ref 稳定排序、计算未读/attention 和分页。每个 source report `observedAt`；它仅描述读取，不作为发生时间或版本。source 支持原事务下 stable keyset，页间不能前进或 identity 漂移即 incomplete。

单次集合最多扫描 10,000 个候选 ref / 8 MiB 安全投影，单页 size100；达到总 deadline/上限但仍有 next 返回 INCOMPLETE，并关闭全部已读，不把截断当 complete。源来源 AVAILABLE/EMPTY/DENIED 都可结束枚举，Denied 不含 count/items；UNAVAILABLE/未完成的已承诺 CURRENT_FACT 源使当前总数 partial。DEPENDENCY_MISSING 的未开放引擎公开 coverage 缺口且不算已接入源的 exact count，exact 标签明确“已接入来源”；不是全系统完成声明。

列表 `count/unread/pending` 在整个去重后的已授权集合计算，不用 browser loaded rows；partial 时只返回已加载数量和 exact=false。cursor 编码 realm/principal/context/category/filter/last sort key，严格校验绑定，不授权新 ref；源集合变化的普通列表允许刷新，不承诺翻页期间历史事件不变。批量已读使用下一节独立 immutable refs snapshot，不能复用动态 cursor 作为快照。

## 4. 持久化、已读与并发

拟使用独立 PostgreSQL schema `notification_center` 和最小权限 runtime role；独立连接池最多 4，默认复用当前应用指定的 PostgreSQL 实例而不是复制 business databases。初始化用既有 init/migration 流程；serving role 不建表、不访问其他 source schema、不继承广泛 owner role。新 schema 只保存本系统新通知事实和阅读回执，禁止 legacy migration/backfill。

最小数据：

- 公告：不可变 ID + revision、类别、plain-text/结构化段落、真实 publisher、audience、published/withdrawn 时间、typed internal target。
- 阅读回执：`(issuer, subject, scope_kind, organization_id, source, entity_id, notification_type, semantic_revision)` 唯一；首次已读时间。个人/官方 scope 不伪造 Organization，企业 scope 必须有组织 ID。
- 公告命令/全部已读命令：scope + operation + idempotency key + payload fingerprint + committed receipt；同 key 不同载荷返回 conflict。

`unread -> read` 为通知 owner 唯一阅读转换；同 revision 重放不产生新效果，新语义 revision 没有旧回执即为 unread。`attention` 是重新读取 source facts 的投影，不能由 read receipt 转换为 completed。撤回公告不出现在新列表/计数，读回旧 ID 返回 withdrawn/not-found，不删除命令证据。

详情 GET 不写阅读事实；浏览器在成功载入 detail 后发显式已读命令。列表预取和浏览器 crawler 不能把消息标为已读。已读成功必须晚于 durable commit；响应丢失后重放同 key/相同 exact ref，或读回确认，不能仅靠本地乐观状态报保存成功。

### 4.1 全部已读协议

1. 用户点击当前分类的全部已读后，BFF 发显式 `POST /snapshot`，只携带类别和期望 identity/org，不提交消息 ID 或正文。它调用 §3.3/3.4 完整收集（包括当前分类全部分页，固定 filter=ALL，含筛选隐藏行），去重后保存 exact refs。普通 GET 列表/详情不建 snapshot、不写任何表。
2. snapshot 在通知库保存 UUID、可信 realm/subject、当前 context（personal 或 exact Organization）、category、filter=ALL、source schema version、按 canonical ref 排序的全部 `(item scope, source, entity, type, revision)`、fingerprint、createdAt、expiresAt。无正文、金额、联系信息或跳转 URL。TTL=5分钟，最多10,000 refs、编码后8MiB；每个 subject/context 最多4个有效 snapshot。建立时短事务清理该主体已过期 snapshot；超上限返回429，不覆盖另一活动快照。不加 scheduler，过期 refs 不参与读取。
3. 响应仅 UUID/fingerprint/expiresAt/exact ref count（受原授权），不传全量 refs。UI 随后 POST `/read-all`，body只有 snapshot ID + fingerprint，单值 UUID Idempotency-Key。公司业务分类完整集合可包含“当前企业 source + 本人 personal source”，每条 receipt 保存自己的真实 scope；没有企业时使用 personal route。官方完全独立。跨企业或 identity 切换立即取消该命令，不复用 snapshot。
4. 执行时首先验证当前身份/context，校验 body fingerprint，并在通知 DB 查询该范围和 key 的 committed command。相同 payload 已提交时返回“原命令已提交”的回执，即使 snapshot 已过期；只返回命令状态，无旧正文/ref count。异载荷 conflict。没有 committed receipt 时才加载 unexpired snapshot，验证 principal/category/schema/fingerprint，然后所有 sources 重新做 fresh auth，并对每个原 ref 纯读验证 exact identity/revision 和原 scope。事项消失、撤回、语义 revision 变化、源不可用、撤权或过期都返回409 STALE/403/503，不提交一部分。
5. 全部验证通过后，短通知库事务锁定 snapshot、再次检查 TTL，INSERT ON CONFLICT DO NOTHING 原 exact refs 的 reading receipts，同时 INSERT command receipt；并发相同 key 以唯一约束串行，异载荷 conflict，任何失败回滚全部写入。只有 durable commit 后成功。源没有锁进通知事务，因此不宣称跨库原子时刻；如果源在最终授权/读取之后变化，事务仍只标原 exact refs，绝不会标新 revision。业务变更不回写 snapshot 或通知库。
6. 新消息/新 revision（含迟到旧时间消息）不属于 snapshot，仍未读；已提交的同 key 重放不扩大范围。commit ACK 丢失时保留 UNKNOWN，不换 key/重建更大快照自动重试；同 key重放或原 command 查询确认。未提交且过期只能刷新后由用户再点，使用新的 snapshot/key。两个不同 snapshot 可以安全标相同 ref，回执唯一约束使之幂等。

snapshot 来源为一次被完整收集且授权的 reference 集合，带每个 source observedAt；不是多个 owner 在同一时刻的跨库历史快照。UI在保存期间显示 pending，成功后重读；不得只清 browser unread dots。时间水位会错误消灭迟到的旧时间消息，客户端自报集合会错过其他页，两者均禁用。

snapshot 创建也使用该主体/context 的 UUID Idempotency-Key：payload固定category+ALL+source schema version，同key同payload返回原snapshot（过期则STALЕ，不重新收集），异payload conflict；新snapshot与其创建命令回执共事务。单条read同样将exact ref和committed command receipt共事务；原read同key重放不标新revision。用户/context snapshot配额用同一scope锁串行检查，不能并发越限。

## 5. 身份、企业与业务操作

- 官方消息和个人经营消息按既有 authenticated identity 的 issuer + canonical subject；无企业也可访问本人及官方通知。
- 当前 AuthenticatedIdentity 没有 issuer 字段。realm 由 app 从同一 current identity verifier 使用的可信 ZITADEL IssuerURL 注入，精确使用该验证器已验证的 issuer 字符串，不接受 request 的 realm/issuer；canonical subject 只取已验证 UserID。issuer 配置变更是不同身份 namespace，不迁移或重用旧回执。
- 企业经营消息按当前 effective Organization 的 live authorization，再按源业务权限/本人/StoreAccess 过滤；与任意 home Organization 不混用。企业代管不扩大本人 Task 或个人收益的可见性。
- 不能用一个新 notification.read 权限自动授予所有业务源。计数、摘要、详情、全部已读和跳转都遵循同一原权限；无权限来源不泄露事项存在性。
- 官方发布/撤回使用既有 `AuthPolicyCurrentIdentityWithVerifiedRoles` + `PermissionListingKitPlatformAdm` + `OrganizationAccessPolicyNone`，不新增管理员体系。官方消息可见不等于有发布权限；创建者团队标签不是 browser 可伪造的身份声明。
- 当前企业 source 请求带 `X-Expected-Organization-ID`；个人写入使用当前主体校验。BFF 使用现有身份与 same-origin CSRF 检查，不把 token 返回浏览器。
- 用户/企业切换、退出、撤权时，清空 query/detail/popover/count 和迟到请求结果；隔离 query keys。不能把上一企业的 pending 或本人提现摘要保留到新的 scope。
- typed target 只生成当前正常站内 GET 入口。通知点击不付款、不接受邀请、不 Apply、不重启 Agent、不重新调用 provider；目标页重复执行自己的权限、version、幂等和 Consent 检查。
- UNKNOWN、unknown_reserved、PAID_PENDING_CREDIT 等显示核实结果或等待原 owner 处理，禁止通知层提供「重试执行/再次扣款」捷径。

个人源与企业源使用独立描述符/读取通道，企业 gate 失败不吞掉官方及本人合法通知，也不能继续显示旧企业缓存。

## 6. HTTP / UI 与资源边界

新增固定 feature routes，保持个人/官方与企业授权通道分离：

```text
GET  /api/v1/notifications/official
GET  /api/v1/notifications/personal
GET  /api/v1/workbench/notifications
GET  上述 scope 的 /:notification_ref
POST 上述 scope 的 /read
POST 上述 scope 的 /snapshot
POST 上述 scope 的 /read-all
GET  上述 scope 的 /commands/:command_id
POST /api/v1/platform/notifications/official
POST /api/v1/platform/notifications/official/:id/withdraw
```

路由 Module 固定 notification-center。通用 source gate 不能放宽原 domain gate：

| route family | AuthPolicy | OrganizationAccessPolicy / Permission | handler 第二层要求 |
| --- | --- | --- | --- |
| official / personal 所有读、read、snapshot、read-all、command | CurrentIdentity | None / 空 | 可信 realm + 当前 UserID、TokenExpiresAt；个人 source 独立检查本人和原 contact/provider身份读取规则 |
| workbench 所有读、read、snapshot、read-all、command | CurrentIdentity | LiveWrite / 空（逐源 permission） | ExpectedOrganization 必须当前 effective org；所有公司 source 与 app 注入的同一 live auth binder +原 domain permission/resource check；personal源再按本人通道读，无新 notification.read broad grant |
| platform official publish / withdraw | CurrentIdentityWithVerifiedRoles | None / PermissionListingKitPlatformAdm | 当前 verified platform role；publisher只从验证 identity构造；body不允许publisher/subject/org |

所有 descriptor RequestTimeout=10秒，GET RejectUnreadRequestBody=true；POST 使用既有 WithRequestBodyReadTimeout(5秒)，有界严格 JSON duplicate/unknown key拒绝，single Idempotency-Key UUID。commands 返回本人/context 的 committed 状态，无正文和旧权限统计。未经组织 gate 的 source 不能通过 personal path读取。公司整个 gate失败时客户端仍独立读取官方/personal；经营列表明确该企业不可用，禁用“全部已读全部经营”直到当前范围可重新确认，不能报告公司部分成功。

公司 route 的经营集合包含当前企业 sources 和本人 sources，所有 source items仍有真实scope；personal route 为无企业时本人经营集合。UI只采用其中一个经营集合，不能同时 union 两个 endpoint 的重复个人事项。企业来源单独 unavailable 时可显示个人和其他成功sources，counts与bulk按 §3.4 标 incomplete。

### 6.1 官方公告与启动注入

官方 audience 固定 ALL_AUTHENTICATED_IN_REALM，仅当前配置 realm 内已登录主体；不接受角色/企业/contact分群，没有新营销分群要求。公告四个类别 allowlist 为 PRODUCT/SYSTEM/ACTIVITY/POLICY。发布 ID 为服务端 UUID，revision=1，正文/标题/段落/typed internal target发布后不可变；无草稿/编辑/计划发布状态。内容更正须撤回旧 ID后显式发布新 ID，不改写已读内容。

publish body只接受 category/title/summary/paragraphs/typed target，Idempotency-Key绑定 realm+publisher+operation+canonical body fingerprint；同key同payload返回原ID，即使其后撤回；异payload 409。publishedAt=该通知 owner 的真实commit时间，publisher=验证主体，团队来源标签=服务端配置，不能伪造具体人名。服务器不接受任意 HTML/URL或外部营销SDK trigger。

withdraw body仅 expectedRevision（须为当前发布version1），path为exact公告ID。fresh平台权限后短事务锁公告，PUBLISHED revision1 -> WITHDRAWN revision2，并写同事务命令回执/withdrawnAt。相同key同payload在撤回后重放原回执；不同key且version不匹配409。撤回不删命令/reading事实，普通detail为NotFound，列表/计数隐藏；不自动触发第二次发布。所有命令无外部发送副作用。

app/runtime/currentapplication 的 typed config 增加可选 notificationCenterDatabase，显式 schema=notification_center、user=notification_center_runtime、MaxConnections<=4；与 source pools 的credentials隔离，realm取 cfg.Identity.IssuerURL。source未配置保留对应 unavailable/coverage，不借default DB。首次初始化复用现有 schema init command 的管理员连接生命周期（新 feature-owned schema-init入口），创建schema/tables/indexes并授最低必要列/表权限；serving startup只ValidateSchema/privileges，不AutoMigrate/DDL。

currentapplication.ApplicationFeatures 注入 notification pool及当前source services/authorized reader factories；app/httpapi/current_application 的builder把 current live auth依赖注入各owner reader，构造notification service，再固定validate descriptors/register feature module。独立 pool由current runtime close，与request无关；不会另启worker或scheduler。初始化和runtime manifest提供正常启动说明，实际共享部署不在授权内。

普通列表默认 20、最多 100；顶部最多 4。过滤值固定为官方 unread、经营 pending；浏览器只接受 allowlisted source/type，并验证单值 query、ID、cursor、schema、数字和时间。

默认完整 request deadline 10 秒；单 source 2 秒（该source全部分页共享此budget）、最多 4 个 source 同时读取；每页 response 上限 128 KiB，普通 read/snapshot/read-all command 上限 16 KiB，公告正文上限 32 KiB。GET 配置 RejectUnreadRequestBody，拒绝未读 body 后再调用 owner。全部已读snapshot上限按 §4.1；超过容量明确失败，不静默截断。

输出包含每个来源的 `AVAILABLE / EMPTY / DENIED / UNAVAILABLE / DEPENDENCY_MISSING` 与 completeness。Denied 不返回 count/ref。来源 unavailable 不报 0；全局总数只有全部当前可见来源有完整计数时才标 exact，partial 明确显示「部分消息暂无法加载」。不把基础设施健康、未开放 source 或没有配置的 provider 生成成每个用户的虚假业务故障消息。

正文用纯文本或有界结构化段落；React 默认转义，不用 raw HTML。URL 由 typed target 映射，拒绝外链、javascript、data、协议相对地址和任意 redirect。附件/支付码/凭据/provider 原始结果不在通知正文或普通日志中。

UI 复用 `WorkspaceAppShell`、当前 Console Page/Panel/Button/tokens；通知是顶部全局入口，不新增一级菜单。复用既有 design 静态资产的精确匹配，新增资产仅使用 Figma 导出文件；截图不作实现资产。未读与待处理分开；图标不是状态的唯一提示；键盘打开/关闭弹层、Escape、返回焦点、loading/error/empty、390px 无溢出和深浅色继续当前实践。

## 7. 开源复用决定

已核对 [Novu 官方自托管与 Inbox 文档](https://docs.novu.co/community/self-hosting-novu/deploy-with-docker)：提供独立 API/worker/dashboard、in-app Inbox、服务端 trigger SDK，Go/React 均有接入路径。它适合站内与多渠道 workflow，不能替代本系统原 owner 的授权、UNKNOWN、业务去重和事实适配。当前没有多渠道需求；引入其完整服务与 subscriber/workflow 配置增加独立运行与身份同步成本。

方案因此复用现有 PostgreSQL 事务/唯一约束、Casbin/当前身份、feature HTTP、Zod/BFF、Console UI。仅实现不可替代的系统业务投影与阅读回执，不另造 queue、template engine、IAM、transport 或重试平台。第一轮独立评审确认该复用选型可接受；未安装 Novu，也未调用任何外部通知 provider。

PostgreSQL 阅读去重以[官方 INSERT/ON CONFLICT 合同](https://www.postgresql.org/docs/current/sql-insert.html)和仓库既有事务模式为依据，不用先查后插的无锁内存去重。公告修改/撤回与命令回执共事务；通知事务绝不修改业务库。

## 8. Legacy decision

Legacy decision: N/A（仅复用当前合格 Console 和 source owner；不是旧内部 Task Dashboard）。

如正式实现发现旧 Task/ListingKit/tenantbridge 参与一个来源，只能 EXTRACT 当前有效行为到当前 owner 或 RETIRE 该通知来源；不能建立通知 adapter 包装旧 Service。旧日志、历史 task/queue、旧平台 Workbench 状态不作为当前通知源。无旧数据导入、旧 ID 映射、双写、fallback 或全环境清理。

## 9. 当前准入缺口与开工边界

本设计覆盖完整用户通知中心及当前业务类型。第一轮独立Architecture Review对初稿724921c0给出NOT_READY，三条finding均命中“核心happy path按当时设计无法完成”，分类BLOCKER、阻塞正式实施。一次增量定稿如下，等待第二轮只复核相关增量：

1. B1：逐源authorized纯读seam、identity/revision/真实时间或unknown、完整keyset枚举与Task/Review去重，已明确于 §3.2–3.4。reader未编码本身不是设计Blocker；生产实现留同一PR。
2. B2：bulk snapshot、realm/context绑定、TTL/容量、完整分类/去重/计数、source改变/撤权/响应丢失与提交协议，已明确于 §4.1。
3. B3：官方audience、不可变发布/带expected-version撤回、可信realm与scope descriptors/初始化注入，已明确于 §5–6.1。
4. Planner遗漏归IMPLEMENTATION_TEST，现已纳入目录/读取合同。权限与业务状态/UNKNOWN/commit响应丢失等实现测试按 §10收敛，不扩建验证平台。

正式业务代码仍等待第二轮明确IMPLEMENTATION_READY并在执行Issue记录Ready；不能以本次文档定稿自行签发准入。第二轮后只有新增的真实Blocker重开架构，非Blocker转实现测试或Backlog。

DEPENDENCY_MISSING sources 保留各自阻塞与 owner；不把库存/广告等业务引擎作为站内通知基础部分开工前置，但完整类型落地仍不能声称完成。执行 Issue 必须公开这些缺口，用户要求的完整通知范围不因设计表格存在而完成。

## 10. 实现与验证、交接

一个 Writer、一个 `codex/notification-center` 分支与一个主要 PR。本文件初稿阶段只交设计/事实映射；准入后的实现、必要测试、findings、CI 修复留同一主要 PR。

业务行为使用 TDD：先覆盖权限/同企业私有任务/个人跨企业、状态映射及同事项去重、read != handled、新 revision 未读、全部已读与并发新消息、unknown commit 重放，再实现最小代码。真实 PostgreSQL 检查 scoped unique/事务/并发回执和 serving privileges，HTTP/BFF 检查 source 授权、期望企业、CSRF、严格/有界 body、deadline 与无状态变更 GET。

不新增 runner、故障注入平台或验收 session 框架。浏览器使用已有隔离测试能力，验证顶部 -> 两类列表 -> 筛选 -> 详情 -> 持久化已读 -> 正常业务页面，并核对深浅色、移动布局和截图。fixtures 明确合成；没有真实业务 source、外部 provider 或用户操作就保留 NOT_RUN。

交接包含真实启动命令/入口、登录步骤、公告发布方法、哪些 source 已接入/被阻塞、数据保存与 stop/restart 方法。实现者只报告开发自检和 CI；独立最终检查覆盖真实 diff 与业务路径，产品验收由用户或指定独立验证者确认。无合并、部署、关闭 Issue、收费调用、真实数据读写或容器数据删除授权。
