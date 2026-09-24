# 支付宝企业钱包充值架构设计

**状态：DRAFT / 待架构评审；不是 IMPLEMENTATION_READY。**  
**关联：#481；复用 #457；订阅消费沿 #479 / #480。**  
**设计日期：2026-09-24。代码核对基线：`c5deccbe7d42e211ab1e2c80987b1e7e67b671f2`。**

本文件是本批次的设计草案，不覆盖已批准的钱包、资源购买和订阅合同。本轮只交付文档；不安装 SDK、不改运行代码、数据库或配置、不执行真实付款或退款。独立评审、产品待决事项、实现和生产开放分别记录，不以文档存在代替准入。

## 1. 设计决定与用户结果

**使用 GoPay 的主要原因是复用支付宝的 Go 接口封装，避免自行维护支付协议；不是为了提前建设多渠道聚合平台。** SDK 技术方向采用 `github.com/go-pay/gopay`；支付宝是本设计的首发渠道。微信、Stripe、通用渠道路由不进入首版。

目标用户是当前已验证 Effective Organization 内有 `workbench.commercial.wallet_topup` 权限的企业管理员。目标链路：

```text
充值中心 → 创建充值订单 → 支付宝付款
        → 服务端核实付款 → 企业钱包入账或优先偿债
        → 订单 / 余额 / 流水可回读
        → 用户另行选择使用钱包购买套餐或资源
```

充值成功不自动购买、激活权益或重新执行失败的消费。套餐 `EXTERNAL_PAYMENT` 直付仍未开放，#479 的 ZERO_PRICE / WALLET 实现不以本批次为前置。

首版限定 CNY、单渠道、单个收款商户配置、用户主动单次充值。不做自动续费、自动充值、跨企业转账、提现新业务、分账、发票、多币种、旧账务迁移或完整自助退款工作台。必要原路退款和外部冲正安全包含在设计中。

### 1.1 首发交互方案

建议先使用**电脑网站支付 `alipay.trade.page.pay`**：适配当前 Web Console，服务端生成渠道付款跳转，浏览器前往支付宝收银台。GoPay 对应 `TradePagePay`；生成付款参数或 URL 不是已经创建远端交易，更不是付款成功。[S1][S2]

这是**待产品及商户产品权限确认的接入建议**，不是宣称已经签约。若商户只具备当面付等其他产品权限，应先调整本节与 checkout 适配契约，再实现该单一产品；不得运行时盲目降级到另一支付产品。首版不同时开发 Page Pay、WAP、APP 和扫码直连。

### 1.2 尚待确认但不阻塞文档设计的事项

| 编号 | 待确认项 | 建议 / 未确认时的行为 | 阻塞层级 |
| --- | --- | --- | --- |
| D1 | 收款主体、商户 PID、应用及企业充值业务是否获准 | 由用户/商户负责人确认，SDK 能调用不代表业务获准 | 渠道联调、真实开放 |
| D2 | 首发支付产品 | 建议电脑网站支付；按实际产品权限核实 | 渠道特定实现准入 |
| D3 | 充值档位或自定义额度、限额、手续费与优惠处理 | 不设默认生产金额；应付和钱包面值建议相等，渠道手续费单独处理 | 创建真实支付 |
| D4 | 充值的推广佣金及业务付款人归属 | 建议首版充值不计佣；不得为适配旧 validator 伪造正佣金或付款人 | money 合同变更准入、真实开放 |
| D5 | 迟到付款与主动退款政策、操作权限 | 建议有效迟到付款按原订单入账；主动退款经单独授权并先保留资金 | 取消/退款实现准入、真实开放 |
| D6 | 沙箱账户、凭据安全交付、回调域名与运行配置 | 沙箱和生产隔离，真实小额付款也需授权 | 对应环境联调/开放 |

D3–D5 是产品政策，不由 SDK、Figma 示例或 Reviewer 代替用户决定。未确认时相关写 capability 保持关闭；读回与已发生资金事实的处理不能被新付款开关关闭。

## 2. 已有能力与实际缺口

以下是基线代码观察，不是全仓审计或运行验收。[R1][R2][R3][R4]

| 对象 | 当前事实 | 本批次最小增量 |
| --- | --- | --- |
| `billing.Order` | 已有 WALLET_TOP_UP；只允许 PENDING / FULFILLED / CANCELLED；未完成态不能携带 PaymentID | 不套用资源购买状态；增加 billing 自有的 top-up payment attempt 和完成凭证 |
| `Service.CreateWalletTopUpOrder` | 当前直接返回 `ErrFeatureUnavailable` | 实现创建订单及支付尝试的事务，不只是打开前端按钮 |
| `PaymentSettlement` | 已有 accepted payment；要求非空 PayerUserID 和正 CommissionableAmountMinor | 按 §9 设计真实的无佣金/未知外部付款人表示，不伪造字段 |
| `OrganizationTopUpSettlement` | 已绑定 payment / order / Organization / currency / amount | 复用此绑定，增加可核实的精确入账结果读取 |
| `CreditSettledTopUp` | 已有钱包、入账绑定、流水事务及 debt-first；返回的是 wallet snapshot | 复用算法与表；增加 source-bound receipt，不能以可变余额证明某笔入账 |
| top-up persistence | payment 主键、commercial order 唯一约束已经存在 | 强化渠道交易身份 claim 和 readback，不另建钱包余额 |
| refund / chargeback | 已有事实类型和钱包冲正入口 | 增加主动退款资金保留及统一确认路径，复用既有冲正语义 |

`CreditSettledTopUp` 当前可接收不大于付款 gross 的金额，且重放时返回当前钱包快照。这不能自动证明本次充值与订单**精确等额**或返回了原次入账结果；新增边界必须作更严格的对应检查，不能直接把现有返回值当完整 receipt。[R4]

## 3. 责任与代码依赖

| Owner | 唯一责任 | 禁止承担 |
| --- | --- | --- |
| `internal/commercial/billing` | 充值订单、支付尝试、checkout admission、已核实证据引用、订单完成、唯一充值/退款恢复协调 | 钱包余额算法、渠道签名、订阅激活 |
| `internal/ledger/money` | 接受付款/退款/拒付事实，交易 claim，企业绑定，资金保留、入账/冲正、不可变 receipt | 商品定价、页面付款流程、商业订单编排 |
| `internal/integration/payments/alipay`（拟新增） | GoPay 调用、响应与通知验签、金额转换、渠道状态及错误映射 | 写订单/钱包表、自行安排业务重试、产生权益 |
| billing 自有 HTTP + app 装配 | 验证用户入口；将固定 callback route 接至可信 adapter；显式启动/停止恢复循环 | 在 handler 中直接加余额；新的总支付 Service |
| 现有 BFF / Console | 当前企业范围传输、付款入口、真实状态、订单/余额读取 | 用浏览器回跳、缓存或按钮点击宣告到账 |

```text
Console → BFF → billing 领域用例 → 本地窄 port → Alipay adapter → GoPay → 支付宝
                    ↓
              money 领域用例 → money persistence

支付宝通知 → 固定 ingress → adapter 验签/映射 → billing 接收证据
                                                    ↓
                                    同一充值 reconciliation 路径
```

仅 adapter 依赖 `github.com/go-pay/gopay/alipay` 和 `gopay.BodyMap`。领域请求/结果不含 SDK 类型、`*http.Request`、任意 URL 或自由透传参数。app 只装配，billing 可以消费已有 money 合同；不创建第二账本、通用支付微服务、Saga 平台或调度框架。

持久化落在现有 `internal/integration/persistence/commercialbilling` 与 `.../money`；拟新增 adapter 路径不是已存在代码。共享 billing / money / app 文件与 #479 串行交接，本轮文档不占其源码。

## 4. 业务身份、金额与幂等

### 4.1 三类身份不能混用

- **InitiatorUserID**：创建订单时的本系统已认证用户，写入 immutable intent 与 audit。
- **BeneficiaryOrganizationID**：创建时的 verified Effective Organization，此后不随成员关系变化。
- **Provider buyer identity**：支付宝付款人标识，仅为渠道事实；不等于 InitiatorUserID，也不授予本系统权限。可以由别人付款，是否允许由 D4 明确；不得把支付宝 UID 填进本系统用户 ID 字段。

回调按持久化 attempt 查找原企业，不使用当前登录用户、当前 membership、客户端 `attach/passback_params` 或回调任意 org 字段推断归属。支付记录仍属于原企业；撤权只限制新的操作和读权限，不抹去已收到的资金。

### 4.2 身份链

```text
commercial_order_id ↔ 一个 durable payment_attempt_id
payment_attempt_id   → 一个持久化 out_trade_no
provider transaction → 一个 canonical payment_id
payment_id + order   → 一个 money top-up receipt
refund_id            → 一个持久化 out_request_no
```

首版一个充值订单只生成一个渠道支付尝试。未决时不新增 attempt；明确结束后确有新的充值意图才创建新订单。超时、刷新、网络重试均不等于新的充值意图。

`out_trade_no` 建议使用固定前缀加随机 UUID 的无连字符形式，生成后持久化且永不改变；不拼接企业/用户长标识。业务 fingerprint 使用带版本的确定性编码与 SHA-256，不用歧义分隔符拼接。SDK Request-Id 只作跟踪，不承担幂等。

至少需要以下唯一约束；实际索引命名在实现确定：

```text
billing: UNIQUE(organization_id, order_kind, idempotency_key)
billing: UNIQUE(commercial_order_id) on top-up payment attempt
billing: UNIQUE(provider, environment, merchant_id, out_trade_no)
money:   UNIQUE(provider, environment, merchant_id, provider_trade_no)
money:   UNIQUE(payment_id) and UNIQUE(commercial_order_id) on top-up binding
money:   UNIQUE(refund_id), UNIQUE(provider, environment, merchant_id, out_request_no)
```

跨 app 的同一渠道交易也不能再次领取：app_id 是必须匹配的业务字段，不通过把 app_id 加入交易唯一键来扩大可入账次数。payment_id 可由渠道交易的规范身份生成固定长度 digest，并保存独立原字段以便核验。

订单 fingerprint 绑定 org、initiator、kind、金额/币种、选定 merchant/app/environment、支付产品、到期时间及业务政策版本。同键重放先读取原订单的服务端冻结字段，不按当前时间或当前商户配置重新生成期限/政策；再核对客户端原始意图。前台同键同意图重放，同键异意图冲突；原支付尝试的 readback 必须从原订单重算期望 fingerprint，而非相信 adapter 回显输入。

### 4.3 金额规则

内部 `int64` 分，HTTP 金额为十进制字符串。充值输入只是用户意图，由服务端按 D3 的规则核准后冻结。渠道使用元字符串时，以整数运算转换，例如 `12345 分 ↔ "123.45"`；禁止 float、隐式舍入、科学计数法、负数和溢出。

以该产品的订单 `total_amount` 核对订单面值，不用 `buyer_pay_amount`、`receipt_amount` 或渠道手续费净额冒充同一个金额。优惠/手续费如何影响钱包面值必须由 D3 决定。未获准的金额差异进入核对，不篡改原订单或丢弃付款证据。

国内 Page Pay 的 CNY 是选定支付产品与受控配置的约束；某响应没有 currency 字段时，记录“由固定国内 CNY 产品确定”，不假称回调明确返回了币种。实际返回其他币种或来源产品不明则拒绝自动入账。[S2][S3]

## 5. 持久化模型与事务边界

### 5.1 billing 侧：同一商业订单的支付子记录

`TopUpPaymentAttempt` 的最小逻辑字段：

```text
attempt_id, commercial_order_id, organization_id, initiator_user_id
provider, environment, merchant_profile_id/version, merchant_id, app_id
payment_product, out_trade_no, currency, amount_minor
request_fingerprint, policy_version, expires_at
phase, close_requested_at, admitted_at
verified_payment_evidence_id, money_receipt_id, completion_fingerprint
next_check_at, retry_count, last_safe_error
version, claim_owner, claim_token, claim_until, created_at, updated_at
```

merchant profile 绑定逻辑商户与环境；密钥不是快照字段，只保存安全配置引用。密钥轮换不能改变原 attempt 的 merchant/app/environment。checkout URL 不作为永久凭证；需要保存时私密、短期、no-store，不出现在普通日志。

可信通知的 durable inbox 同属 billing 的支付子记录，保存渠道事件 ID、语义 digest、attempt 关联、最小规范化证据、验签配置版本及处理进度。验签后的未知订单可留为受限异常记录，不伪造 Organization。

事件去重与资金去重分开：native `notify_id` 只用来识别通知；不同 notify_id、通知与查单仍可能指向同一付款。相同事件身份、不同关键业务字段须报警核对，签名/投递时间差异不作为第二次入账依据。

### 5.2 money 侧：复用事实，补足精确结果

复用 accepted payment、OrganizationTopUpSettlement、钱包 bucket、entries 与 reversal。增加或扩展 immutable `TopUpPostingReceipt`：

```text
receipt_id, operation_id, payment_id, commercial_order_id, organization_id
provider_binding_fingerprint, request_fingerprint, currency, gross_credit_minor
available_added_minor, debt_repaid_minor, credit_entry_ids, posted_at
known_reversal_receipt_ids, result_fingerprint
```

纯充值原始步骤满足 `available_added_minor + debt_repaid_minor = gross_credit_minor`。已知退款的后续冲正另有 receipt，不能改写原充值 receipt。完成后余额可能继续变化，receipt 不因余额变化而改变；全额用于偿债仍是成功入账，不要求可用余额必须增加。

### 5.3 事务划分

| 事务 | 持久化内容 | 失败处理 |
| --- | --- | --- |
| B1：billing 创建 | 订单 + attempt + 幂等结果 + initiator/org 绑定 | 提交前不产生付款入口；响应丢失按原 key 查 |
| B2：billing admission | live permission 后，CAS 写入 checkout 已准入及原输入 | CAS 失败不产生新 checkout；重读原 attempt |
| B3：billing 收证 | 已验签、已核对的规范证据 + inbox 接收结果 | 未可靠落盘不确认通知；不直接改余额 |
| M1：money 接受并入账 | 唯一渠道 claim、accepted payment、top-up binding、钱包/流水、原入账 receipt；合并已知冲正 | 同 owner 的有界事务；失败整体回滚，未知按原身份读回 |
| B4：billing 完成 | 校验 M1 的精确 receipt，原订单 PENDING → FULFILLED，同时写 PaymentID 与 receipt 引用 | 失败只重放 B4，不再次加余额 |

M1 是建议新增的 money 用例 `AcceptAndPostProviderTopUp`，不是直接串行调用若干公共方法后假称原子。实现应复用现有事务内部算法和表；不得嵌套多个各自提交的公共方法。没有跨 billing / money / 支付宝的数据库事务，也不在持有 DB 锁时进行网络请求。

money 锁顺序固定为原 payment/source claim → 原退款资金保留记录（若有）→ 企业钱包；所有本批次写入口使用同一顺序。唯一约束冲突或提交不确定，使用新事务按原身份核验，不能把 DB 错误当作业务拒绝。

若退款已知但原入账尚未完成，M1 在同一事务内记录原付款/充值与已知冲正，提交最终余额，不暴露明知已退回仍可消费的中间余额。若外部冲正在入账后才获知，则按既有 debt-first 冲正处理；不声称能消除信息延迟。

## 6. 状态与完成证明

**商业订单继续使用当前三态；恢复细节在 billing 自有 attempt。** 下表是拟新增 attempt phase，不是已有 OrderStatus：

| attempt phase | OrderStatus | 含义 / 允许动作 |
| --- | --- | --- |
| CREATED | PENDING | intent 已提交，未准入付款入口 |
| AWAITING_PAYMENT | PENDING | checkout 已准入/可能发出；尚无付款证明 |
| RECONCILIATION_REQUIRED | PENDING | 支付/关单/证据结果待核实；保存具体 recovery step |
| PAID_PENDING_CREDIT | PENDING | 已有匹配的可信付款证据；只有入账与 readback 可继续 |
| COMPLETED | FULFILLED | exact money receipt 已确认并绑定到订单 |
| CLOSED_UNPAID | CANCELLED | 有充分未支付结束证据；不携带订单 PaymentID |

unknown phase 不删除已获知的付款、入账或退款证据。若 M1 已提交但 B4 未完成，仍可保留 PAID_PENDING_CREDIT 或恢复标志，下一次读原 money receipt 收敛。

`FULFILLED` 必须同时证明：原订单/企业/金额/币种/渠道身份匹配；canonical payment accepted；exact wallet posting receipt 匹配；result fingerprint 正确。仅 HTTP 200、GoPay 未返回 error、支付宝 `code=10000`、二维码/URL、浏览器 return 都不能替代这些证明。

成功仅指商户系统接受了该产品的付款事实并完成钱包记账，不等于银行最终清算或不可退款。[S3][S4]

## 7. 前台付款与通知/查单协议

### 7.1 创建与 checkout

1. 浏览器提交充值意图；Go 入口验证 live permission、verified Organization、请求大小、金额规则和 Idempotency-Key。
2. B1 原子创建/重放订单与唯一 attempt，不返回可以自选商户、任意回跳或任意金额的接口。
3. 用户请求 checkout 时重新验证本企业权限。首版仅原 initiator 重新获取其付款入口；其他获准企业管理员可按 read policy 查看订单，但不改变原 actor。
4. B2 以版本 CAS 固定 admission，拒绝已关闭、过期、有付款证据或已发出关闭请求的 attempt。
5. adapter 使用冻结的业务参数调用 GoPay 生成付款跳转；同一 attempt 只能使用相同 out_trade_no、金额、商户和**绝对到期时间**。不能重放时延长支付窗口。
6. 返回结果丢失时仍保留“入口可能发出”的事实。恢复进程只查单/关单/入账，不自动替撤权用户生成新的付款入口。
7. return_url 只指向本系统固定页面。回来后重新读取订单；URL 中的状态和金额都不是到账依据。

Page Pay 的签名 URL 可能在浏览器打开后才在渠道产生交易。`TRADE_NOT_EXIST` 因此不证明此前从未发出付款能力；也不能单凭此结果取消并重建订单。有效期和关单策略必须覆盖这种情况。[S1][S2]

### 7.2 通知验签与接收

固定支付宝 callback endpoint 使用部署配置的商户/环境，不依赖浏览器会话或客户自报凭据。sandbox 和 production 使用分离的入口、密钥配置与数据空间；不因为请求自报 `environment=production` 就切换信任域。

建议上限：请求体 64 KiB、有限字段数和每字段长度；超过限制在解析前拒绝。form-urlencoded 仅解码一次，不将 URL query 与 POST body 合并，重复关键参数直接拒绝；保留全部待验签参数，验签后才转成领域允许字段。

在 adapter 内使用 GoPay 的 `VerifySign` / `VerifySignWithCert`；必须同时检查 boolean 与 error。ParseNotify 只负责解析。验签成功后核对 app_id、seller_id、out_trade_no、trade_no、total_amount、交易状态和相应产品约束，再从原 attempt 取得 org/actor。[S1][S4]

不套用微信 V3 的通知解密流程到支付宝；是否存在加密字段依所选支付宝接口定义处理。不自写 RSA/证书验签，不信任通知提供的任意证书下载地址。

B3 可靠接收后才能返回支付宝协议的纯文本 `success`；这只是通知接收确认，不表示订单已完成。签名/格式失败或 DB 接收失败不能返回假成功。已可靠记录的未知订单或业务冲突可以确认接收并进入受限异常队列，由查单/人工核对处理，不能直接入账。[S1][S4]

### 7.3 主动查单与状态映射

查单必须通过固定 merchant profile 和原 out_trade_no；验证同步响应签名、业务成功码、返回订单/交易身份和金额。通知、主动查单、账单差异核实最终都进入同一个 `RecordVerifiedPaymentObservation` → `ReconcileTopUpOrder` 路径。

| 支付宝观察 | 本系统处理 |
| --- | --- |
| WAIT_BUYER_PAY | 继续待付；不得入账 |
| TRADE_SUCCESS / TRADE_FINISHED | 经完整绑定校验后接受为付款成功候选；两者不是两笔资金 |
| TRADE_CLOSED | 可能是未支付关闭，也可能是支付后全额退款；不得直接推断未付款 |
| TRADE_NOT_EXIST | 保存一次有时间和查询上下文的观察，不当成强终态 |
| 网络错误、业务结果无法分类、验签失败 | unknown / 安全失败；不换身份再付 |

`TRADE_CLOSED` 必须结合原付款事实、支付时间、退款查询及必要账单证据分类。证据不足继续核对；不能因为未收到成功通知而假定从未付款。[S3][S5]

## 8. 取消、到期与迟到付款

本地到期是“停止提供新付款入口”的条件，不是资金终态。取消首先持久化 `close_requested_at`；与 checkout admission 在同一 attempt 的 CAS 下互斥。

尚未准入 checkout 且能证明没有已发出能力时，可直接 CLOSED_UNPAID。已准入/结果未知时，先按原商户订单查询及关单；关单超时仅核实原订单，不替换 out_trade_no。对未建单但可能仍有旧签名 URL 的场景，必须保持绝对到期约束，不能把当前 NOT_EXIST 解释成取消成功。

**建议的迟到付款政策（D5 待确认）**：只要付款真实、精确匹配原 intent 且未重复入账，就按原企业入账，不自动发起额外消费或退款。若先前误分类为 CANCELLED，允许一个专门的 late-payment correction：保留取消审计，在原订单上完成核实和精确入账后修正为 FULFILLED，标明纠正原因。不是开放通用“重放即可复活取消订单”，更不能复活被取消的套餐购买。

一旦出现任何可信付款证据，普通未付款取消路径停止。相矛盾的已关闭/已付款证据不覆盖彼此，进入异常核对。已知退款须按原付款冲正；不能将已退回的款重新作为净充值。

D5 尚未确认时不开放实际付款/取消写入口。实现评审必须明确上述例外转换，而不是隐藏在通用 `UpdateOrder` 里。

## 9. 支付事实与推广收益的接缝

当前 `PaymentSettlement.Validate()` 要求非空本系统 `PayerUserID` 且 `CommissionableAmountMinor > 0`。充值可能由不同外部付款人付款，也可能明确不计佣；直接填 0 会失败，随便填一个 actor 或正佣金则会产生错误经济归属。[R3]

建议作 money 合同的有界扩展，而不是建立另一个支付事实表：

```text
payment_purpose = WALLET_TOP_UP
commission_treatment = NON_COMMISSIONABLE | COMMISSIONABLE
payer_binding = UNATTRIBUTED_EXTERNAL | VERIFIED_INTERNAL_USER
```

这是待批准的类型方案，不是当前字段。原已有受控/referral settlement 语义保持；新 top-up 必须显式标注用途和 treatment。D4 确认不计佣后，NON_COMMISSIONABLE 必须允许且要求 `CommissionableAmountMinor = 0`，下游不创建 earning；UNATTRIBUTED_EXTERNAL 不填假本系统用户。InitiatorUserID 仍保存在原充值 intent/audit。

若 D4 选择计佣，先冻结真实经济主体归属和金额计算规则再开放；不把“操作人发起充值”自动视为“该操作人本人完成付款”。SDK 不决定佣金、推荐关系或用户映射。

已接受的 payment/refund 仍可沿既有 observer 单向投影。observer 失败不得撤销已收到的钱或让资金二次入账；如有必须交付的下游通知，与 money 事务持久化投递标记/复用已有可靠投递方式，然后幂等消费。首版不新建通用事件总线或佣金平台。

## 10. 入账结果与 readback 契约

以下是拟新增的领域本地窄接口语义，不是已经实现的 SDK API：

| 接口 | Owner | 输入与结果要点 |
| --- | --- | --- |
| `BuildCheckout` | billing port / Alipay adapter | 原 attempt 的固定商户/环境/订单/金额/期限 → 有界跳转描述；不宣告已支付 |
| `QueryPayment` / `ClosePayment` | 同上 | 原渠道身份 → 已验证规范观察 / 关单结果；unknown 单独表达 |
| `RecordVerifiedPaymentObservation` | billing | 只供可信运行装配调用的已验证证据 → durable 接收回执 |
| `AcceptAndPostProviderTopUp` | money | 精确订单绑定 + 可信付款证据 + 明确经济政策 → immutable posting receipt |
| `ReadTopUpPosting` | money | org / order / payment → 原 immutable receipt；不得返回别的企业或仅返回余额 |
| `ReconcileTopUpOrder` | billing | 原 org / order → 继续查询、原身份入账或完成订单 |
| `PrepareTopUpRefund` / `ConfirmTopUpRefund` / `ReleaseTopUpRefundHold` | money | 原付款、退款身份、金额、授权证明 → 资金保留/确认/释放回执 |
| `Refund` / `QueryRefund` | Alipay adapter port | 原付款 + 稳定 out_request_no + 金额 → 规范退款观察 |

money 入口不能接受浏览器传入的 `verified=true`。可信 evidence 类型及调用者由受控装配形成；代码依赖和实际调用测试同时守住边界，不能只靠 DTO 名字。

入账命令与读取结果都必须携带并核对 org、order、payment、merchant/environment、amount/currency 和 request/result fingerprint。缺失、错配、金额不等或其他订单的 receipt 一律不能完成订单。

## 11. 退款与外部冲正

### 11.1 主动原路退款：先保留资金，再调用渠道

首版建议只提供经批准的后台/support 用例，不开放新的 tenant 自助退款 API。退款资格和审批人由 D5 明确，不能推定 `wallet_topup` 权限包含退款或实际转账授权。

1. billing 持久化 refund intent，绑定原 payment/order/org、金额、批准主体、原因及稳定 out_request_no。
2. money 在原 payment 锁下检查累计已确认退款 + 未决退款保留不超过可退金额；再锁钱包，要求 debt 为 0 且 available 足够，原子移动 available → refund-reserved 并产生 immutable hold。
3. 只有成功取得该 hold 的原退款 intent 才可调用 GoPay `TradeRefund`。网络调用在事务外，不能把 `10000` / 受理成功一概当成已退到账。
4. 成功或响应不明时，按原 out_request_no 查询/重放。unknown 保留资金，不重建退款单，不释放 hold。
5. 退款查询的 `code=10000` 仅表明查询成功；需要核对 `refund_status=REFUND_SUCCESS` 及原交易/退款请求号/金额等证据。[S6] 权威成功后，money 同一事务记录 RefundSettlement、确认对应 hold 的 reserved 减少、写退款结果与冲正凭证。该主动退款**不能再通过普通 available 扣减重复扣一次**。
6. 只有明确终局拒绝且可以证明原退款不会继续执行时，才释放该 hold，释放资金优先还债；超时/NOT_EXIST 单次观察不充分。

`refund-reserved` 是现有 `reserved_minor` 中带退款 purpose 的保留子记录，不建立第四套并列余额。现有 purchase reservation 不能冒充退款保留：其 purpose、唯一身份和消费完成证明不同。新增窄退款 purpose/记录与 ledger entry kinds 归同一 money owner，复用余额算法；购买入口必须拒绝退款保留，退款入口必须拒绝购买保留。必要增量不扩为通用资金调拨框架。

### 11.2 外部退款/拒付

来自商户后台或渠道的退款可能没有本地 hold。确认其真实原付款绑定后，复用 `ApplyTopUpReversal`：扣可用余额，不足记 debt，之后入账与释放优先偿债。外部冲正不因为用户余额不足、被撤权或订单已完成而被丢弃。[R3][R4]

同一退款由通知、查单或人工查证多次发现，必须映射同一 canonical refund identity。已有本地 hold 的退款走 hold-confirmation，不能再按无 hold 的外部冲正扣 available。部分退款、累计金额及与拒付的覆盖关系要由 accepted facts 核对，不能简单把渠道累计退款总额当成每次新增退款。

原付款和已完成充值仍保留，退款是独立事实，不将 FULFILLED 改成“从未支付的 CANCELLED”。首版不处理新提现、分账或自动赔付；无法可靠识别的退款保持受限核对，不伪造退款成功。[S6]

## 12. 自动恢复、账单核对与资源边界

唯一业务协调者是 billing 的充值恢复用例；不用 SDK 自动重试另建第二业务恢复 owner。

建议初始运行参数（工程默认值，可经受控配置调整，不是支付渠道保证）：应用成功组装后立即扫描；其后约每 30 秒扫描；每批最多 50 条，并发最多 4；单次渠道请求最长 10 秒，服从更短的父 deadline；退避带抖动，上限 15 分钟。

选择 `next_check_at <= now` 的未终结 attempts、PAID_PENDING_CREDIT、待处理 inbox 和未决退款。按到期时间加稳定 ID 排序推进游标，不能让同一条异常永久占据前 50 条。连续失败保留记录并告警；告警阈值不是删除或宣告失败阈值。

使用现有 DB claim/CAS 或窄 lease 执行；网络期间不持锁。claim 过期只能重查/重放原幂等身份，不能保证旧进程没有执行远端操作；旧 claim 的回写必须被版本/token 拒绝。原已成功的资金效果由 immutable claim/receipt 防重。

恢复循环绑定长生命周期 runtime context，在 app 停止时取消并有界等待；不得误用短期 startup context。既有账户/订阅恢复机制可复用调度叶能力，但本领域的候选选择与结果分类仍只归 billing。

日账单核对是资金运营的第二证据入口：首次可由受控作业取得所选商户和环境的账单，核对交易号、订单、金额与退款；差异先触发同一查单/接收路径，不直接改余额。不能在失败时从任意下载 URL 拉取内容；固定渠道来源、大小/解压上限与下载超时。不为此先建 BI 或完整对账平台。[S1]

生产开放前必须具备未入账付款、未决退款、身份/金额冲突的可追踪处理入口。精确恢复身份、资金 receipt 和必要审计不使用短期缓存代替，也不因通知重试结束而删除。

## 13. HTTP / BFF / Console

复用现有 Workbench commercial API 和权限，不另建一个支付工作台。下列新 route 是设计提案，实施时核对现有路由与 BFF：

```http
POST /api/v1/workbench/commercial/wallet/top-up-intents
POST /api/v1/workbench/commercial/wallet/top-up-intents/:order_id/checkout
POST /api/v1/workbench/commercial/wallet/top-up-intents/:order_id/cancel
GET  /api/v1/workbench/commercial/orders/:order_id
GET  /api/v1/workbench/commercial/wallet
GET  /api/v1/workbench/commercial/wallet/entries
POST /api/v1/payment-notifications/alipay
```

所有用户写入口使用当前 same-origin/CSRF、live grants、Idempotency-Key 与 expected version；Organization 来自 verified context，不来自 body。充值 intent 可以提交期望金额字符串，服务端核准后才成为 authority；不接收 PaymentID、商户密钥、回调 URL 或可任意改写的 SDK BodyMap。

外部 notify 是独立的渠道认证入口，不套用户 cookie/CSRF，也不向它开放普通 admin 能力。必须保留其签名验证、请求限制、可信装配和 durable acceptance。

UI 展示至少区分“待支付”“正在核实付款”“已付款、入账处理中”“充值完成”“已关闭”“需要人工核对”；退款另列。列表与详情由 billing 的同一 attempt/order 投影生成，不让浏览器维护第二状态机。订单仍 PENDING 时可以显示正在核对，不谎称未付款。

付款跳转只允许本配置对应的支付宝 HTTPS 网关，不接受浏览器自定义链接/HTML；回跳固定同源。BFF no-store、固定 upstream、受限 JSON、取消/隔离旧响应；企业切换或撤权后旧订单不能污染新企业视图。原企业资金处理继续，不等于原用户仍可读取。

SDK/渠道不可用、配置缺失、金额不合法、无权、幂等冲突、资金结果未知分别投影；unknown 返回稳定 order reference，并引导核对原订单，不提示盲目“再付一次”。

## 14. GoPay 接入约束

本设计核对 `v1.5.123` 作为固定评估基线；未来实际安装前重新核查 tag、依赖图和安全情况。不得使用浮动 main/@latest 代替可追踪版本，也不为了这个文档改 go.mod。SDK 文档中的示例签名可能与实际函数参数演进不同，代码实现以锁定 tag 源码和编译为准。[S1][S7]

第一版建议使用 `alipay` 的现有 Page Pay / query / close / refund 组合；不无理由同时使用 OpenAPI V2 与 V3 两套协议。同一 attempt 的协议版本及验证配置固定；若改用 V3，先核对应产品与通知语义再调整 adapter，不让协议差异进入 money。

| 能力 | GoPay 入口（需在实施时绑定精确源码签名） | 本项目额外责任 |
| --- | --- | --- |
| 网站付款 | TradePagePay | 持久 attempt、准入、绝对期限、合法跳转 |
| 查单 / 关单 | TradeQuery / TradeClose | 原身份、可信状态、unknown 与取消竞争 |
| 通知 | ParseNotifyByURLValues + VerifySign / VerifySignWithCert | 有界单次解析、签名配置、商户/app/订单/金额校验、durable ACK |
| 退款 | TradeRefund / TradeFastPayRefundQuery | 资金保留、原请求号、累计限额、确认/释放 |
| 账单 | DataDataServiceBillDownloadUrlQuery | 下载边界、受控核对、差错归原 owner |

公钥模式与证书模式按已批准商户配置二选一，不在未知证书错误时降级为“不验签”。同步查单/退款等响应必须验签；GoPay 文档的证书自动验签需要显式配置，公钥模式应使用其相应同步验签能力。Page Pay 返回付款描述与支付结果通知分开，不能虚构不存在的支付成功同步响应。[S1]

始终启用 TLS 证书验证，生产关闭敏感 Debug；限制请求/响应大小和出站超时。配置包含秘密引用，不把私钥、完整通知、买家账号、签名 checkout URL 或认证头写入 Issue、Git、日志与浏览器持久存储。轮换公钥/证书时保留未决交易所需验证能力，不跨环境回退。

## 15. 风险匹配的验收矩阵

下表是本次资金边界的实施义务，不是已经执行通过的测试，也不授权新建专项测试平台。复用仓库现有 unit、PG、race、HTTP/BFF、浏览器与渠道测试能力。

| 场景 | 预期结果 |
| --- | --- |
| 同键重试/不同键并发/响应丢失 | 同键原订单；一个订单一个 attempt；不同真实充值意图不被错误合并 |
| 同一付款多个通知、查单并发 | 一条 canonical payment claim、一次入账和一个原 receipt |
| 签名错、重复参数、超限、错误商户/app/环境/金额 | 无错误入账；安全错误/受限异常证据，无敏感泄漏 |
| webhook 正确但无匹配订单 | 不推断 org、不加余额；可追踪核对 |
| B1/B3/M1/B4 任一提交前后崩溃或 ACK 丢失 | 不新增支付身份，按原 receipt 恢复，不需用户再付款 |
| checkout 尚未访问、查单 NOT_EXIST | 不误判没有发出付款能力；期限不被续长 |
| 取消与付款竞争、CLOSED 包含退款、迟到成功 | 按 §8 分类；不丢付款，不把已退款款项当新净入账 |
| 全部充值用于偿债 | exact receipt 与余额一致，订单能正常完成 |
| 主动退款并发消费、退款 unknown、重复确认 | 先保留，unknown 不释放；不双扣 available 与 reserved |
| 外部退款先于入账、余额已用完、部分退款重复观察 | 已知事实同事务衔接；准确 debt；累计不重复 |
| 当前正佣金 validator 与不计佣充值 | 合同显式扩展；不伪造 PayerUserID/CommissionableAmountMinor；旧消费/收益回归 |
| 用户切企业、退出/撤权、角色不足 | 禁止新越权操作；已发生款项仍归原 org，旧响应不污染新页面 |
| claim 超时、旧 worker 迟到、关闭新支付开关、重启 | 原身份恢复；旧 token 不覆盖新状态；已发生支付/退款继续可核实 |

本地 fixture 只证明本地边界，不证明支付宝真实协议。沙箱须使用被批准的实际支付宝测试产品；没有商户/凭据时标 NOT_RUN。真实小额、生产收款、退款、正式上线分别授权，CI/代码评审不能代替用户验收。

## 16. 实施顺序与评审收敛

同一 #481 Delivery Batch，一个主要分支/PR。当前只交付本草案和索引，不开多个平行实现任务。

1. **M0 文档与高风险边界评审**：核对 D1–D6 的阻塞层级，重点审查身份/经济事实、M1 原子入账、取消/迟到状态、退款保留。当前状态 DRAFT；未给独立 Reviewer 的判断冒名签字。
2. **M1 最小收款链**：已确认产品后实现 intent/checkout、GoPay adapter、通知/查单和 source-bound posting，先形成单渠道用户结果。
3. **M2 同 PR 收敛恢复与退款**：实现原 receipt 恢复、必要退款/冲正、上述直接相关负例。不另造 scheduler 或故障注入平台。
4. **M3 充值中心交付**：消费真实状态和原 wallet/order API，跑“充值 → 钱包另行购买”链；订阅消费沿 #479，不接管其实现。

按 AGENTS 的新增高风险检查与最终交付检查推进；正常架构评审最多两轮。修复只复核相关增量，真正的新 BLOCKER 才重开相应设计，不把文档不断加长当作交付。

### 16.1 与既有合同的明确关系

本草案**保持** #457 的 money/billing/resource ownership、整数金额、debt-first、top-up 无资源 quote/items/reservation、非完成 Order 不带 PaymentID；保持 #480 的订阅资金协议和独立激活 owner。

本草案**提议补充，尚未生效**：payment attempt 子状态、精确 posting receipt、money 非佣金/付款人表示、主动退款 purpose 与 receipt、受控迟到付款纠正。获批时在同一变更中同步相关稳定合同、类型 validator、下游消费者与直接回归，不能只新增文档却让旧合同与新实现互相冲突。未获批前本文件不作为绕过现有 guard 的依据。

### 16.2 当前交付边界

代码路径与文档已作定点核对；没有编译 GoPay、执行渠道接口、建立商户账户、操作数据库、验证用户浏览器、运行整仓 CI 或完成独立架构评审。具体 PR 与实际检查结果写 #481/PR，不把未来验收勾成 PASS。

## 17. 依据与核查范围

仓库依据使用固定代码基线；外部依据为 GoPay 固定 tag 与支付宝官方文档。支付宝文档站部分页面依赖客户端渲染，本轮部分规则来自官方索引摘录及官方 Easy SDK 的公开 API 文档；实施前必须核对所选产品当前完整文档，不能把索引摘录当完整接口测试。

- [R1] [现有订单类型及 validator](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/internal/commercial/billing/contracts.go)。
- [R2] [现有 billing service；充值入口仍 unavailable](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/internal/commercial/billing/service.go)。
- [R3] [money 支付/退款事实与 validator](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/internal/ledger/money/types.go)；[钱包接口](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/internal/ledger/money/wallet.go)。
- [R4] [钱包入账、唯一绑定与事务实现](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/internal/integration/persistence/money/wallet_repository.go)。
- [R5] [现有钱包/商业合同](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/docs/architecture/commercial-wallet-billing-contract.md)；[订阅购买合同](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/docs/architecture/self-service-subscription-purchase-contract.md)；[AGENTS](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/AGENTS.md)。
- [S1] [GoPay v1.5.123 支付宝文档](https://github.com/go-pay/gopay/blob/v1.5.123/doc/alipay.md)。
- [S2] [支付宝电脑网站支付快速接入](https://opendocs.alipay.com/open/270/105899)；[官方 Easy SDK API 文档](https://github.com/alipay/alipay-easysdk/blob/master/APIDoc.md)；[Page Pay 参数与 time_expire](https://opendocs.alipay.com/open/028r8t)。
- [S3] [支付宝交易查询](https://opendocs.alipay.com/open/c676a9b0_alipay.trade.query)；[如何判断交易是否成功](https://opendocs.alipay.com/support/01ray9)。
- [S4] [电脑网站支付异步通知说明](https://opendocs.alipay.com/open/270/105902)。
- [S5] [支付宝交易状态说明](https://opendocs.alipay.com/support/01raw9)；[订单已关闭的原因](https://opendocs.alipay.com/support/01rfti)。
- [S6] [支付宝退款查询及 REFUND_SUCCESS 语义](https://opendocs.alipay.com/open/8c776df6_alipay.trade.fastpay.refund.query)；[官方 Easy SDK 的退款与退款查询合同](https://github.com/alipay/alipay-easysdk/blob/master/APIDoc.md)。
- [S7] [GoPay v1.5.123 发布](https://github.com/go-pay/gopay/releases/tag/v1.5.123)。
