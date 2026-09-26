# 微信支付与支付宝企业钱包充值架构设计

**状态：DRAFT / 待架构评审；不是 IMPLEMENTATION_READY。**  
**关联：#481；复用 #457；订阅消费沿 #479 / #480。**  
**设计日期：2026-09-24。代码核对基线：`c5deccbe7d42e211ab1e2c80987b1e7e67b671f2`。**

本文件沿用原 `alipay-wallet-topup-design.md` 路径以保留评审链接，正文是本批次唯一的双渠道设计，不另维护单支付宝版本。

本文件是本批次的设计草案，不覆盖已批准的钱包、资源购买和订阅合同。本轮只交付文档；不安装 SDK、不改运行代码、数据库或配置、不执行真实付款或退款。独立评审、产品待决事项、实现和生产开放分别记录，不以文档存在代替准入。

## 1. 设计决定与用户结果

**2026-09-24 用户决定：首版同时支持微信支付和支付宝。** 此决定替代此前仅做支付宝、微信后置的范围限制。统一采用 `github.com/go-pay/gopay`，明确两个渠道枚举 `WECHAT_PAY` / `ALIPAY`；不再评估是否需要微信，也不把单渠道完成当作本批次完成。使用 GoPay 的初衷仍是复用支付宝的 Go 接口封装，现在同一依赖也用于微信 APIv3；不建设通用聚合收单平台。

目标用户是当前已验证 Effective Organization 内有 `workbench.commercial.wallet_topup` 权限的企业管理员。目标链路：

```text
充值中心 → 选择微信支付或支付宝 → 创建绑定所选渠道的充值订单
        → 微信扫码 / 支付宝收银台 → 服务端核实付款
        → 同一企业钱包入账或优先偿债 → 订单 / 余额 / 流水可回读
        → 用户另行选择使用钱包购买套餐或资源
```

充值成功不自动购买、激活权益或重新执行失败的消费。套餐 `EXTERNAL_PAYMENT` 直付仍未开放，#479 的 ZERO_PRICE / WALLET 实现不以本批次为前置。

首版限定 CNY、微信与支付宝两个渠道、每渠道一套直连收款商户配置、用户主动单次充值。只收取本产品费用，不为客户代收或接入服务商子商户。不做 Stripe、自动渠道路由/故障转付、自动续费、自动充值、跨企业转账、提现新业务、分账、发票、多币种、旧账务迁移或完整自助退款工作台。必要原路退款和外部冲正安全包含在设计中。

### 1.1 首发交互方案

| 渠道 | 本文建议的 Web 接入产品 | 付款入口与语义 |
| --- | --- | --- |
| ALIPAY | 电脑网站支付 `alipay.trade.page.pay` | GoPay `TradePagePay` 生成付款跳转；浏览器前往支付宝收银台。生成 URL 不等于远端交易已创建或已付款 [S1][S2] |
| WECHAT_PAY | 微信 APIv3 直连商户 Native 支付 | GoPay `V3TransactionNative` 请求渠道下单，取得 `code_url` 后在 Console 渲染二维码，用户使用微信扫码；下单成功/二维码生成不等于付款成功 [S8][S11] |

**两个渠道都属于首版；具体产品形态仍是待商户产品权限确认的技术方案。** 不凭 SDK 可调用推定已经签约。若实际只能使用其他产品，先替换对应渠道的产品契约，不删除另一渠道或运行时盲目降级。首版不同时开发微信 JSAPI/H5/小程序/APP 或支付宝 WAP/APP/扫码直连；Native 不是手机网页内直接拉起支付的承诺，窄屏应准确提示扫码使用方式。

### 1.2 尚待确认但不阻塞文档设计的事项

| 编号 | 待确认项 | 建议 / 未确认时的行为 | 阻塞层级 |
| --- | --- | --- | --- |
| D1 | 两渠道收款主体、支付宝 PID/app、微信 mchid/appid 及企业充值业务资格 | 由用户/商户负责人分别确认，SDK 能调用不代表业务获准 | 对应渠道联调、真实开放 |
| D2 | 两渠道具体支付产品 | 支付宝 Page Pay + 微信 APIv3 Native；双渠道范围已确定，核实的是产品权限 | 对应渠道特定实现准入 |
| D3 | 充值档位或自定义额度、限额、手续费与优惠处理 | 不设默认生产金额；应付和钱包面值建议相等，费用单独处理；两渠道均核对 | 创建真实支付 |
| D4 | 充值推广佣金及业务付款人归属 | 建议首版充值不计佣；不伪造正佣金或付款人 | money 合同变更准入、真实开放 |
| D5 | 迟到付款与主动退款政策、操作权限 | 建议有效迟到付款按原订单入账；退款回原渠道并先保留资金 | 取消/退款实现准入、真实开放 |
| D6 | 两渠道获准测试条件、凭据、回调与配置 | 支付宝使用获准沙箱；微信 APIv3 不假设沙箱，受控替身与获准真实测试分列；小额付款也需授权 | 对应环境联调/开放 |

D3–D5 是产品政策，不由 SDK、Figma 示例或 Reviewer 代替用户决定。未配置或未获准的渠道显示 unavailable；不能为了满足双渠道演示而伪造成功。配置一方不自动开放另一方，关闭新付款能力不能关闭已发生资金事实的读取与恢复。

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
| refund / chargeback | accepted-fact 的两个入口以 CommissionableAmountMinor 限额；钱包冲正另以原充值面值限额 | 按 §11.3 分离本金与佣金，统一退款/拒付累计与 hold 规则，不只放宽零佣金 validator |

`CreditSettledTopUp` 当前可接收不大于付款 gross 的金额，且重放时返回当前钱包快照。这不能自动证明本次充值与订单**精确等额**或返回了原次入账结果；新增边界必须作更严格的对应检查，不能直接把现有返回值当完整 receipt。[R4]

## 3. 责任与代码依赖

| Owner | 唯一责任 | 禁止承担 |
| --- | --- | --- |
| `internal/commercial/billing` | 同一套充值订单、支付尝试、checkout admission、证据引用、完成与恢复；订单固定渠道 | 钱包余额算法、渠道签名、订阅激活 |
| `internal/ledger/money` | 接受付款/退款/拒付事实，交易 claim、企业绑定、保留/入账/冲正、不可变 receipt | 商品定价、渠道 checkout、商业订单编排 |
| `internal/integration/payments/alipay`（拟新增） | GoPay 支付宝调用、form 通知验签、元字符串和状态映射 | 写订单/钱包表、自行安排业务重试 |
| `internal/integration/payments/wechat`（拟新增） | GoPay 微信 APIv3 调用、Native 下单、原报文验签/解密、分金额和状态映射 | 第二套订单/钱包/恢复状态机、权益写入 |
| billing 自有 HTTP + app 装配 | 验证用户入口，固定渠道通知路由，显式构造两个 adapter 并启动/停止恢复循环 | 在 handler 中直接加余额；动态插件/通用支付平台 |
| 现有 BFF / Console | 支付方式选择、跳转/二维码展示、当前企业订单和余额投影 | 浏览器回跳、扫码动画或按钮宣告到账 |

```text
Console → BFF → billing 领域用例 → 本地窄支付 port
                    ↓                   ├─ Alipay adapter → GoPay/alipay → 支付宝
              money 领域用例            └─ WeChat adapter → GoPay/wechat/v3 → 微信支付
                    ↓
              money persistence

两个固定通知 ingress → 各自 adapter 校验 → billing durable 收证 → 同一恢复/入账路径
```

仅对应 adapter 依赖 `github.com/go-pay/gopay/alipay`、`github.com/go-pay/gopay/wechat/v3` 与 `gopay.BodyMap`。领域不接 SDK DTO、`*http.Request`、任意 URL 或自由透传参数。app 静态装配两个明确实现；billing 按持久订单的 provider 选择，不做渠道轮询、自动切换、动态注册、微服务拆分或复制账本。

窄 port 使用 `CreateOrReadCheckout`，显式承认微信分支可能创建远端交易，不命名为纯本地查询。结果是带 discriminator 的 `CheckoutAction`：`REDIRECT` 只含受控支付宝跳转，`QR_CODE` 只含微信二维码载荷，两者互斥；并带原 order/attempt/channel、到期时间和输入指纹。付款成功只能由 verified payment observation 表达。两渠道共享用例与金额模型，不强制把不同协议伪装成相同 HTTP 行为。

持久化复用 `internal/integration/persistence/commercialbilling` 与 `.../money`。共享 billing/money/app 文件与 #479 串行交接；本轮只改设计文档和文档检查，不占用运行源码。

## 4. 业务身份、金额与幂等

### 4.1 三类身份不能混用

- **InitiatorUserID**：创建订单时的本系统已认证用户，写入 immutable intent 与 audit。
- **BeneficiaryOrganizationID**：创建时的 verified Effective Organization，此后不随成员关系变化。
- **Provider buyer identity**：支付宝买家标识或微信 OpenID（带渠道/app 作用域），仅为渠道事实；不等于 InitiatorUserID，也不授予本系统权限。可以由别人付款，是否允许由 D4 明确；不得把支付宝 UID 或微信 OpenID 填进本系统用户 ID 字段，也不把同名外部标识跨渠道合并。

回调按持久化 attempt 查找原企业，不使用当前登录用户、当前 membership、客户端 `attach/passback_params` 或回调任意 org 字段推断归属。支付记录仍属于原企业；撤权只限制新的操作和读权限，不抹去已收到的资金。

### 4.2 身份链

```text
commercial_order_id ↔ 一个 durable payment_attempt_id
payment_attempt_id   → 一个持久化 out_trade_no
provider transaction → 一个 canonical payment_id
payment_id + order   → 一个 money top-up receipt
REFUND + refund_id   → 一个持久化 provider_refund_request_no（支付宝 out_request_no / 微信 out_refund_no）
payment + kind + id  → 一个冲正回执及至多一个差额核对记录
```

首版一个充值订单只生成一个渠道支付尝试。未决时不新增 attempt；明确结束后确有新的充值意图才创建新订单。超时、刷新、网络重试均不等于新的充值意图。

`out_trade_no` 与主动退款请求号建议使用 32 位小写十六进制随机标识，在两个 adapter 分别验证所选产品限制，生成后持久化且永不改变；不拼接企业/用户长标识。业务 fingerprint 使用带版本的确定性编码与 SHA-256，不用歧义分隔符拼接。SDK Request-Id 只作跟踪，不承担幂等。

至少需要以下唯一约束；实际索引命名在实现确定：

```text
billing: UNIQUE(organization_id, order_kind, idempotency_key)
billing: UNIQUE(commercial_order_id) on top-up payment attempt
billing: UNIQUE(provider, environment, merchant_id, out_trade_no)
money:   UNIQUE(provider, environment, merchant_id, provider_trade_no)
money:   UNIQUE(payment_id) and UNIQUE(commercial_order_id) on top-up binding
money:   UNIQUE(refund_id) in accepted refunds; UNIQUE(chargeback_id) in accepted chargebacks
money:   UNIQUE(provider, environment, merchant_id, provider_refund_request_no) on refund requests
billing: UNIQUE(provider, environment, merchant_id, event_type, provider_event_id) on notification inbox
money:   UNIQUE(payment_id, reversal_kind, reversal_id) on top-up reversal receipts
money:   UNIQUE(payment_id, reversal_kind, reversal_id) on excess reconciliation records
```

跨 app 的同一渠道交易也不能再次领取：app_id 是必须匹配的业务字段，不通过把 app_id 加入交易唯一键来扩大可入账次数。payment_id 可由渠道交易的规范身份生成固定长度 digest，并保存独立原字段以便核验。

退款和拒付的原始 ID 属于独立命名空间；`RefundID="r1"` 与 `ChargebackID="r1"` 可以是不同事实。[R3][R6] 本批次在 money 内统一使用以下逻辑键（拟新增类型，复用现有 WalletReversalKind）：

```go
type TopUpReversalKey struct {
    PaymentID  string
    Kind       WalletReversalKind // 仅 REFUND / CHARGEBACK
    ReversalID string
}
```

`Kind` 来自已验证事实类型，不能由浏览器或随意改标签产生新事实；ReversalID 分别取该类型的 canonical RefundID / ChargebackID。上述三元组用于冲正接收/去重、hold 匹配、回执、差额记录、账本 source identity 和恢复查询。保留 accepted facts 原有的每类型 ID 唯一约束：同类 ID 不能改绑另一 payment；不能通过三元组索引放宽该约束。组织、订单、商户/环境和金额仍须与原 payment 绑定校验，不作为额外去重维度扩大入账次数。

钱包现有 reversal 存储主键为字符串，因此 top-up 路径拟使用 `topup-reversal:v1:` 加 SHA-256 十六进制摘要作为 storage ID；摘要输入是带版本、长度前缀的完整 typed key，原三字段分别保留并核验。不能直接使用原始 ReversalID 作为跨类型主键。回执、差额及每种实际账本效果的标识/指纹都包含完整 key；一笔冲正的不同 entry 另有固定 effect-kind 后缀，不能仅按 notify_id、裸 ID 或 `(payment_id, reversal_id)` 防重。既有非 top-up 身份不迁移、不改写。

渠道原始交易/退款/事件号不得直接作为全局 ID。canonical payment/refund/chargeback ID 按各自类型，由版本化编码的 provider/environment/merchant 与原生稳定身份生成，保存原字段及校验映射。微信 out_refund_no/refund_id 与支付宝 out_request_no/渠道退款凭证必须关联到同一原退款，不能把通知 ID 当另一笔退款；身份不足只保留证据核对。typed key 中的 PaymentID 已包含渠道隔离，不能通过切渠道把同一事实重绑定。

订单 fingerprint 绑定 org、initiator、kind、provider、金额/币种、选定 merchant/app/environment、支付产品、到期时间及业务政策版本。同键重放先读取原订单的服务端冻结字段，不按当前时间或当前商户配置重新生成期限/政策；再核对客户端原始意图。前台同键同意图重放，同键异意图冲突；原支付尝试的 readback 必须从原订单重算期望 fingerprint，而非相信 adapter 回显输入。

### 4.3 金额规则

内部 `int64` 分，HTTP 金额为十进制字符串。充值输入只是用户意图，由服务端按 D3 的规则核准后冻结。渠道使用元字符串时，以整数运算转换，例如 `12345 分 ↔ "123.45"`；禁止 float、隐式舍入、科学计数法、负数和溢出。

支付宝以订单 `total_amount` 核对订单面值，不用 `buyer_pay_amount`、`receipt_amount` 或渠道手续费净额冒充同一个金额。优惠/手续费如何影响钱包面值必须由 D3 决定。未获准的金额差异进入核对，不篡改原订单或丢弃付款证据。

微信 Native 的 `amount.total` 是整数分，核对 `amount.currency=CNY`，不能乘除 100 后再作为分入账；`payer_total` 不替代订单总金额。支付宝才使用元十进制字符串转换，D3 未批准的优惠差异不能隐式抹平。[S8][S9]

国内 Page Pay 的 CNY 是选定支付产品与受控配置的约束；某响应没有 currency 字段时，记录“由固定国内 CNY 产品确定”，不假称回调明确返回了币种。实际返回其他币种或来源产品不明则拒绝自动入账。[S2][S3]

### 4.4 双渠道绑定与切换

浏览器在创建 intent 前选择 `ALIPAY` 或 `WECHAT_PAY`，服务端验证该渠道 capability 后冻结 provider；商户/app/密钥由服务端固定配置决定。**同一幂等键换渠道必须冲突**：不把 provider 加入创建幂等键的唯一约束来生成第二张订单。一个订单始终一个 attempt、一个渠道，不允许同时出现微信二维码和支付宝可付款链接。

订单创建后不提供修改 provider 的 API。用户需要换方式时，先完成原订单安全关闭；旧链接/二维码、在途 Native 下单或关单结果仍不确定时，拒绝自动切换渠道并核对原订单。只有确认原尝试不可再支付，才由用户显式创建另一渠道的新订单，不重用旧幂等键。一次 NOT_EXIST、前端倒计时结束或换 tab 都不是关闭证明；不得自动故障转付。

退款、查单、关单、对账及重启恢复固定使用原渠道和原 merchant profile，不使用用户当前选择或系统默认渠道。原渠道不可用时保持可追踪 pending/unknown，不能用另一渠道退款或生成第二笔付款。不同真实充值意图在不同订单分别付款时，应各自入账；不能只因金额/字符串交易号相同误去重。错误渠道对原订单的真实来款进入受限异常核对，不丢证据、不直接多记余额。

## 5. 持久化模型与事务边界

### 5.1 billing 侧：同一商业订单的支付子记录

`TopUpPaymentAttempt` 的最小逻辑字段：

```text
attempt_id, commercial_order_id, organization_id, initiator_user_id
provider, environment, merchant_profile_id/version, merchant_id, app_id
payment_product, out_trade_no, currency, amount_minor
request_fingerprint, policy_version, expires_at
phase, close_requested_at, admitted_at
checkout_action_kind, checkout_payload_secret_ref, checkout_created_at
verified_payment_evidence_id, money_receipt_id, completion_fingerprint
next_check_at, retry_count, last_safe_error
version, claim_owner, claim_token, claim_until, created_at, updated_at
```

merchant profile 绑定逻辑商户与环境；密钥不是快照字段，只保存安全配置引用。密钥轮换不能改变原 attempt 的 merchant/app/environment。checkout URL/code_url 不作为到账凭证；以任务私密存储保存有效期内的必要重放载荷，不出现在普通日志。微信 code_url 成功响应必须按原 admission token 和请求指纹持久化后才可返回；过期后不再交付付款能力，支付身份和完成证据仍保留。

可信通知的 durable inbox 同属 billing 的支付子记录，保存 provider/environment/merchant/event_type 范围内的渠道事件 ID、语义 digest、attempt 关联、最小规范化证据、验签配置版本及处理进度。验签后的未知订单可留为受限异常记录，不伪造 Organization。

事件去重与资金去重分开：支付宝 `notify_id`、微信通知 `id` 只用来识别本渠道通知；不同 notify_id、通知与查单仍可能指向同一付款。相同事件身份、不同关键业务字段须报警核对，签名/投递时间差异不作为第二次入账依据。

### 5.2 money 侧：复用事实，补足精确结果

复用 accepted payment、OrganizationTopUpSettlement、钱包 bucket、entries 与 reversal。增加或扩展 immutable `TopUpPostingReceipt`：

```text
receipt_id, operation_id, payment_id, commercial_order_id, organization_id
provider_binding_fingerprint, request_fingerprint, currency, gross_credit_minor
available_added_minor, debt_repaid_minor, credit_entry_ids, posted_at
known_reversal_receipt_ids, result_fingerprint
```

纯充值原始步骤满足 `available_added_minor + debt_repaid_minor = gross_credit_minor`。已知退款的后续冲正另有 receipt，不能改写原充值 receipt。完成后余额可能继续变化，receipt 不因余额变化而改变；全额用于偿债仍是成功入账，不要求可用余额必须增加。

冲正 receipt 必须显式保存完整 `payment_id、reversal_kind、reversal_id`，并将三者纳入 request/result fingerprint；差额记录与 receipt 的关联也使用完整 typed key，不能只关联裸字符串 ID。还须分开保存 `provider_amount_minor`（完整已确认金额）、`wallet_principal_effect_minor`、`excess_provider_minor`、原 hold 身份及其 consumed/released/debt-repaid 分解、关联差额核对记录和 result fingerprint。accepted refund/chargeback 保留原金额，不能截断成 wallet effect；累计钱包作用与超本金差额按 §11.3 分配。差额核对记录属于同一 money owner 的受限事实，不是第二钱包或新的可用额度。

### 5.3 事务划分

| 事务 | 持久化内容 | 失败处理 |
| --- | --- | --- |
| B1：billing 创建 | 订单 + attempt + 幂等结果 + initiator/org 绑定 | 提交前不产生付款入口；响应丢失按原 key 查 |
| B2：billing admission | live permission 后，CAS 写入 checkout 已准入及原输入 | CAS 失败不产生新 checkout；重读原 attempt |
| B3：billing 收证 | 已验签、已核对的规范证据 + inbox 接收结果 | 未可靠落盘不确认通知；不直接改余额 |
| M1：money 接受并入账 | 唯一渠道 claim、accepted payment、top-up binding、钱包/流水、原入账 receipt；合并已知冲正 | 同 owner 的有界事务；失败整体回滚，未知按原身份读回 |
| B4：billing 完成 | 校验 M1 的精确 receipt，原订单 PENDING → FULFILLED，同时写 PaymentID 与 receipt 引用 | 失败只重放 B4，不再次加余额 |

M1 是建议新增的 money 用例 `AcceptAndPostProviderTopUp`，不是直接串行调用若干公共方法后假称原子。实现应复用现有事务内部算法和表；不得嵌套多个各自提交的公共方法。没有跨 billing / money / 支付渠道的数据库事务，也不在持有 DB 锁时进行网络请求。

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

1. 浏览器提交金额意图、所选渠道；入口验证 live permission、verified Organization、capability、金额规则和 Idempotency-Key。
2. B1 原子创建/重放订单与唯一 attempt，绑定 provider 和固定 merchant profile，不允许客户端指定商户、期限或通知 URL。
3. 获取 checkout 时重新验证本企业权限，首版仅原 initiator 获取付款能力；其他有 read 权限的企业管理员可查看原订单，但不改变 actor/provider。
4. B2 以版本 CAS 持久化 admission，拒绝已关闭、过期、有付款证据或已发出关闭请求的 attempt。保存原请求指纹与派发资格后，才能生成/派发渠道请求。
5. 支付宝分支生成 `REDIRECT`；微信分支在事务外以原 out_trade_no 调用 Native 下单，验签成功响应后持久化 `QR_CODE` 载荷。两者均使用原金额、商户和**绝对到期时间**，不能在重放时延长支付窗口。
6. 相同 attempt 的重复 checkout 返回仍有效的原结果。若首次请求可能发出但结果未持久化，先查原单；有付款证据即转入账，无充分证据保持 RECONCILIATION_REQUIRED，不换渠道/身份。仅在锁定产品契约已证明原单同参数安全重放、且原 dispatch admission 仍有效时，允许取回原下单结果；该行为必须有对应渠道测试，不能只依赖 GoPay 返回 error 的分类。
7. 已付/已关/过期后不再返回可付款载荷；旧 claim 结果不能覆盖新终态。恢复循环只核实原效果，不替已撤权用户生成新的付款能力。浏览器轮询读取原订单，不每次轮询调用 Native 下单。

Page Pay 的签名 URL 可能在浏览器打开后才产生远端交易；微信 Native 下单本身是远端写请求，响应丢失可能已创建交易。两者都不能用一次 NOT_EXIST 判定从未发出能力。[S1][S2][S8][S11] 本轮不承诺可从查询接口取回丢失的 code_url；无法安全恢复时保留原身份核对/关闭，不生成替代付款。

支付宝 return_url 只指向固定同源页面；返回后重新读取订单。微信 Native 不依赖浏览器 return_url，扫码后的到账同样只看服务端事实。二维码展示、扫描或收银台返回都不是入账证明。

### 7.2 通知验签与接收

两个固定 callback endpoint 分别绑定渠道及获准环境/merchant profile，不从 body/query 或尝试另一套密钥来选择渠道。支付宝沙箱与生产隔离；微信受控测试组合与实际渠道组合隔离，不能自报环境切换信任域。错误渠道的有效签名也不能为原订单入账。

**支付宝：** 建议 body 上限 64 KiB，并限制字段数/单字段长度；form-urlencoded 只解码一次，不合并 URL query 与 POST body，重复关键参数拒绝。保留完整待验签参数，使用 GoPay `VerifySign` / `VerifySignWithCert` 并检查 boolean/error；随后核对 app_id、seller_id、out_trade_no、trade_no、total_amount、状态与原 attempt。ParseNotify 不等于验签。[S1][S4]

**微信：** 建议外层 body 上限 2 MiB、解密后 JSON 上限 1 MiB，并限制结构深度/字段规模；这是本项目接收边界，不声称 SDK 默认值。保留原始 body，按 `Wechatpay-Serial` 对应的受信平台公钥/证书验证 `Wechatpay-Timestamp`、`Wechatpay-Nonce`、签名和原始字节，不能先重排 JSON 再验签。未知序列号、重复安全关键头/字段、签名失败均 fail closed；密钥加载来自受控配置，不接受回调提供的下载地址。[S9]

微信验签通过后，使用该配置的 APIv3 密钥与 GoPay 的解密能力处理 `AEAD_AES_256_GCM` resource，异常 nonce、算法、密文或字段安全拒绝。只分派批准的支付/退款事件类型，再核对 mchid/appid（接口实际返回时）、out_trade_no、transaction_id、金额/币种及原绑定；接口未返回的字段必须由受信调用上下文与已存原交易证明，不伪造返回字段。成功解密不代替签名或业务绑定。支付宝不套用这套 JSON/GCM 流程。[S8][S9][S10]

两者均**先可靠落盘再 ACK**：B3 提交已验证的规范证据/接收结果后，支付宝返回纯文本 `success`；微信返回 `HTTP 204`、空 body。微信官方接受 200/204，此设计固定使用 204，不复用支付宝文本回执。微信应答预算应满足官方 5 秒要求，B3 不等待 M1 入账或外部查单；失败返回相应非成功响应，不先成功应答再只放内存队列。[S4][S9][S10]

ACK 表示可靠接收，不表示 FULFILLED。未知订单/业务冲突若已作为受限异常可靠保存，可以确认接收并由同一恢复路径核查，不能猜 org 加余额。支付和退款事件的去重使用渠道命名空间；不同 notify_id/id 仍可能是同一资金效果。

### 7.3 主动查单与状态映射

查询、关单按原 provider/merchant profile/out_trade_no，验证响应签名、接口结果、交易身份/金额。两个 adapter 将渠道观察规范化，再进入同一个 `RecordVerifiedPaymentObservation` → `ReconcileTopUpOrder`，保留原状态供审计，不让渠道枚举直接驱动 wallet。

| 渠道观察 | 本系统处理 |
| --- | --- |
| 支付宝 WAIT_BUYER_PAY；微信 NOTPAY / USERPAYING | 继续待付/处理中；不得入账 |
| 支付宝 TRADE_SUCCESS / TRADE_FINISHED；微信 SUCCESS | 完整绑定后形成付款成功候选；多个成功通知不是多笔资金 |
| 支付宝 TRADE_CLOSED | 可能是未付款关闭或付款后全额退款；结合原付款/退款证据分类 |
| 微信 CLOSED / REVOKED / PAYERROR | 按该产品的明确终态及原请求核实；不能覆盖已知付款事实，不把订单关闭当成退款到账 |
| 微信 REFUND | 转退款核对，不能把这一状态本身当作新的成功充值或精确退款金额 |
| 查询不存在、网络错误、无法分类、验签失败 | 单次观察/unknown/安全失败；不换渠道或订单再次付款 |

支付宝 TRADE_CLOSED 与微信支付订单状态、微信退款 CLOSED 分别处理。证据不足继续核对，不能因缺少成功通知就认定从未付款。可信的支付/退款事实保留，旧待付观察不回退已确认状态。[S3][S5][S9][S10]

## 8. 取消、到期与迟到付款

本地到期是“停止提供新付款入口”的条件，不是资金终态。取消首先持久化 `close_requested_at`；与 checkout admission 在同一 attempt 的 CAS 下互斥。

尚未准入 checkout 且能证明没有已发出能力时，可直接 CLOSED_UNPAID。已准入/结果未知时，先按原商户订单查询及关单；关单超时仅核实原订单，不替换 out_trade_no。对未建单但可能仍有旧签名 URL 的场景，必须保持绝对到期约束，不能把当前 NOT_EXIST 解释成取消成功。

**建议的迟到付款政策（D5 待确认）**：只要付款真实、精确匹配原 intent 且未重复入账，就按原企业入账，不自动发起额外消费或退款。若先前误分类为 CANCELLED，允许一个专门的 late-payment correction：保留取消审计，在原订单上完成核实和精确入账后修正为 FULFILLED，标明纠正原因。不是开放通用“重放即可复活取消订单”，更不能复活被取消的套餐购买。

一旦出现任何可信付款证据，普通未付款取消路径停止。相矛盾的已关闭/已付款证据不覆盖彼此，进入异常核对。已知退款须按原付款冲正；不能将已退回的款重新作为净充值。

微信必须同时核实原 Native 下单是否仍在途与原渠道关单结果；code_url 不再显示不是远端不可支付的证明。换渠道遵守 §4.4，不用本地取消来绕过未知远端效果。

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

**零佣金不等于零可退本金。** 当前 `RecordRefundSettlement` 与 `RecordChargebackSettlement` 都以 `payment.CommissionableAmountMinor` 减去累计退款和拒付作为剩余额度；仅修改 payment validator 会导致零佣金充值的全部冲正被拒绝。[R6] 本批次必须同时落实 §11.3 的新退款准入预算、完整已确认事实、钱包本金作用上限与未决 hold 规则；真实已确认事实的接收不能再被新申请的预算门禁拒绝。佣金字段只决定收益投影，不决定 WALLET_TOP_UP 的货币冲正额度；不计佣付款的退款/拒付也不创建或扣减 earning。

若 D4 选择计佣，先冻结真实经济主体归属和金额计算规则再开放；不把“操作人发起充值”自动视为“该操作人本人完成付款”。SDK 不决定佣金、推荐关系或用户映射。

已接受的 payment/refund 仍可沿既有 observer 单向投影。observer 失败不得撤销已收到的钱或让资金二次入账；如有必须交付的下游通知，与 money 事务持久化投递标记/复用已有可靠投递方式，然后幂等消费。首版不新建通用事件总线或佣金平台。

## 10. 入账结果与 readback 契约

以下是拟新增的领域本地窄接口语义，不是已经实现的 SDK API：

| 接口 | Owner | 输入与结果要点 |
| --- | --- | --- |
| `CreateOrReadCheckout` | billing port / 两渠道 adapter | 原 attempt 的 provider/商户/环境/订单/金额/期限 → REDIRECT 或 QR_CODE；显式远端效果与 unknown，不宣告已支付 |
| `QueryPayment` / `ClosePayment` | 同上 | 原渠道身份 → 已验证规范观察 / 关单结果；unknown 单独表达 |
| `RecordVerifiedPaymentObservation` | billing | 只供可信运行装配调用的已验证证据 → durable 接收回执 |
| `AcceptAndPostProviderTopUp` | money | 精确订单绑定 + 可信付款证据 + 明确经济政策 → immutable posting receipt |
| `ReadTopUpPosting` | money | org / order / payment → 原 immutable receipt；不得返回别的企业或仅返回余额 |
| `ReconcileTopUpOrder` | billing | 原 org / order → 继续查询、原身份入账或完成订单 |
| `PrepareTopUpRefund` / `ConfirmTopUpRefund` / `ReleaseTopUpRefundHold` | money | 原付款、退款身份、金额、授权证明 → 资金保留/确认/释放回执；确认结果分开完整渠道金额、本金作用与差额 |
| `ReadTopUpReversal` | money | 原 org / order + `TopUpReversalKey{PaymentID, Kind, ReversalID}` → immutable 冲正 receipt、hold 终态及差额核对引用；不是当前余额 |
| `Refund` / `QueryRefund` | 两渠道 adapter port | 原付款 provider/商户 + 稳定 provider_refund_request_no + 金额 → 规范退款观察；不得跨渠道退款 |

money 入口不能接受浏览器传入的 `verified=true`。可信 evidence 类型及调用者由受控装配形成；代码依赖和实际调用测试同时守住边界，不能只靠 DTO 名字。

入账命令与读取结果都必须携带并核对 org、order、payment、provider、merchant/environment、amount/currency 和 request/result fingerprint。缺失、错配、金额不等或其他订单的 receipt 一律不能完成订单。

冲正命令、退款确认/释放和读回使用 §4.2 完整 typed key；退款 intent/hold 固定 Kind=REFUND。`ReadTopUpReversal` 在当前授权的 org/order 范围内按三字段精确查询并回检返回 key、原绑定和 fingerprint：缺失或未知 Kind 返回 ErrInvalid；完整 key 未找到返回 ErrNotFound，不允许省略 kind、跨类型查找或返回第一条匹配。即使该字符串 ID 只在另一类型存在也不回退；另一 org/order 的记录不返回。相同 key 异载荷的写入返回 ErrConflict，合法 REFUND/r1 与 CHARGEBACK/r1 则各自读回自己的结果。

## 11. 退款与外部冲正

### 11.1 主动原路退款：先保留资金，再调用渠道

首版建议只提供经批准的后台/support 用例，不开放新的 tenant 自助退款 API。退款资格和审批人由 D5 明确，不能推定 `wallet_topup` 权限包含退款或实际转账授权。

1. billing 持久化 refund intent，绑定原 payment/order/org、金额、批准主体、原因及稳定 provider_refund_request_no（支付宝 out_request_no，微信 out_refund_no）；intent 与 money hold 都保存 `TopUpReversalKey`，Kind 固定为 REFUND，确认/释放不得只按裸 refund_id 匹配。
2. money 在原 payment 锁下按 §11.3 检查已确认退款及拒付 + 未决退款保留 + 本次金额不超过原充值可退本金；再按 §5.3 锁顺序锁定钱包，要求 debt 为 0 且 available 足够，原子移动 available → refund-reserved 并产生 immutable hold。
3. 只有成功取得该 hold 的原退款 intent 才可调用原渠道 GoPay 退款接口（支付宝 `TradeRefund`、微信 `V3Refund`）。首次派发准入还须在原 payment 锁下复核 §11.3 的本金预算（本 hold 已在未决总额中，不再加一次），并持久化派发资格；预算冲突不得准入，旧 worker 的未准入派发须被版本校验拒绝。网络调用在事务外，不能把 `10000` / 受理成功一概当成已退到账。
4. 成功或响应不明时，按原渠道的 provider_refund_request_no 查询/重放。unknown 保留资金，不重建退款单，不释放 hold。
5. 支付宝退款查询的 `code=10000` 仅表明查询成功，需核对 `refund_status=REFUND_SUCCESS`；微信退款需核对查询 `status=SUCCESS` 或已验证通知的相应成功状态。两者均核对原交易/退款请求号/金额，细节见 §11.4。[S6][S10] 权威成功后，按 §11.3 同一事务记录完整 RefundSettlement、结清原 hold、写本金作用及超本金差额回执；不能因其间发生其他冲正而拒绝成功确认。该主动退款**不能再通过普通 available 扣减重复扣一次**；hold 中未用于本次本金作用的余款按 §11.3 先偿债，不能留成永久 unknown。
6. 对尚未确认成功的退款，只有明确终局拒绝且可以证明原退款不会继续执行时，才整笔释放该 hold，释放资金优先还债；超时/NOT_EXIST 单次观察不充分。§11.3 成功确认中的本金余款分配不是将退款伪装成失败释放。

`refund-reserved` 是现有 `reserved_minor` 中带退款 purpose 的保留子记录，不建立第四套并列余额。现有 purchase reservation 不能冒充退款保留：其 purpose、唯一身份和消费完成证明不同。新增窄退款 purpose/记录与 ledger entry kinds 归同一 money owner，复用余额算法；购买入口必须拒绝退款保留，退款入口必须拒绝购买保留。必要增量不扩为通用资金调拨框架。

### 11.2 外部退款/拒付

来自商户后台或渠道的退款可能没有本地 hold。确认真实原付款绑定后，先按 §11.3 接受完整事实，再复用 `ApplyTopUpReversal` 的 debt-first 算法对分配出的本金作用 d 扣可用余额、不足记 debt；超本金部分 e 单独记为待核对差额，不转嫁为钱包债务。之后入账与释放优先偿债。外部冲正不因为用户余额不足、被撤权或订单已完成而被丢弃。[R3][R4]

同一退款由通知、查单或人工查证多次发现，必须映射同一 `TopUpReversalKey`。只有 REFUND 且完整 key、金额及原绑定均匹配本地 hold 时才走 hold-confirmation，不能再按无 hold 的外部冲正扣 available；CHARGEBACK 即使 ReversalID 相同也不能消费同名退款 hold。部分退款、累计金额及与拒付的覆盖关系要由 accepted facts 核对，不能简单把渠道累计退款总额当成每次新增退款。

原付款和已完成充值仍保留，退款是独立事实，不将 FULFILLED 改成“从未支付的 CANCELLED”。首版不处理新提现、分账或自动赔付；无法可靠识别的退款保持受限核对，不伪造退款成功。[S6]

### 11.3 货币冲正上限与未决退款保留

以下是 `payment_purpose = WALLET_TOP_UP` 的拟新增 money 合同，NON_COMMISSIONABLE 与 COMMISSIONABLE 都适用；不是全仓所有历史用途的静默规则替换。对 D3 建议的等额充值分支，必须区分**新退款准入预算、已确认渠道事实与钱包本金作用**：

```text
B = payment.GrossAmountMinor
C = top_up.AmountMinor
B = C                         # 已接受付款 gross 与原充值面值精确相等
R = confirmed_refund_minor + confirmed_chargeback_minor
H = outstanding_refund_hold_minor
W = 累计已作用于钱包的本金冲正
E = 累计渠道超本金差额
x = 本次尚未接受的正数渠道冲正，或新的主动退款申请金额
```

B/C 是原付款及不可变充值绑定中的本金，不是当前 available，也不是扣手续费后的净收入。不能使用 CommissionableAmountMinor 作为货币冲正上限。若 D3 不选择等额充值，须先明确付款本金到钱包可退本金的映射；本节 min 只对已冻结的等额分支分配剩余本金，不代替不同金额分支的政策。

R 按同一原付款的不同经济冲正完整累计，允许超过 B；W 是这些事实已经在钱包产生的本金扣减，不能超过 C；E 保留两者差额。退款与拒付共用 W，不能各自扣一遍本金。同一事实的 accepted settlement、wallet receipt 与通知不能重复累计。H 只含尚未确认/释放的退款保留，包括已派发但结果 unknown 的保留，不含 purchase reservation。

先核对原冲正身份及 fingerprint：身份必须是 §4.2 的 `(payment_id, reversal_kind, reversal_id)`；相同身份、相同载荷返回原 immutable receipt，即使 W 已到上限也重放成功；异载荷冲突进入受限核对。通知/查单若无法识别是重复报告还是不同经济事实，先留存可信证据，不猜测相加。对已完整验证且确为不同的渠道成功事实，即使 R + x > B 也必须记录完整事实，不能截断 x 或永久停在“待接受”。

在原 payment 锁下，对新的已确认事实作如下原子分配，使用检查过的整数计算；先校验既有 `0 <= W <= C`，不能由负数/溢出绕过约束：

```text
d = min(x, C - W)              # 本次还可作用于钱包的本金
e = x - d                     # 已确认但不能再扣钱包的渠道差额
R' = R + x
W' = W + d
E' = E + e
R' = W' + E'                  # 等额分支的完整事实分解
0 <= W' <= C
```

不得把 R + x <= B 作为已确认事实的接收或 hold 结清门槛。该预算只控制尚未发生的新主动退款，不控制外部已经发生的事实。

| 入口 | 原 payment 锁下的准入 / 原子结果 |
| --- | --- |
| 新主动退款 `PrepareTopUpRefund` | 要求 `R + H + x <= B`，且钱包有可保留的本金；首次派发再次要求 `R + H <= B`。用经检查的减法判定，R 已超过 B 时直接禁止新准入；不能据此拒收在途请求的成功结果 |
| 已有 hold 的退款确认 | 校验原 refund identity、payment/order/org、渠道成功金额 x 与原 hold 相等；记录完整 x 并按上式分配 d/e；同事务 `H' = H - x`、reserved 减少 x，只将 d 计为本次本金冲正；其余 x - d 按释放规则先偿还既有 debt，再增加 available。hold 进入 CONFIRMED，不再扣 available，不再等待一个已证明成功的请求失败 |
| 无对应 hold 的已发生外部退款/拒付 | 不得仅因未决 hold 拒绝已发生的外部冲正；记录完整 x，只有 d 按 §11.2 从 available/debt 冲正，e 写差额核对记录。不得消费其他退款或 purchase 的 hold |
| 外部通知匹配本地 hold | 同一 refund identity 只走上述 hold-confirmation，不再走无 hold 冲正。重复通知、确认后查询与重启 readback 返回同一分解及 hold 终态 |

存在 hold 时，`hold_consumed_minor=d`、`hold_released_minor=x-d`，两者之和等于原 hold 面值；释放部分沿既有 debt-first 规则记账。d=0 时不创建零金额 wallet entry，但仍记录完整渠道事实、零本金作用的冲正 receipt、差额及必要的 hold 释放/偿债流水。成功事实、hold 结清、本金作用、余款分配、差额记录和 receipt 必须同事务提交；不能先标 CONFIRMED，再留下未释放 reserved。

外部冲正确认可能使 `R + H > B`：停止新的退款准入及尚未准入的派发，保留原身份核实已经在途的请求。unknown hold 保留；未派发且已在同一锁/CAS 下撤销派发资格，或已经证明渠道终局拒绝且不会执行，才能整笔释放。若在途请求成功，必须按上表确认，不能继续等待“不执行”证明。本地锁不宣称能阻止渠道后台的外部冲正。

e>0 时，同一 money 事务建立以 `(payment_id, reversal_kind, reversal_id)` 为唯一身份的差额核对记录；包括 d=0/e>0 的两种同名冲正也必须分别保留，不得因字符串 ID 相同而互相覆盖。该记录保留完整渠道凭据引用、x/d/e、原本金和处理状态，状态为 `OPEN`；通过 `ReadTopUpReversal` 可恢复为“事实及 hold 已结清、差额待核对”。差额不是钱包债务，不产生 earning，也不授权自动赔付、冲销或向用户追偿。人工/渠道后续核实沿该原记录追加结果、证据和审计，不改写旧 receipt；其处理须按 D5 单独授权。差额未解决不阻止已经确认的 hold 结清，也不触发再次退款或重复扣钱包。

`RecordRefundSettlement`、`RecordChargebackSettlement` 及通知包装、`ConfirmTopUpRefund`、`ApplyTopUpReversal` 和读回必须使用同一 money 内部 typed-key 事实去重与本金分配规则：accepted fact 金额为 x，wallet effect 为 d，不要求两者相等。按 §5.3 原 payment → hold → wallet 锁顺序，原子写 accepted fact、hold 变化、wallet effect、差额和 receipt；不得把现有按本金硬拒绝的入口作为 top-up 的旁路。退款先于入账时，M1 同事务衔接并按相同 W/E 分解，不暴露已退本金。原非 top-up 的受控/referral 合同保持原义并直接回归；计佣 top-up 的下游不得按超本金部分多扣收益。[R4][R6]

### 11.4 双渠道退款映射

退款固定使用原支付的 provider、merchant/app/environment 及原交易绑定。统一 `provider_refund_request_no` 在支付宝映射为 `out_request_no`，在微信映射为 `out_refund_no`；持久化原映射，微信响应/通知 `refund_id` 与该原请求关联，不能新造另一 canonical refund identity。

| 渠道 / 结果 | money/billing 处理 |
| --- | --- |
| 支付宝退款查询 REFUND_SUCCESS | 完整验证原请求和金额后，按 §11.3 确认 hold；不凭 code=10000 判断到账 |
| 微信退款查询 SUCCESS / 已验证退款成功通知 | 核对 out_refund_no/refund_id、transaction_id/out_trade_no、amount.refund/total/currency，按相同契约确认 |
| 微信 PROCESSING / 超时 / 未知 | 不释放 hold，按原请求查询，不改号退款 |
| 微信 ABNORMAL | 不释放 hold，保留原退款与受限处理状态；异常资金仍需渠道核实，不自动另路退款 |
| 微信退款 CLOSED | 在验证当前原退款的终局不执行事实、且无已知成功/冲突之后，才按既有释放规则收敛；不是支付订单 CLOSED，不自动创建新退款 |

微信退款受理不等于用户已收到款。异常退款的换卡、商户后台处置等需要 D5 单独授权，不在本轮扩成自动转账。支付宝仅依实际签约接口获取退款证据，不假设它具有微信相同的退款事件/字段。[S6][S10]

## 12. 自动恢复、账单核对与资源边界

两渠道唯一业务协调者是 billing 的充值恢复用例；不用 SDK 自动重试另建第二业务恢复 owner。

建议初始运行参数（工程默认值，可经受控配置调整，不是支付渠道保证）：应用成功组装后立即扫描；其后约每 30 秒扫描；每批最多 50 条，并发最多 4；单次渠道请求最长 10 秒，服从更短的父 deadline；退避带抖动，上限 15 分钟。

选择 `next_check_at <= now` 的未终结 attempts、PAID_PENDING_CREDIT、待处理 inbox 和未决退款。§11.3 的 OPEN 差额单独核对，不把已 CONFIRMED 的退款重新当作 unknown 派发；持久 receipt 用于恢复 hold 终态。按到期时间加稳定 ID 排序推进游标，不能让同一条异常永久占据前 50 条。按渠道隔离请求并发/退避，避免一方持续失败饿死另一方；只复用这个有界循环，不建立新 scheduler。连续失败保留记录并告警；告警阈值不是删除或宣告失败阈值。

使用现有 DB claim/CAS 或窄 lease 执行；网络期间不持锁。claim 过期只能重查/重放原幂等身份，不能保证旧进程没有执行远端操作；旧 claim 的回写必须被版本/token 拒绝。原已成功的资金效果由 immutable claim/receipt 防重。

恢复循环绑定长生命周期 runtime context，在 app 停止时取消并有界等待；不得误用短期 startup context。既有账户/订阅恢复机制可复用调度叶能力，但本领域的候选选择与结果分类仍只归 billing。

日账单核对是资金运营的第二证据入口：首次可由受控作业取得所选商户和环境的账单，核对交易号、订单、金额与退款；差异先触发同一查单/接收路径，不直接改余额。不能在失败时从任意下载 URL 拉取内容；固定各自渠道来源、大小/解压上限与下载超时。支付宝账单和微信交易/资金账单分别按原 provider 匹配，不能仅按同名交易号或金额串账。不为此先建 BI 或完整对账平台。[S1]

生产开放前必须具备未入账付款、未决退款、身份/金额冲突的可追踪处理入口。精确恢复身份、资金 receipt 和必要审计不使用短期缓存代替，也不因通知重试结束而删除。

## 13. HTTP / BFF / Console

复用现有 Workbench commercial API 和权限，不另建一个支付工作台。下列新 route 是设计提案，实施时核对现有路由与 BFF：

```http
GET  /api/v1/workbench/commercial/wallet/top-up-options
POST /api/v1/workbench/commercial/wallet/top-up-intents
POST /api/v1/workbench/commercial/wallet/top-up-intents/:order_id/checkout
POST /api/v1/workbench/commercial/wallet/top-up-intents/:order_id/cancel
GET  /api/v1/workbench/commercial/orders/:order_id
GET  /api/v1/workbench/commercial/wallet
GET  /api/v1/workbench/commercial/wallet/entries
POST /api/v1/payment-notifications/alipay
POST /api/v1/payment-notifications/wechat
```

所有用户写入口使用当前 same-origin/CSRF、live grants、Idempotency-Key 与 expected version；Organization 来自 verified context，不来自 body。充值 options 返回两个渠道各自的 product、available、受限 reason 与获准金额规则，不暴露密钥。充值 intent 提交期望金额字符串和闭合枚举 provider，服务端核准后冻结；相同 key 改 provider 返回冲突；不接收 PaymentID、商户密钥、回调 URL 或可任意改写的 SDK BodyMap。

外部 notify 是独立的渠道认证入口，不套用户 cookie/CSRF，也不向它开放普通 admin 能力。必须保留其签名验证、请求限制、可信装配和 durable acceptance。

UI 展示至少区分“待支付”“正在核实付款”“已付款、入账处理中”“充值完成”“已关闭”“需要人工核对”；退款另列。列表与详情由 billing 的同一 attempt/order 投影生成，不让浏览器维护第二状态机。订单仍 PENDING 时可以显示正在核对，不谎称未付款。

付款操作以 §3 的 discriminated union 返回：支付宝 REDIRECT 只允许本配置对应的 HTTPS 网关，回跳固定同源；微信 QR_CODE 只作为获准 Native 二维码载荷在本地渲染，不作为浏览器跳转或服务端抓取 URL，不交第三方二维码服务。不得接受浏览器自定义链接/HTML，也不能用仅允许 HTTPS 的跳转校验器误拒合法微信二维码载荷。BFF no-store、固定 upstream、受限 JSON、取消/隔离旧响应；企业切换或撤权后旧订单不能污染新企业视图。原企业资金处理继续，不等于原用户仍可读取。

两种支付方式在同一个充值中心可见、可选且显示各自真实 capability；未开放项说明原因。订单列表/详情显示不可变渠道和其付款动作，不因为用户切换选择器改写旧订单。

SDK/渠道不可用、配置缺失、金额不合法、无权、幂等冲突、资金结果未知分别投影；unknown 返回稳定 order reference，并引导核对原订单，不提示盲目“再付一次”。

## 14. GoPay 接入约束

继续以 `v1.5.123` 为固定设计核对基线，实际安装前核查精确 tag、依赖图、安全与编译；不使用浮动 main/@latest，不为文档改 go.mod。两渠道均使用 GoPay，但 SDK 并不统一它们的协议语义。[S1][S7][S8]

| 能力 | 支付宝 adapter | 微信 APIv3 adapter |
| --- | --- | --- |
| 首发付款 | TradePagePay → REDIRECT | V3TransactionNative → QR_CODE/code_url |
| 查询 / 关闭 | TradeQuery / TradeClose | V3TransactionQueryOrder / V3TransactionCloseOrder |
| 通知 | ParseNotifyByURLValues + VerifySign / VerifySignWithCert | V3ParseNotify + 受信公钥/证书验签 + APIv3 resource 解密 |
| 退款 / 查询 | TradeRefund / TradeFastPayRefundQuery | V3Refund / V3RefundQuery |
| 账单 | DataDataServiceBillDownloadUrlQuery | V3BillTradeBill / V3BillFundFlowBill / V3BillDownLoadBill |
| 金额 | 元十进制字符串 ↔ 内部整数分 | amount.total/refund 整数分；不做二次换算 |

上表只定位 SDK 能力；函数签名、返回字段和异常以锁定源码/编译为准。支付宝选择既有 Page Pay 协议组合，不同时再接一套支付宝 V3；微信固定 APIv3，不因错误回退 V2/免验签。SDK 示例与当前渠道规范冲突时，以渠道规范为准，例如微信 ACK 使用 §7.2 的 204 空响应，不机械照搬示例 JSON。[S8][S9]

两个商户配置分别保存 provider、合法 environment、merchant/app、固定通知地址、秘密引用及验证材料。支付宝选择公钥或证书模式并显式开启/调用同步验签；微信使用商户 API 私钥/序列号、APIv3 密钥及受信微信支付公钥 ID/平台证书，显式配置响应验签（如 AutoVerifySignByPublicKey）并校验错误。通知验签另按 §7.2，不能以响应已验签替代。未配置渠道正常 unavailable；显式启用但材料无效应在产生付款能力前拒绝，不回退另一套配置。已存在未决交易需要保留对应验证/查询配置，不能删除后声称已恢复完成。[S1][S8]

始终启用 TLS 校验、生产关闭敏感 Debug，限制 body 和超时。两渠道不能共用或混选秘密；轮换保留未决交易所需验证能力。不把私钥、完整通知、买家账号/OpenID、付款 URL/code_url 或认证头写入 Git/Issue/普通日志/浏览器持久存储。

**微信 APIv3 不虚构沙箱开关。** GoPay 该版本文档说明微信 v3 不支持沙箱；本地 fixture 只能验证受控协议边界，微信实际付款/退款须另获授权。支付宝使用其获准沙箱产品。不能把支付宝沙箱 PASS 写成微信已验收，也不因示例建议小额测试而自动收付款。[S1][S8]

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
| 当前正佣金 validator 与不计佣充值 | 合同显式扩展；不伪造 PayerUserID/CommissionableAmountMinor；§15.1 同时验证零佣金后的本金退款、拒付及 hold 竞争 |
| 用户切企业、退出/撤权、角色不足 | 禁止新越权操作；已发生款项仍归原 org，旧响应不污染新页面 |
| claim 超时、旧 worker 迟到、关闭新支付开关、重启 | 原身份恢复；旧 token 不覆盖新状态；已发生支付/退款继续可核实 |

本地 fixture 只证明本地边界，不证明微信或支付宝真实协议。支付宝沙箱须使用被批准的实际产品，微信实际渠道验证单列；没有商户/凭据时标 NOT_RUN。真实小额、生产收款、退款、正式上线分别授权，CI/代码评审不能代替用户验收。

### 15.1 非佣金充值与冲正的组合用例

本节是两个 adapter 共用的 money 契约用例；REFUND/CHARGEBACK 组合使用受控事实，不宣称两个渠道均提供同名拒付 API，投诉也不是拒付。已有资金规则不因新增微信而删减。

以下为受控 fixture，全部金额单位为分，不是生产充值档位。除最后一项外均为 `NON_COMMISSIONABLE, CommissionableAmountMinor=0`，原付款/充值 `B=C=10000`，使用各自独立付款。每个用例均须核对 accepted facts 与 wallet entries/receipt；不计佣用例还须证明零 earning，不能仅证明 payment validator 接受 0。

| 用例 | 输入 / 交错 | 必须证明 |
| --- | --- | --- |
| NC_FULL_REFUND | 无消费、无 hold；接受原付款全额退款 10000 | accepted refund=10000、wallet reversal=10000、available=0；不产生 earning；同一退款重放不变 |
| NC_MIXED_REVERSALS | 不同冲正身份的退款 4000 与拒付 6000 并发；再申请主动退款 1 | R=W=10000、E=0；新的主动申请 1 被拒绝派发；两项回放不重复扣减。已确认的外部事实不得按新申请拒绝，见 NC_PRINCIPAL_EXHAUSTED |
| NC_HOLD_CONFIRM_REPLAY | H=6000、available=4000；原退款响应与同一退款的通知并发确认 6000 | R 从 0 到 6000，H 从 6000 到 0，refund-reserved 仅减少一次；available 仍 4000，不再双扣 |
| NC_EXTERNAL_DURING_HOLD | 未决 H=6000、available=4000；不同身份的外部冲正 5000 | 外部事实被接受：R=5000、H=6000、available=0、debt=1000；禁止新退款派发，unknown hold 不释放；证明原 hold 不会执行后释放，先还债再恢复 available=5000 |
| NC_INFLIGHT_SUCCESS_AFTER_REVERSAL | 原退款 6000 已派发并持有 H=6000；另一已确认拒付 5000 先入账，使 available=0、debt=1000；随后原退款确认成功 6000 | 完整 RefundSettlement=6000 与 chargeback=5000 均被接受；退款分配 d=5000、e=1000，消费 hold 5000、余 1000 先偿债；最终 R=11000、W=10000、E=1000，H=0、reserved=0、available=0、debt=0，hold=CONFIRMED，差额核对记录=1000 且 OPEN；不重复扣本金、不产生 earning |
| NC_REVERSED_DELIVERY_ORDER | 同一经济事件集合，先确认原退款 6000 并消费其 hold，后接收拒付 5000；同时覆盖两者并发 | 前者 d=6000/e=0，后者 d=4000/e=1000；最终 R=11000、W=10000、E=1000 且无 hold/debt，所有成功事实均保留；单笔分配反映锁序但整体结果一致，不改写原回执 |
| NC_PRINCIPAL_EXHAUSTED | R=W=10000、H=0；又取得不同经济事实的可信外部冲正 1 | 完整事实增加 1，d=0/e=1，W 不再增加；只有冲正 receipt 和差额，不产生零金额扣款流水或用户 debt；未验证/身份不明报告继续隔离核对 |
| NC_CROSS_KIND_ID_COLLISION | 原付款 P 已由其他冲正耗尽本金，R=W=C=10000、H=E=0；随后不同经济事实 `RefundID="r1"` 与 `ChargebackID="r1"` 各确认 1，覆盖顺序/逆序/并发 | 两个事实均完整接受，各 d=0/e=1；按 (P, REFUND, r1) 与 (P, CHARGEBACK, r1) 保存两份不同 receipt、两条各为 1 的 OPEN 差额记录；最终 R=10002、W=10000、E=2，钱包/hold 不变且无零金额 entry。分别按类型读回、重复投递及提交响应丢失后重启仍各得原回执/差额；缺失/未知 kind、同 key 异载荷、跨 org/order、同类 ID 改绑 payment 均拒绝，不回退另一类型 |
| NC_CROSS_KIND_WALLET_AND_HOLD | 独立 P：REFUND/r1 的 H=1000、available=9000；不同事实 CHARGEBACK/r1 确认 1000，随后 REFUND/r1 成功 1000 | 拒付先令 available=8000，不消费退款 hold；退款再确认自己的 hold。两种非零本金作用均有独立 storage/source identity 和回执，最终 R=W=2000、E=0、H=reserved=debt=0、available=8000；逐类型重放不重复扣减 |
| NC_REFUND_BEFORE_POST | 已核实全额退款 10000 先于原付款的 M1 入账完成 | 同事务保留原付款/充值与冲正，最终无可消费净充值；重启/readback 不补出第二次可用余额 |
| TOPUP_COMMISSION_REGRESSION | 若 D4 另批准计佣：同额充值，CommissionableAmountMinor=2000，退款 10000 | 新 top-up 的本金仍按 10000 而非 2000；收益调整按已批准 D4 合同；原非 top-up 受控/referral 测试不被机械改写 |

上述成功交错还必须覆盖重复通知和事务提交响应丢失后重启：按原 org/order 和 `TopUpReversalKey` 读取完整事实、分解 receipt、CONFIRMED hold 及原差额引用，不能重新冻结、扣减、退款或新增差额记录。

实施阶段必须执行对应 money contract、真实 PostgreSQL、并发/重启和消费侧回归。当前 `TestAlipayWalletTopUpDesignReversalContract` 仅守护本文的关键规则及组合用例不被删除，忽略 Markdown 换行差异；它通过不是资金行为测试已通过，以上运行验收在实现前仍为 NOT_RUN。

### 15.2 双渠道验收

两渠道各自完成原订单→付款→正确企业钱包/流水→查询→原路退款的链路，并各自标注受控替身、渠道测试和生产状态；未验证一方不能标为双渠道完成。一个 SDK、两个按钮或一方沙箱成功均不满足完整验收。

| 用例 | 必须证明 |
| --- | --- |
| DUAL_HAPPY_PATH | ALIPAY 与 WECHAT_PAY 分别走真实应用/持久化，共用订单和钱包 owner；付款入口分别为 REDIRECT/QR_CODE，receipt 回读正确 |
| DUAL_SAME_KEY_DIFFERENT_CHANNEL | 同 org/kind/key 的两个渠道请求并发只能一个原订单，另一冲突；不各建一张，不修改 provider |
| DUAL_CALLBACK_CONFUSION | 支付宝通知送微信入口/反向、错误商户/app/环境、同名 notify_id/交易号不会串信任域或错误入账 |
| DUAL_ID_NAMESPACE | 不同渠道/商户的相同原生付款/退款 ID 不碰撞；同一原交易的多次事件及退款别名仍合并为一个经济事实；typed reversal 的旧负例保留 |
| DUAL_NATIVE_RESPONSE_LOSS | Native 已下单但响应丢失/持久化前重启，以原单查询或经证明安全的同参数重放，不另造 out_trade_no、续期或自动切支付宝；无法取回二维码不伪造可付结果 |
| DUAL_NOTIFY_CRYPTO | 支付宝重复 form 参数；微信原 body 被改、错误签名/序列号/nonce/GCM/重复 JSON 字段均拒绝；B3 失败不得 ACK 成功，成功 ACK 分别符合各自协议 |
| DUAL_REFUND_ORIGIN | 微信订单不能调用支付宝退款，反向亦然；PROCESSING/ABNORMAL 不释放 hold，成功/明确关闭、并发重复与重启正确 |
| DUAL_CAPABILITY_RECOVERY | 一方缺配置/停新付不伪造可用、不自动转付；原渠道已发生支付退款继续恢复，另一方不被饿死；切换企业不污染旧订单 |
| DUAL_AMOUNT_AND_EXPIRY | 支付宝元字符串、微信分金额分别正确，优惠字段不冒充本金；重放保留绝对期限，两种付款能力都遵守安全关闭再换方式 |

新增 TestWalletTopUpDesignDualChannelContract 只验证文档要求存在；实现阶段必须分别运行上述协议/持久化测试，不以字符串断言当作渠道功能完成。

## 16. 实施顺序与评审收敛

同一 #481 Delivery Batch，一个主要分支/PR。当前只交付本草案、索引和针对评审缺口的文档契约检查，不开多个平行实现任务。

1. **M0 文档与高风险边界评审**：核对 D1–D6 的阻塞层级，重点审查新增双渠道身份/验签/Native 外部效果，以及既有 M1 原子入账、退款保留；双渠道范围已由用户确定，D1–D6 的商户/政策条件仍未批准。当前状态 DRAFT；未给独立 Reviewer 的判断冒名签字。
2. **M1 双渠道收款链**：在同一 PR 内连续实现公共 intent/checkout、支付宝与微信两个 GoPay adapter、各自通知/查单和共享 source-bound posting。可分提交验证，但不能交一方后将另一方自动降为 Later。
3. **M2 同 PR 收敛恢复与退款**：实现原 receipt 恢复、必要退款/冲正、上述直接相关负例。不另造 scheduler 或故障注入平台。
4. **M3 充值中心交付**：同页两种方式、分别真实 capability、状态和原 wallet/order API；两渠道分别跑“充值 → 钱包另行购买”链；订阅消费沿 #479，不接管其实现。

按 AGENTS 的新增高风险检查与最终交付检查推进；正常架构评审最多两轮。修复只复核相关增量，真正的新 BLOCKER 才重开相应设计，不把文档不断加长当作交付。

### 16.1 与既有合同的明确关系

本草案**保持** #457 的 money/billing/resource ownership、整数金额、debt-first、top-up 无资源 quote/items/reservation、非完成 Order 不带 PaymentID；保持 #480 的订阅资金协议和独立激活 owner。

本草案按用户新决定将首版扩为双渠道，保留旧评审修复。以下技术方案仍为**提议补充，尚未生效**：双渠道 adapter/信任域/固定 provider 与 typed checkout、payment attempt 子状态、精确 posting receipt、money 非佣金/付款人表示、top-up 完整渠道事实与受限本金作用/差额分解、未决 hold 成功结清规则、主动退款 purpose 与 receipt、受控迟到付款纠正。获批时在同一变更中同步相关稳定合同、类型 validator、下游消费者与直接回归，不能只新增文档却让旧合同与新实现互相冲突。未获批前本文件不作为绕过现有 guard 的依据。

### 16.2 当前交付边界

代码路径与文档已作定点核对；没有编译 GoPay、执行渠道接口、建立商户账户、操作数据库、验证用户浏览器、运行整仓 CI 或完成独立架构评审。具体 PR 与实际检查结果写 #481/PR，不把未来验收勾成 PASS。

## 17. 依据与核查范围

仓库依据使用固定代码基线；外部依据为 GoPay 固定 tag、支付宝与微信支付官方文档。新增微信通知/退款规则按本轮官方页面核对；Native 接入能力同时参考腾讯云官方集成说明。支付宝文档站部分页面依赖客户端渲染，本轮部分规则来自官方索引摘录及官方 Easy SDK 的公开 API 文档；实施前必须核对所选产品当前完整文档，不能把索引摘录当完整接口测试。

- [R1] [现有订单类型及 validator](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/internal/commercial/billing/contracts.go)。
- [R2] [现有 billing service；充值入口仍 unavailable](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/internal/commercial/billing/service.go)。
- [R3] [money 支付/退款事实与 validator](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/internal/ledger/money/types.go)；[钱包接口](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/internal/ledger/money/wallet.go)。
- [R4] [钱包入账、唯一绑定与事务实现](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/internal/integration/persistence/money/wallet_repository.go)。
- [R5] [现有钱包/商业合同](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/docs/architecture/commercial-wallet-billing-contract.md)；[订阅购买合同](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/docs/architecture/self-service-subscription-purchase-contract.md)；[AGENTS](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/AGENTS.md)。
- [R6] [现有退款/拒付累计限额实现](https://github.com/qq550723504/task-processor/blob/c5deccbe7d42e211ab1e2c80987b1e7e67b671f2/internal/integration/persistence/money/repository.go#L127-L206)。
- [S1] [GoPay v1.5.123 支付宝文档](https://github.com/go-pay/gopay/blob/v1.5.123/doc/alipay.md)。
- [S2] [支付宝电脑网站支付快速接入](https://opendocs.alipay.com/open/270/105899)；[官方 Easy SDK API 文档](https://github.com/alipay/alipay-easysdk/blob/master/APIDoc.md)；[Page Pay 参数与 time_expire](https://opendocs.alipay.com/open/028r8t)。
- [S3] [支付宝交易查询](https://opendocs.alipay.com/open/c676a9b0_alipay.trade.query)；[如何判断交易是否成功](https://opendocs.alipay.com/support/01ray9)。
- [S4] [电脑网站支付异步通知说明](https://opendocs.alipay.com/open/270/105902)。
- [S5] [支付宝交易状态说明](https://opendocs.alipay.com/support/01raw9)；[订单已关闭的原因](https://opendocs.alipay.com/support/01rfti)。
- [S6] [支付宝退款查询及 REFUND_SUCCESS 语义](https://opendocs.alipay.com/open/8c776df6_alipay.trade.fastpay.refund.query)；[官方 Easy SDK 的退款与退款查询合同](https://github.com/alipay/alipay-easysdk/blob/master/APIDoc.md)。
- [S7] [GoPay v1.5.123 发布](https://github.com/go-pay/gopay/releases/tag/v1.5.123)。

- [S8] [GoPay v1.5.123 微信 APIv3 文档](https://github.com/go-pay/gopay/blob/v1.5.123/doc/wechat_v3.md)。
- [S9] [微信支付普通支付成功通知、签名/解密与应答](https://pay.wechatpay.cn/doc/v3/merchant/4012791861)。
- [S10] [微信订单退款开发指引](https://pay.wechatpay.cn/doc/v3/merchant/4013071031)；[退款回调通知](https://pay.wechatpay.cn/doc/v3/merchant/4012085921)。
- [S11] [腾讯云开发微信 Native 支付集成](https://tcb.cloud.tencent.com/integration-center/detail?id=7eeea64669a97a40004bd447324975af)；具体 Native 产品权限/参数在实施前按商户当前官方产品文档冻结。
