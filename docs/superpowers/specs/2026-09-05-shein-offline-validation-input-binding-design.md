# SHEIN 离线诊断输入、freshness 与组织范围读取设计

日期：2026-09-05。执行依据：[Issue #316](https://github.com/qq550723504/task-processor/issues/316)；Parent #34，后续接入 #315。

状态：**DESIGN_REVIEW_READY；完整入口 BLOCKED_ON_CONTRACT_DECISION（D1：当前归属资料包的生产/证明边界）**。本文是设计提案，不改变已冻结合同，不表示 reader 或新入口已实现。最终独立评审、HEAD、CI 和协调决定保存在 PR/Issue；达到设计准入不自动恢复 #315。

## 1. 范围、依据与不变量

审计 main：`6b06929f5ca77b1af3020ca92c544774c9416901`，启动时相关增量为空。#314 及 main CI 33972706741 已成功；[#315 原始阻塞](https://github.com/qq550723504/task-processor/issues/315#issuecomment-5552615228) 获协调接受。

适用 [AGENTS](../../../AGENTS.md)、[Issue 驱动规则](../../engineering/issue-driven-development.md)、[Legacy Policy](../../refactoring/legacy-hard-cut-policy.md)、[Register](../../refactoring/legacy-register.md)、[已确认 Multi-Organization 合同](2026-08-30-shuomi-workbench-store-center-zitadel-multi-org-design.md)（尤其 §6、8、11、18）、[Validator 当前说明](../../../internal/marketplace/validator/README.md)。无新增 Accepted Risk。授权读取缓存继续遵守既有最多 60 秒且不超过 token/grant 到期时间的合同，不追求零缓存撤权。

Must：授权范围先于资料读取；hash 和校验用同一读取副本；无假时间；已知 stale/expired 不降格；离线结论不能授予写权限；无规则双跑/复制；无平台/模型调用或业务写。Out of Scope：schema、历史 task 迁移、IAM/tenantbridge、新事实库、缓存/刷新服务、UI/Tool/Apply/Agent/submit 接线、部署和真实数据。认证基础设施可能访问 ZITADEL 与其既有授权缓存；“无平台调用”指不为资料校验访问 SHEIN，不声称认证也无 I/O。

本轮只新增本文。建议的后续运行时修改跨身份/读取/计算边界，属于架构敏感工作，需先分片。没有新持久状态机、幂等写入或恢复 owner；Outbox/Saga/UoW 对本次单次读取计算不适用。

## 2. 已有能力与缺口

| 事实 | 源码证据（相对本设计） | 推论/限制 |
| --- | --- | --- |
| v1 Snapshot 强制 revision、evaluation、observation、expiry | [contract.go](../../../internal/marketplace/validator/contract.go)，Snapshot.Validate | 缺时间是错误，不能删校验后沿用 v1 成功语义 |
| Package 含内嵌解析/模板但无整体有效期 | [model.go](../../../internal/publishing/shein/model.go)，Package / *Resolution | 缓存 UpdatedAt、task UpdatedAt、SourceSnapshotVersion 都不等于完整输入版本/有效期 |
| 克隆会保留私有解析 map；Package JSON 有 alias normalization | [package_persistence_clone.go](../../../internal/publishing/shein/package_persistence_clone.go)、[semantic_fields.go](../../../internal/publishing/shein/semantic_fields.go) | 不能给任意内存 Package 做 JSON hash 就宣称覆盖全部状态 |
| preview route 无 OrganizationAccessPolicy | [routes_descriptor_task.go](../../../internal/listingkit/httpapi/routes_descriptor_task.go)、[server_auth.go](../../../internal/app/httpapi/server_auth.go) | 不走已批准工作区组织解析；不是已证实的线上越权结论 |
| 既有身份 verifier 得到 Home；resolver 绑定有效 org 与对应 roles | [verifier.go](../../../internal/authruntime/zitadel/verifier.go)、[resolver.go](../../../internal/workbenchcontext/resolver.go) | Home 不可冒充 Effective；selector 不是 grant |
| 显式 actor 的窄读取模式已存在 | [canonical_subject.go](../../../internal/listing/task/canonical_subject.go)、[task_repo_listing.go](../../../internal/listingkit/store/task_repo_listing.go)，ReadCanonicalSubject；[task_repo_scope.go](../../../internal/listingkit/store/task_repo_scope.go) | 可复用严格 tenant/user SQL 与返回后校验模式；现有 port 只返回 Product subject，没有 SHEIN package |
| preview decorators/在线 freshness 并非纯计算 | [task_preview_service_support.go](../../../internal/listingkit/task_preview_service_support.go)、[submit_freshness_shein_flow_support.go](../../../internal/listingkit/submit_freshness_shein_flow_support.go)、[resolution_cache_store.go](../../../internal/publishing/shein/resolution_cache_store.go) | 不调用 cookie/store decorators、在线模板、sale repair SaveTaskResult 或 cache hit 统计更新 |

## 3. 输入语义：只比较两个方案

| 方案 | 实际代价与 happy path | 决定 |
| --- | --- | --- |
| A：保持 #314 Snapshot 原样 | 需要已有权威整包版本、ObservedAt、ValidUntil；当前读链不存在此来源。新增历史时间/TTL、观察服务或版本持久化均超出本片。所有当前包永久 invalid_input 不是完成 happy path。 | 不推荐作为本入口方案；保留 v1 的严格行为，不伪装已接入 |
| B：版本化的离线内容绑定 + 显式 freshness coverage | 新的输入语义区分内容身份、读取时刻、外部有效性；复用原规则 evaluator。无需补历史数据；无外部证据时只产离线结构结果。需要窄合同修订及调用方切换的独立前置。 | **推荐**，不就地放松 Snapshot.Validate |

拟新增 `shein.offline_package.v2` 的输入/结果语义；rule composition 本身继续复用 v1 现有 owner。中性合同增加独立命名的 BoundInput/DiagnosticResult 类型和校验方法，不把 Snapshot 必填字段改成 optional，不给零值默认成 valid。v1 在无人依赖切换完成后由明确后续切片 RETIRE；若实现时查出消费者则先列调用点及迁移验收。两个版本共享一份计算函数，不运行 v1+v2 双规则判定或 runtime fallback。当前 v1 生产消费者扫描范围为 neutral/SHEIN validator 和全仓 import；已有文档和测试属于合同消费者，必须迁移或保留其有效语义。

推荐请求在认证读取之后构造，客户端不能提交 package、read time、freshness state 或实际 digest：

| 字段 | 来源 | 校验/含义 |
| --- | --- | --- |
| contract_version / binding_version | 服务端已部署版本常量 | 分别固定 v2 与 `shein.persisted-input.go-json.v1`，未知版本拒绝 |
| target、action、rule_version | action/site 来自有界请求，marketplace 固定 SHEIN，rule 由服务端版本匹配 | 只支持显式 save_draft/publish；preview 拒绝；非空 site 拒绝，绝不删除/改写 |
| actual_digest | 服务端从下述唯一副本计算 SHA-256 | 内容绑定不是授权证据/历史 revision/数据库 CAS；不复用 Product key/PublicationIdentity |
| expected_digest（可缺） | 客户端上次获授权的该资源报告 | 有值必须格式合法且与 actual 独立比较；缺省是“评估本次读取”，不意味着接受后续变化 |
| read_at | 已授权 SELECT 完成后的服务端时钟 | 仅副本取得时刻，不命名为 ObservedAt，不参与内容 digest |
| evaluated_at | 调用计算前服务端时钟 | 不早于 read_at；时钟倒退返回 evaluation error，不校正/造假 |
| external_freshness | 下述显式 tagged union；仅可信事实 owner 可供给 | 当前持久资料包读链是 `not_evaluated`，不是 fresh |

### 3.1 内容绑定算法与资源限制

仅支持 **一次持久读取得到的 JSON 资料包**，不支持传入 resolver/cache 产生的任意内存 Package。持久适配器取得同一行的一致 package JSON、归属字段和已存在的权威证据（如有）；不得先读 package 再拼另一时刻模板。读取副本的内嵌模板就是本次结构规则输入，不声称平台现行模板。

1. 在受组织/owner 限制的查询内先限结果大小，再加载 JSON；拟定包上限 2 MiB（新入口资源限额提案，不是 freshness TTL，也不声称现有包均符合）。PostgreSQL 用同一语句 CASE/长度检查避免将超限正文传给应用；不为预检跨两次读取。超限只返回 generic input-too-large，不回正文。具体适配器按 D1 确定，禁止无界加载后才检查。
2. 校验存储 JSON 为有效 UTF-8，拒绝非法数值/编码、超深结构（64 层），严格解码为当前 Package 持久合同；未知字段拒绝，不能静默吞掉未来 freshness/规则字段。自定义 UnmarshalJSON 必须纳入严格解码测试，不能只假设外层 DisallowUnknownFields 会穿透它。若需拒绝重复 key/孤立 surrogate，使用成熟严格 JSON 校验能力或对应存储保证，不能让 Go 的替换行为伪造“相同”内容；该编解码保证属于后续绑定切片的验收。
3. 解码到独占对象，复用当前 publishing 的 normalization/clone；私有 resolution maps 必须来自这一持久解码（当前为空），不得注入内存 resolver 状态。当前持久化合同中的公开 SKC/SKUValueAssignments 包含在 hash 内。若后续规则依赖不可重建的私有字段，停止并提升 binding 版本/字段合同；不得忽略该字段。
4. 用 Go `encoding/json` 编码固定 struct：binding_version、marketplace、site、action、rule_version、完整规范 Package。标准库按 map key 稳定排序；struct 字段顺序冻结、默认转义冻结、不加缩进/尾随换行。数组保留原序与重复项，不擅自当集合排序；nil/empty 按当前持久规范编码处理，`omitempty` 字段可合并，不能宣称保留原始 JSON 的 null/empty 区别。固定向量锁定该语义；若规则区分被折叠值，必须显式覆盖并升级 binding 版本，不能用此编码遗漏差异。所有公开持久字段（含 Metadata、嵌套模板、价格/图片/备注）均覆盖；多绑定无关字段只导致保守失效，不排除可能影响结果的字段。没有新的通用 canonical JSON 框架。
5. 对该字节串做 SHA-256，并给 Validator 使用这次 hash 所对应的独占副本；Validator 可做纯 clone，但不得重新读取/装饰。Package.MarshalJSON 本身会 normalization，必须先在独占副本上完成并确认重复编码稳定。既不 hash 原对象再校验已改对象，也不向响应暴露 package/凭据/profile/原始缓存键。
6. 序列化算法/Package JSON 语义/字段/normalization 变化需升级 binding_version；规则语义变化需升级 rule_version。只承诺指定版本 Go 编码合同，不承诺任意语言 JSON 都产生同摘要。加入跨 map 顺序、nil/empty、alias、SDK 嵌套结构的固定向量测试锁定版本。

读取与授权沿用请求取消，拟定入口总预算 5 秒且不延长上游 deadline；GET body 拒绝，query 仅 action/site/expected_digest，合计上限 1 KiB，task ID/actor 复用现有 listing/task 长度上限。这些是窄入口资源限制提案，不能推导旧调用量已验收。有限输入的同步计算在前后检查 context，不派 goroutine 逃逸；取消即丢弃结果，底层有界计算可能跑完当前步骤，没有持久副作用。响应 findings 总体上限 2 MiB，超过则 evaluation error，不能截断 blockers 后返回成功。

### 3.2 Freshness union 与结果范围

外部 freshness 包含 status 和 coverage。status 必须显式为 `not_evaluated | valid | stale | expired`，没有零值默认为 valid。可信 evidence（若存在）含 subject_digest、policy_version、source、observed_at、valid_until；subject_digest 必须绑定本次副本，observed_at ≤ evaluated_at < valid_until 且 valid_until > observed_at。无权威 policy/source 或矛盾时间为 invalid evidence。客户端输入不能成为证据。

- 当前 package 路径没有整体 evidence，返回 `not_evaluated` 和原因 `no_authoritative_package_freshness`；read_at 不填入 observed_at。
- 已有可信 expired/stale 证据必须保留其原因/范围并返回 stale_input 错误，无成功结构报告；known partial stale 也不降格 unknown。证据 subject 不匹配同样 stale，不悄悄丢弃。读取合同必须返回其 owner 所拥有的全部相关有效性证据；当前无证据是经字段/来源核验的事实，不是 decoder 丢字段的结果。
- 后续需要在线 freshness 的消费模式在 unknown、partial coverage、过期/冲突时全部拒绝；本片不实现在线模式、policy registry 或 evidence 生产者。
- 即使 evidence valid，也只表示其声明范围有效，不能把 cookie/Store/POD、人审或 ApprovedAsset 写成通过。当前无证据时允许的结果为 `offline_checks.status=ready/ready_with_warnings/blocked`，外层 `diagnostic_only=true`、`external_freshness.status=not_evaluated`、明确的 `not_evaluated` 项列表，不设置外层 ready/allowed/can_publish。
- 输出保留 scope、action、rule/binding 版本、digest、read/evaluation time、rule/code/category/paths/guidance/blockers/warnings。ReadinessBlockersAllowed 只放 action_policy 内并注明不是权限，不清除 blockers。not_evaluated 固定含在线模板有效性、Store/cookie、POD、ApprovedAsset provenance/consent、提交前完整 gate。

相同 package/action/rule/binding 产生相同 digest 和结构 findings；不同读取时间不会改变 digest，整个报告含时钟因而不要求逐字相等。相同完整绑定请求的计算结果保持确定性。

### 3.3 并发与失败

SELECT 时一致副本 A → hash A → Validate A。重读已变 B 时，expected=A 返回 stale_input；未指定 expected 则报告 B。计算期间或响应后持久行变 B，A 的报告仍只描述 A；不自动重读拼接、不锁全系统、不写回结果。即使 A→B→A 摘要复原也不是未发生写入的证明。任何写操作继续用其既有写前读取/授权/版本并发控制和完整校验，不接受该报告作 CAS 或许可。

DB/授权依赖失败返回 unavailable；缺包与不可读资源用不泄漏存在性的结果；错误绝不编码成空 blockers 的成功。响应丢失/重试/重启重新完成授权读取，无需去重表/恢复任务，前一次报告不被持久化为事实。请求取消无后台任务。

## 4. 入口与组织范围：两个候选，只推荐一个

| 入口候选 | 成本与边界 | 决定 |
| --- | --- | --- |
| 改现有 `GET /api/v1/listing-kits/tasks/:task_id/preview` | 要改变旧 route 授权、旧身份/任务读取语义和 preview 输出；builder 与 cookie/store decorators 混合，完整 readiness 与其他写调用共享。已确认 multi-org §11 不要求续接旧页面/数据。 | 不推荐；不批量迁旧路由，不替换原完整 readiness |
| 当前 Listing owner 的独立诊断入口 | 窄 HTTP 装配、显式 actor/package read port、当前 Validator 计算；避开 root service/preview/缓存刷新。仍须解决可信当前记录来源 D1。 | **推荐提案**：`GET /api/listing/tasks/{task_id}/shein/offline-diagnostic?action=publish`；不是本轮注册/实现授权 |

task_id 仅资源定位符，不重建 Task-first Product 或 Task Dashboard；当前域事实仍归 Product/Listing，诊断没有新 durable object。后续 #315 是否改为此入口由协调方更新正文，本文不能自行取消“现有入口”的原验收。

拟议完整链（已存在能力与未实现环节明确区分）：

1. **已有** `server_auth` 的 workbenchAuthenticationMiddleware → `authruntime/zitadel.Verifier` 验证 token/subject/expiry；清除伪造身份头。
2. **提案 route 声明** `OrganizationAccessPolicyCachedRead`，复用 **已有** `workbenchcontext.Resolver.Resolve`。`X-Requested-Organization-ID` 是既有选择器，按 verified grants 选择 EffectiveOrganization；Home=A 可选择被授权的 B，TenantID 必须等于 B，roles 只取 B。selector 缺省时沿用已批准默认选择：优先有 grant 的 Home，否则唯一获授权组织；多组织且无合法默认才要求显式选择。resolver 无法决定有效组织、无授权、过期或依赖失败时，在业务读取前失败关闭。遵守既有 60 秒缓存/撤权和 suspension deny overlay，不新增 IAM 或零缓存要求。
3. **提案 role 装配**使用既有 read permission（viewer/operator/admin 读取能力），不靠 URL 自动匹配默认通过；route 测试验证具体授权映射。构造 `listing/task.Actor{TenantID: effectiveOrg, UserID: verifiedSubject, Roles: effectiveRoles}`，要求非空、匹配 verified identity，不调用 legacy requestContext/default tenant。系统管理员也先有 B grant，仅在 B 内按现有 TenantAdminChecker 决定 owner bypass。
4. **未实现窄 port**由 `internal/listing/diagnostic` 拥有 `ReadOfflinePackage(ctx, actor, taskID)`，返回不可变持久 package bytes、exact row resource/org/owner 及已存在的相关 evidence；不扩 CanonicalSubject 为万能 reader。持久 adapter 复用严格 actor SQL/返回后校验模式。查询先 WHERE id AND verified organization AND owner 条件；admin 只可省 owner，不省 org。缺可信 owner 同样拒绝，不能退回 Request.UserID。返回后再检 task ID、org、owner，防错误替身/适配器。
5. **D1 未满足**：所读行的 org 字段必须来自可证明的当前记录创建合同，不能仅靠 SQL tenant 字符串等于 selector。没有该前提就不加载旧敏感包，不静默回退 Legacy reader。若原表不具备可区分的可信记录集合，当前 adapter 不得在该表开放查询；需要协调方先指定当前生产者/存储准入边界，而不是本设计自造 allowlist 或 provenance 表。
6. **未实现**当前 Listing 诊断 service 对一次读取副本执行 §3 内容绑定 → Marketplace v2 evaluator → scoped DTO。当前 owner 不导入 root ListingKit/tenantbridge/compatibility。HTTP composition 仅装配；底层既有 legacy store 文件可向当前 port 实现接口作为 EXTRACT，但只能在 D1 证明可用且不新增 legacy consumer/import baseline 时实施，否则独立前置决定适配位置。

无身份 401；角色/组织授权拒绝沿用现有 workbench 403/选择错误，发生在查包前；组织内 nonexistent、wrong-org ID、非 owner 资源、归属无法证明均统一 404，不返 org/owner/正文/digest；授权或 DB 依赖 unavailable 为脱敏 503；缺包元数据无法计算为明确 not-evaluable；expected mismatch/stale 409；unsupported action/site 400。对不可读资源不计算/返回 hash，以免产生资源存在性旁路。

## 5. D1：生产归属证据缺口与最小待决

现有 Task.TenantID 并非在所有入口都证明是 Effective Organization：

- `internal/authruntime/zitadel/verifier.go` 构造 TenantID/HomeOrganizationID；仅 workbench resolver 赋 EffectiveOrganizationID。
- `internal/listingkit/api/handler_tasks.go` 的 GenerateListingKit 取 requestTenantID，UserID 只在请求为空时补身份；`task_lifecycle_service_support.go` 从 identity.TenantID / req.UserID 建 Task。旧 route 未声明 org policy，故不能把这些来源当当前多组织创建合同。
- `store.CreateTask` 可从 ambient tenantctx 补 tenant；数据库字段名称/数值形状、SourceSnapshotVersion、读取时 token grant 都不能反向证明原行创建归属。
- 已查 `CreateGenerateTask` 的实际 handler/compatibility sourcehandoff caller，尚未找到可据此宣称当前 effective-org SHEIN package 生产链的入口。ReadCanonicalSubject 的严格 SQL 是可复用模式，不改变该历史语义事实。

**唯一阻塞决策 D1**：请 Marketplace/Listing 协调 owner 指定“当前规范 Organization 归属的 SHEIN 资料包由哪个已批准生产者/持久边界提供”，及读前如何区分该可信集合；如尚无，拆出该最小前置并明确 owner。不能批准“tenant 相等即可读所有历史 task”来消除缺口，不能把 #30/#307 clean-slate 自动扩用，不能在本轮迁移/清理/加 schema/新生产者。

这是 `BLOCKER`，命中核心 happy path 无法完成（若靠猜归属放行还会产生租户混淆），不是已证明生产越权。输入 v2 可以完成独立设计；完整链仍 **BLOCKED_ON_CONTRACT_DECISION**，不虚称 IMPLEMENTATION_READY。D1 以外，v2/新 URL 作为推荐工程合同供本轮评审；是否拆共享前置及改写 #315 是协调落地步骤，不是自动授权。

## 6. 具体示例与验证矩阵

隔离 happy path：临时 fixture 的受控创建者明确用 verified Effective=B、subject=U 写入规范 B/U 行 T 和持久 package P（无外部 evidence）；请求用户 Home=A，B 中有 viewer grant。resolver 得 B → scoped reader 仅选 B/U/T → 解码 P → hash H → v2 publish 结构规则无 blocker → HTTP 200，offline_checks ready，external_freshness not_evaluated，diagnostic_only true，缺失在线/资产审批 coverage 明列。这证明设计可表达成功，不证明实际业务生产者已存在；D1 不因 fixture 消失。

| 场景 | 预期/后续验证 owner |
| --- | --- |
| Home A，选择 B，B viewer，自己的当前可信行 | resolver + handler + SQL 集成成功；A 的 admin 不带入 B |
| 无 B grant / 缺角色 / 缺身份 / 多组织无合法默认 | 在调用 reader 前失败；伪造 body/header tenant 无效；selector 缺省但有合法 Home/唯一组织时正常解析 |
| B grant + A 资源 ID；B 他人资源；管理员无 B grant | org/owner SQL 不返回正文，统一资源不可读；admin 不跨 org |
| legacy 空/数字 tenant、归属无证明或未知 owner | 不访问旧包；不把 numeric 文本与 Organization 数字 ID 相等当证明 |
| 缺 package、未知字段/私有注入、非法 UTF-8/数值/JSON、超限 | 明确不可评估，无成功报告；测试解码失败与 canceled 分开 |
| 无外部 evidence | 只离线结果；coverage not_evaluated，future freshness-required mode 拒绝 |
| expired 边界等于 evaluated_at、known stale/partial stale、subject mismatch | stale_input；不能清空 evidence 后换成功 |
| 相同内容 map 插入顺序；数组/规则/action/嵌套模板改变 | 前者相同 H，后者相应不同 H；原输入不变，hash/Validate 同一副本 |
| expected H → 实读 H2；未传 expected；计算后发生写入 | mismatch 错误；无 expected 评本次副本；报告不是 CAS，写前 gate 仍重验 |
| 阻塞/警告/成功与 save_draft action policy | 保留 blockers/warnings/paths/guidance；允许 draft blockers 不授写权 |
| unsupported preview/site，调用者注入 observed/expiry | 明确拒绝，不静默转换/剥离 |
| 撤权/缓存超期/依赖故障/token expiry/suspension | 既有 resolver 测试 + route 集成；读缓存最多既定 60 秒，失败关闭 |
| DB 取消/读后取消/计算后取消/响应丢失 | 无后台任务/重试写；重复请求重新授权读取，不记录第二事实 |
| 相邻写 gate 与无副作用 | spies 确认不调用 decorators、cache store、平台/模型、SaveTaskResult；既有 cookie/POD/人审/ApprovedAsset/submit 回归不退化 |

已执行的临时本地 Go fixture：使用当前 `publishing/shein.Package` 持久 JSON 解码与 ClonePackageForPersistence，验证 map 顺序稳定、内容/action/数组顺序变更改变摘要、NaN clone 失败。**仅证明这些绑定假设**；完整严格编解码、私有字段排除、授权/SQL/v2/HTTP 尚未实现，不记为 TDD 或集成 PASS。fixture 不进入提交，执行记录写 PR。文档检查使用既有测试，不改共享测试/baseline。

## 7. 后续最小切片与停止点

| 顺序/owner | 预计变更位置（提案，非本轮实现） | 验收/停止点 |
| --- | --- | --- |
| D1 协调：Listing 当前资料包生产 owner | 指定现有可信创建/读取边界；如缺则独立任务，本文不设计历史迁移/schema | 无可证明来源则整入口保持 blocked |
| S1 Marketplace/publishing：v2 bound input | `internal/marketplace/validator` 的独立输入/结果语义与严格验证；`internal/marketplace/shein/validator` 共享 evaluator；publishing 的持久内容编解码边界、专用测试 | 先红后绿验证 §3/6；保留 v1 strict 校验，决定 v1 退休点；不接路由 |
| S2 Listing persistence：窄 reader | `internal/listing/diagnostic` 窄 port 与一个 D1 指定 adapter；可复用 task actor/checker 模式，不反向依赖 legacy | 单次 org/owner scoped 查询与回检、大小/取消、可信 fixture；禁止未知归属旧行 |
| S3 Application/Listing：一个诊断入口 | `internal/app` 下有界 HTTP adapter、既有 httpapi composition、当前 Listing 诊断 service/DTO 和集成测试 | 只接一个 consumer，§6 全链和原 gate 回归；协调方先更新 #315 是否承担此片 |

S1/S2 是真实共享前置，不因本文合并就存在；各片可独立验证/回滚，S3 撤除入口不会回退旧 preview。实际实施命中 AGENTS 文件/行数门槛时继续拆分，不扩通用框架。

Legacy decision: **EXTRACT**（设计复用严格 actor/查询及规则行为）；Current owner：Listing 读取、Marketplace 规则、Application 装配、既有 Organization resolver。旧 preview/Task-first UI 不续建；历史未知归属不兼容读取。Cutover/deletion condition：协调方确认新入口和依赖交付后单入口切换，v1 无消费者时另片 RETIRE；当前无删除/数据操作权限。

停止于本份设计的独立评审及 D1 结论，最多两轮正常架构评审。不得为了评审归零添加新 IAM、通用 freshness 服务或历史迁移。设计 PR 不合并、不关闭 #316/#34/#315，不恢复 #315 编码，不发布生产。
