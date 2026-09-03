# 硕米智能引擎第一阶段硬切产品与架构契约

**日期：** 2026-09-02  
**状态：** 已确认  
**代码库：** `qq550723504/task-processor`  
**设计来源：** Figma 文件 `tg48P46SSXl6TBy9lZwg63`，页面 `31:463`

> 本文只保留 Console 第一阶段的产品口径、页面边界和高层架构决策。手机号自助注册/首次业务开通的详细可靠性设计已拆到 **PR #283**；企业资源账本和店铺服务生命周期的详细一致性设计已拆到 **PR #284**。本文不再定义这两个领域的数据库状态机、幂等算法、补偿或清理协议。

---

## 1. 核心原则

1. **同一代码库、同一 Console 基座、两种工作区模式。** 第一阶段交付 `tenant`，`platform` 保留架构入口但不创建空壳页面。
2. **新 Console 直接硬切。** 不搬运旧 `/listing-kits/*` 页面，不保留旧导航，不建立长期视觉兼容层。
3. **复用能力，不复用旧产品界面。** ZITADEL/Auth.js、多企业上下文、Store Center、Subscription/Usage 等现有基础设施继续复用。
4. **身份问题交给 ZITADEL。** 不在 task-processor 重建密码、OTP、Session、OIDC 或长期手机号身份目录。
5. **业务事实归业务域。** 套餐、企业权益、店铺服务、资源余额不放进 ZITADEL。
6. **真实数据优先。** Figma 示例用户名、金额、店铺数、余额和认证状态不得成为生产默认数据。
7. **先交付闭环能力。** 支付、人民币钱包、成员自定义 RBAC、实名认证、退款、发票、推广收益等拆到后续阶段。

---

## 2. 已确认产品口径

### 2.1 账号

```text
注册必填：手机号 + 短信验证码 + 协议确认
注册不填：展示名称、用户名、密码、邮箱、经营画像
默认登录：手机号 + 短信验证码
可选登录：后台设置密码后，支持手机号 + 密码
```

Figma 中“用户名”统一解释为**展示名称**，不作为登录账号。产品不支持“用户名 + 密码”登录。

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
手机号注册
手机号验证码登录
手机号密码登录（上游能力验证通过后开放）
忘记密码/重置密码
标准 Auth.js 会话
安全 returnTo
```

技术边界：

```text
ZITADEL 官方 Login V2
→ OTP / Password / Reset / Session / OIDC

硕米手机号自助注册与首次业务开通
→ 详细设计见 PR #283
```

企业邀请不进入账号入口 Slice 1，后续归“企业空间 / 成员管理”。

### 4.3 Console Shell

- 新品牌区、左侧导航、顶部栏、内容区和移动端抽屉；
- 语义化浅色/深色 Token；
- 当前企业上下文、切换保护、请求取消与缓存清理；
- 联系客服、通知、用户菜单等真实入口；
- 未上线模块不渲染导航，不创建“建设中”页面。

### 4.4 我的账户

第一阶段包括：

```text
账户总览
账户资料
展示名称/地区等普通资料
手机号/邮箱/密码状态
设置或修改登录密码
经营画像
企业空间总览
真实空状态/无权限状态
```

展示名称只用于展示和审计协作，不作为凭据。

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

`base_payg`、初始订阅和企业资源账本的实现分别归 **PR #283** 和 **PR #284**。本文不定义账本表结构和资源事务算法。

### 4.6 店铺中心

第一阶段产品行为：

```text
店铺绑定
店铺列表/详情/编辑
连接状态与服务状态分离
显式激活
使用中续费
到期后重新激活
```

组织隔离和现有 Store Center 能力继续复用。资源扣减与 Store Service 一致性见 **PR #284**。

---

## 5. 路由契约

### 5.1 公开入口

```text
/register
/login?method=otp
/login?method=password
/forgot-password
```

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

### 5.3 硬切规则

- 新导航不生成 `/listing-kits/*`；
- 新页面不 iframe/嵌入旧页面；
- 不维护两套长期前端；
- 旧后端能力只有新页面仍依赖时才保留；
- 新工作区切换后旧入口从用户侧移除。

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

手机号自助注册时不可避免的临时 Registration Intent、Provider Provisioning 和首次业务开通边界，独立在 **PR #283** 评审。

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
续费期数初始 0
AI 点数初始 0
数据额度初始 0
```

如何在现有 `internal/listingsubscription` 中原子应用且不覆盖历史套餐，见 **PR #283**。

历史 `basic / professional / enterprise` 不在本 PR 定义迁移策略。

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
- 服务类动作才消耗续费期；
- AI 点数/数据条数消耗资源，不再次直接扣人民币；
- 人民币钱包未来独立设计。

Bucket、Operation、Reservation、Event、补偿和 Store 事务边界见 **PR #284**。

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
退款/发票/完整账单
个人实名认证/企业认证提交审核
推广/收益/提现
平台工作区页面
运营驾驶舱及其他市场/工具模块
```

---

## 14. 分拆后的权威关系

```text
PR #281
→ Console 产品契约、Figma、页面、术语、架构边界

PR #283
→ 手机号自助注册 + Registration Intent + Onboarding + Consent + base_payg + 最后角色授予

PR #284
→ 企业资源账本 + Reservation/Settlement + Store Activate/Renew/Reactivate
```

若三份设计发生冲突：

- 身份注册/Onboarding 细节以 #283 为准；
- 资源/店铺服务一致性细节以 #284 为准；
- 页面、产品命名、信息架构以 #281 为准。

---

## 15. 完成定义

#281 合并前只要求确认：

- 产品术语和页面边界稳定；
- ZITADEL/Auth.js/task-processor 职责边界明确；
- 新 Console 硬切策略明确；
- 第一阶段路由、导航和 Figma 基线明确；
- 账号可靠性和资源账本算法已从本 PR 移到独立设计，不再在 Console PR 内重复展开。

**原则：身份能力优先复用 ZITADEL；业务事实留在业务域；高复杂度一致性问题拆到所属领域单独评审。**