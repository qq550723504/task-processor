# 个人与企业主体认证：腾讯电子签接入设计候选

> Status: **企业首次引导认证（§13）：IMPLEMENTATION_READY；个人首次绑定：NOT_READY**。§13 限定本轮企业实现范围，其余候选不成为生产合同。
>
> Issue: #510；Design Basis: Independent Architecture；日期：2026-09-26。
>
> 仓库基线：`main @ 7dc9a19a5043169cb4e014909da0549aa2a921cc`。
>
> 2026-09-26 用户已要求开始实现 #510，并确认下述首版产品边界；实现须先满足 Architecture First。供应商准入由用户答复已确认，后续技术选项授权 Agent 决定；本轮选择企业版自建应用。该选择不是账号已核验或供应商行为的实测证明，也不授权采购、收费调用或真实证件处理。

## 1. 用户结果与范围纠正

目标是在硕米“我的账户 → 认证信息”中发起个人或企业主体认证，完成供应商要求的核验，并回读与正确主体绑定的真实结果。

这不是登录 Authentication、手机/邮箱控制权验证，也不是统一 Auth API 的重构。#510 原先的 `AUTH-1` / `auth-backend-api-v1.md` 方向被本次主体认证决定替代；不将抽取 `account_identity.go`、新增 `accountidentity` 或注册/Onboarding 作为本轮前置。

产品入口由 [Figma Authority](../product/final-ui-ia-authority.md) 决定。此前直接核对的 Figma `tg48P46SSXl6TBy9lZwg63` / `1514:815` 包含个人认证、企业认证入口、证件录入及提交后核验语义。本轮用户已接受证件和人脸由腾讯托管采集：硕米保留入口和状态投影，不再同时建设本站证件上传。企业具体节点、终端和状态映射仍待核对；本决定不修改 Figma 原稿。

当前 [Account VerificationPanel](../../web/listingkit-ui/src/components/workbench/account/account-views.tsx) 已把主体认证标为暂不可用，并与联系方式验证分开。没有可靠认证结论时继续 unavailable；不把 ZITADEL 邮箱/手机号 verified 或组织 grant 当成实名/企业认证。

## 2. 本轮决定与非目标

- 第一候选：腾讯电子签的个人实名与企业认证链接/结果接口；不是泛指腾讯云 OCR 或慧眼 API。
- 用户授权 Agent 决定具体方案后，本轮选择企业版自建应用与官方 `ess` Go SDK；不混用第三方应用/子客合同。如果实际开通模式不同，须按实际接口调整，不能隐式切换。
- 用户本轮已确认：个人允许复用供应商已有实名，不承诺每次重新活体；企业必须核实本次经办授权；认证不自动增加硕米平台权限。允许复用实名不是允许跳过账户与实际主体绑定。
- 阿里云仅为备选，不双接入，不跨厂商自动重试。企业要素一致性核验不能自动替代企业经办人授权核验。
- 优先托管核验，不预建证件存储、OCR 平台、人脸算法或内部审核工作台。正常通过后不默认再加一次内部人工审核。
- 不改变 ZITADEL / Auth.js 登录与会话、`authidentity`、`workbenchcontext` 或现有 `authz` 的事实来源。
- 不新增企业成员权限、钱包能力、签约/印章/自动签能力；认证通过不自动授予管理员权限，不自动放行提现或其他业务。
- 不新增数据库实例、通用 KYC 平台、跨供应商路由或独立工作流平台。后续如需持久化，先评估现有业务 PostgreSQL 的合适逻辑库与角色；本文不创建表或指定未经检查的共享连接。

## 3. 已核对的公开接口与限制

下表是 2026-09-26 对官方公开文档的核对，不是已开通账号的运行测试。四接口 API 版本均为 `2020-11-11`，企业版候选 SDK 固定为 `github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/ess v1.3.184`（源码提交 `17f73d9c25cefa5980f6aba5d296de1002aa61e5`）。已核对该版本的请求模型及四个 `WithContext` 方法；尚未引入项目依赖。此基线只适用于当前 `ess` 企业版候选，不能据此假定第三方应用使用相同合同。

| 能力 | 候选接口 | 已知限制及设计含义 |
| --- | --- | --- |
| 发起个人实名 | [CreateUserVerifyUrl][p1] | 需要单独实名套餐；可指定终端类型。已有实名认证用户可能直接进入成功页面，不能据此宣称本次做了新的活体检测。明确传 Endpoint，不依赖文档中不一致的默认值描述。 |
| 查询个人实名状态 | [DescribeUserVerifyStatus][p2] | 限本公司通过指定链接引导认证的用户；查询使用姓名、证件类型和号码，返回 VerifyStatus。它不是通过 RequestId 查询本次申请的接口，也不是任意用户的实名目录。 |
| 发起企业认证 | [CreateOrganizationAuthUrl][p3] | 需要单独套餐；支持法人授权、授权书加对公打款，以及不同终端。预填企业信息不一定锁定，相关 Same 参数需按批准范围显式设置。不要额外启用印章/签署初始化。 |
| 查询待认证企业 | [DescribeOrganizationAuthStatus][p4] | 查询本公司引导的企业，返回 AuthStatus、IsVerified、AuthRecords 等；可使用认证流标识的场景需核实标识实际来源。全企业已认证不等于本次经办人授权已通过。 |
| 异步结果 | [企业与员工回调说明][p5] | 个人 UserAccountVerify、企业 CreateOrganization 等事件包含供应商主体及关联信息。回调存在脱敏字段；不能假定可还原完整证件号码，或只按手机号后四位匹配。 |

公开文档中的 `Operator.UserId` 是供应商调用操作人，不是硕米当前用户 ID；供应商 OrganizationId 也不是 ZITADEL Organization ID。适配层必须显式区分。

个人创建接口返回的 RequestId 用于请求定位，不是通用认证申请编号；UserData 透传可以关联本地申请，但不是供应商幂等保证、签名或本人身份证明。四个接口的创建/查询能力不能被包装成一个未经证明的 `CreateAndCertify` 原子操作。

### 3.1 四接口合同核对（公开资料，不是实测）

所有调用均由服务端使用腾讯凭据和供应商 `Operator`；客户端不能覆盖该身份。供应商权限与硕米本人/当前组织权限分别检查。具体套餐、调用权限和计费规则以用户待补充的开通信息为准。

| 接口 | 必要输入 / 主要输出 | 绑定与权限含义 | 副作用、失败与恢复 |
| --- | --- | --- | --- |
| [CreateUserVerifyUrl][p1] | `Operator`；可传姓名、证件、手机、`UserData`；显式 `Endpoint`。返回链接、`ExpireTime`、可选小程序 ID、`RequestId`。 | 请求模型没有个人字段 `Same` 锁定开关；公开说明没有完整解释错人打开或已实名复用时的锁定和证明合同。不能从“可传参数”推导不可修改。 | 生成有效期 7 天的链接；响应丢失不能安全重建。未发现按本地幂等键找回原链接的公开合同。 |
| [DescribeUserVerifyStatus][p2] | `Operator`、完整姓名、证件类型和号码；只返回 `VerifyStatus`、`RequestId`。 | 查询本公司经指定链接引导的实名记录；没有本次申请 ID、实际操作人 ID 或会话证明。查询 true 不能独立给当前硕米账户认证。 | 查询而非创建；false 不代表一次申请已被拒绝。托管采集决定不自动授权硕米收集/长期保存查询所需完整证件。 |
| [CreateOrganizationAuthUrl][p3] | `Operator`；企业标识、经办信息、`AuthorizationTypes`、`UserData`、显式 `Endpoint`。返回链接、`ExpiredTime`、`RequestId`。 | 可明确锁定企业和经办字段的 `Same` 参数，仅对有值的字段生效；授权方式为法人授权或授权书加对公打款。省略印章/签署初始化。 | 链接有效期 30 天；创建无公开幂等键。已引导认证企业再次进入可能报已认证且不再回调，不能把重复创建作为重新经办授权。 |
| [DescribeOrganizationAuthStatus][p4] | `Operator`；企业名/信用代码至少一个；可带 `AuthorizationInfoId`。返回 `IsVerified`、`AuthStatus`、`AuthRecords`、成功时的腾讯企业 ID。 | 认证流标识来自回调，不来自创建响应。全企业结果与本次经办结论分开；`AuthRecords` 的脱敏经办字段不能用于唯一身份匹配。 | 未开通白名单有明确拒绝码；超时/暂无记录不等于拒绝。查询无法凭空补出未绑定到本申请的认证流。 |

版本证据：[官方 SDK 模型][sdk-models]、[官方 SDK 客户端][sdk-client]。`ess/v20201111` 是 API 包版本，`v1.3.184` 是 Go module 版本；它们不是账号已开通或功能已验证的证明。

### 3.2 回调可确认部分与剩余合同缺口

[企业版回调协议][p6]给出原始 HTTP body 的 `Content-Signature` HMAC-SHA256 校验及可配置的 AES-256-CBC 加密；接入须开启签名校验，不能只解密就视为可信。持久化完成后才 ACK 200；按应用隔离和消息 ID 去重，不因重放重复更新结论。这些是可复用的协议能力，无需新建回调分发平台。

[企业与员工回调][p5]中的个人事件包含供应商用户 ID、姓名、脱敏证件/手机和透传数据；企业事件包含企业与超管信息。`OrganizationAuthorization` 的认证流字段名与查询参数不同，不能直接按名称推定合同。签名和 `UserData` 解决来源/关联，仍不解决硕米账户与实际操作主体的一致性。

按本轮独立边界检查分类：

1. **个人 / BLOCKER**：当前 Must 是正确身份归属，命中“错误身份所有权”。最小缺失事实是预填的已验证手机号是否约束实际实名人，已有实名复用是否同样约束并返回本次关联。未证明前，不按掩码匹配或查询 true 确认当前账户；仅阻塞个人路径。
2. **企业 / IMPLEMENTATION_TEST**：使用 §13 的单一成功事件证明链即可覆盖首个引导认证。[官方 FAQ][p7]中已引导企业再次进入无回调的限制，只影响再认证；本轮不实现再次申请/换绑，不要求每次新提交授权书。已有 SaaS 认证企业首次被本应用引导的真实表现留 G4 验证。
3. **未知创建结果 / IMPLEMENTATION_TEST**：公开创建没有客户端幂等键，故本轮保留原申请为 `OUTCOME_UNKNOWN`，不自动重建、不持续轮询、不宣称失败或未计费。迟到有效回调仍可完成原申请。供应商找回接口不再作为该停止协议的开工前置。

这些分类不宣称供应商不支持未覆盖能力。真实样例仍为 NOT_RUN；个人准入不足不得拖住已经可证明的企业边界。

还核对了两个相邻接口：[DescribeThirdPartyAuthCode][p8]用于带白名单的电子签小程序到客户小程序引流，返回一次性码对应的实名状态，不能直接替代本站网页的身份绑定；[CreateModifyAdminAuthorizationUrl][p9]只允许审核失败后的原认证人补交超管授权书，不能作为任意已认证企业的新经办授权接口。本轮不引入两者，也不通过员工/签约 API 改变原本的非签约范围。

## 4. 轻量业务接入与职责

```text
硕米认证页面 / 同源 BFF
  → 已验证 UserID；企业请求再 live resolve Organization 与适用权限
  → 主体认证应用：登记申请、固定主体/actor/用途/版本
  → 腾讯电子签适配器：创建链接
  → 用户在供应商页面完成必要动作
  → 经验证回调 + 适用的服务端查询/核对
  → 主体认证应用：保存证据与结论
  → 页面回读脱敏状态
```

| 职责 | 所有者 |
| --- | --- |
| 登录用户、会话、组织访问权限 | 既有 ZITADEL / Auth.js / authidentity / workbenchcontext / authz |
| 实名核验、企业核验、供应商支持的授权审核 | 已选定并开通的供应商服务 |
| 本地申请、主体绑定、核验方法/证据引用、最终产品状态 | 有界主体认证模块；包名/数据库角色在 G2/G3 确认 |
| 供应商签名、SDK 类型与错误/回调解析 | 单一腾讯电子签适配器，不能进入领域模型或浏览器 |
| HTTP 启动与模块注入 | 现有 app/httpapi 装配层，不在这里实现认证状态机 |

目标依赖为 `应用装配 → 主体认证应用的窄合同 ← 腾讯电子签适配器`；领域不 import SDK 或 app/httpapi，适配器由应用注入。候选窄能力为创建个人/企业链接、获取适用结果和解析验证后的通知，不增加 Login/Register/OTP/Grant 等操作。

## 5. 主体与结果绑定必须先证明

个人 subject 由服务端当前 UserID 确定，不能接受浏览器指定他人 userId。企业 subject 由已验证 EffectiveOrganizationID 确定，并独立记录 actor UserID、预期企业标识与经办资格证据。

以下条件不能直接使申请成为 verified：

- 浏览器回跳带有 success 参数，或用户点击“我已完成”；
- 仅收到未验证来源的回调或找到 UserData；
- 按用户输入的姓名/证件号查询到另一个人已有实名；
- 企业名称/信用代码匹配，但实际完成认证的企业或经办人不匹配；
- 只有企业全局 IsVerified，没有本次要求的授权证据。

G2 必须证明原申请、硕米 subject、供应商实际实名主体和要求的授权证据如何连接。须覆盖链接转发给别人、另一个已实名用户打开、预填字段变更等样例。不能仅凭随机 UserData 或脱敏字段相似度宣称本人绑定已成立。个人查询需要完整输入而回调可能只给脱敏信息时，应冻结必要输入的来源/最小保留方式或选择已证实的其他证明接口；未解决则该能力 NOT_READY。

供应商企业认证或电子签授权不等于已获得“代表企业使用硕米所有业务”的授权；G1/G3 须界定结果使用范围。若产品只接收较弱的资料核验，必须单独批准并明确展示，不静默降级为已实名。

## 6. 本地状态、幂等与恢复候选

只定义三类必要事实，不要求三套服务或三张表：申请（subject/actor/用途/版本）、供应商观察（来源与可核对的关联）、认证结论（方法/证据/核验时间及适用范围）。完整 schema 与状态转换表在 G2/G3 通过后冻结。

- 首次外部创建前，先持久化本地申请和同主体、同用途的幂等身份。同键同输入回读原申请；同键不同输入冲突。
- 创建响应丢失/超时不能假定未创建、未计费或认证失败。保留结果待核实；不自动重新生成链接、不换供应商重试。供应商能否找回原请求、原链接或最终结果属于 G2，不能以 UserData 透传代替。
- 浏览器回来只触发本地回读/获准的核对，不写成功。普通 GET 只读本地状态，不偷偷发起新认证、短信或收费动作。
- 回调先按确切供应商协议验证来源/完整性、应用与事件类型，再对原申请去重及核对。回调 ACK、重试、乱序处理必须在 G2 固定；未经持久化不得先报已处理。
- 只有原申请绑定和核验标准满足时，服务端才能确认通过；观察与结论更新应原子。重复通知不能重复授信/授权，也不产生第二份认证权益。
- 查询返回 false、暂无记录、超时、未知状态均不自动等同 rejected。链接过期与认证记录有效性是两件事；旧申请迟到事件不得覆盖新版本结论。
- 单一主体认证应用负责恢复；优先复用现有任务机制或有界状态核对，不为此建设通用 reconciler 平台。频率、期限、停止条件及凭据权限须在实施前明确，禁止无限轮询或依赖用户永久等待。

## 7. 候选 API 职责（尚未冻结，不能据此接生产路由）

| 动作 | 候选路径 | 边界 |
| --- | --- | --- |
| 本人认证摘要 | GET `/api/v1/account/verification` | CurrentIdentity，个人结果与企业选择分离 |
| 发起本人认证 | POST `/api/v1/account/verification/applications` | 本人、明确用途/同意、幂等与供应商费用限制 |
| 当前企业认证摘要 | GET `/api/v1/account/organization/verification` | 已验证组织与适用读取权限 |
| 发起企业认证 | POST `/api/v1/account/organization/verification/applications` | LiveWrite；适用企业提交权限和经办资格不得混为一谈 |
| 原申请状态/显式核对 | GET `/api/v1/verification/applications/:id`；POST 同路径 `/refresh` | 每次检查原申请主体与权限；refresh 只核对原申请，不创建新认证 |
| 供应商通知 | POST `/api/v1/callbacks/tencent-esign/verification` | 供应商协议验证，不使用浏览器会话，也不能是无验证的 public mutation |

权限名称、请求字段、HTTP 状态与 DTO 在 G2/G3 后一次性固定。复用现有 BFF 的同源写保护与服务端 token 转发；不把供应商凭据、原始错误、证件材料或可使用的认证链接放进审计日志。客户只看到当前授权范围内的状态、核验方法、时间和下一步动作。

## 8. 隐私、材料与成本边界

优先由供应商采集证件照片和生物信息；不为 Figma 的上传控件提前自建证件桶。托管不等于“硕米零个人数据”：查询可能需要完整姓名/证件号，回调也可能含身份数据，须在 G3 冻结必要字段、加密/脱敏、访问者、留存期限及删除方式。

认证链接按短期敏感凭据处理：只向授权申请人提供；不放日志、公开 Issue、截图报告或持久分享链接。不把真实手机号、证件号放 UserData；由服务器生成不透明关联值。不要从供应商链接中提取或拼接未定义参数。

供应商调用成本与向硕米用户收费是两个决定。本轮不新增用户扣费或价格策略；真实测试前另行批准次数/预算和测试主体。开通、按次查询、失败/重复/已实名复用是否计费须由供应商确认。

## 9. 待决项与阻塞层级

| Gate | 待确认与完成证据 | 负责方 | 阻塞什么 |
| --- | --- | --- | --- |
| G1 服务准入 | 用户已确认非签约用途、套餐和接口权限，并授权 Agent 选择企业版自建应用；实际账号模式、终端/结果用途/测试资源和费用仍按实际授权核对 | 用户/商务与腾讯客户经理 | 不再把方案选择作为待用户决定；账号配置与真实调用另核对，费用未定不阻塞公开核对 |
| G2 技术合同 | 账号可用的版本与 SDK；个人/企业真实主体绑定；四个 API 的查询材料/流程标识；创建未知结果、已实名用户、回调校验/重放/乱序、过期/撤销语义 | 架构/执行 Agent，必要时供应商技术支持 | 缺失项对应的正式实现 NOT_READY；可继续文档和只读核对 |
| G3 产品与数据 | 已确认腾讯托管采集、可复用个人实名、必须核实本次企业经办授权、认证不增权；余项为终端/准确 UI 状态映射、更名/失效/重认证和重复主体、必要字段留存/告知与权限 | 用户/产品与适用数据合规负责人 | 只阻塞未冻结边界，不重复询问已批准四项；未批准不收真实材料、不改变全局业务准入 |
| G4 受控样例 | 个人、法人企业、非法人经办企业三条真实支持流程及结果证据；测试主体、环境、次数、费用和清理动作单独授权 | 获授权执行 Agent/用户 | 阻塞各路径实际开放，不自动阻塞已冻结边界内的离线开发；不能用 fixture 冒充 |

G1–G3 中影响本次实现边界的事项解决、独立架构评审通过后，才允许将相应明确范围标为 IMPLEMENTATION_READY。无关范围不要拖住已批准的有界实现；也不能把未批准部分默认放开。最终价格可只阻塞采购/真实调用，不必阻塞无成本的文档核对。

## 10. 可转给腾讯客户经理的确认文本

> 我们在自有电商经营平台“硕米”内，需要同时支持中国大陆个人实名和境内企业主体认证，当前不要求用户签署业务合同。拟使用 CreateUserVerifyUrl、DescribeUserVerifyStatus、CreateOrganizationAuthUrl、DescribeOrganizationAuthStatus。请确认这一非签约用途、适用产品/集成模式、单独实名套餐与白名单、个人/企业 PC 与 H5/手机接续支持、法人及非法人经办流程、已实名复用与当前请求绑定、回调和结果查询范围，以及认证/查询/失败/重复调用的费用。也请提供获准测试方式、三类样例资源、用户授权文本和必要数据处理要求。暂不需要开通印章、合同签署或自动签能力。

该文本仅供沟通，不是已发出的邮件、询价或采购。

## 11. Agent 当前执行细则与停止点

1. 接手原 #510，先读当前正文、AGENTS 与本文；原设计已随 #511 合并，后续沿用本设计文件和一个主要工作分支，不另建竞争 Issue/版本文档，不继续旧 Auth 重构方向。
2. §3.1 已交四接口公开核对表并固定企业版候选 SDK。下一步补齐 §3.2 的供应商技术证据；接口返回有限布尔值时如实记录，不补造认证级别或凭据。
3. 把 G1/G3 用户及供应商待答问题集中一次交接；没有凭据或权限，不索要在聊天中粘贴 Secret，也不调用线上示例假装测试。只读核对与独立隔离 POC 不能进入正式运行路径。
4. 取得明确测试授权后，才运行 G4 三个有界样例；已实名跳过、链接转发/主体不一致、超时/重复通知与回跳不判成功是必要合同核对，不扩建通用审核或故障注入平台。
5. 回填证据后，在原文档中冻结窄合同、持久化/事务、恢复责任与准确 UI/API 映射，完成适用独立评审。没有实际证据保持 NOT_RUN/待确认，不连续制造 V2/V3 文档链。
6. 只有明确 IMPLEMENTATION_READY 后再派业务 Writer。实现、合并、采购、部署、实名提交和数据操作分别授权。

## 12. 官方参考（2026-09-26 公开文档核对）

- [个人认证链接][p1]；[个人状态查询][p2]。
- [企业认证链接][p3]；[企业状态查询][p4]。
- [企业与员工回调][p5]。
- [阿里云企业身份识别][a1]：保留备选核验能力，不据此声称已经具有同等经办人授权流程。

## 13. 本轮企业首次引导认证合同

本节是 §4–§9 候选在企业首申范围的具体化。用户授权 Agent 决定方案后，先交付企业路径，个人仍留在 #510，不写成完成。独立 Reviewer 已完成本节权限、持久化、回调证明与未知结果增量复核，无 BLOCKER；仅本节为 `IMPLEMENTATION_READY`。评审证据写入 #510 和实现 PR。不引入新的认证供应商、登录系统、人工审核或自动恢复平台。

### 13.1 用户路径与 UI

- 入口：`/workbench/account/profile/verification`。当前组织管理员选择企业认证、输入营业执照企业名称/信用代码及可选法人姓名，阅读用途说明并同意；当前已验证大陆手机号由服务端读取，不能由请求覆盖。
- Figma：`tg48P46SSXl6TBy9lZwg63` / 企业 `1514:1043`、个人 `1514:815`，本轮已读取 design context 与截图。复用现有 Console 导航、Panel、Button、表单与颜色；保留认证类型、状态和企业信息布局。依用户新决定，将上传/OCR 区域改为腾讯托管说明和“前往腾讯认证”，不采集证件、人脸或授权书。
- 提交后提供同一申请的受保护腾讯 PC 链接；用户在腾讯完成其要求的法人授权或授权书流程。返回硕米后刷新本地结果；不把浏览器返回、查询 true、手机号已验证直接当企业认证完成。
- 支持企业首次被本应用引导认证（包括腾讯已有实名但本应用首次引导）。更换经办、重新认证、换绑企业、证明持续任职、自动授予任何硕米权限不在本轮。未配置服务或缺手机号/权限时显式 unavailable/denied，个人标签继续明确未开放。

### 13.2 Owner、装配与权限

- `internal/subjectverification` 拥有申请、证明匹配和状态转换；窄 `Store` / `Provider` 合同由 `internal/integration/persistence/subjectverification` 和 `internal/integration/tencentesign` 实现。SDK 只在供应商适配器；HTTP 在 `internal/subjectverification/httpapi`，app/httpapi 只注入与挂载。
- 复用当前账户资料所在业务 PostgreSQL `source_accounts` 的独立认证表；schema owner 为既有 `source_account_owner`，runtime 为既有 `source_account_runtime`，仅授权所需 SELECT/INSERT/UPDATE，不新增数据库实例或身份事实源。通过现有 schema-init 链调用认证 schema；API 启动只验证，不建表。
- 使用当前 `CurrentIdentity + LiveWrite + accountOrganizationTarget` 的有效组织解析；企业读/写均复用管理员操作权限 `workbench.organization_member.manage`。每次请求均按当前 grant 检查，不相信 browser org/user/phone。
- `SelfProfileReader.ReadSelf` 以本次服务端 access token 和已验证 subject 读取现有 UserInfo；只有精确相同 UserID 且 `PhoneNumberVerified=true` 的 `+86` 大陆手机号可发起。企业认证不改变手机号验证或 IAM。
- 回调只记录此前获授权申请的供应商事实，不授予新权限；成员权限撤销不伪造已发出外部流程的撤销。撤权后用户不能读取敏感链接、提交或更改原申请；新请求继续 live resolve。

### 13.3 精确证明链与持久化

创建参数固定 `Endpoint=PC`、`AuthorizationTypes=[2,5]`、`OrganizationNameSame=true`、`UniformSocialCreditCodeSame=true`、`AdminMobileSame=true`；AdminMobile 来自 UserInfo。可选法人姓名非空时同时设置 `LegalNameSame=true`。不传 Initialization、证照图片或授权书。

服务器生成 UUID 申请 ID 与随机不透明关联值，编码为 UserData。只有 `CustomApp / CreateOrganization` 成功事件可以更新结论；必须验原始 body 的 HMAC-SHA256，再按腾讯协议解密，精确匹配原申请 UserData、规范化企业名称/信用代码、完整 AdminMobile，并要求非空供应商 OrganizationId/AdminUserId。手机号比较使用服务端 HMAC；脱敏手机号必定不匹配。来源验证和 UserData 之外，完整手机号与供应商强制锁定共同建立 actor 绑定。不拼接 Authorization/FileReview 的组织、时间推定同次申请。

- `subject_verification_applications`：每个本地组织至多一条首申，包含 application ID、actor ID、scope、幂等键/输入摘要、公司标识、手机号 HMAC/掩码、关联值、状态、加密链接/到期时间、供应商 org/admin ID、供应商认证时间和本地观察时间。作用域包含配置的应用标识，轮换配置不跨应用消费旧申请。
- `subject_verification_messages`：应用+MsgId 唯一，保存原始 body 的摘要、对应申请 ID 和处理结果，不保存原始回调/证件/手机号。相同 MsgId 不同 body 拒绝；同事件重放无重复结论。事务中同时写消息和申请结论，失败不 ACK 200。
- 先插入申请并提交，再调用供应商一次。组织唯一约束解决并发，同 actor/幂等键/同载荷回读原申请；不同载荷或已有另一申请 409，不换键创建第二链接。重启不会再次发送创建。
- 敏感链接使用 Go 标准库 AES-256-GCM、随机 nonce 和申请/组织/actor/app 关联数据加密；独立密钥通过私密配置注入。成功时清除链接。不保存原始手机号、法人姓名、身份证、证照或人脸；摘要与 HMAC 不回给客户端。申请和最小证明作为认证状态依据保留，未授权实际数据删除；实际开放前 G4 同时核对告知/留存要求。

### 13.4 状态、失败与 API

| 当前事实 / 事件 | 持久化与结果 |
| --- | --- |
| 无申请 + 合法 POST | 先保存 `OUTCOME_UNKNOWN`（外部调用可能发生），再发出唯一一次创建 |
| 创建成功、响应可验证 | 原申请 `PENDING`，保存加密链接和供应商到期时间；不得覆盖先到的成功回调 |
| 网络/超时/响应丢失/进程退出 | 保留 `OUTCOME_UNKNOWN`，展示“结果待核实，未重新发起”，无自动重建/无限轮询；同键只回读 |
| 签名有效且所有主体字段精确匹配 | 事务保存消息和 `VERIFIED` 结论/供应商 IDs，清除链接；重复事件不增权 |
| 正确关联但主体字段不匹配 | 不认证，记录消息处理结果并保留原状态；不让错误通知覆盖已有正确结论 |
| 链接到期 | GET 投影为 `EXPIRED`，不返回链接；不把它当认证被撤销，不自动重建，原申请有效迟到回调仍可确认 |

API：GET `/api/v1/account/organization/verification`；POST 同路径 `/applications`；POST `/api/v1/callbacks/tencent-esign/verification`。GET 仅本地读，返回 available/state/company/maskedPhone/timestamps，只有原 actor 可拿未过期链接。POST 输入为企业名称、信用代码、可选法人、consent=true、幂等键；不接受 userId/phone/provider IDs。BFF 使用现有同源检查、服务端 token 与有界请求，不代理公共回调。

请求上限：普通 JSON 8 KiB、供应商回调 64 KiB；普通 API 15s、供应商单次调用 8s、回调 8s。SDK 网络/限频重试关闭；上游错误不原样输出。缺配置返回 FEATURE_UNAVAILABLE，非法输入400、未认证401、权限403、申请冲突409、依赖失败503；可能已发出创建时返回原申请 OUTCOME_UNKNOWN，而不是诱导重提。

### 13.5 验证与开放

TDD 覆盖真实规则：同键/并发只创建一次、跨组织/错 actor/未验证手机号拒绝、签名篡改/脱敏手机号/错公司不通过、重复消息/回调先于创建响应、创建超时/重启不重发。持久化事务使用已有 PostgreSQL 测试方式，UI/BFF 使用既有 Vitest；不建设新验收工具。G4 真实个人/法人/非法人样例仍分别 NOT_RUN，企业 runtime 默认关闭，获独立真实测试授权及通过对应验证后才能开放；开发自检不宣称上线。

[p1]: https://cloud.tencent.com/document/product/1323/105961
[p2]: https://cloud.tencent.com/document/product/1323/106080
[p3]: https://qian.tencent.com/developers/companyApis/organizations/CreateOrganizationAuthUrl/
[p4]: https://cloud.tencent.com/document/product/1323/110844
[p5]: https://qian.tencent.com/developers/company/callback_types_staffs/
[p6]: https://qian.tencent.com/developers/company/callback_types_v2/
[p7]: https://cloud.tencent.com/document/faq/1323/116841
[p8]: https://qian.tencent.com/developers/companyApis/users/DescribeThirdPartyAuthCode/
[p9]: https://qian.tencent.com/developers/companyApis/organizations/CreateModifyAdminAuthorizationUrl/
[sdk-models]: https://github.com/TencentCloud/tencentcloud-sdk-go/blob/17f73d9c25cefa5980f6aba5d296de1002aa61e5/tencentcloud/ess/v20201111/models.go
[sdk-client]: https://github.com/TencentCloud/tencentcloud-sdk-go/blob/17f73d9c25cefa5980f6aba5d296de1002aa61e5/tencentcloud/ess/v20201111/client.go
[a1]: https://help.aliyun.com/zh/id-verification/enterprise-identity-authentication/product-overview/product-introduction-1
