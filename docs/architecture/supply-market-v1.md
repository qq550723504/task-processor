# 供应市场 v1 — 明确发布、优选申请与 SDS 定制成品

Execution: [#622](https://github.com/qq550723504/task-processor/issues/622), parent #137。
Design Basis: **Independent Architecture**。
Status: **DESIGNING / NOT_READY**；正式生产代码与 schema 尚未修改。
Investigation baseline: `main @ 2e40643f63a4314a33b0f59f21def10a0cd2ecc9`，2026-10-09。

## 1. 用户结果、产品决定与范围

当前内部试用企业的获授权成员可以浏览硕米自营和优选商品，明确选入本人
“我的数据”，再沿已交付的“我的供应链”适配和上传；申请企业可以提交自己的
优化商品和供货资料，由平台专员人工评估、要求补充、线下确认合作，审核后明确发布。
平台专员有同一 Console 内的实际发布与评估入口。

本会话 2026-10-09 用户已明确决定：

- 自营由平台专员发布；优选由企业申请、人工审核后由专员明确发布。
- 优选仅限本人/企业自有商品、已有实际优化结果、声明库存或生产能力、必要资质；
  人工评估及线下合作确认，不做 AI 自动准入。
- 1688 与 SDS 本批都实际接入，不以示例卡或外链作为全部交付。
- SDS 本批包含定制成品，路径为选模板、选已批准图案、配置设计、生成效果图并
  保存远端成品、保存到本人 Product 与供应链。
- SDS **统一使用硕米平台账号**；每项设计、素材、操作和成品仍有原企业及成员归属。
- 不做在线采购、付款、交易结算；SDS 生产下单、履约、删除成品不在本批。

以上新阶段覆盖以前“市场稍后/仅公开模板”的提议。未完成的能力仍据实未开放。
本批不新建 AI/Agent、ERP/PLM 连接器、通用审批平台、通用恢复平台或新 IAM；
不迁移旧业务数据、不兼容旧 ListingKit Workspace、不访问共享试用/生产数据。
账号归属决定不替代真实账号联调、真实远端写入、付费调用或共享部署授权。

Must：完整市场选品；有真实版本证据的人工优选申请及明确发布；1688 原采集链；
真实 SDS 定制成品；企业隔离、撤权、精确来源及批准、幂等和未知结果不重发。
Should：复用现有 Console、当前 Product/Asset/Review 和成熟存储/编排组件。
Accepted risks：目前没有新增被用户明确接受的安全风险；人工经营判断由专员负责，
系统保存其记录，不替代产品质量、资质真实性或线下合同的法律判断。

## 2. Product / Figma Authority 与界面

引用 [final-ui-ia-authority](../product/final-ui-ia-authority.md)、
[greenfield baseline](../product/greenfield-no-legacy-migration.md) 和上述当前决定。
2026-10-09 只读实查 Figma `tg48P46SSXl6TBy9lZwg63` / 当前可见非归档页面 `31:463`：

| 操作 | 当前 light 节点 | 本批使用方式 |
| --- | --- | --- |
| 硕米自营 / 优选 | `429:528` / `429:1074` | 当前为“暂未开放”说明；不能作为现有商品布局或能力证明 |
| 货盘首页 | `864:541`，内容 `864:659` | 两张入口卡及下方流程；已读 design context / 截图 |
| 货盘对接申请 | `429:2166` / `429:2255` | 名称、网站、联系人及品类；人工线下技术评估 |
| 货盘选择 | `429:3258` / `429:3347` | 1688 与 SDS 真实能力卡、发货地；不复制示例接入状态 |
| 优选说明 / 记录 | `429:2712` / `429:3804` | 本人资格、进行中与已结束分开，真实统计和查询 |
| 选择申请商品 | `997:359` / `997:763` | 从当前本人自有商品选择，过滤和分页 |
| 供货及资质 | `1004:649` / `1004:758` | 库存、MOQ、周期、发货省市、价格/售后、附件；已读截图 |
| 申请进展 | `1008:536` / `1008:645` | 已提交、评估、待补充及终态，真实时间和说明 |

自营/优选可操作布局及 SDS 编辑器未在上述当前原型提供。设计在现有 Console 的
ConsolePage/Card/Table/Select/Dialog 等组件内完成：市场列表及详情、显式加入批次；
有权限的专员看到“发布管理/处理申请”；货盘 SDS 卡进入模板列表/详情，再进入编辑器。
市场详情仅显示明确发布的字段和供货说明；专员评估页与公开详情分离。
SDS 编辑器展示选中款式、可编辑区域、已批准图案、位置/缩放/旋转和最终操作摘要，
“生成效果图并保存 SDS 成品”为明确产生远端素材和成品的操作，结果页再显式加入供应链。
实现前以本设计的产品状态和当前 Figma 样式为依据，不宣称存在完整 SDS Figma 页面。

1688 卡进入现有选品/采集及插件，结果进入既有“我的数据”；本批不重建采集器、
不恢复旧 Task-first API。网页端采集继续沿原 Commercial/Resource 计价合同，插件渠道
沿原批准语义，不把市场入口变成免费绕过计量的通道。

## 3. 当前可复用能力与根因

| 能力 | 当前证据 | 使用边界 |
| --- | --- | --- |
| 我的供应链 | #605 / 已合并 PR #606；[设计](my-supply-chain-shein-v1.md) | 复用批次、待适配、批准、SHEIN 上传及原 UNKNOWN；市场不是新上传器 |
| 本人来源集合 | `internal/product/collection` | scope 为 Organization + Actor + 原 Member；不得改为全企业/全市场可读 |
| 原子来源发布 | `internal/product/sourcing`，`internal/app/productsourcing/collection.go` | 同 Product UoW 发布 Catalog、来源 evidence 和集合引用 |
| 优化证据 | `internal/product/review/lineage.go` | 当前实际 title Apply 回执；不能伪造整份图文已优化/审核 |
| 图案批准 | `internal/product/asset/source_approval.go` | 用户显式选择 RoleDesign 的确切 ApprovedAsset；来源候选不自动批准 |
| 外部执行 | `internal/listing/submission` ExecutionKernel | 唯一 SendPermit、不可变 intent、UNKNOWN 与 qualified readback；不建立第二重发机制 |
| 不可变对象 | `internal/integration/s3` | 复用内容哈希、PutImmutable、ReadObject；私密资质不进入 PublicURL 路径 |
| 平台权限 | 现有 verified global platform authorizer | 不让企业 admin、自定义模块或 body 字段获得跨企业权限 |

**市场根因**：已有 Collection 是成员私有来源；直接扩大其 reader 会泄露未发布商品。
需要新增“精确版本的明确市场披露授权”，而非给平台专员伪造客户成员身份。

**SDS 根因**：旧 `internal/sds/design/service.go` 的 PrepareAndSyncDesign 自动重试
sync/save，并在 defer 中删除成品与素材；`rendered_images.go` 含按父商品取最新、
素材名搜索、静态图及普通商品详情替代。默认 client 还有写请求重试及本地登录态 bootstrap。
这些行为不符合“共享平台账号下保留定制成品、绑定原企业、结果未知不重发”。
旧 workflow/usecase/httpapi 不能作为新入口的 facade 或 fallback。

公开只读实测：2026-10-09，带公开页面 Referer 的 `/products/page` 返回 HTTP 200、
真实分页 items/totalCount（size=1，响应 48,351 bytes）。该证据只证明公开列表可读，
不证明登录账号、设计保存、成品匹配或用户验收。旧成功样本亦不证明当前账号可用。

## 4. 事实 owner、合同与注入

采用当前 Product 数据库中的有界子域，避免市场授权与 Product 引用跨库部分提交：

- `internal/product/supplymarket`：明确发布版本/披露授权、优选及货盘申请、专员事件。
- `internal/product/pod`：成员设计意图、SDS 账号绑定版本、素材/成品关联及共享模板互斥。
  它不是 Product 资料、Asset 批准或外部 attempt 状态的第二 owner。
- Product Sourcing/Catalog/Collection：canonical 商品、不可变来源 evidence、成员批次。
- Product Review/Asset：优化和图案批准；Asset 保留独立数据库与原角色。
- Submission：唯一远端 mutation intent/attempt/SendPermit/UNKNOWN。
- `internal/integration/sds`：当前协议的窄 transport/映射，server-only 账号凭据；
  不依赖旧 workflow/usecase，不拥有企业授权或恢复决定。
- App：仅组装 owner ports、实际数据库/UoW、identity、Asset、transport 和 worker。

contract → implementation → injection → consumer：

SupplyMarket 的 PrivateProductSelection / OptimizationEvidence / ReleasedProduct / UoW
→ 当前 Product/Review adapters 与新 Product schema persistence
→ feature-specific route/module 中注入 live identity、platform authorizer、Product pool
→ 同源专属 BFF → 自营/优选/申请/货盘 Console。

POD 的 ApprovedArtwork / TemplateReader / SDSMutation / SDSObservation / ExecutionKernel
→ Asset bounded reader、固定 SDS transport、现有 Submission repository、Product POD repository
→ server-only 账号 binding + 窄 Worker + current live execution authorization
→ BFF → 模板/设计/成品结果页。

公开市场仍要求当前登录及本企业 read grant。“公开”指获授权企业之间可浏览明确发布的
投影，不新增匿名商品API。仅 ReleasedProduct Port 可跨原企业读取精确版本的 allowlist
投影，不允许客户端传原企业/ProductKey/version 任意读取。披露授权绑定内部 original scope
与不可变 ref；proof 不可由 HTTP DTO 构造，短期有效，并在输出/选品提交前重新确认有效。
市场不返回 raw evidence、内部 trace、私密原始URL、资质对象键、登录信息或未批准内容。

市场/模板/成品选品由 current Product owner 创建**接收成员自己的来源 publication**：
准入 producer 为 `supply_market/v1`、`sds_template/v1`、`sds_finished/v1`，
SourceID 包含接收 scope + 明确源 revision，SourceRunID 绑定本次本地 operation。
新 Collection source kinds 对应 market / sds_template / sds_finished。
immutable evidence 保留发布版本或 SDS receipt refs，不同步原商家后续修改。
不同成员不会通过相同 upstream ID 共用一个私人 Product。审批、Asset、店铺和上传权
不随选品复制；模板显示“未定制”，成品来源只在确切成品已核实后产生。

## 5. 权限、披露与申请

组织 GET 使用 CurrentIdentity + CachedRead，写入使用 LiveWrite；每次 mutation 在事务
提交前重新确认原 actor/organization/member 及 scoped permission，换企业或撤权则拒绝。
新增 supply-market.read/select/apply/design 权限常量和 feature-local policies/module mappings；
select 同时需要 Collection manage；申请/design 还需原商品及批准读取权限。
Collection 私有约束不变。POD 创建者是唯一成员消费者，企业 admin 不自动成为成品所有者。
平台专员只有当前 verified global platform 权限，可处理申请/发布及受控核实；
企业普通角色、自定义模块、请求header/body 不授予平台权。

平台路径精确沿现有 AuthPolicyCurrentIdentityWithVerifiedRoles + OrganizationAccessPolicyNone
+ PermissionListingKitPlatformAdm；事务回调重新检查相同平台actor、token时效和权限。
先锁申请/发布行取得事实 owner，再读取该行授权范围内的资料，不推导客户成员权限。
普通 HTTP body 不接受 organization/actor/member/role/SDS账号/cookie/provider URL。

优选提交引用原 Collection item/revision、original publication/version 和实际 Apply
receipt/effective version；当前有效来源必须为本人自有商品（own），有可解析的真实
title optimization Apply lineage。采集商品或只是候选 proposal 不满足自有/实际优化准入。
来源更新后不能用旧 Apply 假冒新商品已优化；用户选择清楚可检查的有效版本。
申请包含原供货声明、必要资质 refs、明确准许平台评审该申请及在批准后发布列明字段的确认。
市场授权不涵盖资质文件；“审核通过”不自动发布，不获得企业剩余私有商品访问。

自营由专员从其有权管理的当前自有 Product 中选择真实版本；不任意填客户 Product ID，
不在市场模块手工存第二份商品正文。自营更新同样生成明确的新发布 revision。

资质附件用已有 S3 immutable client 的**私密 bucket**，最大 20 MiB/件，JPG/PNG/PDF，
最多6件且总计60 MiB；内容签名/图片解码、digest及size校验；PDF仅作为下载附件。
受控 upload先 live apply授权，private object key绑定 Organization/Actor/随机upload ID/hash；
附件 receipt经授权存在性核实后才能写申请。申请/专员 reader验证精确request ownership再读对象。
返回 `no-store`、`nosniff`、attachment disposition；不分配 PublicURL、不让body提供存储key。
对象写入/DB提交失败最多留下不对外发布的孤立不可变对象；不补写申请、不自动删除真实文件。
当前 SourceMedia 的公共图片URL设计只用于商品图案，不能复用作资质分发权限。

共享 authz 默认策略/module_catalog、导航及 currentapplication 公共装配由协调后的唯一
共享 Writer 接线；本 Writer 提供依赖和权限合同，不在其活跃共享路径抢写。
缺 identity/authorizer/私密附件/SDS binding/owner schema 的能力据实不可用，不用空实现兜底。

## 6. 数据、事务和人工状态

Product database 中新增有界 `supply_market_*` / `product_pod_*` tables，
复用现有同库 Catalog/Collection/Submission UoW，Asset 仍跨库只读批准。
fresh installer 显式建表/约束及窄 runtime DML grants；运行只验证，不 AutoMigrate、不写旧数据。

市场：applications 保存 original scope/ref、Apply ref、immutable original supply input、
stage/revision；application_inputs/events 追加补充和评估；releases 保存 channel、
original canonical ref、公开字段 allowlist、供货说明 ref、申请/合作确认 ref、revision/active；
commands 保存(scope,actor,key)及canonical intent hash/immutable result。
供应说明与合作评估是新市场事实，不是 Product 正文副本。所有actor/owner不可后改。

优选状态：SUBMITTED → EVALUATING → APPROVED | REJECTED；
EVALUATING → SUPPLEMENT_REQUIRED → EVALUATING。
专员转 approved前记录线下合作确认及评估说明；补充只由原成员在明确请求下追加，
原输入不改写。终态冻结；再次申请用新的精确商品/申请意图，不覆盖历史。
进行中和结束页面基于实际stage，统计由数据库查询，不从当前页推导。
货盘对接独立 kind：SUBMITTED → EVALUATING → PLAN_CONFIRMED | CLOSED；
记录人工对接进度及线下方案，不因PLAN_CONFIRMED注册新连接器或宣布已接入。

发布是单独显式命令：优选仅 approved且合作确认仍有效；active release → revoke。
重新发布/改版本用新 immutable release revision。拒绝替换原发布字段或自动跟随来源最新版本。
撤销后停止新浏览与新选品；已明确导入的成员 Product 是其 retained source evidence，
不自动删掉或回写，页面说明来源发布已撤销。撤销不能回收用户已经获取的资料。

写命令：单值 canonical UUID Idempotency-Key，更新用确切 If-Match revision。
相同scope/actor/key + 同意图返回原receipt（先重新授权）；不同载荷/预期版本/operation为冲突。
行锁/唯一约束覆盖并发，原子提交stage/event/receipt；无丢失更新。
发布/撤销与市场选品通过同 Product UoW锁同release；选品同事务写来源、Catalog、
Collection ref及receipt，来源标题/图片从exact released projection生成，body不能伪造正文。
选品提交失败回滚全部SQL；提交响应丢失重放原receipt，不重新复制或增加批次条目。

## 7. SDS 平台账号、设计与不可重发操作

账号配置仅在 server私密配置：binding ID、实际merchant identity、credential revision、
固定 API origins、当前协议能力 revision；密钥/cookie不进入DB业务JSON、UI、日志、PR。
不从旧 `.local/sds/auth_state.json` 自动捡取、不替用户刷新登录或重新登录后重发写操作。
换账号/凭据必须产生新binding revision；旧operation只能以原账号identity的匹配观察核实。

POD design intent冻结：原scope/member、已保存template publication/version、当前chosen variant、
server读取的parent/prototype/editable layer manifest、explicit ApprovedArtwork action/asset IDs、
bytes hash、各区域transform、账号binding revision、protocol revision、目标命名/实际payload hash。
用户从本人已有 Product / 已批准图案中选择；需要新图案时沿现有“我的数据”上传和显式批准，
不将模板示例图或上传成功自动当成RoleDesign批准。模板与图案可以是同成员不同Product，
分别绑定exact input，Asset reader验证原tenant/product/version/platform `sds` / RoleDesign。
使用现有SourceApproval合同的窄sds SourceSelection adapter，不让SDS模板中的blank图覆盖图案。
设计支持当前模板声明的可编辑区域与所选款式；无合格manifest的模板不可开始设计，
不默选“第一layer”或忽略不支持区域。复用成熟canvas组件及可提取纯几何映射，禁止模型改payload。

跨企业共享模板必须有数据库持久fence：key为(account merchant identity, binding revision,
merchant template/result-group identity)，active value为原 POD operation ID。
所有企业竞争同key，而非按Organization划分；先提交保留fence及immutable intent，
再允许独立 acquire SendPermit。取得fence后任何原operation重复启动都只能读其回执/attempt。
同账号其他设计被占用时显示“该模板正在定制/待核实”，不能用新operation覆盖。
租约过期或worker失联不自动释放未知fence；需原operation核实所有远端副作用及模板不再
被延迟执行改变的证据后才能释放，不能用时间窗冒充provider完成证据。

每个不安全重复步骤（OSS写、material create、syncDesign、add_and_design）使用现有
ExecutionKernel自己的immutable intent与唯一SendPermit，Target包含平台账号和原operation
稳定 step身份，payload绑定manifest。外层fence只保护共享template竞争，不成为另一个attempt状态机。
成功回执和POD step引用用现有NewTransactionFinalizer同库提交；不会在跨库重复创建Permit。
步骤未提交permit才允许第一次发送；permit响应丢失、网络timeout、cancel、worker crash、
不明确5xx/429/认证失败均保持UNKNOWN，不自动retry、fallback、更换账号或delete cleanup。
只有协议明确且可验证的无副作用拒绝可记failed_definitive。
已成功的步骤只复用exact receipt；整体未完成不把全部步骤重新跑一遍。

使用现有 Temporal SDK承担长时间设计编排；SQL意图是durable事实，Temporal不是身份/发送authority。
Activity重放最多重读原intent，任何远端mutation只能持新获得的一次Permit执行。
restart/resume先读取原steps；原申请成员经tokenless live membership/permission复核后才进行
尚未发送步骤。撤权时停止新副作用，保留已发送/unknown记录及fence；平台专员可只读核实，
不得伪造客户身份继续设计。保存动作在worker受限deadline内执行；UI轮询读取进度即可。

## 8. SDS 协议资格与成品核实（当前开工阻塞）

旧代码提供待核实候选 endpoints：公開/products/page、/products/:id；登录后material/设计
manifest、/ps/design/syncDesign、/ps/design/add_and_design，以及mapi2的/design_products查询。
没有已验证的当前官方协议/账号成功样本，不能根据旧DTO指定不存在的关联字段。
特别是SaveDesign旧路径忽略body，SyncDesign只记200空响应；成品列表DTO有parent/variant、
material_img_name、prototype、ItemID/img_urls，缺exact layer/material-ID关联证明。

provider adapter必须先通过有界protocol qualification：

1. 明确平台账号identity和当前credential revision；固定HTTPS origins、写endpoint/结果合同。
2. 证明素材名称/对象键能保留唯一原operation marker，远端不会跨租户内容去重后混合归属；
   返回素材ID及可核实bytes/identity关联，body不能提供其他企业远端素材ID。
3. 证明sync/save使用的template/result-group共享状态、异步完成边界及成功/拒绝判定；
   证明何时可以安全释放共享fence，UNKNOWN时不释放。
4. 证明成品观察至少绑定原account、operation marker或由已匹配response返回的stable finished ID、
   chosen parent/variant/prototype及actual design material/manifest关联；具备非模板的全部预期render图。
   如果SDS只返回操作唯一素材名，须证明这是不可变exact字段且远端能核查对应素材/设计，
   不能把search模糊结果、最新时间、parent-only、图片相似或普通模板图当证据。
5. 证明finished ItemID稳定可复查，远端成品和素材保留；选择入Product后可正常查看。

成品query只能由server用已授权原operation的saved refs查询；对重复/多候选/字段缺失
保持UNKNOWN。若端点无法提供以上最小合同，应先取得SDS提供的正式对接方式或具体当前证据，
不得发明关联字段、弱化隔离或以假fixture宣布“实际接入”。本设计在此证据缺失时仍NOT_READY。

成功路径：各mutation qualified receipt → 渲染完成exact observation →
同Product UoW完成原Submission receipt并写POD finished ref → 明确选择导入Product。
原render URI及绑定hash作为source candidate保存，不自动成为可刊登批准；入供应链后沿原
Asset审批和Listing readiness。只有远端成品存在而不是图生成即宣布成品保存完成。
已确认成品永不被自动cleanup；本批没有delete入口或订单/生产mutation。

唯一恢复入口为原POD operation的“查询并核实”；worker/GET只能进行有界只读查询。
无qualified证据不resolve_unknown，不提供通用“强制重试/标成功”按钮。
平台专员若需要人工处理UNKNOWN，只能记录指向原账号和原远端对象的可验证证据，
提交给现有Kernel resolution合同；没有证据则保留状态/fence，不改为failed来解锁重发。

## 9. 请求和资源边界

企业API feature prefixes `/api/v1/workbench/supply-market`、`/api/v1/workbench/pod`；
平台prefix `/api/v1/admin/supply-market`，单独verified-roles策略；BFF同源/no-store。
strict JSON（unknown/重复字段拒绝）、单值precondition/key、UUID ID、GET拒绝unread body。
市场/申请list最多50，商品选择继承Collection100/page；真实游标和bounded filters。
非附件JSON最大64 KiB，普通请求总deadline10秒、详情response最大2 MiB；
说明文本4000字符，人工进度2000字符；事件独立分页，历史不因累计长度删除或失败。
资格upload最大20 MiB和30秒；图案复用当前SourceMedia3 MiB/40MP，明确UI限制。
provider只读响应最大2 MiB、每页最多50、单次15秒；mutation每步最多45秒且无retry，
render查询单次15秒、最多一次/5秒，15分钟后显示待核实且保留原operation。
transforms数量限manifest且最多16区域；所有数值finite且在provider允许范围内，payload2 MiB内。
输出对SDK/JSON中credential/token/signed query作redact，不记录完整provider payload或账号cookie。

固定provider HTTPS endpoints，不接受用户URL作为请求endpoint；禁用跨origin redirect和
认证header外传。图案下载复用已有public HTTPS/image probe的DNS/IP/大小限制；
signature返回OSS host只从protocol批准的HTTPS host allowlist选取，拒绝内网/HTTP。
凭据缺失/过期/协议未准入显示未配置/需管理员处理，不以公开模板可读伪造成品能力就绪。

## 10. Legacy 决策和实施切片

Legacy decision: **EXTRACT | RETIRE**。
Reusable behavior: SDS公开模板DTO字段映射、已证实当前协议的请求编码、纯Fabric几何算法；
当前Collection、Catalog、SourceApproval、Review和Submission合同直接复用。
Current owner: Product SupplyMarket/POD + integration SDS窄transport及当前Product consumers。
Cutover/deletion condition: 新业务仅调用当前owner；旧workflow/usecase/httpapi、自动重试、
parent/latest/fallback readback、cleanup和legacy Task消费者不进入新consumer graph。
不为旧内部入口新增兼容层/双读/双写；旧代码删除只按明确consumer证据实施。

一个主要PR `codex/supply-market-v1`。实现/提交可分为：明确发布与市场导入；优选/对接申请
及专员入口；SDS模板与图案配置；持久设计/远端成品及恢复；Console和批准后的运行接线。
这是一个完整用户结果的开发边界，不按文件数拆多个PR或派多个Writer。
当前仅设计；协议资格和共享owner协调不足时，不以首个切片可做为由绕过独立准入。

## 11. 验证、交付与准入

复用已有测试/受控fixtures，不新增验收平台。业务实现采用risk-matched TDD：
公开projection/平台与成员权限；申请自有/Apply精确lineage；补充/评估/发布状态及
revision/同键重放；撤销与选品同UoW；资格文件私密读；两个企业同SDS模板fence；
每步crash/响应丢失不重发；账户漂移、错误素材/成品及静态图不收敛；成功成品无cleanup。
本地受控SDS fixture证明合同实现，真实账号qualification/真实provider及用户使用单列。
必要本地checks、稳定候选CI、最终独立检查留在同一主要PR。作者/AI review不签发产品验收。

运行交付须给正常Console入口、登录与操作步骤、persisted owner/存储、未配置原因与保留数据
的重启方式。共享runtime由协调owner接线；本批未获授权时不部署共享/生产实例。
最终交付需要真实1688和SDS指定范围证据，缺少就保留NOT_RUN，不说全部可用。

| 开工条件 | 当前结论 | 解除条件 |
| --- | --- | --- |
| 用户范围/账号归属 | CONFIRMED | 已记录本会话明确决定 |
| Figma/当前owner映射 | READ | 不将占位或示例当能力证据 |
| SDS成品精确关联及fence完成边界 | **BLOCKER / UNKNOWN** | 当前协议文档或脱敏成功/查询样本；必要时指定账号受控qualification授权 |
| 共享authz/navigation/runtime协调 | PENDING | Issue记录feature合同的唯一接线owner和准入责任；本Writer不抢写共享文件 |
| 独立Architecture Review | NOT_RUN | 本文具体高风险边界review及finding分类；BLOCKER消除后IMPLEMENTATION_READY |
| 正式生产/schema修改 | NOT_STARTED | 上述准入后才能开工 |
| provider/用户验收 | NOT_RUN | 单独授权并真实执行，开发测试不替代 |

SDS阻塞命中AGENTS的“核心happy path按当前设计无法完成”，以及共享账号结果不能归属时
“跨租户访问/数据混淆、重复不可安全恢复外部副作用”。阻塞层级为正式实施准入及实际接入；
不是恢复为public-catalog-only范围，也不自动要求通用平台重建。
