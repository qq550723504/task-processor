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
| 业务任务待确认、完成、异常、暂停、结果待核实 | `internal/aiworkbench/contracts.go`、`projection.go`；Agent/Review 当前事实；`internal/app/httpapi/ai_workbench_tasks.go` | 当前企业 + task ActorID；任务详情，不扩大为企业全员任务列表 | CURRENT_FACT；需抽出可复用的授权投影 reader |
| 人工审核待决策、已接受待 Apply、拒绝/应用结果 | `internal/product/review/service.go` 与当前 Review owner | 继承 Review 原权限和可见范围；既有审核/Apply 页面 | CURRENT_FACT；有 BusinessTask 绑定时合并其通知，独立 Review 保持独立来源 |
| 1688 获取结果、采集受阻、可安全恢复/结果 UNKNOWN | `internal/product/sourcing/acquisition_operation.go`、`acquisition_charge.go` 与当前 Acquisition 应用 | 当前企业 + 原操作可见范围；采集结果/恢复入口 | CURRENT_FACT；浏览器本地文件不是服务端通知事实，无发布证据不报已保存 |
| 店铺连接失效/断开、服务到期/暂停、原操作结果待核实 | `internal/storecenter/store.go`、`connection_status.go`、`official_connection.go` | 继承 store.read 与 StoreAccess；店铺详情/连接/服务页面 | CURRENT_FACT；依赖 unavailable 不伪装成 expired，首次主动未连接不每次提醒 |
| 企业 AI/数据/续费资源用尽、本人分配或月度额度阻塞 | `internal/ledger/orgresource/balance_read.go`、`member_allocation.go`、`member_limit.go`；`internal/app/httpapi/unified_commercial_read.go` | 企业余额按 commercial.read；个人额度按本人原读取合同；资源页面 | CURRENT_FACT；不把基础方案解释成订阅即将到期，不发明低余额阈值或赠送权益 |
| 充值待支付、付款待核对、已付款待入账、入账结果、资源购买待核对 | `internal/commercial/billing/topup_contracts.go`、`contracts.go` | 原订单/企业商业权限；账单订单详情；UNKNOWN 仅查看核对 | CURRENT_FACT；不返回付款链接、二维码、商户凭据或把通知当资金证明 |
| 邀请投递失败/结果未知、待处理 Consent、接受/拒绝/过期结果 | `internal/organization/membership/inviteflow/domain.go`、`service.go` | 邀请管理者或可靠 RecipientID/当前验证联系绑定的本人；原邀请入口 | CURRENT_FACT；没有可靠已绑定收件主体时不把邮箱直接猜成 UserID，不复制 IAM grant |
| 成员/授权操作待核对或失败 | 当前 `internal/organization/membership` 操作回执；不是旧 memberinvite owner | 本人或已有成员管理权限；原 operation 核对入口 | CURRENT_FACT；不枚举其他成员的私有动作；权限撤销先隐藏受限经营数据 |
| 知识文件处理完成/失败、需重新处理 | `internal/knowledge/types.go`、`processor.go` | 原 Knowledge read/manage 可见范围；知识文件页面 | CURRENT_FACT；不从解析日志取私有内容，首次上传的临时状态不报错误 |
| 认证待本人继续/链接到期/结果待核实/已验证 | `internal/subjectverification/service.go`、个人认证 owner | 原个人/企业认证可见范围；本人继续认证仍检查 ActorID | CURRENT_FACT；不复制证件、人脸、手机号、签名 URL，不另造失败状态或重新发起 UNKNOWN |
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

语义 revision 只在本通知含义改变时更新：原 task run/review revision、原 store service/connection fact revision、订单/Withdrawal version、知识 processing revision、邀请 delivery/state revision。没有可靠 revision 或稳定变化 identity 的源不能用 `now()`、每次读取的 observedAt 或推测的 digest代替；补齐 owner 的只读窄合同后才接线。

排序用 source 的真实 `occurredAt`，同时间以稳定 source ref 排序。未知发生时间明确 unknown，不能拿任务创建时间冒充完成时间。source cursor 必须绑定当前 principal/企业/filters，按稳定原 source key 分页，不能把 count 当分页终止条件。

## 4. 持久化、已读与并发

拟使用独立 PostgreSQL schema `notification_center` 和最小权限 runtime role；独立连接池最多 4，默认复用当前应用指定的 PostgreSQL 实例而不是复制 business databases。初始化用既有 init/migration 流程；serving role 不建表、不访问其他 source schema、不继承广泛 owner role。新 schema 只保存本系统新通知事实和阅读回执，禁止 legacy migration/backfill。

最小数据：

- 公告：不可变 ID + revision、类别、plain-text/结构化段落、真实 publisher、audience、published/withdrawn 时间、typed internal target。
- 阅读回执：`(issuer, subject, scope_kind, organization_id, source, entity_id, notification_type, semantic_revision)` 唯一；首次已读时间。个人/官方 scope 不伪造 Organization，企业 scope 必须有组织 ID。
- 公告命令/全部已读命令：scope + operation + idempotency key + payload fingerprint + committed receipt；同 key 不同载荷返回 conflict。

`unread -> read` 为通知 owner 唯一阅读转换；同 revision 重放不产生新效果，新语义 revision 没有旧回执即为 unread。`attention` 是重新读取 source facts 的投影，不能由 read receipt 转换为 completed。撤回公告不出现在新列表/计数，读回旧 ID 返回 withdrawn/not-found，不删除命令证据。

详情 GET 不写阅读事实；浏览器在成功载入 detail 后发显式已读命令。列表预取和浏览器 crawler 不能把消息标为已读。已读成功必须晚于 durable commit；响应丢失后重放同 key/相同 exact ref，或读回确认，不能仅靠本地乐观状态报保存成功。

「全部已读」针对当前分类、当前 principal 与当前企业的一次服务器授权快照，包括全部分页，不只第一页/最近 20 条。用户请求绑定服务器的快照身份与 fingerprint；服务器重新验证范围和所有 ref 的原可见性后，在通知库一个事务写所有已读回执和命令回执。并发新 revision/新消息不在原快照内，仍未读。source 读取不完整、撤权、快照过期或事务失败时，不返回全部已读成功。快照过期返回明确 stale，需要用户刷新后再次操作，不扩大原快照。

实现须先细化 source 枚举、快照承载及限制（§9 的准入缺口），不能以客户端自报 ID 集合或 source timestamp 水位实现全部已读。时间水位会错误消灭迟到的旧时间消息，也不能代替 exact refs。

## 5. 身份、企业与业务操作

- 官方消息和个人经营消息按既有 authenticated identity 的 issuer + canonical subject；无企业也可访问本人及官方通知。
- 企业经营消息按当前 effective Organization 的 live authorization，再按源业务权限/本人/StoreAccess 过滤；与任意 home Organization 不混用。企业代管不扩大本人 Task 或个人收益的可见性。
- 不能用一个新 notification.read 权限自动授予所有业务源。计数、摘要、详情、全部已读和跳转都遵循同一原权限；无权限来源不泄露事项存在性。
- 官方发布/撤回使用既有 `AuthPolicyCurrentIdentityWithVerifiedRoles` + `PermissionListingKitPlatformAdm` + `OrganizationAccessPolicyNone`，不新增管理员体系。官方消息可见不等于有发布权限；创建者团队标签不是 browser 可伪造的身份声明。
- 当前企业 source 请求带 `X-Expected-Organization-ID`；个人写入使用当前主体校验。BFF 使用现有身份与 same-origin CSRF 检查，不把 token 返回浏览器。
- 用户/企业切换、退出、撤权时，清空 query/detail/popover/count 和迟到请求结果；隔离 query keys。不能把上一企业的 pending 或本人提现摘要保留到新的 scope。
- typed target 只生成当前正常站内 GET 入口。通知点击不付款、不接受邀请、不 Apply、不重启 Agent、不重新调用 provider；目标页重复执行自己的权限、version、幂等和 Consent 检查。
- UNKNOWN、unknown_reserved、PAID_PENDING_CREDIT 等显示核实结果或等待原 owner 处理，禁止通知层提供「重试执行/再次扣款」捷径。

个人源与企业源使用独立描述符/读取通道，企业 gate 失败不吞掉官方及本人合法通知，也不能继续显示旧企业缓存。

## 6. HTTP / UI 与资源边界

拟新增固定 feature routes，保持个人/官方与企业 source 分离；具体 descriptor 必须在准入前定稿：

```text
GET  /api/v1/notifications/official
GET  /api/v1/notifications/personal
GET  /api/v1/workbench/notifications
GET  上述 scope 的 /:notification_ref
POST 上述 scope 的 /read
POST 上述 scope 的 /read-all
POST /api/v1/platform/notifications/official
POST /api/v1/platform/notifications/official/:id/withdraw
```

普通列表默认 20、最多 100；顶部最多 4。过滤值固定为官方 unread、经营 pending；浏览器只接受 allowlisted source/type，并验证单值 query、ID、cursor、schema、数字和时间。

默认完整 request deadline 10 秒；单 source 2 秒、最多 4 个 source 同时读取；每页 response 上限 128 KiB，普通 read command 上限 16 KiB，公告正文上限 32 KiB。GET 配置 RejectUnreadRequestBody，拒绝未读 body 后再调用 owner。全部已读快照上限/传输方式须按 §9 定稿；超过容量明确失败，不静默截断。

输出包含每个来源的 `AVAILABLE / EMPTY / DENIED / UNAVAILABLE / DEPENDENCY_MISSING` 与 completeness。Denied 不返回 count/ref。来源 unavailable 不报 0；全局总数只有全部当前可见来源有完整计数时才标 exact，partial 明确显示「部分消息暂无法加载」。不把基础设施健康、未开放 source 或没有配置的 provider 生成成每个用户的虚假业务故障消息。

正文用纯文本或有界结构化段落；React 默认转义，不用 raw HTML。URL 由 typed target 映射，拒绝外链、javascript、data、协议相对地址和任意 redirect。附件/支付码/凭据/provider 原始结果不在通知正文或普通日志中。

UI 复用 `WorkspaceAppShell`、当前 Console Page/Panel/Button/tokens；通知是顶部全局入口，不新增一级菜单。复用既有 design 静态资产的精确匹配，新增资产仅使用 Figma 导出文件；截图不作实现资产。未读与待处理分开；图标不是状态的唯一提示；键盘打开/关闭弹层、Escape、返回焦点、loading/error/empty、390px 无溢出和深浅色继续当前实践。

## 7. 开源复用决定

已核对 [Novu 官方自托管与 Inbox 文档](https://docs.novu.co/community/self-hosting-novu/deploy-with-docker)：提供独立 API/worker/dashboard、in-app Inbox、服务端 trigger SDK，Go/React 均有接入路径。它适合站内与多渠道 workflow，不能替代本系统原 owner 的授权、UNKNOWN、业务去重和事实适配。当前没有多渠道需求；引入其完整服务与 subscriber/workflow 配置增加独立运行与身份同步成本。

候选方案因此优先复用现有 PostgreSQL 事务/唯一约束、Casbin/当前身份、feature HTTP、Zod/BFF、Console UI。仅实现不可替代的系统业务投影与阅读回执，不另造 queue、template engine、IAM、transport 或重试平台。正式选型须随架构评审确认；未安装 Novu，也未调用任何外部通知 provider。

PostgreSQL 阅读去重以[官方 INSERT/ON CONFLICT 合同](https://www.postgresql.org/docs/current/sql-insert.html)和仓库既有事务模式为依据，不用先查后插的无锁内存去重。公告修改/撤回与命令回执共事务；通知事务绝不修改业务库。

## 8. Legacy decision

Legacy decision: N/A（仅复用当前合格 Console 和 source owner；不是旧内部 Task Dashboard）。

如正式实现发现旧 Task/ListingKit/tenantbridge 参与一个来源，只能 EXTRACT 当前有效行为到当前 owner 或 RETIRE 该通知来源；不能建立通知 adapter 包装旧 Service。旧日志、历史 task/queue、旧平台 Workbench 状态不作为当前通知源。无旧数据导入、旧 ID 映射、双写、fallback 或全环境清理。

## 9. 当前准入缺口与开工边界

本设计覆盖完整用户通知中心及当前业务类型，仍为 NOT_READY，以下工作在生产 Writer 开工前完成：

1. 为 §2 的 CURRENT_FACT source 给出精确授权 reader、合法语义 revision/时间与分页映射，确认缺少 reader 属于本批最小局部补齐，不能从原始 repository/日志绕过授权。确认按语义版本去重与 BusinessTask/Review binding 的唯一展示规则。
2. 定稿全部已读的服务器快照存储/身份、完整枚举边界、事务协议、TTL 和上限，及 counts/completeness 合同；不能以源 owner 的局部最近页充当完整范围。
3. 定稿官方发布 audience、正文版本/withdraw 合同及每个 scope 的 descriptor；记录当前 route/module injection 与初始化连接配置。不得创建新 identity/authz owner。
4. 完成独立 Architecture Review。按 AGENTS 最多两轮；findings 先分类、指出受影响 Must 和阻塞层级。仅满足后记录 IMPLEMENTATION_READY，冻结设计再开始正式业务实现。

DEPENDENCY_MISSING sources 保留各自阻塞与 owner；不把库存/广告等业务引擎作为站内通知基础部分开工前置，但完整类型落地仍不能声称完成。执行 Issue 必须公开这些缺口，用户要求的完整通知范围不因设计表格存在而完成。

## 10. 实现与验证、交接

一个 Writer、一个 `codex/notification-center` 分支与一个主要 PR。本文件初稿阶段只交设计/事实映射；准入后的实现、必要测试、findings、CI 修复留同一主要 PR。

业务行为使用 TDD：先覆盖权限/同企业私有任务/个人跨企业、状态映射及同事项去重、read != handled、新 revision 未读、全部已读与并发新消息、unknown commit 重放，再实现最小代码。真实 PostgreSQL 检查 scoped unique/事务/并发回执和 serving privileges，HTTP/BFF 检查 source 授权、期望企业、CSRF、严格/有界 body、deadline 与无状态变更 GET。

不新增 runner、故障注入平台或验收 session 框架。浏览器使用已有隔离测试能力，验证顶部 -> 两类列表 -> 筛选 -> 详情 -> 持久化已读 -> 正常业务页面，并核对深浅色、移动布局和截图。fixtures 明确合成；没有真实业务 source、外部 provider 或用户操作就保留 NOT_RUN。

交接包含真实启动命令/入口、登录步骤、公告发布方法、哪些 source 已接入/被阻塞、数据保存与 stop/restart 方法。实现者只报告开发自检和 CI；独立最终检查覆盖真实 diff 与业务路径，产品验收由用户或指定独立验证者确认。无合并、部署、关闭 Issue、收费调用、真实数据读写或容器数据删除授权。
