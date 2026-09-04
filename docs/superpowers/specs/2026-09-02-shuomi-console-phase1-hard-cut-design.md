# 硕米智能引擎第一阶段硬切产品与架构契约

**日期：** 2026-09-02  
**状态：** IMPLEMENTATION_READY / 冻结产品基线  

> **登录决策更新（2026-09-05）：** 本文中与现有用户登录、专用 `otp_login` / `password_login` 和 Login V2 Fork 登录职责有关的 Phase1 决策，已由 `2026-09-05-shuomi-login-phase1-zitadel-native-simplification.md` 替代。Console 其他产品决策及手机号注册边界不受该补充影响。

**代码库：** `qq550723504/task-processor`  
**设计来源：** Figma 文件 `tg48P46SSXl6TBy9lZwg63`，页面 `31:463`

> 本文只保留 Console 第一阶段的产品口径、页面边界、能力迁移规则和高层架构决策。手机号自助注册/首次业务开通的详细可靠性设计以 **PR #283 V7** 为唯一实现基线；企业资源账本和店铺服务生命周期的详细一致性设计以 **PR #284 V7** 为唯一实现基线。本文不再定义这两个领域的数据库状态机、幂等算法、补偿或清理协议。

---

## 1. 核心原则

1. **同一代码库、同一 Console 基座、两种工作区模式。** 第一阶段交付 `tenant`，`platform` 保留架构入口但不创建空壳页面。
2. **新 Console 做 UI / IA Hard Cut，不做 Capability Hard Cut。** 新开发不复制旧 `/listing-kits/*` 的视觉和信息架构，但在新页面尚未替代某项既有生产能力前，不得仅因 Shell 上线就让该能力从用户侧消失。
3. **复用能力，不复用旧产品界面。** ZITADEL/Auth.js、多企业上下文、Store Center、Subscription/Usage 等现有基础设施继续复用。
4. **身份问题交给 ZITADEL。** 不在 task-processor 重建密码、OTP、Session、OIDC 或长期手机号身份目录。
5. **业务事实归业务域。** 套餐、企业权益、店铺服务、资源余额不放进 ZITADEL。
6. **真实数据优先。** Figma 示例用户名、金额、店铺数、余额和认证状态不得成为生产默认数据。
7. **先交付闭环能力。** 支付、人民币钱包、成员自定义 RBAC、实名认证、退款、发票、推广收益等拆到后续阶段；Phase1 通过一次性新企业体验额度闭合首次 Store 激活路径。
8. **未有资源获取渠道的付费动作必须 Capability Gate。** Domain 可以先实现 Renew/Reactivate，但客户侧不能在余额为 0 且没有在售续费资源获取渠道时展示成“可完成”的动作。
9. **Product Decision 高于历史 Review 假设。** 已明确接受或移出范围的风险不得被旧 amendment/评审意见重新升级成当前实现要求。

---

## 2. 已确认产品口径

### 2.1 账号

```text
注册必填：手机号 + 短信验证码 + 协议确认
注册不填：展示名称、用户名、密码、邮箱、经营画像
默认手机号登录：手机号 + 短信验证码
可选手机号登录：后台设置密码后，支持手机号 + 密码（Capability Gate 通过后开放）
```

Figma 中“用户名”统一解释为**展示名称**，不作为登录账号。产品不支持“用户名 + 密码”登录。

**手机号是否已注册允许明确提示。** 注册入口可以返回/展示“该手机号已注册，请直接登录”；Account Existence 不属于 Phase1 需要隐藏的信息。仍不得在未授权状态返回姓名、邮箱、Organization、角色、Store、Factor 列表等账户或业务资料。

手机号注册使用的 `u-<opaque-id>@phone.invalid` 等技术邮箱是 Provider 内部技术字段：

- 永不作为用户真实邮箱展示；
- 永不因为 Provider 标记 verified 就在“我的账户”显示为“已验证邮箱”；
- 不触发用户侧邮件修改、找回或通知动作；
- UI 应把它映射为“未设置邮箱”。

现有历史用户可能并非手机号账号，例如由既有邀请/Provisioning 创建的 email-only 用户。**新手机号入口不能删除他们的登录路径。** 受保护页面的默认未登录重定向继续进入通用 Login V2 chooser，而不是强制 OTP。

### 2.2 资源和计费术语

```text
AI Token       → AI 点数
数据资源       → 数据额度，单位“条”
我的权益       → 企业权益
充值中心       → 企业钱包
企业认证       → 企业空间下的企业认证
```

### 2.3 店铺

```text
绑定店铺 ≠ 激活店铺服务
绑定成功不开始计费
用户显式激活后才开始服务周期
使用中的店铺续费与到期后重新激活属于不同业务动作
```

详细资源扣减、幂等、状态转换由 **PR #284** 定义；本文只锁定产品语义。

### 2.4 企业资源与成员额度

企业持有真实资源：

```text
企业续费期数
企业 AI 点数
企业数据额度
```

成员不拥有独立资产。后续企业管理员配置的是成员月度消费限额：

```text
成员实际可用 = min(企业当前可用，成员当月剩余额度)
```

成员月度额度不进入第一阶段。

### 2.5 Phase1 新企业首次体验额度

新**直接注册**并成功完成首次业务开通的 Organization，一次性获得：

```text
store_renewal_period = 1
ai_point = 0
data_row = 0
```

产品含义是：用户可先绑定 1 家店，再显式消费这 1 个续费期启动首个 30 天服务周期。

约束：

- 该 1 期不是“绑定即计费”，绑定动作仍不扣资源；
- 该 1 期是一次性新企业体验 Grant，不是人民币钱包余额；
- 同一首次开通不得因为重试重复赠送；
- 历史 Organization 不在本 PR 自动补发，除非后续单独做迁移决策；
- 触发顺序由 #283 Onboarding 定义，资源入账的 trusted Provisioning / source-bound 幂等合同由 #284 定义。

### 2.6 体验期后的续费边界

Phase1 **不实现客户在线购买 `store_renewal_period`**；人民币钱包、支付回调和资源购买仍延期。

因此首次赠送的 1 期被消费后：

- 若企业没有其它已由可信 Billing/Platform-Finance/Provisioning 入账的 renewal periods，Console 不得把 Renew/Reactivate 展示成当前可完成的客户自助动作；
- UI 应展示资源不足/续费购买尚未开放的真实状态，而不是一个必然失败的按钮；
- #284 可以先完成 Renew/Reactivate domain/API 及余额充足场景测试，为后续购买能力复用；
- 后续一旦有经过批准的资源购买/发放来源，只需增加资源 acquisition，不重新设计 Store lifecycle。

这保持“支付后置”的既定范围，同时避免 Phase1 对用户承诺无法完成的持续续费闭环。

---

## 3. Figma 视觉基线

| 页面 | Figma 节点 |
|---|---|
| 注册 | `374:325`，去掉用户名和密码字段 |
| 密码登录 | `373:303` |
| 验证码登录 | `378:307` |
| 重置密码 | `1266:359` |
| Console Shell 深色 | `393:321` |
| Console Shell 浅色 | `411:323` |
| 我的账户总览 | `411:2489` |
| 账户资料总览 | `432:534` |
| 经营画像 | `464:540` |
| 企业空间总览 | `432:4694` |
| 套餐与权益总览 | `411:2293` |
| 套餐方案 | `431:3654` |
| 企业权益 | `431:4158` |
| 用量明细 | `431:4662` |

后续节点：

```text
个人认证 1514:815
企业角色权限 1536:653
成员资源与额度 1627:539
企业操作记录 1532:857
企业钱包 431:5166
账单与订单 1839:766
```

---

## 4. 第一阶段交付范围

### 4.1 交付顺序

```text
1. 账号入口
2. 新 Console Shell
3. 我的账户
4. 企业空间总览
5. 套餐与权益只读链路
6. 店铺中心新界面
```

### 4.2 账号入口

产品要求：

```text
通用 Login V2 登录入口（保留既有 email-only 等历史身份）
手机号注册
手机号验证码登录
手机号密码登录（上游能力验证通过后开放）
忘记密码/重置密码（Phone-only Recovery Capability Gate 通过后开放）
标准 Auth.js 会话
安全 returnTo
```

技术边界：

```text
ZITADEL 官方 Login V2
→ Generic Login / OTP / Password / Reset / Session / OIDC

硕米手机号自助注册与首次业务开通
→ 详细设计见 PR #283 V7
```

上线 Gate：

- SMS OTP Session 必须在 staging 证明能完成完整 OIDC Auth Request → CreateCallback → Auth.js Session；只证明 OTP API 单独成功不等于通过；
- 手机号 Password Flow 未证明时不展示密码 Tab；
- Phone-only Password Reset 未证明时不展示“忘记密码/重置密码”入口；
- 这些 Gate 失败只关闭对应能力，不得为了保留 Figma 按钮而在 task-processor 新建密码/OTP 系统。

企业邀请不进入账号入口 Slice 1，后续归“企业空间 / 成员管理”。

### 4.3 Console Shell

- 新品牌区、左侧导航、顶部栏、内容区和移动端抽屉；
- 语义化浅色/深色 Token；
- 当前企业上下文、切换保护、请求取消与缓存清理；
- 联系客服、通知、用户菜单等真实入口；
- 未上线模块不渲染导航，不创建“建设中”页面；
- 对仍未完成新 Console 替代的既有生产能力提供**临时 Legacy Capability Entry**，只做导航桥接，不 iframe、不复制旧 UI、不允许新功能继续在旧 IA 扩展。

### 4.4 我的账户

第一阶段包括：

```text
账户总览
账户资料
展示名称/地区等普通资料
手机号/邮箱/密码状态
设置或修改登录密码（Provider-native self-service Gate 通过后）
经营画像
企业空间总览
真实空状态/无权限状态
```

展示名称只用于展示和审计协作，不作为凭据。

密码状态与设置/修改动作必须来自 Provider-native、自助且需要适当 re-auth 的路径；在 ZITADEL/Login V2 当前版本没有被验证前：

- 页面不得调用未评审的管理凭据 API；
- 不伪造 password status；
- 设置/修改密码入口隐藏或明确显示当前不可用。

### 4.5 套餐与权益

第一阶段页面目标：

```text
套餐与权益总览
基础方案展示
企业权益
企业资源余额只读展示
用量明细
资源单位与人民币成本分栏
```

`base_payg`、首次开通与初始体验 Grant 的业务触发归 **PR #283**；资源账本的 trusted Grant / source claim / balance mutation 归 **PR #284**。本文不定义账本表结构和资源事务算法。

### 4.6 店铺中心

第一阶段产品行为：

```text
店铺绑定
店铺列表/详情/编辑
连接状态与服务状态分离
显式激活
Renew/Reactivate domain 能力（客户入口受资源 acquisition Gate）
```

新企业首次业务开通后有 1 个 `store_renewal_period`，因此 Phase1 存在真实首次使用闭环：

```text
注册完成 -> 绑定第 1 家店 -> 显式 Activate -> 消耗 1 期 -> 服务 30 天
```

首次 30 天之后，如果企业没有可用 renewal periods 且在线资源购买尚未上线，Console 必须显示真实的 unavailable/insufficient-resource 状态，不把 Renew/Reactivate 当作当前可自助完成的购买流程。

组织隔离和现有 Store Center 能力继续复用。资源扣减与 Store Service 一致性见 **PR #284**。

---

## 5. 路由与认证 Entry 契约

### 5.1 公开入口

```text
/login
/register
/login?method=otp
/login?method=password
/forgot-password
```

`/login` 是**通用上游 Login V2 入口**，用于 protected-route 默认重定向并保留既有 email-only / 非手机号身份的登录能力。不得把 bare `/login` 强制解释成手机号 OTP。

其余带明确意图的路由不能全部丢弃参数后落到同一默认页面。

服务端维护 allowlist 映射：

```text
/login                   -> Login V2 generic chooser/default upstream login
/register                -> Login V2 registration entry
/login?method=otp        -> Login V2 SMS-OTP login entry
/login?method=password   -> Login V2 password entry（仅 capability enabled）
/forgot-password         -> Login V2 phone recovery/reset entry（仅 capability enabled）
```

实现可以选择受控 authorize 参数或独立 Login Fork entry URL，但必须满足：

- entry 值只能来自服务器 allowlist，浏览器不能指定任意 Login URL/handler；
- `returnTo` 继续使用已有安全 allowlist/normalization；
- protected-route 的 bare `/login?returnTo=...` 继续能进入 generic upstream flow；
- password/reset capability 关闭时，相关导航不渲染，直接请求也返回明确的 feature-unavailable 或安全回到 generic/OTP 登录；
- contract test 必须证明 5 个公开入口进入各自预期 flow；
- 至少增加 email-only historical user 从 protected-route redirect -> generic Login V2 -> Auth.js session 的回归测试。

实际认证 UI 可部署在独立 Login V2 域名；`listingkit-ui` 仍只消费标准 OIDC/Auth.js 会话。

### 5.2 租户工作区

```text
/workbench
/workbench/account
/workbench/account/profile
/workbench/account/profile/settings
/workbench/account/profile/business-profile
/workbench/account/organization
/workbench/entitlements
/workbench/entitlements/plan
/workbench/entitlements/benefits
/workbench/entitlements/usage
/workbench/stores
/workbench/stores/new
/workbench/stores/[storeId]
```

第一阶段不创建：

```text
/workbench/entitlements/wallet
/workbench/entitlements/billing
/workbench/account/organization/roles
/workbench/account/organization/resources
/workbench/account/organization/audit
/workbench/account/profile/verification
/workbench/account/organization/verification
```

### 5.3 Hard Cut 的准确含义

**Hard Cut = 新 IA / 新视觉 / 新路由优先，不等于一次性删除仍在生产使用的能力。**

规则：

- 新开发不继续扩展旧 `/listing-kits/*` IA；
- 新页面不 iframe/嵌入旧页面；
- 已经有新 Console replacement 的能力，只展示新入口；
- 尚未被新 Console 替代、但仍属于现有生产产品的能力，继续保留旧 route 与一个明确临时入口；
- 当前至少要保护 task creation、execution records、canonical products、SDS，以及仍在使用的 platform operations 等既有能力不因 Shell 切换丢失；
- 每项 legacy capability 只有在新 route 完成、权限/状态/回归测试通过后，才逐项 retire 对应旧入口；
- 不维护两套长期前端：Legacy Capability Entry 是迁移桥，不接受新产品需求扩展。

---

## 6. 身份架构边界

```text
ZITADEL
= User / Phone / Password / OTP / Session / Organization / Project Authorization / OIDC

ZITADEL Login V2 Fork
= 浏览器登录体验 + 硕米视觉 + 最小手机号注册入口

Auth.js
= listingkit-ui 浏览器业务 Session

task-processor
= 硕米业务投影、套餐、权益、店铺和资源事实
```

明确不做：

```text
业务侧密码哈希
业务侧 OTP code 表
业务侧 Login Session
PhoneIdentity 长期目录
Password Decoy User
自研 Callback Delivery/ACK
登录 Temporal Workflow
```

手机号自助注册时不可避免的临时 Registration Intent、Provider Provisioning 和首次业务开通边界，以 **PR #283 V7** 为唯一实现基线。

---

## 7. 企业与权限边界

ZITADEL 继续提供粗粒度身份与项目角色：

```text
listingkit_viewer
listingkit_operator
listingkit_admin
platform_admin
```

第一阶段不实现自定义企业 RBAC。

组织作用域规则继续沿用现有 Workbench：

```text
读取可使用 cached effective organization
敏感写必须 live resolve organization
客户端不能通过 body/query/header 任意覆盖 organization_id
```

用户自己的安全动作（手机号、密码等）不由企业角色树控制。

---

## 8. 经营画像

经营画像只用于：

```text
统计
用户分层
后续推荐
客户服务沟通
```

不作为第一阶段硬授权依据。

数据归属：

```text
用户角色 / 服务需求 → 个人级
店铺阶段 / 供应链 / 平台 / 站点 / 店铺类型 → 企业级
```

Shop Center 是店铺事实来源，经营画像不能覆盖真实店铺数据。

---

## 9. 套餐产品契约

第一阶段对新直接注册企业使用：

```text
code: base_payg
name: 基础方案 · 按需使用
```

产品含义：

```text
允许绑定 1 家店
首次开通赠送 store_renewal_period = 1
AI 点数初始 0
数据额度初始 0
```

`base_payg` 仍只表达套餐/店铺数量能力；一次性 `store_renewal_period=1` 是独立资源 Grant，不能塞进现有 StoreQuota 字段或假装成 store_count。

如何在现有 `internal/listingsubscription` 中原子应用且不覆盖历史套餐，以及首次开通何时触发一次性 Grant，见 **PR #283**。资源入账规则见 **PR #284**。

历史 `basic / professional / enterprise` 不在本 PR 定义迁移策略，也不自动获得该首次注册赠送。

---

## 10. 企业资源产品契约

资源类型：

```text
store_renewal_period
ai_point
data_row
```

产品约束：

- 企业拥有余额；
- 店铺绑定不扣续费期；
- 新直接注册企业首次业务开通一次性 Grant 1 个 `store_renewal_period`；
- 显式 Activate 才消费 1 个续费期并启动 30 天；
- Renew/Reactivate 按 #284 生命周期合同消费资源，但 customer-facing action 必须同时满足企业已有足够 renewal period 或已有明确资源 acquisition 能力；
- 余额不足且资源购买未上线时，Console 返回/展示真实 `insufficient_resource / feature_unavailable`，不伪装成可续费；
- AI 点数/数据条数消耗资源，不再次直接扣人民币；
- 人民币钱包与在线购买未来独立设计。

Bucket、Operation、Reservation、Event、trusted positive mint、Store 事务边界见 **PR #284**。

---

## 11. 页面视觉约束

### 11.1 Shell

桌面基线：

```text
Sidebar 240px
Topbar 72px
Content 自然滚动
```

必须使用 flex/grid 和响应式结构，不做 1440×900 绝对定位复刻。

### 11.2 字体

Figma 中 9–11px 文本不直接照搬：

```text
正文：13–14px
辅助信息：12px
极少数标签：11px
```

### 11.3 主题

- Console 使用语义 Token；
- 浅色是淡蓝灰体系，不是通用纯白/Zinc；
- Auth 页面可固定深色；
- 详情页未准备完整暗色前可以隐藏主题切换。

---

## 12. 第一阶段状态覆盖

所有页面按能力至少覆盖：

```text
loading
empty
ready
error
forbidden
conflict
retryable failure
feature_unavailable（仅 capability-gated 功能）
insufficient_resource
```

任何 Figma 示例数据不能代替真实状态。

账号、Onboarding 和资源域的详细故障/并发矩阵分别由 **PR #283/#284** 负责。

---

## 13. 明确延期

```text
企业成员完整管理
自定义企业角色
成员月度消费限额
企业人民币钱包
在线支付/支付回调/对账
AI 点数/数据额度在线购买
store_renewal_period 在线购买/自助充值
退款/发票/完整账单
个人实名认证/企业认证提交审核
推广/收益/提现
平台工作区新页面
运营驾驶舱及其他市场/工具模块
```

注意：延期“平台工作区新页面”不等于删除仍被现有生产用户使用的旧 platform capability；其临时可达性遵循 §5.3。

---

## 14. 分拆后的权威关系

```text
PR #281
→ Console Product Decisions、Figma、页面、术语、能力迁移、系统边界

PR #283 V7
→ 手机号自助注册 + Registration Intent + Onboarding + Consent + base_payg + 首次体验 Grant 触发 + 最后角色授予

PR #284 V7
→ 企业资源账本 + trusted Grant + Reservation/Settlement + Store Activate/Renew/Reactivate
```

若三份设计发生冲突：

- **产品行为/命名/Phase1 范围**以 #281 最新 Product Decision 为准；
- 身份注册/Onboarding 的实现细节以 #283 V7 为准；
- 资源/店铺服务一致性实现细节以 #284 V7 为准；
- 下游 V7 必须同步已确认的新 Product Decision，不能保留相反的历史假设。

---

## 15. Review Stop Rule

本文件是产品与架构边界，不以“Reviewer 0 finding”为完成目标。

只有以下 finding 可以重新打开 #281 Product Contract：

- Phase1 用户核心路径无法闭环；
- 已有生产能力因为 Hard Cut 被无替代删除；
- 页面承诺了实际上没有 authority/capability/resource acquisition 的动作；
- #281 Product Decision 与 #283/#284 Implementation Baseline 直接矛盾；
- 跨租户/越权或明确泄露不应公开的账户/业务资料。

Provider finality、Registration recovery、Resource locking、Migration concurrency 等具体实现问题必须回所属 #283/#284，不在 #281 重建第二份协议。

---

## 16. 完成定义

#281 合并前只要求确认：

- 产品术语、页面和 Figma 边界稳定；
- ZITADEL/Login V2/Auth.js/task-processor 职责边界明确；
- Account Existence 可公开、技术邮箱不冒充真实邮箱；
- generic Login V2 入口保留既有 email-only 等历史身份，不因手机号产品化被锁死；
- OTP→OIDC、手机号 Password、Phone-only Reset、Authenticated Password Management 都有明确 Capability Gate；
- `/login` + 4 个明确认证入口有 allowlisted Login V2 entry routing；
- 新 Console 是 UI/IA Hard Cut，但未替代的既有生产能力不会消失；
- 新直接注册企业有 1 个 `store_renewal_period`，Phase1 首次 Store 激活路径闭环；
- 在线续费资源购买延期时，Renew/Reactivate customer UI 不承诺无资金来源的续费闭环；
- 账号可靠性和资源账本算法已从本 PR 移到 #283/#284，不再在 Console PR 内重复展开。

**原则：身份能力优先复用 ZITADEL；业务事实留在业务域；Hard Cut 不制造能力回归；没有真实 authority/resource acquisition 的动作就 feature-gate；复杂一致性问题只在所属领域实现基线中解决。**
