# Account Audit 四项真实汇总 V1

状态：DESIGN_PREPARATION / NOT_READY。Issue #478 当前顶部范围为产品依据；只有独立评审完成并在 Issue 写明 IMPLEMENTATION_READY / Ready 后才实现生产路径。

## 产品结果与依据

当前授权企业用户在 `/workbench/account/organization/audit` 看见近30天操作、成员变更、权限变更、资源与续费四项真实数字；保留原列表、actor/operation 筛选、稳定分页与不可变数量。
Authority：Issue #478 当前增量；[Final UI/IA](../product/final-ui-ia-authority.md)；Figma `tg48P46SSXl6TBy9lZwg63 / 1532:525 / 1803:359`（2026-09-29 实际读取四卡代码及截图）。四卡继续现有 Console/CSS tokens；86/3/2/12 是原型示例，不进入实现。
沿 [统一基础方案](unified-base-prepaid-resources-v1.md) §3/§6 的当前 Resource audit 与 Account Audit CurrentAuditSources。历史 account-audit-v1.md 源账号首片范围已被当前多源代码与 #478 替代，不恢复旧 Token 来源。

Scope：新增有界汇总 GET 合同及页面消费；沿当前已接入列表的事实类别统计完整时间窗口。
Out of Scope：搜索/时间/成员新筛选、人名补全、新统计或审计平台、schema/index/缓存/物化/双写、新事实 owner、支付/报价/provider/SHEIN、legacy兼容、专项验收工具、现有实例更新。
本轮不引入尚未消费的 Store/Billing 独立历史：卡片明确显示当前覆盖的续费期数分配/回收，不能声称覆盖所有店铺服务续费或支付。这是沿当前完整列表事实的读取范围，不将缺失来源当成0。

## 口径、身份与一致性

一次请求冻结服务端 UTC `asOf`，`from = asOf - 30*24h`，区间 `[from, asOf)`，不是自然月；四项均采用同一时间窗口，以原 owner 事件时间为准。客户端不能提交时间、企业或统计筛选。
只按当前列表的成功已提交事实计数，每条原操作计1，不累计数量/Token/余额：

| 指标 | 纳入事件 |
| --- | --- |
| 近30天操作 | source_account.operation_committed；account_business_profile.updated；organization_membership.changed；account_member_resource.changed；account_member_ai_point_limit.changed；account_ai_points.committed |
| 成员变更 | organization_membership.changed 的 invite / role / remove |
| 权限变更 | 上述 membership 的 role（成员变更的子集，卡片不相加解释为总数） |
| 资源与续费 | 成员期数/数据 allocate_member_resource、reclaim_member_resource，set_member_ai_point_limit，图片已提交点数扣款 |

源账号按企业+account/version；profile按企业+user/version；membership按企业+project/actor/operation（当前内部游标含project/actor/operation；传输身份含actor、operation reference/version）；Resource按企业+operation ID；图片点数按企业+event ID。计数复用传输事件的 canonical relation/type/reference/version/actor，不使用当前页长度，也不从当前余额或当前月限重建。每个身份最多一次；同身份矛盾载荷 fail closed。成功操作同键重放不新增原事实，失败/UNKNOWN/预留不计；ai_invocation.usage_observed 表示观察到实际用量，不能证明操作成功，因此仅保留原列表显示、不计成功汇总。
聚合复用 Query.ReadFiltered 的 allowlist/validation 与多源 scoped cursor，内部每页100，从首页遍历直至所有在窗口内的事实遍历完成或已越过窗口下界。不设静默行数上限；超时取消时返回错误而非部分总数。全查询10s、响应有界，临时去重集合只活于请求中。总数十进制非负int64字符串，不丢JavaScript整数精度。
跨owner为现有实时只读浏览，不引入跨库冻结快照或锁；返回asOf/from和覆盖说明。窗口之后的事件排除；读取期间刚提交且原时间较早的事实可能在下次刷新体现，与原实时列表语义一致，不宣称数据库commit时刻全局一致性。统计不消费页面筛选/游标，切页/筛选不改变统计请求；显式刷新/重新授权/恢复焦点重新读并在读取时隐藏旧值。

## owner / 接线 / 权限

Fact owner保留：
- SourceAccount HistoryService/repository：成功事务回执、原 account version、service read authorization。
- accountprofile Repository：原账户经营资料更新事务审计。
- organization/membership Repository：原provider结果已提交的成功审计；project/org绑定。
- ledger/orgresource + integration/orgresource：原member operations/audit/events及已提交图片扣款。
Account Audit只投影，客户端/BFF不成为统计事实 owner。

contract → implementation → injection → consumer：
`GET /api/v1/account/audit/summary` → accountaudit.Query.ReadSummary → 已装配 CurrentAuditSources / 原读取ports → accountAuditModule routes → `GET /api/account/audit/summary` Auth.js BFF → strict client → 现有 AuditPage 四卡。
沿 source_account.read、VerifiedIdentity、Effective Organization、LiveWrite fresh resolver；每次summary独立重新授权，拒绝请求body/query与不匹配预期身份/企业。不增加写权限或放宽角色。源account仍调用原read授权；其他源只用当前已验证企业。
沿现有10s后端/15s认证及BFF整体deadline、abort、no-store、安全错误与128KiB上限。四整数合同固定schemaVersion、userId/effectiveOrganizationId、window、counts及固定coverage，严格拒绝附加字段、负数、精度溢出/不合法窗口。
列表与汇总独立读取；汇总未配置/失败不隐藏仍可读列表，列表失败也不以统计替代列表；企业/身份/角色切换卸载旧scope，403/撤权隐藏统计。真实0只有完整读取成功才能显示。
必需源：source/profile/membership/member-resource/image-point；缺少配置返回503 SUMMARY_NOT_CONFIGURED。任一已配置源失败/事实不完整返回503 DEPENDENCY_UNAVAILABLE；超时504；鉴权401/403；完整空历史返回200四个\"0\"。不得降级为部分统计。usage并非汇总必需来源，summary不读取observed-only usage。
不新增持久化、状态机、事务、恢复、mutation、外部副作用、retry/UNKNOWN owner。无后台reconcile、自动重发或provider调用。

## Threat model / Legacy / 验证与交接

Must：当前企业隔离与实时授权、原成功事实/身份、全窗口完整计数、重放不重复、缺源/错误与0区分、原数量/月限不可变、原筛选/分页保留。保护错误授权、跨企业混淆和不完整统计冒充真实数字；不扩大为新全局安全/审计机制。未新增Accepted Risk豁免；保留现有实时keyset非冻结快照边界。
Legacy decision：N/A，本次只消费已装配当前owner；旧Token/订阅/Task owner仍RETIRE，不恢复兼容。

TDD：先记录目标失败，再最小实现。必要测试覆盖>100/多源多页、上下边界/未来/旧历史、分类与overlap、原身份重放/冲突、企业隔离/到期/当前read权限/撤权、缺源与读取错误/超时/正常0、不可变quantity回归、strict HTTP/BFF/client与原列表筛选分页。复用现有owner/runtime集成测试和任务独立临时PG，无共享服务/浏览器/数据操作，无新runner。
实现完成在同一主要PR交准确HEAD、必要CI与一次最终独立检查；修复只复核增量。开发自检不签用户产品验收。入口/正常启动方法/数据仍存原owner/未接入独立Store-Billing来源一并交接。新PR合并、部署现有22444、关Issue或真实数据操作均NOT_AUTHORIZED。

