# 统一基础方案与预付资源 V1

Issue: #478 · Delivery Batch: 账户中心与套餐权益统一模型

Design Basis: **Independent Architecture**

Admission Status: **IMPLEMENTATION_READY**

模型点数扩展（第 9 节）Admission Status: **IMPLEMENTATION_READY**。2026-09-29 用户明确将
Product Agent 标题评审的模型点数计费合同与接线纳入本批；第 1–8 节冻结准入继续有效，
独立 reviewer 于同日核对增量 blob `90a25b05419802ef71edb05bb3a371f7641129e5`，无设计级 BLOCKER。冻结价格/native terminal CAS、严格未 dispatch 释放、实际费用与余数原子守恒、model-only recovery 装配作为 IMPLEMENTATION_TEST，在本批收敛。

2026-09-29 独立 reviewer `payment_architecture_review` 明确准入，无设计级 BLOCKER。
所读设计 blob `3436be93d29ca3467da67047631b7dd63af000a1`，design-only commit
`a9b59071682371a00fd3f683c17ea972535f9d26`。本状态更新不改变已审核合同。
Native Store 单库审计/授权/业务指纹及 PG 冲突收敛、resource reserve decision/scanner 恢复、
购入事件 allocated-after 与真实 bucket 一致、current config/audit/UI/API hard-cut 为 IMPLEMENTATION_TEST，
在本批 TDD 与最终交付检查收敛；此准入不签发产品验收或合并批准。

## 1. 产品决定与交付结果

2026-09-29 用户明确决定：**统一基础方案 + 店铺按期收费 + AI／数据预付资源**。
这替代 #478 / #479 / #480 原来的“选择套餐、购买并激活订阅”作为当前新系统的产品路径。
旧实现、评审和验收记录保留为历史证据；被替代的订阅产品要求不写成 PASS，也不继续作为新模型的开工或使用前置。

使用者是已登录并有当前企业授权的管理员与成员。管理员进入“套餐与权益”，查看统一收费规则，
从企业钱包购买店铺服务期、AI 点数、数据条数，确认真实报价，查看原订单及入账结果；
在账户中心分配或回收资源、设置成员 AI 月度消费上限。成员仅操作获授权的具体店铺并消费自己的分配或月限。

Must:

- 一个基础方案，不再提供体验版／专业版／企业版的订阅购买选择；基础方案不另建订阅有效期或免费额度事实。
- 店铺记录创建、编辑、成员授权不需要购买订阅或占用订阅的店铺数量额度。
  记录存在不代表付费服务、真实平台连接或发布能力已可用。
- 店铺服务必须先有真实官方连接，用户显式开通后计时，1 期 = 30 天；续费沿已批准服务合同。
- AI 点数、数据条数、未使用店铺期数是企业预付资源，余额不按月清零。
  模型 Token 是调用审计量，不是 AI 点数，也不再提供旧订阅 AI Token 分配。
- 购买使用现有 server-owned offer → immutable quote → RESOURCE_PURCHASE order → money reserve → Resource grant → money commit。
  资源入账或资金预留结果未知时保留原身份，不能换新订单再扣款。
- 企业余额、成员分配和实际消费分别展示；我的权益展示真实服务期和余额，用量明细展示真实资源账本。
- 权限、租户隔离、撤权复核、账务幂等和未知结果恢复不能因为取消订阅而绕过。

Out of Scope: 新订阅等级、升级／降级／按比例计费、自动续费、赠送或折扣规则、新支付渠道、
真实数据迁移／清理、SHEIN 应用申请、新的通用恢复或验收平台、未开放的 AI Chat 或发布功能。
实际 SHEIN 连接仍缺用户已说明尚未申请的官方应用；正式支付仍须既有渠道配置及适用验收。

## 2. Figma 与现有权威

当前 Figma: [套餐方案 431:3455](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=431-3455)、
[我的权益 431:3959](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=431-3959)、
[充值中心 431:5166](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=431-5166)、
[账户资源 1627:359](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=1627-359)。
本次已读取前两个当前 frame 的截图和设计上下文；修改其余页面前读取相应当前上下文。

- [Final UI / IA Authority](../product/final-ui-ia-authority.md)：沿现有 Console Shell、页面入口和组件。
- [Commercial Wallet/Billing Contract](commercial-wallet-billing-contract.md)：复用资源购买与资金/资源 owner；旧 subscription 分支被本决定替代。
- [Member Resource Allocation V1](member-resource-allocation-v1.md)：具体店铺授权、期数与条数分配、native proof 与 Resource 跨数据库结算保留。
- [Greenfield 产品基线](../product/greenfield-no-legacy-migration.md)：新安装、空业务数据，EXTRACT / RETIRE。

沿已经明确的语义差异：Figma “绑定后计时”采用**真实连接后显式开通**；
“token积分”显示为 **AI 点数**；Figma 样例价格、余额、赠送和能力标签不能成为业务事实。
正式价格来自可售 offer。DATA_ROW 单位价格已由 2026-10-03 用户决定为人民币 5 分，仅服务端成功发布的 1688 采集消耗 1 条；插件及复用插件的本地采集免费，见 [采集渠道计价设计](2026-10-03-acquisition-channel-pricing.md)。AI 点数与店铺期数正式价格仍待分别决定；无可售配置显示“暂不可购买”。
当前资源 offer 使用整数分/资源单位；如后续批准价格无法由此表达，必须先明确最小报价合同变化，不能静默四舍五入或另造价格源。

## 3. Owner 与调用链

| 事实 | 当前唯一 owner | 当前消费路径 |
| --- | --- | --- |
| 身份、企业 membership/role | ZITADEL + 已批准 membership receipts | Workbench context → live identity/permission middleware |
| 钱包、资金预留、真实支付 | money | billing → money port，渠道回调/核对沿 #481 |
| offer、quote、resource order | commercial/billing | Console → 同源 BFF → billing handler → current assembly |
| 三类资源余额、预留、消费、成员分配、AI 月限 | ledger/orgresource | billing grant、消费 owner port、账户成员资源、只读权益/明细 |
| 店铺记录、具体成员授权、连接、服务期、服务操作 proof | storecenter native DB | current Store module → member-scoped native repository |
| 商品成功入库 proof | Product native DB | Acquisition native operation → Resource ConsumerCharge |
| AI 调用量与价格结算证据 | 当前 AI invocation / Resource point settlement | 原调用 recorder 与点数结算路径；不另记订阅 Token 额度 |
| 基础方案说明 | 当前应用的固定产品合同 | commercial read projection；没有第二套持久化套餐事实 |

现有商业只读投影不能继续依赖 subscription/entitlement 表解释新基础方案。
新的 `/api/v1/workbench/commercial/overview` 返回带版本的当前模型说明、按组件声明的 availability、真实资源余额和店铺服务汇总。
汇总按当前企业输出非识别性 aggregate（记录数、有效服务数、已过期数、即将到期数）；
具体店铺 ID、名称、服务详情仍必须经过原具体成员授权。即将到期定义为 `now < expiresAt <= now + 7 days`，
日期判断用服务 owner 的当前 UTC 时钟；disabled/suspended/deleting/deleted 不能计为有效服务。
依赖未配置或读失败明确 unavailable；有合法 schema 但没有资源 bucket 使用既有 `not_initialized`，不伪造 0 或成功。

当前配置/装配删除旧订阅商业 read pool 和 Store quota pool 的必需依赖，以及旧 Token allocation 模块。
保留原各业务 owner 的独立 runtime role、连接池/权限校验，不能把 money/Resource/Store/Product 合并为一个全权数据库。
Account audit 提取仍有效的 profile/membership/Resource/AI 审计读，停止查询旧 Token allocation 事件。
现有消费者的新资源明细从 Resource events 读取；不通过读取动作结算、修正或 seed 数据。

当前成员资源审计读映射（复用上述 owner 与 Account Audit AdditionalHistory，不改变写入合同）：

- 只读 Resource 已有 `succeeded` operation 及其原 audit snapshot。期数/数据分配和回收数量取同企业、同操作的原资源事件，成员和版本取原不可变 receipt；AI 月限取原 MemberLimitSnapshot，不用当前限额覆盖历史。
- 投影 `account_member_resource.changed` 与 `account_member_ai_point_limit.changed`；`resource.type/quantity` 分别为原转移量或原配置月限（整数十进制字符串，月限允许 0）。relation 绑定原 Resource operation ID 与原成员版本。actor 保留提交审计 actor。
- 沿现有 Account Audit GET、实时企业 read 准入、同源 BFF 和表格；原 actor 筛选及三项当前操作筛选、按审计时间/ID 的 Resource 流位置参与既有多源分页，cursor 绑定企业及筛选。原提交事实不完整时明确 unavailable，不伪造空历史或当前余额。
- 不新增 schema、审计写入、状态机、IAM、恢复/计费协议或统计聚合；旧 Token audit DTO、读取与筛选退役，无兼容回退。

## 4. 店铺记录 Hard-Cut 与单库事务

旧订阅 store_count reserve/commit/deallocate、租约和补偿被 **RETIRE**。
不提供无限额度适配器、假 limit、旧 quota fallback 或新 quota 表。
店铺 create idempotency、唯一身份、原创建者、成员权限、乐观版本和审计是 **EXTRACT** 到 native Store owner。

创建使用 native 数据库单事务：

1. 在 LiveWrite 和 StoreCreate 权限后，member-scoped repository 重新验证当前用户/成员及创建权限。
2. normalized create payload 的指纹只含当前业务输入（platform/name/region/externalStoreID），
   不含随机生成 store ID、actor、时间或已退役 quotaAllocationID。
3. `(organizationID, createKey)` 唯一；同键同输入返回原 Store 和原 CreatedBy，不能补授已撤销成员权限；同键异输入返回 conflict。
4. 新键生成原 Store ID；原子写 `active + pending_activation` Store、普通创建者的 member grant/receipt、StoreCreated 审计。
   管理员不隐式获得成员资源分配。audit/member grant 任一步失败，Store 写也回滚。
5. 唯一平台/地区/externalStoreID 的冲突返回 existing/conflict；未提供 externalStoreID 时以原 Store ID 隔离身份。
   soft-deleted 身份和 create key 保留，不复活原记录或同键重建。

并发同键通过数据库唯一约束与事务冲突后只读原记录收敛，不靠进程内锁证明正确性。
事务提交响应丢失只允许用原 create key 和原指纹重放；读取原 Store 必须重新经过当前成员授权。
删除继续绑定 `(org, store, operationKey, expectedVersion)`，在 native 单事务写审计、撤销 member grants、soft delete；
不再释放不存在的订阅额度，不退已消费服务期，不删除 Resource 操作或证据。
并发服务写、连接写、授权撤销与删除沿现有 native Store row lock/version 边界。
删除后 Resource recovery 仍可读取原 native 服务 proof，不得把不见记录解释成失败并释放未知消费。

新安装 schema 去掉 `quota_allocation_id` 和 audit allocation 字段，移除无意义 provisioning/quota resume 状态和路由。
现有真实数据库不 ALTER/backfill/drop；不实现旧 schema 兼容启动。
schema verifier、installer、runtime permissions 和前端 DTO 同时切换，旧 schema 启动明确失败。

## 5. 资源报价与购买

在现有 billing 添加 bounded `ResourceOfferCatalog`（三个资源种类、当前 active/version/window、最多 100 项）。
GET resource offers 沿 CommercialRead；POST quotes/orders 沿 CommercialPurchase（当前企业管理员）；
禁止 caller-supplied enterprise、价格、币种、resource type、actor、赠送额。
无 catalog 或配置时明确不可购买，不恢复旧 subscription offer。

店铺期数输入正整数数量。AI/数据允许输入钱包金额：服务端按当前 offer 整数单价向下兑换数量，
验证 Min/MaxQuantity，再复用现有 CreateQuote 冻结真实数量/金额/版本/有效期；
显示实际扣款与未使用余额（输入预算 - 实际扣款），不将余数扣成钱或伪造赠送。
前端 BigInt decimal string，禁止浮点 authoritative conversion。
报价创建前后价格版本不一致返回 quote-expired，需要重新确认；下单以已签存的 quote 合同为准。

状态、事务与恢复**不新增**：复用 #457 resource purchase。
`PENDING → FUNDS_RESERVED → FULFILLING → FULFILLED`；确定未授予的失败才释放；
授予/扣款未知保持 reconciliation required，原 resource order/source grant 身份和钱包 reservation 不变。
资源入账证据与钱包成功扣款均存在才展示已完成。购买成功也重新读取当前企业 wallet/resources。
当前存在单订单 ReconcileResourceOrder，但未装配资源订单扫描；最小补齐同一 billing owner 的 bounded 原订单读取/续行，
复用当前应用既有 30 秒恢复 loop，每批最多 50；不能增加第二 scheduler 或接受 caller proof。
原下单 actor 从当前 trusted identity 写入 immutable resource order（复用 canonical Order.ActorID 字段），不从 JSON 读取；
本次只改变 resource order 当前合同，旧订阅状态机不复制到资源购买。
原调用资金 reserve 后响应丢失或 order 更新丢失时，必须使用现有 money
`ReadCommercialPurchaseReserveDecision(org, operationID=orderID, commercialOrderID=orderID)` 读取原 durable decision。
核对金额/币种/org/order/operation，再读其 reservation；找到 RESERVED 才续行 source-bound grant/commit，
找到明确拒绝才取消；读失败或决议尚未出现保持原 UNKNOWN，不因余额后来改变重新判定原失败。
扫描可包括 PENDING（首次 reserve 响应后 billing 尚未写状态的崩溃窗口），但扫描**永不创建资金预留**。
没有 durable decision 的订单只保持原状态；只有原用户命令经当前 LiveWrite/CommercialPurchase 重新准入才可执行原 reserve。
已 reserve 的原订单不能因用户退出或撤权而忽略未知财务结果；恢复只完成已经批准并预留的原交易，
不创建新订单/新预留，不扩展消费授权。单订单/并发 scanner 使用现有 version CAS 和 money/grant source 幂等收敛。

前端确认页展示当前企业、资源/数量、价格、币种、有效期、真实扣款、剩余预算。
原操作在受限 session storage 按 user/org/member/key 保存 quote/order/idempotency；保存失败不 dispatch。
未知结果、刷新和切换企业只能查询或重放原命令；报价过期而原订单未知时不得自动创建新报价/订单。
迟到 A 企业响应不能覆盖 B 企业 UI。现有钱包、账单及 top-up/refund 不新发明协议。

## 6. 权益、用量与账户资源

方案页按照 Figma 展示一个基础方案 + 三个计费组件 + 真实可购买 offers/不可用状态。
“我的权益”展示真实资源池和 Store 服务汇总；成员额度入口继续在账户中心。
只显示当前已装配并有权限的能力，不把基础方案说明作为工具 dispatch 的鉴权 proof。
移除当前前端/服务端的旧 subscription purchase、旧 entitlement 数值/expiry、旧 AI Token allocation 消费路径。
保留所有有效 AI 点数 UTC 月限与 Store/data 成员分配合同。

用量明细以 Resource event 唯一事实源，按当前 enterprise、resource type、时间段有界读取，
每页最大 50，稳定 `created_at/event_id` cursor；资源/时间/cursor 非法拒绝，不能查询另一企业。
返回 original event/op/source identities、reason、quantity、可用/已分配/预留/已消费 delta 与余额后值。
UI 区分购买入账、分配/回收、预留/释放、已消费；未知预留不显示消费成功。
AI audit 保留模型 Token 与真实点数的区别；本批不重新定模型价格或伪造 Token。
读链路不增加持久化 usage projections 或第二消费 owner。

## 7. 不变量与有限验证

Threat model 沿当前已批准 Workbench tenant/live permissions、money/resource contracts；
不追加理论 side channel、全局 IAM、无限故障矩阵或新验收工具。

| 当前不变量 | 必要开发证据 |
| --- | --- |
| 无订阅也能建 Store；不获得付费服务/连接 | native create + pending activation + existing lifecycle rejection |
| 相同 key 同 payload 一个 Store/原创建者；异 payload conflict | parallel/replay/restart native repository test |
| grant/audit 失败 Store 不半创建；撤权不被重放补授 | native transaction rollback + member access tests |
| 删除与服务操作竞争无错误 proof/重复期数 | affected native lifecycle + original Resource recovery tests |
| 拒绝跨企业/成员越权 | existing current-route auth + read/quote/order tests |
| 价格/数量只由 server 冻结，余数不扣 | amount quote bounds/version/overflow + catalog tests |
| 响应丢失/重试不重复扣款入账 | existing resource order replay/UNKNOWN tests + current recovery wiring |
| 权益/用量与账本一致 | real bucket/event read + stable pagination/time filter tests |
| 新 current serving 不再靠 subscription quota/token | assembly/schema/config tests and route deny/absence checks |
| 前端恢复有 scope/持久身份，金额无浮点失真 | existing typed client/BFF and focused UI recovery tests |

业务行为按 AGENTS TDD，先确认目标测试 RED 再最小 GREEN。
高风险准入独立 review 本设计；完整用户路径后一次最终交付检查，finding 只复核实际增量。
实现完成为 In Review，交付正常启动入口与操作步骤；CI/作者自检不签发产品验收。
真实支付、官方 SHEIN 连接若缺配置保持 BLOCKED/NOT_RUN；无正式价格影响正式可购买验收，不阻止无价格路径的代码交付。

## 8. Legacy 决策与切换

Legacy decision: **EXTRACT | RETIRE**

Reusable behavior: money/resource/quote/order safety、真实 Store 和 member grant、AI 调用审计、live auth、Console/BFF 与原命令恢复。

Current owner: 上述第 3 节；不包装 listingsubscription/accountallocation 或无限 quota ledger。

Cutover/deletion condition: 当前新安装入口只装配本模型，旧订阅 UI/API/准入及 quota pool 停止使用，
触及的纯旧测试删除/改写为当前不变量；未装配的 legacy 代码按既有 RETIRE 登记停止使用，不为批量清理扩大本交付。
没有历史业务数据转换、双读、双写、fallback 或真实环境 destructive action。

## 9. Product Agent 模型点数计费扩展

用户结果：已配置模型调用点数价格且企业/成员余额足够时，可以继续当前显式 opt-in 的标题评审试用；
每次真实模型调用占用并最终扣减同一个企业 AI 点数池和成员 UTC 月度消费上限。
本节替代当前 `cmd/current-application` 的旧订阅 `AIInvocationUsageAdapter`；
不改变 Agent/Eino 预算、工具调用准入、provider、prompt、标题提案/保存或发布权限。
没有配置调用价格时返回不可用，不能用预付资源兑换价格推算模型费率或免费调用。
AI 资源购买价格、模型 provider 成本预算与模型调用点数费率是三个不同合同。

点数规则由可信部署配置指定 `pointPricing.priceVersion`、正整数 `inputPointsPerMillionTokens`、
`outputPointsPerMillionTokens`。调用扣减为
`ceil((promptTokens * inputRate + completionTokens * outputRate) / 1_000_000)`；
调用前按当前治理模型的完整输入/输出 Token 上界用同一公式预留，最低消费为 1 点。
使用整数/任意精度计算并拒绝 int64 溢出；不自行填写正式价格。
本节仅定义 AI 模型调用点数费率的配置与计算，不引入默认模型费率。DATA_ROW 资源购买单价的 2026-10-03 决定不等于模型调用费率。

事实 owner：仍是 native `ai_invocations`（调用身份、输入 hash、原 org/user/member、路由、
已观测 Token、结果）与 Resource（点数预留/消费及月计数）。不新增调用表或第二用量账本。
在当前新安装 invocation schema 增加冻结点数价格版本、输入/输出费率、Token 上界字段；
这些字段在 ClaimInvocation 前从治理配置冻结，参与 input hash，调用终态更新和 operator resolution
必须保留并核对原字段。旧 schema 明确拒绝，不迁移真实数据。

路径：grsaitext trusted policy → native ClaimInvocation → 固定 model-point adapter → Resource reserve
→ 原单次 provider dispatch → native terminal observation → Resource finalize。
保留 recorder 的既有 reservation/settlement seam，固定 adapter 始终重新读取原 native fact，
接口上的 caller token 总数不能成为扣费 proof。adapter 仅接受 `product_agent_decision` 的明确当前 owner。
当前 ImageAgent 图片按张计价保持原合同；不把这个模型接口推广到其他 legacy AI worker。

Resource 使用原 operations/reservations/events/audit 与 member month 表：
owner_type=`model_invocation_v1`，owner_attempt_id=原 invocation ID，原 org + invocation 唯一。
原 reserve operation 绑定原调用指纹及冻结价格；原 receipt 保存价格、原 member、
UTC 月开始、limit version、预留数量。reserve 在一个 Resource 事务中锁原 operation、
member limit/month 与企业 AI bucket，核对当前月/限额并扣可用、增预留，写 event/audit。
缺余额/缺月限/撤权不 dispatch；同键异载荷、不同 org/member/route/input/price 冲突。
LiveWrite/provider fresh check 沿现有治理模型；初次 reserve 必须匹配 trusted current identity，
恢复只完成原已准入持有，不创造新调用、资源或授权。

finalize 重新读取原 native immutable terminal proof，在单个 Resource 事务锁原 reserve/finalize
operation、原 reservation、原 month counter 与企业 bucket：

- `succeeded` 或白名单 `usage_observed_failed` 且完整可信 Token：按原冻结费率消费，释放上界余数；
  不因结构化输出无效免除已发生模型费用。实际 Token 必须非负、sum 一致、在原治理上界内。
- `failed` 且 native owner 已明确 provider 未 dispatch：释放原预留；
  若 reserve 尚未出现，关闭原 reserve 身份，使迟到 reserve 不能再创建持有。
- dispatched、usage 未知、读取失败、缺 native fact 或终态证据不完整：保留预留/UNKNOWN，永不重发 provider。

原调用可见 terminal 与点数结算不是跨库原子事实；RecordInvocation 先持久化原 native terminal，再幂等 finalize。
原 adapter 重试只读相同终态并结算。若 native terminal 保存成功但 finalize 失败/响应丢失，
Resource 原 reservation 的既有 NextCheckAt 供当前应用同一 30 秒 loop 扫描（最多 50），
只读原 native proof 续行，不重新 reserve/调用；unknown 设置下次时间，使有界扫批不饿死后面的终态。
跨月完成使用预留时原 UTC 月，不重新扣新月。月限降低只影响新 reserve，不阻断已发生费用的 finalize。
最终原 reservation/operation receipt/event source identity 证明恰好一次；native invocation 删除/凭据变更
不能推断未调用并释放，保留原未知事实。当前不开放删除调用历史。

必要增量验证：冻结费率/整数边界/不同 token 价、余额/月限/撤权拒绝无 provider effect、
真实独立调用/Resource DB 上 reserve/terminal/finalize 响应丢失与重启幂等、
输出失败计费、明确未 dispatch 释放与迟到 reserve fence、未知保持、跨月归属、
bounded recovery 进展、原 Claim/dispatch 不重复、旧 Token owner 当前装配缺席。
实现者只做这些开发检查；真实 GRSAI 调用/产品验收按原获准范围另行交接，不在本节自动触发付费调用。
