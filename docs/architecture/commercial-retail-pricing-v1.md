# 店铺与 AI 预付资源正式价格 V1

Design Basis: Independent Architecture
Admission Status: IMPLEMENTATION_READY — 2026-10-08 独立 reviewer 复核设计 blob 5700e1cb85ccf443053800991abef62e336c2f3a、Issue #600 和引用合同，无设计级 BLOCKER；三个 IMPLEMENTATION_TEST 在本批收敛。

## 产品决定与本批用户结果

2026-10-08 用户批准将店铺价格调整为人民币 168 元，合同统一为 30 天一期，并授权实现及确定其余定价细节。沿前一条建议，本 Delivery Batch 首先完成既有店铺收费链路和统一价格展示；复用既有 AI 点数购买，确定 1 点 = 1 分（¥1 = 100 点）的资源购买价格。模型调用费率和 provider 成本预算仍是不同合同，不能由兑换价格推导；本批不改变或开放模型调用。

Must：
- 新安装提供 STORE_RENEWAL_PERIOD 正式商品：16800 分/期，1 期为 30 天。真实连接后用户显式开通/续费，沿 Store 原合同计时；购买资源不会自动开通、连接或续费。
- 新安装提供 AI_POINT 正式商品：1 分/点，企业预付余额不按月清零。DATA_ROW 沿已批准的服务端 1688 采集 5 分/条价格，本地/插件采集规则不变。
- 首页公开价格与登录后资源购买使用同一 commercial_offers 事实。没有当前可售价格时显示暂不可购买/价格暂不可用，不用静态默认价替代查询失败。
- 报价/订单冻结原价格、版本和企业；调价不重算历史订单。资金与资源到账证明、权限和 UNKNOWN 恢复完全沿现有购买合同。
- 重跑初始化不覆盖已经由 owner 调整或停用的商品。存在其他 ACTIVE 状态商品（包括未来/过期销售窗口）或目标 ID 被不同产品占用时，拒绝自动安装竞争价格，由 owner 明确处理；不迁移、不删除、不更新既有财务事实。
- 首页 15 天/1 店/¥5 券仍属于待开放计划，不能宣称注册即已到账；本批明确展示尚未开放。专项服务沿咨询入口，不接入资源账本或自动利润分成。

Out of Scope：试用资格/赠送/优惠的新状态与持久化、模型/图片消费费率、真实 provider 调用、自动续费、多店折扣、专项服务订单/合同/利润结算、新支付渠道、真实环境价格变更、真实支付、部署、合并、Issue 关闭。

## 产品与架构权威

- 当前 Figma [套餐方案 431:3455](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=431-3455)（2026-10-08 已读取设计上下文与截图）：收费构成、店铺 ¥168/店/期、1 期 30 天、AI 与数据分开购买、权益/钱包/账单入口。Figma 绑定后计时、多店优惠、赠送示例由当前领域合同及本批范围约束，不直接转为事实。
- [Final UI / IA Authority](../product/final-ui-ia-authority.md)、[统一基础与预付资源](unified-base-prepaid-resources-v1.md)、[Commercial Wallet/Billing](commercial-wallet-billing-contract.md)、[成员资源分配](member-resource-allocation-v1.md)、[支付合同](alipay-wallet-topup-design.md)、[Greenfield](../product/greenfield-no-legacy-migration.md)。
- 首页沿已交付 MarketingHomepage 布局、资产和咨询入口；本批仅替换价格数据和未开放试用状态。用户的最新价格决定取代首页 ¥99 文案及旧文档的“店铺/AI 购买价格待决定”，不改变其他 owner 合同。

## Owner、实现和注入

| 事实 | Owner / 调用链 |
| --- | --- |
| 可售商品与价格 | billing.Offer / commercialbilling Repository / commercial owner DB |
| 安装已批准价格 | 现有 commercial-owner-schema-migrate → Repository 控制安装；只在获准新安装/隔离测试执行 |
| 公共价格投影 | billing HTTP handler → 现有 commercial-billing module → current application route registry → Next server-only 固定路径读取 → MarketingHomepage |
| 企业价格与购买 | 已有资源目录/quote/order BFF → authenticated billing handlers → billing.Service |
| 钱包预留/扣款 | 原 money owner、原企业订单绑定 |
| 资源授予/消费 | 原 ledger/orgresource、原购买订单 source 身份 |
| 店铺服务期 | 原 storecenter native 服务合同，30 天/期 |

不新增 schema、价格表、套餐/权益 owner、付款方式、第二份余额/账本或定时恢复机制。初始化默认值只是受控首次安装输入，浏览器、SSR 和 runtime 不以这些常量兜底。

## 公共只读边界

新增 GET /api/v1/commercial/resource-offers，AuthPolicyPublic、OrganizationAccessPolicyNone、Permission 空、RejectUnreadRequestBody=true、RequestTimeout=5s。仅输出 schema_version、当前可售商品的 offer_id/product_kind/resource_type/currency/unit_price_minor/min_quantity/max_quantity/pricing_version 和 store_period_days=30；使用既有 ListResourceOffers 过滤状态/销售窗口并限制 100 项。不返回组织、用户、钱包、余额、订单、凭据、数据库错误或 provider 配置。拒绝 query/body；固定 GET 路径不提供 mutation。

Next server-only 通过现有 COMMERCIAL_API_ORIGIN 的严格 origin 校验读取固定路径，no-store、禁止重定向、5s deadline、最多 32KiB strict JSON、严格 schema/数字校验；不转发 Cookie/Authorization/user/org headers。失败仅使价格显示不可用，首页其余内容可正常访问。价格 API 与私有购买 API 共用 catalog，公共价格投影不构成购买授权。

## 状态、权限、幂等与副作用

已购资源仍经原 LiveWrite/CommercialPurchase 权限检查。quote → RESOURCE_PURCHASE order → money reserve → resource grant → money commit 不变。同键不同载荷冲突；结果未知沿原 order/reservation/source 查询恢复，禁止再次下单。不创建新的外部副作用或自动消费。

首次安装只插入缺失的批准 offer；重复执行保留既有金额、版本、有效期和停用状态。商品检测及安装在同一 commercial catalog 事务内执行；已有 schema 初始化合同不扩为业务数据迁移。新 store/AI 商品各一个，限量分别 1–120 期、1–1000000 点；已有 data 商品范围不变。

## Legacy 与有限验证

Legacy decision: N/A — 不消费旧 subscription/entitlement/task/tenantbridge；复用当前 billing/money/orgresource/Store owner。

TDD：先捕获缺少正式 store/AI 商品、报价金额、重复初始化覆盖/竞争商品、公共字段泄露/请求边界及首页错误价格的失败测试，再实现最小补齐。使用现有 SQLite/PG 测试与商业 API/前端测试；必要类型检查、相关 lint、build 和必需 CI。原钱包预留/授予/扣款测试保留复用，不新增验收工具。

交付入口：首页 /、套餐方案 /workbench/plans/options、我的店铺 /workbench/stores、订单 /workbench/plans/orders。真实支付、官方连接、provider 与用户验收分列 NOT_RUN；代码自检不签发产品验收。正式初始化命令与 stop/restart 不删除数据的说明随当前 Compose 文档交付。
