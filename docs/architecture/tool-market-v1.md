# 工具市场 v1 — 企业启用与人工定制需求

Status: **IMPLEMENTATION_READY**

2026-10-09 独立Architecture Review第二轮确认设计blob
`f9298fe72c11e10586411f538323eafad0738a69`原平台授权BLOCKER已消除；
企业权限、原子提交/并发重放和发布包校验归IMPLEMENTATION_TEST，按下文实现验证。

Execution: [#613](https://github.com/qq550723504/task-processor/issues/613), parent #137
Design baseline: main d95807f13475d27c0b8691f044cca6320a540d06, 2026-10-09.

## Product outcome, scope and authority

当前硕米内部试用用户可在“官方工具”查看真实能力，企业管理员启用商品采集工具，
成员在“我的工具”获取本企业清单、下载匹配当前安装的插件并进入现有1688采集；
在“工具定制”提交需求，查询硕米专员记录的进度。专员有实际站内处理入口。

用户在本执行会话 2026-10-09 已确认：
- 清单按当前企业共用，管理员启用后成员可查看；使用仍需原业务权限。
- 硕米专员人工处理；站内提交需求和更新进度，线下报价付款。

UI/IA依据为 [final-ui-ia-authority](../product/final-ui-ia-authority.md)、
Figma tg48P46SSXl6TBy9lZwg63 / 31:463 当前可见原型：
官方工具430:3479、我的工具430:5511、工具定制430:4495。
三个准确节点的高保真design context和截图已于本轮读取。

原型只有“商品采集插件”为已开放；其余七张卡片保持真实未开放状态，分类筛选可用。
本批不开发指纹浏览器、批量刊登、ERP、库存/订单引擎或动态插件平台。
新决定覆盖三个菜单，不据此扩大为全部卡片能力实现。
当前采集支持1688，不能照原型宣称Amazon、按关键词/页采集已可用。
当前价格和渠道规则复用 [采集计价合同](2026-10-03-acquisition-channel-pricing.md)：
插件/本地免费；服务端成功采集消耗1 DATA_ROW；额度资源购买价格由Commercial读取，
不在市场清单新建收费事实。原已批准的计价/UNKNOWN/recovery不变。

本批不包含在线定制订单、付款、退款、分账、开发任意用户代码、真实provider调用、
生产/共享部署或业务验收。定制需求及进度不是Tool安装指令；
原型“交付后自动进入我的工具”须由另行获准的实际工具实现/代码发布产生，当前需求
追踪首版不自动注册或安装未审核代码，不以专员将进度标完成伪造可执行工具。

## Current reality and architectural distinction

#128/#134及internal/commercetool提供Agent受控领域调用合同；
当前canonical/source evidence/approved asset/readiness reader已存在。
它们不是用户“商品采集插件”的商品定义，不直接投影成四个官方工具卡片。
市场只管理企业选择和人工需求；Product/SRC/Catalog、Asset、Listing、Agent、
Commercial和Resource保留事实、执行、授权、计量及恢复owner。

当前商品采集插件可复用extensions/1688-capture及/capture/1688接收端，
在线采集复用/workbench/supply/acquisition，结果复用当前“我的数据”入口。
扩展release必须由已有build.mjs使用明确CAPTURE_APP_URL构建，
没有默认生产地址；dist-fixture绝不作为正式下载。
本批只补发布包下载/安装说明，不创建第二extractor、采集API或Tool执行器。

## Domain and fact owners

- internal/toolmarket：代码拥有的产品目录、企业启用选择、定制需求和人工阶段事件。
- internal/commercetool：唯一Agent工具合同、exact-version allowlist和治理，未改变。
- productsourcing及Product/SRC/Catalog：采集操作、渠道身份、publication/evidence。
- Commercial/Resource：价格、报价、DATA_ROW预留/计量，未改变。
- 现有ZITADEL、workbenchcontext和authz：身份、当前组织成员权限及平台管理员。
- internal/integration/persistence/toolmarket：独立tool_market schema的数据库实现。
- 专属HTTP模块/BFF：请求边界及投影；公共runtime Writer负责当前安装注入。

产品目录代码定义稳定ID、版本、名称、分类、说明和支持渠道，不在DB存executor、
endpoint、权限或任意代码。首个可启用ID为product-acquisition；
登记能力状态由安装注入的可信readiness/provider绑定推导，不由启用行决定。
开发中条目不能通过伪造ID启用。目录变更不改Agent allowlist。

contract → implementation → injection → consumer：
toolmarket Repository/Catalog/Authorize窄Port → tool_market PG及专属HTTP module →
当前runtime Writer注入已验证identity、权限、采集状态和插件发布包 →
专属/api/tool-market BFF → /workbench/tools/official、mine、custom。
管理专员从custom页的权限受控“处理需求”入口进入，无需新增顶层导航。

## Permissions and trusted scope

企业GET清单/需求用CurrentIdentity和CachedRead组织策略；企业写入用LiveWrite，
服务调用前解析新鲜有效成员与对应scoped authz许可。
企业写入前/事务内通过注入回调重新确认相同actor+organization；撤权则回滚。

拟新增workbench.tools.read/manage/customize权限：
- 当前企业成员可查看清单/需求（现有viewer/operator/admin角色的read许可）。
- 只有现有受保护企业admin能改变启用；不将自定义模块的read授权视为管理员身份。
- operator/admin可提交需求；viewer只读。提交不是授权执行所述流程。
- 专员跨企业list/detail/update仅由现有PermissionListingKitPlatformAdm授权。
  平台权限判断使用现有平台authorizer；企业角色、模块权限、JSON布尔字段不授予平台权。
  可读取的企业事实范围和平台权限分开，不让工具manage自动获得跨企业进度权限。

平台list/detail/progress精确复用现有ecoservices路径：
AuthPolicyCurrentIdentityWithVerifiedRoles + OrganizationAccessPolicyNone +
PermissionListingKitPlatformAdm；不要求专员是客户企业成员，不将角色替换为客户企业角色。
平台事务内回调重新检查同一已验证平台actor、未过期token和平台许可，
不能使用企业成员resolve替代平台检查。锁定请求行后取得owner organization，
仅用于事实查询/回执命名空间，不能据此生成客户企业成员权限。

新permission常量可落独立authz/tool_market.go。
共享authz默认策略、WorkbenchPermissions/module_catalog及console-navigation/runtime
必须由协调后的唯一共享Writer接线；本批未协调前不修改共享文件。
HTTP Authorize必须注入，缺失拒绝构建；可配置的目录/下载状态不是授权替代。

Scope从可信context来，body不能包含organization/actor/roles/platform。
企业read使用organization predicate，跨企业ID返回NOT_FOUND；
平台入口先平台授权再全局查询，任何平台mutation锁定原请求所属企业，不修改所属企业。
用户提交文本是数据，不执行URL/指令/代码，也不收集密码、token或账号凭据。

## Persistence, transaction and idempotency

tool_market schema只含：
- activations：(organization_id,tool_id)主键，enabled、revision、updated_by/time。
- requests：UUID主键，organization_id、created_by、kind、title、description、
  stage、revision、created_at/updated_at；原需求内容冻结。
- events：(request_id,revision)主键，stage、note、actor_id、occurred_at；追加事件。
- commands：(organization_id,actor_id,idempotency_key)主键，operation、
  canonical-intent SHA256、immutable result，保存确定性重放回执。

create由If-None-Match:*表达不存在，后续启用/禁用和阶段更改由If-Match精确revision绑定。
Idempotency-Key为canonical UUID；相同scope/key不同operation、ID、expected或payload
返回IDEMPOTENCY_CONFLICT，不覆盖原意图。同意图返回原回执，先重新授权。
状态、事件和回执在同一PostgreSQL事务提交；首次激活并发使用INSERT ON CONFLICT
及原行锁；后续使用FOR UPDATE/精确revision。冲突返回412；不静默覆盖。

请求create使用稳定由服务端UUID产生的request ID，原同key重放返回同ID。
平台命令命名空间取已锁定请求的owner organization，回执包含平台actor。
所有DB作用域predicate明确，只有已授权平台查询可跨企业。

初始化为fresh schema-owner显式安装；runtime只验证schema契约并以窄角色DML。
不在请求中AutoMigrate，不自动覆盖旧表，不写现有业务库/保留试用实例。
依已有同库事务/GORM实现，不建立outbox/Saga/队列；本批没有外部副作用。

## Request stages and human evidence

SUBMITTED → EVALUATING → PLAN_CONFIRMED → DEVELOPING → DELIVERED。
专员可从非终态转CLOSED，并提供原因；终态不继续推进。
每次推进只向下一阶段，不从提交直接跳“已交付”；允许同阶段追加真实进度说明。
阶段改变必须expected revision及非空说明；历史不可改写。
DELIVERED必须说明实际交付结果，页面明确“专员记录的进度”，不是付款或可执行Tool证明。
没有定制款项、结算、支付状态或自动注册工具字段。平台专员线下处理并据实更新，
本批不替用户生成订单/付款/报价承诺。提交成功只意味着需求已保存、待评估。

enterprise启用/禁用只改变清单。禁止宣称禁用会撤销已安装扩展或停止已开始的采集；
工具实际使用/已有API仍由原业务权限和原operation协议决定。
readiness失效时保留启用事实但显示不可用原因，不假删除选择或制造“在线”。

## HTTP/BFF and bounded operation

GET market/mine；PUT activations/:tool_id；GET/POST requests；
GET requests/:id；平台GET admin/requests(/:id)和POST admin/requests/:id/progress。
平台URL无organization body override；BFF将平台路径映射到已批准的platform-admin路由。
企业API前缀为/api/v1/workbench/tool-market；平台分支为/api/v1/admin/tool-market，
同模块单独使用上述verified-roles/None策略和平台权限。

JSON strict decode（含重复字段/unknown字段）、body最大16KiB、title120 Unicode字符、
description4000字符、stage note2000字符。list最多50、UUID游标、响应最多256KiB。
GET拒绝未读body；请求总deadline10秒，写事务遵守context。
headers单值Idempotency-Key/If-Match，版本为精确十进制字符串。
no-store；错误使用INVALID_REQUEST/FORBIDDEN/NOT_FOUND/IDEMPOTENCY_CONFLICT/
REVISION_MISMATCH/PRECONDITION_REQUIRED/DEPENDENCY_UNAVAILABLE现有语义。

插件包是现有build生成的部署产物，runtime明确注入包路径、sha256及接收origin，
仅接受非fixture manifest和已知文件集；有界读取，不以用户URL下载。
manifest match忽略端口，不能仅凭hostname证明接收地址绑定；下载必须校验整个archive
与可信部署构建记录中的sha256、完整CAPTURE_APP_URL为同一个绑定，
复用既有编译地址/exact origin+path/tab交接检查。配置缺失或绑定不符则不可用。
没有包或接收能力时下载按钮显示不可用。不能通过当前浏览器origin临时生成任意可信包。
客户端未知响应保留原actor+organization+key+请求意图，切换组织/账号不自动重发；
恢复原上下文后原key查回执或重放同命令。成功后从服务端重读，不能前端假保存。

## Failure and recovery evidence

- 回执/状态/事件任一步失败：整个事务回滚；原key重试只产生一份事实。
- commit后响应丢失：同key重放返回原结果；未确认前不换key提交新需求。
- 同版本不同并发命令：一个生效，另一个revision conflict；原UI需刷新。
- restart：从DB读清单、需求、事件、回执，不用进程内事实。
- 撤权/企业切换：新授权失败且零写；UI清除旧scope展示，保留原意图的隔离恢复。
- 状态更新失败不影响采集/付款，采集失败不修改清单或人工进度。
- package/capability unavailable：显示配置/未开放状态，不能伪造download成功或runtime可用。

## Verification and delivery

先通过focused TDD捕获跨企业ID读取/修改、非admin启用、伪造platform、
同key不同意图、并发revision和响应丢失重放；使用已有隔离Postgres测试模式证明原子提交/
独立连接重读，复用既有source pricing测试不人工重复完整采集验收。
HTTP与BFF覆盖认证/Origin/strict body/timeout/错误映射；UI覆盖启用后真实回读、
成员只读、未知结果保留、企业切换隔离、需求提交与平台进度。
按三个准确Figma节点比对页面内容和资产；复用现有shell，记录其无关既存差异。
稳定候选仅执行相关Go/Web检查及必需CI；最终独立复核检查实际diff和完整消费者路径。

本批一Writer、一主要PR；适用设计review最多两轮，finding先按AGENTS分类。
准入前不写生产业务路径/schema。独立设计review已完成；新实现测试尚未执行。
实现完成进In Review，交独立HTTP模块、schema安装/权限/包构建与BFF入口给指定runtime
Writer；共享接线、真实1688/付费provider及用户试用未执行保持NOT_RUN，不称可上线。

Legacy decision: EXTRACT / RETIRE。
Reusable behavior：现有扩展build/extractor/receiver、当前Acquisition/SRC/Catalog、
当前Casbin/组织resolve、事务与共享UI组件。
Current owner：原采集/授权/价格owner不变，新增市场管理事实在toolmarket。
Cutover/deletion condition：三个占位页面由真实模块消费后退休；不回依赖旧Service/Task-first，
不迁移旧数据或建立双事实源。
