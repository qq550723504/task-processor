# 硕米账号入口架构边界：ZITADEL Login V2 + Auth.js

**状态：** 已确认

> 本文只定义账号入口的系统职责，不再定义手机号注册的数据库状态机、Provider 恢复、Consent 事务、`base_payg` 初始化或 Cleanup 算法。上述详细设计已拆到 **PR #283**。

---

## 1. 核心决策

账号入口不在 `task-processor` 中重建 IAM。

```text
ZITADEL 官方 Login V2 Fork
├── Login UI
├── Session Cookie
├── Password / Reset
├── OTP SMS
├── AuthRequest
└── CreateCallback
        ↓
ZITADEL
├── User / Phone / Password
├── Session / OTP Factor
├── Organization
├── Project Authorization
└── OIDC Token
        ↓
listingkit-ui
└── Auth.js ZITADEL Provider
        ↓
task-processor
└── 只处理硕米业务事实
```

第一阶段只对 Login V2 做两类修改：

```text
1. 按硕米 Figma 定制视觉
2. 增加手机号自助注册的最小入口/桥接
```

---

## 2. ZITADEL 负责什么

ZITADEL 是身份权威：

```text
用户 ID
手机号/login name
密码与密码策略
短信 OTP 因子与验证
Session
Organization
Project Authorization
OIDC/OAuth2 Token
```

`task-processor` 不保存第二套密码、密码哈希、OTP code 或登录 Session。

---

## 3. Login V2 Fork 负责什么

Login Fork 是独立部署的认证前端：

```text
login.shuomi.example
```

负责：

- 复用官方 Login V2 Session/OIDC 结构；
- 手机号验证码登录；
- 手机号密码登录（当前版本验证通过后开放）；
- Password Reset；
- 手机号自助注册 UI；
- 硕米品牌视觉；
- 调用内部 Registration/Onboarding 能力完成首次业务开通桥接。

Login Fork 不成为第二个业务后端，不存企业权益、套餐余额或店铺数据。

---

## 4. listingkit-ui / Auth.js 负责什么

`listingkit-ui` 继续使用现有 Auth.js ZITADEL Provider：

```text
OIDC authorize
code exchange
access_token / id_token / refresh_token
Token refresh
业务浏览器 Session
```

`listingkit-ui` 不复制 Login V2 页面，不接收 ZITADEL 管理凭据，不自行验证 OTP/Password。

公开 `/login`、`/register` 只负责进入标准 authorize/Login V2 流程。

---

## 5. task-processor 负责什么

身份认证完成后，`task-processor` 只管理硕米业务：

```text
业务用户/企业投影
套餐和企业权益
协议接受证据
店铺
资源账本
业务权限与审计
```

手机号自助注册所需的短期 Registration Intent、Provider Provisioning、首次 Consent/`base_payg`/角色顺序只属于“首次业务开通”差异，不升级为通用 IAM 框架；详细设计见 **PR #283**。

---

## 6. 明确不建设的自研轮子

账号入口第一阶段不建设：

```text
internal/accountentry 通用身份状态机
PhoneIdentity 长期表
FingerprintAlias / Phone HMAC 长期索引
业务侧 OTP 验证器
业务侧 Password 验证器
Password Decoy User / Decoy Pool
自研 OIDC Callback Delivery / ACK
登录 Temporal Workflow
登录专用通用 Operation Runtime
```

如果 ZITADEL 官方 Login V2 在某个具体安全点不足，先证明缺口，再在 ZITADEL Policy、Ingress 或 Login Fork 最窄位置补齐。

---

## 7. 手机号产品语义

```text
canonical login name = E.164 手机号
```

目标：

```text
一个手机号对应一个 ZITADEL 可登录身份
手机号可以走 OTP 登录
设置密码后可以走 Password 登录
```

唯一性事实以 ZITADEL 为准，不在 task-processor 建长期手机号身份目录。

自助注册的并发、稳定 Provider ID、Pending 生命周期和 Provider Object 清理问题见 **PR #283**。

---

## 8. 注册 UI 契约

注册页只包含：

```text
手机号
短信验证码
服务协议 + 隐私政策
注册并进入
已有账号？立即登录
```

不包含：

```text
用户名
展示名称
注册密码
邮箱
经营画像
```

Figma 注册基线：`374:325`，字段按上述产品口径裁剪。

---

## 9. 登录 UI 契约

支持：

```text
手机号验证码登录
手机号密码登录
忘记密码
```

若当前 ZITADEL/Login V2 版本不能可靠支持手机号 login name 的 Password Flow，第一阶段隐藏密码 Tab；不得为了保留界面而新增业务侧密码认证。

---

## 10. Password Reset

密码属于 ZITADEL。

重置流程继续使用官方 Login V2 / ZITADEL Password Reset 能力。

`task-processor` 不生成或保存 Reset Token。

---

## 11. 安全边界

第一阶段必须真实验证：

```text
OTP 发送冷却
OTP 错误尝试上限
Password lockout
Origin / Host / Trusted Domain
Session Cookie 属性
可信代理链
浏览器伪造 X-Forwarded-* 不改变安全判断
账号存在性不可通过明显 UI/错误枚举
```

安全补丁优先级：

```text
ZITADEL Policy
→ Ingress/CDN
→ Login V2 Fork
→ 最后才考虑业务后端
```

---

## 12. 企业和角色

ZITADEL Project Role 继续作为第一阶段粗粒度身份授权：

```text
listingkit_viewer
listingkit_operator
listingkit_admin
platform_admin
```

登录成功不等于可以访问任意企业。WorkBench 敏感操作仍要求 live Effective Organization 和对应权限。

首次注册什么时候授予 `listingkit_admin`、如何保证 Consent/套餐先完成，见 **PR #283**。

---

## 13. Actions v2

ZITADEL Actions v2 第一阶段不是登录强依赖。

可以后续用于：

```text
User / Organization / Role 变化
→ 唤醒 task-processor 投影/缓存收敛
```

不用于：

```text
OTP 校验
Password 判定
维持浏览器 Login Flow
成为 base_payg 唯一初始化事务
```

---

## 14. 企业邀请

企业邀请不进入账号入口 Slice 1。

归属：

```text
企业空间
→ 成员管理
```

后续单独定义邀请 Token、角色白名单、LiveWrite Organization 和权限升级边界。

---

## 15. PR 分拆关系

```text
PR #281
→ 账号架构边界、Console 产品契约、Figma

PR #283
→ 手机号自助注册 + Registration Intent + Provider Provisioning + Onboarding

PR #284
→ 企业资源账本 + Store Service
```

本文与 #283 冲突时，手机号注册/Onboarding 实现细节以 #283 为准。

---

## 16. 完成定义

#281 在账号入口方面只要求：

- ZITADEL/Login V2/Auth.js/task-processor 职责边界明确；
- 注册/登录页面字段与 Figma 产品口径稳定；
- 不再计划自研密码、OTP、Session、Callback 或长期手机号身份系统；
- 手机号注册的复杂可靠性问题已移动到 #283 单独评审。

**原则：能由 ZITADEL 提供的身份能力不重复实现；硕米只维护真正属于自己的首次业务开通与业务数据。**