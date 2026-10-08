# 生态服务 V1：第三方入驻、服务交付与在线收款结算

状态：**IMPLEMENTATION_READY**。本文件是 [Issue #603](https://github.com/qq550723504/task-processor/issues/603) 的冻结设计基线；独立高风险 Architecture Review 已通过，允许唯一 Writer 按本范围实施和必要自检，不授权真实渠道操作、合并或部署。
设计日期：2026-10-08。只读代码基线：`main @ 01cdce76040f56fc4eb7cdef301f5a4850893190`。设计负责人：会话 `01a1199e-0e7e-7441-9173-6365078417a0`。

## 1. 已确认产品决定与待决项

用户在本会话明确确认：按当前 Figma 生态服务设计交付；允许第三方服务商入驻，由第三方处理需求与交付；平台在线收款，再向第三方结算；**平台抽成 10%，支付手续费由平台承担，第三方服务不产生个人推广佣金；客户验收后，通过支付渠道分账**。同日用户批准下述取消/退款规则，并回复“按这两项推进”：首版采用微信平台收付通，一次付款、验收后分账；超过渠道冻结期限的服务暂不开放在线下单，分阶段付款留到后续。
费率必须在成交报价/订单中封存，后续费率变更不能改写既有交易。本文件技术合同经独立评审冻结；技术选型不冒充用户产品决定。

| 项目 | 当前状态 | 对本批次的影响 |
| --- | --- | --- |
| 第三方入驻与第三方履约 | 已确认 | 不由平台或 Agent 自动代办业务 |
| 在线付款与第三方结算 | 已确认 | 必须覆盖资金归属、退款及结算结果 |
| 平台抽成 | 已确认 10%，费率为 1000 basis points | 服务成交价、计费基数及不可变分配快照需要绑定原订单 |
| 结算触发及方式 | 已确认：客户验收后，通过支付渠道分账 | 付款/服务商自报交付不能代替客户验收；不改为平台人工转账 |
| 支付手续费承担方 | 已确认：平台承担 | 不从服务商的 90% 份额扣除；平台佣金收入和渠道费用分别记录 |
| 第三方服务是否产生个人推广佣金 | 已确认：不产生 | 以明确 SERVICE_PURCHASE / NON_COMMISSIONABLE 合同排除全部推广消费者 |
| 取消、拒收、退款及已结算退款 | 已确认，见下表 | 不得自动推定不可退款、平台垫资或服务商欠款处理 |
| 支付/结算渠道与首版付款范围 | 已确认：微信平台收付通，一次付款 | 不开放分阶段付款，超出渠道冻结期限的服务不能在线下单 |
| 商户产品资格与渠道实际配置 | 尚未核实 | 凭据、开通和实际期限属于真实联调/开放条件，不阻止已准入的本地实现 |

已确认的分配例子：服务成交金额为 ¥1,000、没有退款调整时，平台佣金 ¥100、服务商份额 ¥900；渠道支付手续费由平台另行承担，服务商份额不因此减少。平台净收益为佣金减实际渠道费用。退款、取整和渠道分账完成仍须按冻结合同与核实事实处理，不用示例宣称已经到账。

### 已确认的首版退款规则

2026-10-08 用户回复“采用上述规则”，批准以下处理方式；具体并发、金额取整与外部操作协议仍须独立架构准入：

| 场景 | 已批准处理 | 金额与结算约束 |
| --- | --- | --- |
| 服务开始前取消 | 原路全额退款 | 开始与取消互斥；退款处理中不允许开始服务或触发分账 |
| 服务开始后取消或拒收 | 客户与服务商协商具体退款金额，由平台审核后执行 | 拒收本身不等于退款成功；审核绑定订单、交付及协商版本；争议期间不派发新分账 |
| 已完成分账后退款 | 先核实并完成原交易所需的渠道分账回退，再原路退款 | 渠道回退、原收款商户退款余额与客户退款分别核实，不以一个成功代替全部成功 |
| 部分或全额退款的分配 | 平台佣金随退款金额同比例退回，手续费仍由平台承担 | 累计分配基于累计有效成交净额计算；全额退款后平台佣金及服务商销售份额均为零；每次变化新增调整 receipt |

这些退款规则不授权平台垫资、服务商透支或强制扣款。余额不足或回退结果未知时保留原操作的处理中/待补足资金状态与原因，不宣称客户已退款。是否提供平台垫付是另一个产品决定，不由实现者推定。渠道对手续费是否退回以其核实账单为准，不将实际未退手续费转嫁给服务商。

## 2. 用户结果、范围与产品依据

一个 Delivery Batch 对应一条路径：服务商提交机构资料 → 平台审核资质与协议 → 服务商发布服务 → 客户选服务、提交需求并在线付款 → 第三方交付 → 客户验收 → 平台触发第三方结算。一个主要实现 PR；不按表、接口或内部模块机械拆 PR。

[最终 UI / IA Authority](../product/final-ui-ia-authority.md) 指向 Figma `tg48P46SSXl6TBy9lZwg63` / page `31:463`。2026-10-08 本会话只读实读并查看三个页面截图：

| 页面 | 当前深/浅色节点 | 实读功能 |
| --- | --- | --- |
| 服务市场 | [431:323](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=431-323) / 431:525 | 公司注册、商标注册、店铺开通、店铺代运营；企业/店铺分类；查看详情、提交需求 |
| 我的服务 | [431:833](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=431-833) / 431:1035 | 状态数量、名称/编号搜索、类型/提交时间筛选、服务进度详情 |
| 申请加入 | [431:1343](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=431-1343) / 431:1545 | 机构资料、服务领域/覆盖区域、资质/案例审核、合作协议、服务发布 |

模块介绍是 404:1827 / 411:2094。归档的独立代运营/物流/财税页面不成为新增菜单。当前未找到详情、需求提交及入驻表单的独立终稿；这些交互沿本组原稿和现有组件补齐，不发明新的顶层产品。
为使已确认第三方路径可用，需要服务商发布/处理自己的服务入口，以及平台审核/结算入口；具体位置在本次设计中映射到现有 Console，不另建独立后台或 IAM。UI 数量、记录、价格、审核期限只能来自实际事实与运营规则，不能复制原稿示例。

当前未授权真实入驻审核、合同签署、商户进件、付款/退款/结算、共享/生产数据访问、合并、部署及 Issue 关闭。本批不自动加入客户店铺操作、AI/Agent、跨境结算、平台通用 ERP、旧数据迁移、专项验收工具或通用支付治理平台。

适用架构： [目标架构](project-target-architecture.md)、[身份与租户](auth-and-tenancy.md)、[现有商业/钱包合同](commercial-wallet-billing-contract.md)、[现有双渠道充值设计](alipay-wallet-topup-design.md)、[Greenfield](../product/greenfield-no-legacy-migration.md) 及 [AGENTS.md](../../AGENTS.md)。新第三方服务决定只改变本批范围，不能把旧充值合同中的代收付/分账排除项解释为已实现能力。

## 3. 当前实现现实与复用边界

| 代码证据 | 可复用内容 | 必须补齐或禁止误用 |
| --- | --- | --- |
| `internal/authidentity/authenticated_identity.go`、`internal/workbenchcontext/resolver.go` | ZITADEL verified user、Effective Organization、当前 grant、撤权与请求隔离 | 不创建服务商第二身份源，不把服务商加入客户企业来授予服务访问 |
| `internal/authz/enterprise_roles.go`、`module_catalog.go` | scoped role/Casbin、现有模块权限登记 | services/services-mine/services-join 当前均 unavailable；模块启用必须伴随真实 action permission 与业务关系检查 |
| `internal/httproute/descriptor.go`、`internal/app/httpapi/current_application.go` | typed descriptor、current identity、LiveWrite、显式 feature/route inventory | 当前没有生态服务模块；app 只装配，不承载服务或资金规则 |
| Console navigation、`lib/server/workbench-proxy.ts` | 单一 Shell、同源 BFF、请求上限、组织 fence、严格 payload 合同 | pending 菜单不等于业务；补 allowlist/消费者时仍按原 owner 授权 |
| `internal/integration/s3/uploader.go` | 私有 immutable put、inspect、bounded read；现有 AWS S3 SDK | 新的窄 adapter/私有权限前缀；不借 Knowledge 生命周期或 `knowledge/knowledge/*` 凭据访问生态文件 |
| `internal/commercial/billing/*`、`integration/wallettopup/*` | 当前商业订单/稳定支付身份、GoPay v1.5.123、渠道验签/查单/退款的有效行为 | 当前正式支付合同是本平台资源/充值；不能将第三方服务强制记为 WALLET_TOP_UP/RESOURCE_PURCHASE |
| `internal/ledger/money/types.go`、`integration/persistence/money/*` | 唯一 accepted payment/refund/chargeback owner、整数金额、不可变 receipt | 当前无生态第三方应付款/分账合同；个人 payout method 的 SubjectUserID 不等于服务商企业收款身份 |
| `integration/persistence/referral/economics.go` | 既有已确认的个人推广合同 | 普通付款按 CommissionableAmountMinor 计佣且仅显式排除充值；服务款不能借此自动产生推广收益 |

最后两项是实际合同缺口，分类为本批设计准入缺口：若误用现有付款或个人收款方式，会产生错误金额/身份归属。它们阻止直接拼接生产支付路径，不要求先重构全局资金系统。

## 4. Domain / Fact Owner 与调用链（草案）

| 唯一事实 | 拟定 owner | 边界 |
| --- | --- | --- |
| 服务商申请、资质/协议证据、准入状态 | 新有界 `internal/ecoservices` | 入驻是业务资格，不替代 ZITADEL 或法定主体认证 |
| 服务目录、服务商归属、发布版本、服务地区/平台/内容 | 同一 ecoservices owner | 仅已准入服务商可发布自己的条目；不会创建第二 Store owner |
| 客户需求、客户/服务商绑定、交付节点、验收、售后记录 | 同一 ecoservices owner | 一服务需求对应明确交易关系，不赋予整企业访问权限 |
| 报价/商业订单、10% 分配规则快照、交易及支付编排 | 既有 `internal/commercial/billing` 的有界服务购买合同 | 金融语义先准入；服务目录版本不代替成交报价 |
| accepted 收款、退款、第三方应付款与结算执行资金事实 | 既有 `internal/ledger/money` 扩展窄合同 | 不写客户充值余额或 orgresource，不将服务商应付款伪装成客户钱包 |
| 身份/企业 grant、角色授权 | 现有 authidentity/workbenchcontext/authz | 业务资格只能增加必要条件或拒绝，不能生成 IAM 权限 |
| 文件字节、支付与结算外部适配 | `internal/integration` 的窄 adapter | SDK、GORM 和 provider 状态留在边界外层；归属/授权由对应 owner 决定 |

链路：Console → 同源 BFF → ecoservices/billing 拥有的 HTTP descriptor → 当前身份、作用域与 action permission → 领域合同 → owner repository / 私有对象 / payment adapter → 原 owner 的持久化结果 → UI 投影。
`internal/app/httpapi` 与 currentapplication runtime 负责注入、配置、pool lifecycle 和显式 route inventory；服务状态、分配金额和恢复规则不进入 app。继续 Go 模块化单体、现有 GORM/PostgreSQL/schema 管理、Zod/React Query/Casbin/GoPay/S3 SDK，不新造通用流程或支付引擎。

## 5. 权限、跨企业协作与私有资料

客户、服务商均使用当前 Effective Organization。服务商 qualification 绑定其现有企业；客户需求中客户企业从 verified context 派生，服务商企业从服务条目的 canonical owner 派生。请求字段和 Expected-Organization header 不是授权事实。

客户可提交、读取和验收本企业需求；服务商可发布本企业服务、读取和处理绑定本企业的需求。每次读写都检查持久化绑定与当前成员权限；附件查询也必须经过同一关系检查。服务商只获得该服务必要的联系/材料/交付资料，不获得客户成员、Store、账号凭据、钱包或其他需求。

平台审核/结算角色复用当前配置的 platform-admin authority；不能使用 `listingkit.admin.write` 或 tenant-admin 当作平台权限。组织中立操作采用现有 `AuthPolicyCurrentIdentityWithVerifiedRoles` + `OrganizationAccessPolicyNone` 模式，操作目标必须从原申请/订单读取，不能借浏览器指定企业赋予权限。具体 route/permission 候选见§11.7。

资质、协议、客户材料和交付附件存于独立私有前缀，元数据记录原 owner、文件 digest/size/type 与绑定版本。按授权结果经后端 bounded download，不产生公共 URL、知识库引用或公开 Asset。原证据不可在审核/报价/验收后被同 ID 静默替换；替换产生新版本并按受影响流程重新确认。

## 6. 状态与持久化不变量（待审）

原稿中的“待确认/服务中/待验收/已完成”是用户可见服务进度，付款/退款/结算是独立资金事实。付款成功不能自动宣称交付或结算完成；服务已完成也不等于第三方已到账。

- 入驻：提交的资料与协议版本绑定原申请，审核动作写 decision/actor/time/reason 与原版本；资格/协议条件未完成时不可发布服务。平台业务审核和渠道子商户进件分别持有可核实事实。
- 发布：服务条目含服务商企业与内容版本；成交后的条目修改不能改客户已经确认的范围、价格或服务商。
- 需求：客户企业、服务商企业、服务条目版本和原报价/订单绑定必须 durable；待确认期间不伪造已收款。只有核实原付款和第三方确认范围后进入交付。
- 交付/验收：第三方记录交付版本，客户验收精确版本；同版本重复确认回读原 receipt，并发修改不能覆盖已验收事实。拒收/售后不自动抹除已付事实。
- 资金：付款、退款、佣金分配、第三方应付款、外部结算各有原业务身份、version 与不可变 receipt；服务状态表不能维护第二份可消费余额。
- 所有本地业务 mutation：状态/版本改变、操作 receipt 和相应审计在同 owner transaction 内提交；相同 org/action/key 不同 canonical payload 返回冲突。失败和重启回读原结果。

这些只是准入草案约束。完整 action/precondition/transition/effect 表及 route DTO 需按已确认退款规则与渠道合同一次冻结，不先创建生产 schema 或用实现倒推合同。

## 7. 10% 抽成、支付与第三方结算边界

金额沿现有 money 合同使用 CNY integer minor units 与 JSON decimal strings；CNY 是本草案拟复用范围，渠道/币种冻结前不扩大到汇兑。费率记录为 1000 basis points、fee bearer 为 PLATFORM、个人推广处理为 NON_COMMISSIONABLE、结算触发为 CUSTOMER_ACCEPTED；支付/结算必须绑定报价版本、服务商、客户、实际币种/金额、渠道商户与原操作身份。

技术提案：对冻结服务成交金额 G，平台名义份额 P = floor(G × 1000 / 10000)，服务商名义份额 S = G − P；采用防溢出的整数计算，P + S = G。平台承担实际支付手续费 F，服务商分配 S 不因 F 减少；平台佣金 P 与渠道费用 F 分开记账，平台净收益为 P − F。取整与计费基数作为待审技术方案冻结。按用户已确认的同比例退款规则，对累计已退款金额 R，剩余平台份额 P' = floor((G − R) × 1000 / 10000)，剩余服务商份额 S' = G − R − P'；本次平台回退额是上一次确认份额与 P' 的差，服务商调整额同理。这样多次小额退款与一次相同总额退款结果一致，全额退款后两方销售份额均为零。退款导致的收入/应付款调整必须有新 receipt，不改写旧分配；退款请求与真正成功的退款分开记录，未成功部分不能提前扣除为已退款事实。

现有直连钱包充值不授权第三方平台收付。本批复用 GoPay/现有有效验签行为，采用已选微信平台收付通。微信官方[平台收付通介绍](https://pay.wechatpay.cn/doc/v3/partner/4012086891)描述二级商户入驻、交易冻结/解冻、平台佣金与分账；其[开发准备](https://pay.wechatpay.cn/doc/v3/partner/4012086921)区分普通支付及平台账期/抽佣需求。该产品由渠道控制资金，平台触发分配，不代表资金先进入平台自有钱包。客户验收后渠道分账已获用户确认；微信平台收付通已获用户确认；平台和服务商的实际产品资格仍须核实，不凭官方有 API 推定本商户已开通。[微信官方开发准备](https://pay.wechatpay.cn/doc/v3/partner/4012086921)提供平台承担支付手续费的开通选项；该配置及手续费账户实际可用性属于渠道开放条件，不能默认使用现有充值配置。

结算接收主体来自服务商已确认且版本绑定的渠道收款身份，不使用个人 referral 的收款方式或 UI 输入的任意银行卡作为已验证资金目标。渠道接口受理不是到账；订单、进件、退款、分账/转账 UNKNOWN 保留原 identity，通过原渠道查单/查询结果核实，禁止新 key 盲重发或切渠道重付。

已选渠道的具体资金路径必须与产品分配区分：使用微信平台收付通时，客户款项进入服务商二级商户并冻结；验收后向平台分账 10%，再按核实结果解冻服务商剩余 90%。不是平台先收到全部款项再向服务商转出 90%，也不是一笔普通提现。已分账退款时，平台作为佣金接收方回退相应佣金到原分账方，服务商原商户账户提供应退份额，最后执行原付款退款；回退平台佣金成功不证明服务商已有足额退款资金。其[分账常见问题](https://pay.wechatpay.cn/doc/v3/partner/4012525463)明确区分分账回退、余额与原退款；回退功能及原收款身份能力须核实，不能假定渠道能强制收回服务商已提现资金。

**实际支付产品约束**：微信平台收付通[官方产品介绍](https://pay.wechatpay.cn/doc/v3/partner/4012086891)给出的默认最长冻结期是 180 天，超期剩余款项会自动解冻；最早可在支付成功后30秒发起分账，30秒不等于到账承诺。普通服务商分账的期限不同，不能把不同产品合同混用。本地“未验收”字段不能阻止渠道超期解冻，也不能把自动解冻视为客户验收。服务商发布及成交必须绑定真实可验收的交付结果与周期，首次支付采用一次付款；用户已决定不开放超出实际冻结期限的在线服务，分阶段付款后续再做。

**Accepted Risk AR1（2026-10-08 用户确认）**：限制预计服务周期不等于客户一定及时验收；验收延期仍可能导致渠道自动解冻。用户明确选择“保留人工审核，并接受漏处理时渠道自动解冻风险”。本批保留人工审核及原退款规则，不引入超期自动全额退款或自动验收。系统以原订单冻结期限记录到期/临期信息，平台可在现有审核入口按原订单处理；到期自动解冻的核实结果记录为渠道外部资金事实，服务验收事实保持原值，不冒充平台主动分账、客户验收或退款成功。此风险属于用户已批准范围，按 AGENTS 分类为 ACCEPTED_RISK，不阻塞正式开发；实际渠道期限和状态查询能力属于联调开放条件。

服务购买需要新增明确 purpose/type 和推广处理语义：不借充值豁免填充字段，也不沿普通 cash payment 隐式产生 10% 个人推广收益。平台抽成 10% 与现有个人推广佣金是两个不同合同；用户已确认第三方服务不产生个人推广佣金，所有现有推广接受/回读/结算消费者必须直接验证这个新 purpose，不能只在生态 UI 隐藏佣金。

## 8. 事务、恢复与外部副作用

ecoservices、billing 与 money 之间复用现有原订单/稳定操作身份/不可变 receipt 的窄协议。不同 owner database 不能由普通 GORM transaction 宣称跨库原子。具体提交与失败落点采用§11.5的原request durable command → billing原订单 → money精确receipt → 原owner完成投影协议；独立评审前仍是候选。

每个下游命令绑定上游已经持久化的原请求/订单版本；写响应丢失不能重建客户需求、支付订单或结算。现有 billing recovery 的调度叶能力在其覆盖范围内复用；生态原命令和账务操作采用各owner内部事务及§11.5有界durable outbox，不建设全局 Saga/Reconciler 平台。

渠道回调须验证签名、商户信任域、原订单、币种金额及事件身份后持久化；ACK 成功只能在相应 durable 接受后发生。分账必须消费不可变的客户验收 receipt，并重新检查原订单、付款、退款/争议与服务商渠道收款身份；第三方将进度改为完成不具备派发分账资格。分账 API 受理、平台佣金入账和服务商余额解冻均有分别核实的结果，不能以其中一个完成冒充全部成功。状态查询失败不等于未支付。撤权阻止新 dispatch；已派发的原操作仍由唯一 owner 核实并保留事实，不能用恢复创建新授权或改变收款主体。

应用启动使用显式可选 feature、owner pools、schema/权限 preflight；缺配置时真实 unavailable，不启用假收款或假服务发布。数据库与对象存储不是一个事务；附件确认按 immutable object identity + inspect + 原操作回读形成窄协议，未确认完整的文件不能成为已审核证据。文件类型/大小、分页、请求上限、deadline 与原 runtime 机制保持有界，数值在合同表冻结时给出，不引入扫描/故障平台。

## 9. Legacy、验证和交付

Legacy decision：N/A（当前只读映射与新增文档）。后续发现有价值的旧行为只 EXTRACT 到当前 owner，其余 RETIRE；无迁移、wrapper、fallback、双读写、tenantbridge 或第二身份/资金事实源。

验证围绕完整当前用户路径和实际新增风险：单服务商/客户提交与交付；平台准入与权限；精确验收；同键冲突、撤权/跨企业附件、原付款/退款/结算的重复回调与 UNKNOWN、资金分配/反转、失败响应及应用重启。业务规则与 bug 使用 TDD，复用既有 Go/隔离 PG/组件/API/浏览器能力，不新增验收 runner。
所有当前业务验证为 **NOT_RUN**；本轮只读与文档检查不声称产品验收。真实商户进件、收退款、分账/转账与客户数据也为 NOT_RUN，需要单独环境/操作授权。

实现交付需要持续可用的获准实例：客户三个原稿入口、服务商发布/交付入口、平台审核/结算入口、保存/重启说明和限制。开发自检/CI/独立代码评审不替代用户试用；一个主要 PR 的最终交付检查核对真实 diff、完整路径与准确 HEAD。

## 10. 本轮准入与下一步

Independent Architecture：**NOT_READY**。当前已完成 Issue 建立、Figma实读、owner/复用缺口映射，并同步本会话全部已确认资金规则；文件是可讨论草案，不是 IMPLEMENTATION_READY。

1. 已将验收后渠道分账、10% 抽成、平台承担手续费、无个人推广佣金及用户确认的取消/退款规则写入 #603；用户也已确认微信平台收付通、一次付款及超期服务暂不在线下单，并接受AR1，保留人工审核。
2. 基于选定支付/结算产品冻结 owner/action/状态/跨库事务及恢复合同，并补原稿缺少的最小交互。
3. 做适用独立高风险 Architecture Review；findings 先按 AGENTS 分类，最多两轮正常评审，达到 IMPLEMENTATION_READY 后再派唯一 Writer。
4. 同一主要 PR 连续实施和必要自检，完整路径形成后进行最终交付检查；合并、真实渠道开放、用户验收分别取证。

新需求边界和财务产品选择未明确时，不通过小范围编码、旧付款 adapter 或 fake success 绕过准入。该停止点来自仓库 [Architecture First](../../AGENTS.md)，不是要求新建额外治理系统。

## 11. 本批冻结候选：事实身份、动作与完成证明

下述是供独立高风险评审的具体合同候选；产品决定和AR1已经批准，技术合同在独立评审前仍未准入。只使用一个生态服务owner、一个既有commercial owner和唯一money owner；三个数据库不共享事务。

### 11.1 身份与不可变快照

- Ecoservices application ID、listing ID、request ID、delivery version、acceptance receipt和financial command ID在第一次本地事务中生成，组织来自verified context。
- Commercial服务购买订单使用明确SERVICE_PURCHASE类型；一request只绑定一个原订单和一个付款attempt。报价快照固定客户/服务商组织、条目版本、服务内容、交付/验收标准、CNY分金额、1000bps、PLATFORM手续费承担方、NON_COMMISSIONABLE、AR1说明、渠道期限、原平台商户profile与服务商sub_mchid绑定版本。
- 稳定外部身份分别是out_request_no、out_trade_no、share out_order_no、finish out_order_no、out_return_no、out_refund_no；share与finish必须使用不同身份，每个在派发前持久化。原key及canonical fingerprint重放，同key异载荷冲突；幂等重试不能轮换商户、收款人、金额或政策版本。
- 每个accepted资金事实仍使用唯一money accepted表/claim，不建立生态余额。Canonical付款身份按provider/environment/实际收款商户/原交易号作用域生成，profile或app版本是校验字段，不能靠改变版本重复领取。
- 本批不将外部付款人伪装为本系统发起人；服务份额属于原客户/服务商交易关系。SERVICE_PURCHASE付款/退款/拒付均显式NON_COMMISSIONABLE，既有个人推广接受及回读消费者必须拒绝计佣。
- Listing修改产生新版本；已成交报价、merchant binding、协议/文件及验收receipt不可原地改写。不能把收款身份变更回填到旧订单；原绑定缺少恢复配置时原单待核实。

### 11.2 Ecoservices状态/动作表

| 动作 | Actor与当前条件 | owner事务与结果 |
| --- | --- | --- |
| 提交入驻 | 当前企业有services-join管理权限，资料和协议版本完整 | application + 附件引用 + receipt同事务；SUBMITTED |
| 审核资料 | 既有platform-admin，精确申请版本，文件仍可核实 | APPROVED或REJECTED，记录actor/reason/原版本；不伪造渠道商户已开通 |
| 完成协议/渠道准入 | 本企业完成指定协议版本；商户进件核实成功 | 协议证明及原onboarding observation绑定；只有资料、协议、渠道三项满足才ACTIVE |
| 发布服务 | 当前服务商企业ACTIVE，具有发布权限 | 固定内容/价格/服务范围/交付周期版本；周期不超过当前获准冻结合同；PUBLISHED |
| 提交需求 | 当前客户有购买权限，条目仍PUBLISHED且可售 | REQUESTED，固定双方/条目版本/材料；此时没有已付事实 |
| 服务商确认需求/报价 | 当前服务商具有处理权限且是绑定服务商 | QUOTED，固定范围/标准/周期/成交金额；客户尚未确认不能创建付款能力 |
| 客户确认报价 | 当前客户企业、原报价版本，明确10%/手续费/退款/AR1及一次付款 | 原service order ID + CREATE_PURCHASE命令 + receipt同事务；ORDER_PENDING |
| 支付结果回读 | 原billing订单精确付款/资金receipt已匹配 | PAID_READY，保存receipt引用；服务状态是投影，不产生第二收款事实 |
| 开始服务 | 当前绑定服务商，PAID_READY且没有取消/退款financial fence | SERVICING；与开始前取消使用同一request锁/版本，先提交者决定下一步 |
| 提交交付 | 当前绑定服务商，SERVICING，无未决取消 | AWAITING_ACCEPTANCE，固定交付版本/文件/内容；修改交付创建新版本 |
| 拒收/修正 | 当前客户，精确交付版本，尚无对应acceptance | 返回SERVICING并记录原因；不自动退款、不自动分账 |
| 验收 | 当前客户企业有验收权限，精确交付版本，无争议/退款fence | acceptance receipt + SETTLE命令同事务；ACCEPTED，结算仍可处于处理中 |
| 开始前取消 | 当前客户，尚未开始且金融命令可互斥 | CANCEL_REQUESTED + 原关单/全额退款命令 + receipt同事务；未核实退款不得显示已退款 |
| 开始后售后 | 当前客户/服务商，仅原订单 | 保存双方协商金额/版本；平台审核后REFUND_APPROVED命令；待核实与已退款区分 |
| 到期/自动解冻 | 原渠道可信查询/动账观察 | 保留原服务状态与acceptance；更新渠道资金投影和人工处理原因，按AR1不自动验收/退款 |

UI的“已完成”表示已有客户验收，结算状态另列。取消/售后状态通过原列表附加状态呈现，不能强制塞进四个进度导致事实丢失。统计按筛选条件对完整owner集合计算，不取当前页行数当总量。

### 11.3 渠道进件与商户归属

入驻业务审核不等于微信商户审核。Ecoservices拥有onboarding attempt及其申请版本，业务申请号由服务端生成并绑定原企业和申请。新操作只能从平台已批准申请生成固定的渠道进件意图；SDK DTO、任意商户号和自由JSON不能成为领域合同。

窄MerchantOnboardingPort接收已固定的申请/法人/联系/银行资料及私有证明文件引用，由integration转换为微信规定的企业进件字段；创建前须有本企业及平台审核的版本证明。复用GoPay V3EcommerceApply和V3EcommerceApplyStatus，敏感字段按微信公钥要求加密，银行/法人资料以现有密钥管理和加密库加密保存，日志不记录原始资料。渠道要求账户验证/签约时显示原申请给出的受控链接或操作指引，不替服务商同意协议。

只有原平台配置下查询原out_request_no，得到匹配原申请且FINISH/签约完成的sub_mchid，才写入唯一merchant binding；NEED_SIGN返回商户号也不算已开通。响应丢失按原申请号查询，未知时不建第二申请。一个渠道收款绑定不得重复归属多个无关企业。平台与服务商的真实进件/签约操作仍需单独授权；本地替身不证明渠道资格。

#### 11.3.1 渠道 REJECTED 后更正增量

状态：`IMPLEMENTATION_READY`。评审4220555048的有界增量在两轮独立审查后冻结：第二轮绑定b33f0222d及本节23行增量，确认无剩余BLOCKER；后续生产实现只消费本节准入，验证余项为IMPLEMENTATION_TEST，不重复全局架构评审。设计Ready不代表试用或真实渠道PASS，实施与验证状态由执行Issue/PR记录；此前已批准、未变化的合同保持原准入。

用户结果与边界：原企业可按渠道拒绝原因修正资料，重新确认适用的平台审核与协议，再沿原渠道申请继续入驻。仍使用原企业、原申请、原平台配置和原 `out_request_no`；不允许换主体、换号、删除原 intent/拒绝证明、绕过平台审核、迁移旧数据或新增通用恢复平台。当前产品/Figma Authority 和 §11.2 入驻 owner 保持。

渠道依据：[提交申请单](https://pay.wechatpay.cn/doc/v3/partner/4012713017) 明确被驳回时可沿同一业务申请编号修改原申请，成功应答给出 `applyment_id`/`out_request_no`；[原业务号查状态](https://pay.wechatpay.cn/doc/v3/partner/4012691376) 不回显本地资料版本或请求指纹。文档未证明更正会生成不同 `applyment_id`，不得把不同号或相同号作为版本证明的默认假设。

拟复用 E 的 immutable versions、原 application/intent、私有文件和 claim/CAS：

- 更正入口要求原企业当前 join 权限、精确当前 application 与资料版本，以及可信原 terminal REJECTED 证明；未绑定商户、无未知或可能在途的旧资料派发。相同 key/指纹回读原更正，异载荷冲突；原拒绝与文件保留。
- 原 intent 原样保留（包括初始 sealed payload/指纹），不 UPDATE 或删除；它持有稳定申请/企业/profile/渠道号。当前资料的唯一消费依据是新增 immutable revision，不读取 root 初始 payload 作为缺版本时的 fallback。复用 `ecoservices_versions`：kind=`MERCHANT_DETAILS`、entity_id=原intent ID、version=单调资料版本，payload含sealed details、审核/协议引用、文件集合、指纹；progress继续位于 `ecoservices_merchant_progress`，ID=该资料revision ID，各自有media/dispatch/claim。新安装创建v1时即写该revision；不增加历史数据迁移/兼容路径。
- `Application`既有payload保存唯一 `currentMerchantRevisionId`/版本。E锁原application及当前progress，同一事务校验actor/org、If-Match、current pointer、原terminal proof、无旧未知/在途派发，追加下一revision/progress和application版本/history、更新current pointer及操作receipt；同key回读原revision，异载荷冲突。claim以当前revision ID为单位；上传、mark dispatch、受理证明写入、query observation、release及binding写入均校验该current pointer及原claim/CAS。lease过期不证明旧派发没有发生。
- 企业名称/登记主体保持原身份。改变平台已审核的内容或许可证时，原 application 回到 SUBMITTED 追加版本，重新审核、协议；仅渠道补充字段的更正仍绑定有效批准及协议版本。不能以渠道 REJECTED 代替平台批准。
- 每次新派发前再次检查 live 权限、当前版本指针和原证明，在 E 保存 MAY_HAVE_DISPATCHED 后才调用 SDK。窄 `SubmitMerchant` 返回 typed `MerchantSubmissionAcceptance`：实际发送当前sealed revision的同一次SDK HTTP调用验证200签名后，构造revision ID/版本、资料指纹、原profile/outNo、响应applyment_id、verification version和签名响应摘要。revision/指纹是本地调用关联，不能宣称微信回显这些字段；错误、丢响应、错号或无签名均没有受理证明。
- 受理证明在E事务中再次核对当前revision及claim，追加不可变 `ecoservices_versions(kind=MERCHANT_SUBMISSION_ACCEPTANCE, entity_id=原intent ID, version=资料版本)`，sealed payload及指纹保留原SDK证明；同证明重放回读，冲突证明拒绝。提交成功/E保存失败仍属UNKNOWN，不能用状态查单补造受理证明。原progress只保存该proof引用/投影，不成为第二受理事实源。
- 更正revision只有在此受理证明durable以后发起的新可信查询，且原profile/outNo/响应applyment_id精确匹配，才可消费到该revision的状态/链接/资格；本地查询调用绑定当前revision及受理proof，旧查询/旧sealed observation不可重标新版本。新更正再次被拒绝时，下一个不同payload同样须以当前受理proof之后的匹配可信REJECTED为前提；无proof或只有旧拒绝保持UNKNOWN。原v1未发生更正时保留既有原号查询恢复合同，只有开始更正才引入这个更强的版本关联消费门槛；进入更正后不得退回v1查询规则。
- 旧 version/claim 不能写当前 projection、签约链接或 merchant binding；读取只查询原号，不提交。只有已审核的当前资料、协议及可关联当前派发的可信 FINISH+SIGNED 才能 ACTIVE，merchant binding 仍唯一归属原企业。

响应丢失的边界：同号状态响应不回显资料版本；受理证明缺失、只有旧REJECTED或不能确认当前派发关联时，只保存原号及本次sealed payload/dispatch与待核实原因。不能打开下一份不同资料、当前签约操作或ACTIVE，不自动重发、换号、删除证明、使用申请号/响应签名时间当版本证明或假设更正生成新申请号。UNKNOWN按现有安全合同保留，不承诺自动恢复；本增量不增加人工改绑/强制成功命令或通用恢复平台。

验证范围限该增量：同企业/live/CAS、相同 key 异资料、版本/审核/附件原子保存、旧 worker 不写当前、新资料受理丢失与旧 REJECTED 不开放下一更正/ACTIVE，以及正确原号更正后 FINISH+SIGNED。沿用 Go/实际受限 PG/签名 SDK 与现有 Console/BFF；不追加专项验收系统。当前进件修正的正常浏览器试用与真实渠道、产品验收仍为 `NOT_RUN`，需按各自授权和交接验证。

### 11.4 资金目的和money合同

money新增有界ServicePaymentInput、ServiceOperationReservation、ServiceEffectReceipt和ServiceFundsView，归既有money owner。输入/结果都包含service request/order、双方组织、原渠道交易、币种/金额、policy fingerprint及完整操作kind/ID。

- AcceptServicePayment在money同事务接受canonical payment、服务绑定与不可变allocation receipt；不增加客户钱包、orgresource或推广余额。相同channel claim异order/payload冲突。
- PrepareServiceOperation在原service binding锁下保留本次经济效果，校验退款累计加未决保留不超过原可退净额、分账/退款不重叠。它保留的是原渠道交易的操作额度，不声明平台持有第三方资金。
- AcceptServiceEffect在同事务保存原share/finish/return/refund可信事实、更新累计已确认效果、settle reservation和immutable receipt。退款成功只在渠道核实后减少有效成交净额；收到PROCESSING/接口受理不减少为已退款。
- ReadServiceFunds/ReadServiceEffect按原order+kind+ID读取精确receipt；错误组织、kind或payload不能回退查找另一事实。服务状态或某次可变余额不替代receipt。
- SERVICE_PURCHASE退款/拒付上限来自原gross及原事实，不借CommissionableAmountMinor=0拒绝合法退款；统一money内部事务接受入口，禁止旧普通退款方法形成只记cash却漏掉服务allocation的旁路。
- 对已确认外部冲正/自动解冻，如实际金额/时序与本地保留不一致，保存原资金事实和RECONCILIATION_REQUIRED原因；不伪造新债务、自动核销或从另一客户交易补足资金。
- §7累计净额公式是本批取整方案；每次退款的两方调整之和等于本次实际退款。原退款失败/取消后，已完成的分账回退事实仍保留，不能盲重分账恢复收入，交平台原单核对。

### 11.5 唯一恢复责任与跨owner协议

Ecoservices负责本域资料/交付/验收及原financial command的接收/版本fence；billing负责原商业订单、所有支付/分账/回退/退款派发和恢复；money仅接受、保留与回读资金效果，adapter只映射协议。恢复不是新前台购买，也不新建IAM授权。

| 本地边界 | 原子提交内容 | 响应丢失/失败后的唯一继续方法 |
| --- | --- | --- |
| E1：客户确认报价 | 原request/报价版本、稳定service order ID、CREATE_PURCHASE命令、receipt | ecoservices按原command回读；pending commands经原用例继续 |
| B1：创建购买 | SERVICE_PURCHASE订单、attempt、原service command引用、冻结快照及receipt | billing按原service request/order唯一键回读；不能生成第二付款attempt |
| E2：绑定订单 | 校验B1精确receipt后保存原绑定/投影 | E1恢复只补E2；已经B1成功不再创建交易 |
| B2：付款准入 | live购买权限，原attempt版本CAS，派发身份与原输入 | 丢失后查询原交易；未授权/撤权拒绝新付款入口 |
| B3：外部收证 | 已验签且匹配原商户/交易/金额/币种的inbox、observation、receipt | durable前不ACK；原事件和查单结果去重 |
| M1：付款入账 | payment claim、accepted payment、服务绑定与allocation receipt | billing调用money原身份readback，不再次领取资金 |
| B4/E3：完成与可交付 | B4绑定M1receipt；E3回读B4后更新PAID_READY | 两个owner各自事务；只补未完成投影，不直接触发交付 |
| E4：验收/审核退款 | 原acceptance或退款审核、finance command、fence与receipt | pending command仍带原版本/业务授权证明；与拒收/取消在同request锁下互斥 |
| B5/M2：金融操作准备 | B5原intent与稳定外部号；M2原money reservation | 从原intent和reservation收敛；共享事务只在同owner内部 |
| E5：派发前源准入 | 原command一致、原request符合状态，持久化一次dispatch admission/token | 先收到退款/争议fence则不准入新分账；准入后属于原在途命令，后续售后排队核实 |
| B6：派发标记 | 原源proof+reservation绑定、claim/version，MAY_HAVE_DISPATCHED | 先持久化再调用渠道；崩溃按原号查询，不能凭本地没有成功响应重建身份 |
| M3/B7：确认资金与完成 | M3资金receipt/额度保留结清；B7校验精确receipt后DONE | M3成功B7失败只补B7；UI只读owner证明 |

每个财务命令在原billing order下串行化；一原订单同一时刻最多一个mutating financial command进入准入/派发。并发approve refund/share、取消/start、accept/reject经E的request锁和B/M各自原order锁/版本fence收敛；跨owner调用不持数据库锁，不宣称三库原子。

E1/E4的pending command本身就是有界durable outbox，不另建通用平台。原key+payload指纹一旦创建不可改；claim/lease/token只用于调度，一旦过期不能声明旧进程没有调用渠道。旧token回写由CAS拒绝，渠道稳定身份及money唯一receipt阻止重放资金效果。一个已有runtime恢复循环按需消费pending commands与billing原在途订单，各自调用唯一owner，不增加第二业务状态机。

新用户命令必须live授权；缺权限时未派发命令终止或保留明确拒绝结果，不生成支付能力。客户已经合法提交验收/平台已经合法批准退款的不可变证明可用于完成该次原业务命令；其后成员撤销不删除已存在业务资产或已派发资金事实。恢复只核实/继续同一已准入原操作，不换收款主体、不产生新报价/新授权。平台或商户被渠道限制时保留原单待核实。此区分必须由实现的并发/撤权测试证明。

### 11.6 微信平台收付通适配约束

首发复用现有Console二维码支付体验，适配微信服务商Native产品并将settle_info.profit_sharing固定为true。使用单服务商/单订单付款，避免无必要的多商户购物车；实际平台产品资格和商户绑定必须在真实联调核实。

| 领域命令 | SDK/渠道职责 | 完成条件 |
| --- | --- | --- |
| CreateOrReadServiceCheckout / QueryPayment / ClosePayment | GoPay微信服务商Native/原单查询/关单，固定sp_mchid/sp_appid/sub_mchid | 二维码不是收款；只有原交易可信SUCCESS、币种/金额/商户匹配才能入money |
| SharePlatformCommission / QueryShare | V3EcommerceProfitShare，finish=false；只分给原平台商户的精确P | 分账单FINISHED不足；指定receiver/type/amount及result=SUCCESS才确认佣金 |
| FinishServiceSettlement / QueryFinish | V3EcommerceProfitShareFinish，独立finish out_order_no；核实原finish结果 | 佣金已确认且没有新退款fence才能解冻剩余S；接口受理不能宣称释放完成 |
| ReturnPlatformCommission / QueryReturn | V3EcommerceProfitShareReturn，绑定原成功share和独立out_return_no | 匹配原share/return_mchid/amount/result=SUCCESS；不能回退不存在/失败分账 |
| RefundServicePayment / QueryRefund | 平台收付通原交易退款；退款来源固定原二级商户，禁止PARTNER_ADVANCE | 原退款身份/金额/SUCCESS精确匹配；余额不足保留WAITING_FUNDS/UNKNOWN，不换号或代垫 |
| QueryUnsplit / QueryOnboarding | 原平台服务商身份下的待分金额及申请查询 | 查询只是原外部事实，不授予新的企业/收款关系 |

平台抽佣失败不得调用finish把全部G释放给服务商；退款与正常finish共用原金融fence。部分分账后的退款按微信实际要求可能需要先解冻剩余资金才能退款：该解冻仅属于已批准退款的原命令，不冒充正常结算；核实原回退及退款资金后执行，仍保留余额不足的真实限制。退款不会自动触发已失败服务的再购买。

只使用所选ecommerce产品API；不能误用普通30天冻结分账或直连钱包Native。adapter禁止自由URL/任意BodyMap透传；GoPay结构留在integration。复用原微信公钥验签、错误响应验签和有界静默HTTP行为，EXTRACT有效叶能力而不依赖钱包top-up业务port。SDK验签覆盖范围之外的错误/回调独立核实；原交易标识不足不作已不存在证明。

渠道超时/PROCESSING/未验签错误是UNKNOWN。明确终态失败且原渠道合同允许原号重放时才重放；必须另建号的终局失败，先保存终局证据及原操作从未/不能再产生效果的证明，再经原owner建立下一原业务尝试。不能把“未找到”或余额不足改成成功，不能在结果未知时换号。

### 11.7 HTTP、BFF与最小消费者合同

普通生态接口在/api/v1/ecoservices前缀；平台审核在/api/v1/admin/ecoservices；微信通知是固定独立ingress。所有JSON请求strict unknown-field拒绝，金额/版本用decimal strings、ID有界。身份来自现有descriptor和authidentity。

| 路由组 | Policy / permission与关系条件 | 消费者 |
| --- | --- | --- |
| GET catalog、listing detail | CurrentIdentity + CachedRead，workbench.ecoservices.read；仅published公开字段 | 服务市场 |
| GET/POST application、agreement、onboarding | CurrentIdentity + LiveWrite，workbench.ecoservices.join；目标原申请属于当前服务商org | 申请加入/申请进度 |
| GET/POST provider listings / publish | CurrentIdentity + LiveWrite，workbench.ecoservices.manage；原服务商org、准入状态 | 申请加入后的服务发布面板 |
| GET requests / summary / detail | CurrentIdentity + LiveWrite，workbench.ecoservices.read；当前org为原buyer/provider之一，按侧投影 | 我的服务/我承接的需求 |
| POST quote / start / delivery / refund proposal or confirmation | CurrentIdentity + LiveWrite，workbench.ecoservices.manage；原provider org，退款确认绑定双方一致的精确金额/版本 | 我的服务里的服务商处理面板 |
| POST request / quote confirmation / accept / reject / cancel / refund proposal | CurrentIdentity + LiveWrite，workbench.ecoservices.purchase；原buyer org/精确版本 | 客户详情和付款面板 |
| checkout / financial status | CurrentIdentity + LiveWrite，purchase + 原buyer org；只读真实billing结果 | 二维码付款和订单资金进度 |
| files create/confirm/download | CurrentIdentity + LiveWrite，当前业务action与原parent关系；platform只按平台路由取目标 | 私有资质、客户材料、交付文件 |
| admin application reviews / refunds / due orders | CurrentIdentityWithVerifiedRoles + None，既有platform-admin permission | 当前Console内平台审核面板；不能由租户角色映射授予 |
| notify / recovery | 验签信任域或静态runtime调用；无浏览器identity header授权 | 原billing收证/恢复 |

新权限接入既有WorkbenchPermissions/Casbin和三个module IDs：services-read，services-mine-read/purchase/manage，services-join-join/manage；业务service仍检查双方绑定及准入。菜单/前端不是权限事实源；平台入口能力来自后端真实鉴权结果。查询用当前org过滤，不能把浏览器传入org写进目标事实。

BFF沿现有workbench同源proxy/严格route allowlist、CSRF/origin、cookie/bearer和Expected-Organization校验；禁止前端自由转发provider API。模块app注入只构造deps/port/config/pool、注册域httpapi.Routes描述符并启动有界恢复，业务规则留owner。

JSON 1MiB上限（敏感商户申请可用2MiB独立上限）、文件10MiB、单申请/交付至多10附件；分页默认20/最大100，search≤200字符。普通请求10s、金融/文件30s并绑定调用deadline；渠道HTTP有界10s、响应1MiB且关闭敏感日志。这些是当前实现资源上限，不是新增容量框架。

### 11.8 持久化、文件与注入

Ecoservices使用独立owner pool及schema：applications/application_versions、merchant_onboarding_attempts/bindings、listings/versions、requests、deliveries、acceptances、refund_agreements/reviews、files、financial_commands/receipts。commercial新增有界service_orders/payment_attempts/financial_operations/inbox，money新增service_payment_bindings/reservations/effect_receipts；复用canonical accepted payment/refund/chargeback表，不复制余额。

数据库约束至少覆盖：原org/action/key唯一、application/merchant attempt一原版本、一channel sub_mchid归属、request一service order、order一payment attempt、渠道原外部operation identity唯一、acceptance(request,delivery version)唯一、financial command payload不可变，以及money服务binding与canonical claim/effect(kind,ID)唯一。money写入依序锁原payment/service binding→原reservation→effect记录；退款累计/保留与receipt同事务，不在多个各自提交的公共方法间假称原子。

Schema按现有owner SQL/provision模式交付，全新安装、空业务数据；运行仅preflight/schema/role验证，不自动建表或复用legacy DB。部署开关至少分离ModuleEnabled/NewPaymentsEnabled，关闭新付款不能关闭已有订单读取、退款与恢复。原merchant profile凭据保留至原在途订单完成；配置缺失返回DEPENDENCY_UNAVAILABLE，不使用另一profile/fake adapter兜底。

文件使用现有私有S3 immutable Put/Inspect/BoundedRead叶能力的新adapter；key由server原file ID及owner prefix生成。先持久化原upload intent，put immutable object，再inspect digest/size/type提交confirmed metadata；中途失败只能原key读回，未确认对象不可引用为审核/交付证据。对话/日志不暴露PII、secret或公共URL；下载前重新检查原parent的当前授权。尚未确认对象的清理由既有存储生命周期管理或后续有界维护，不能扩大为本批新治理平台。

### 11.9 实施与必要证据

同一主要PR内先完成权限/入驻/私有资料和目录，再接需求/报价/交付/精确验收，再接SERVICE_PURCHASE资金owner和微信port，最后Console/BFF与完整路径；每片是可回滚提交边界，不机械拆PR。业务规则TDD，先证明目标规则失败，再最小实现。

必须直接验证：跨org与撤权；平台role不能被企业module grant替代；start/cancel、accept/reject、share/refund并发；原claim/同key异载荷；M1/M3成功后B/E提交或响应丢失的readback；share FINISHED但receiver CLOSED；UNKNOWN不换号；部分退款累计取整/无佣金与既有充值/计佣回归；私有附件父关系；关闭new payment仍保留恢复。测试复用当前Go、隔离PG和既有UI/浏览器工具，不新增专项runner。

本地受控渠道测试、实际微信产品资格/进件、真实收退款分账、客户使用验收分别记证据。缺真实开通不阻止已准入的本地实现，但真实微信链路保持NOT_RUN。最终交付必须给出持久实例/正常启动方式和可用边界；独立评审和CI不宣称产品验收或生产开放。

## 12. 独立评审记录

2026-10-08，独立只读 Reviewer `ecoservices_architecture_review` 审查准确设计提交 `0e4d1683477fcd5af21c1f5cf8484ef5d40b1730`（PR #604）。结论：未发现成立的设计 BLOCKER，可进入 IMPLEMENTATION_READY。评审时正式生产代码、schema、配置均未修改。本轮为第 1 轮，不代表产品验收或真实资金开放。

以下 Finding 均分类为 IMPLEMENTATION_TEST，必须在同一主要 PR 合并前实施并验证，不重新打开已冻结设计：

| Finding | 受影响 Must / 后果 | Action |
| --- | --- | --- |
| 非计佣的旧接受入口及消费者旁路 | 服务退款合法且不产生推广佣金 | service purpose 精确接受/回读；旧普通入口拒绝；退款/拒付和推广消费者验证，保留充值/普通计佣回归 |
| cancel/checkout 跨 owner 与迟到付款 | 开始前取消全额退款；不能重新生成支付能力 | cancel 先提交禁止新 checkout；在途只核实/关闭原单；迟到可信支付入原资金事实并转原退款，不进入可交付 |
| 零佣金、验收前部分退款 | 10% 与累计同比例退款 | 以已确认累计退款净额计算；零 share/return 由本地不可变无经济效果证明跳过；验证小额累计、全退及回退后退款失败 |
| 双方退款协商精确版本入口 | 开始后双方协商、平台审核 | 服务商提案/确认绑定原订单、金额及版本；变更金额/版本使旧同意失效；平台只消费双确认同版本 |

AR1 为 ACCEPTED_RISK，不增加自动验收/超期退款；真实渠道资格、手续费配置、冻结/退款/回退时限在开放前核实。渠道已知终态失败记明确人工原因，不能无限记为 UNKNOWN。正式实施及最终交付检查沿本设计与 AGENTS，不创建版本化设计文档链。

### 12.1 Native 付款码恢复增量（2026-10-08，IMPLEMENTATION_READY）

PR #604 的 `be5445c30` 评审发现：缓存付款码未在解密后重新核实当前取消/授权；付款码响应、加密或持久化失败后，原派发标记使客户永久无法继续支付。分别影响“取消后不能新增支付能力”和正常付款路径，分类为 BLOCKER。只重新打开以下原 Native checkout 恢复边界，既有产品、E/B/M owner、原交易身份、资金与 AR1 规则保持冻结。

- 原缓存码与新生成码都在返回前执行当前客户授权与 E 原命令准入核实；解密/网络耗时期间发生取消或撤权时不返回付款能力。已经交付的二维码无法由本地撤回，仍由既有 cancel → 原关单 → 迟到付款原退款协议处理。
- 原订单没有保存付款码但 `PaymentDispatched=true` 时，仅在客户正常 checkout 操作中、持有同一 B claim 的情况下查询原支付身份。先持久化匹配的可信查询，再接受原 PAID/REFUND 事实；只有直接查询成功且匹配原 profile、商户、订单、币种的 UNPAID 结果允许重新请求 **相同原参数** 的 Native 付款码。不清除派发标记，不换 trade/order/profile/到期时间/金额，也不在后台自动生成新的支付能力。
- 可信查询失败、签名或身份不匹配、CLOSED、已支付及未核实结果均不下单。渠道返回下单错误仍保留原派发标记，下一次正常请求重新查原单。每次原参数重入前重新执行 live 授权与 E 原 checkout 准入，返回前也核实；取消、撤权、关闭新付款、原到期或 profile 不可用时不得重入。M 的资金接受身份与唯一 receipt 不改变。
- 查询回执显式携带本次 Native 重入证明，不能仅凭旧 UNPAID 状态：成功查询必须是原 NATIVE/NOTPAY 且渠道原金额、CNY 均匹配；签名核实的原商户/原号 ORDER_NOT_EXIST 使用独立缺失证明，不能伪造渠道金额或已付款结果。两种证明仅允许同一原参数重入；USERPAYING、普通未支付旧回执和缺失证明未核实均不授权重入。证明随原观察保存，不新增 schema；恢复消费本次直接查询，不从旧回执中推定新许可。
- 渠道依据：[微信 Native 常见问题](https://pay.wechatpay.cn/doc/v3/merchant/4012791890) 明确原参数重入可刷新付款码，旧 code_url 会失效；[服务商 Native API](https://pay.wechatpay.cn/doc/v3/partner/4012738659) 使用原 out_trade_no，SYSTEM_ERROR 要求相同参数重新调用。结合两项官方说明采用同一 Native 原订单重入，不将“没收到响应”当成未派发，也不引入新交易或新外部幂等号。实际商户行为仍在真实联调核实。
- 验证复用现有 Go/PG/SDK 测试：缓存解密期间取消及撤权；原响应/加密/保存丢失后可信原单未支付恢复同一参数；查单 UNKNOWN/错身份/CLOSED/PAID、查询期间取消/撤权不重入；原派发标记与原号保持，迟到支付不重复入账。只做受影响增量，不新增恢复平台、schema 或 legacy 兼容路径。

Design Basis：原 Independent Architecture 的有界 Native checkout 增量。2026-10-08，独立 Reviewer ecoservices_architecture_review 完成此增量复核，未发现新的设计 BLOCKER，确认 IMPLEMENTATION_READY。原两项生产 BLOCKER 仍须修复；严格未支付/缺失查询证明、错误金额/币种及UNKNOWN零重入归为 IMPLEMENTATION_TEST，在受影响SDK/合同测试收敛。其余冻结基线不重审。
