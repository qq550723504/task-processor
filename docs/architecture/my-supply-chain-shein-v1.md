# 我的供应链：SHEIN 美国站完整交付设计

Refs [#605](https://github.com/qq550723504/task-processor/issues/605)。

- Product Decision: **PD-SUPPLY-CHAIN-FULL-SHEIN-FIRST-2026-10-08**。
- Design Basis: **Independent Architecture**。
- Status: **IMPLEMENTATION_READY / FROZEN**；独立准入结论写入 §12 与 Issue 当前正文。
- Investigation baseline: `main @ 01cdce76040f56fc4eb7cdef301f5a4850893190`。
- One Writer / one primary branch: `codex/my-supply-chain`。

## 1. 用户结果和范围

用户于 2026-10-08 明确要求“按当前 Figma 完整交付，包含实际上传与可选智能体优化”，并指定“先接 SHEIN 美国站”。本批次不能以只读商品列表、介绍页面或本地草稿替代完整路径：

`我的数据商品批次 → 待适配 → 待补全（按需）→ 已适配 → 直接上传 / 可选智能体优化及待审核 → 已上传`。

面向当前企业内获授权的运营人员。支持已准入采集来源和自有商品；商品/批次管理包括查询、详情、下载、改名、移动与归档、转入待适配。发布按商品独立记录，批次展示真实逐项结果；原始资料和原渠道记录保留。用户可以在已有美国站店铺之间新增发布目标；其他平台按后续准入顺序开放，不能把图上示例选项当作已经可用。

范围内需要补齐 SHEIN 官方商品接口、当前运行时接线、规则/资料与批准绑定。可选优化只选择当前已准入、已启用的 Agent 和版本化模板，复用 Product Agent、ImageAgent、Review/Asset 的现有预算和审批；不因为原型中的示例名称新增 POD 智能体或第二套 Runtime。

不做 ERP/PLM 等新连接器、硕米自营/优选市场建设、通用数据集市场、全部平台同时开放、历史数据迁移、旧 ListingKit/Workspace 兼容或通用批处理平台。真实店铺写入、付费 provider 调用、部署、合并与用户验收分别需要适用的运行授权；开发用确定性 fixture，不签发产品验收。

## 2. Product / Figma Authority

依据 [final UI / IA](../product/final-ui-ia-authority.md)、[greenfield baseline](../product/greenfield-no-legacy-migration.md) 和以上本轮用户决定。2026-10-08 只读实查文件 `tg48P46SSXl6TBy9lZwg63` / 页面 `31:463`，只采用当前可见、非归档节点：

| 产品操作 | Figma 节点 | 当前语义 |
| --- | --- | --- |
| 我的供应链介绍 | `926:569` | 商品先进入我的数据；直接上传与智能体优化分支 |
| 商品采集批次 | `430:2067` | 原始商品查看/下载，转入待适配 |
| 批次管理、转入确认 | `3342:372` / `3342:359` | 改名/归档，原始数据保留 |
| 商品详情/移动批次 | `3395:389` / `3404:440` | 来源资料只读、修改成员引用 |
| 自有商品库/添加 | `3500:1100` / `3502:405` | 手动新建、Excel导入、图片与资料录入 |
| 待适配 | `3507:1965` / `3507:3143` | 选择批次、启用规则、平台与站点 |
| 待补全 | `2667:1948` | 查看必填缺失项、手动/已准入 AI 补全 |
| 已适配 | `977:359` | 智能体可关闭，选择平台/站点/店铺 |
| 待审核 | `2738:359` | 展示本次修改、修改/重构/通过 |
| 已上传 | `2738:655` | 真实平台标识及渠道；新目标不影响原记录 |

当前 `926:569` / `3507:*` 是批次入口依据。较早 `429:1620` 的“加入后自动进入待适配”不恢复为自动发布或跳过当前转入确认的依据。原型示例数量、平台 ID、模板或来源均不是事实。

## 3. 可复用能力与真实缺口

| 现有能力 | 当前证据 | 本批次使用方式 |
| --- | --- | --- |
| 采集与来源发布 | `internal/app/productsourcing`、`internal/product/sourcing` | 复用 publication、原始 evidence、UNKNOWN 与当前费用规则 |
| Catalog | `internal/product/catalog/repository.go`、`internal/integration/persistence/product/catalog` | 精确不可变版本；新增有界集合读取，不复制商品正文 |
| Product Review | `internal/product/review/model.go` | 当前为 title-only；必须承认其字段限制，不能假称完整图文审核已存在 |
| Asset / ImageAgent | `internal/product/asset`、`internal/imageagent` | 来源图是候选；显式批准后消费精确 inventory |
| DRAFT-S1 | `internal/listing/record`、`internal/marketplace/shein/draft` | 复用不可变本地资料/校验；尚不是可上传 payload |
| SUB-K1 | `internal/listing/submission`、对应 persistence | 复用唯一 SendPermit、target fence、UNKNOWN；没有当前平台接线 |
| Store Center | `internal/storecenter`、`internal/app/storecenter` | 当前成员店铺访问、权益/连接、加密官方凭据 |
| SHEIN OfficialClient | `internal/integration/shein/official_client.go` | 当前只有授权交换/店铺查询；复用签名与 transport，新增商品窄 adapter |
| 运行时 | `internal/app/runtime/currentapplication` | 已有 Product/Store/Agent 等独立 pool；禁止照搬 DRAFT-S1 单库组合 |

**架构问题**：DRAFT-S1 的隔离组合同时读取 Product/Asset/Store 单库，当前运行时要求 Store 独立数据库。直接挂载该 constructor 会违反实际 ownership/role 边界。须提取其合法窄 Port 与业务用例，由当前 App 注入各 owner，不能复制 Store 表或开放 Product 角色读取 Store 凭据。

另外，当前 Product Review 的 `DecisionInput` 只支持 Title，且生产候选 policy 是 `title-review-v1`。完整图文优化必须逐字段引用 Product Review / Asset 的实际批准回执；没有对应已准入模板时显示不可用，不能把 title 回执当成整份资料通过。

## 4. Domain / Fact Owner 与调用链

| 事实 | owner | 新增事实的最小边界 |
| --- | --- | --- |
| 原始来源 evidence、publication | Product Sourcing | 沿既有原子发布/证据保存；不改采集 UNKNOWN |
| canonical Product | Catalog | 已有不可变产品版本；自有录入走同一 owner |
| 商品批次与成员 | 拟新增 `internal/product/collection` | 只持 Organization、名称、版本、归档与 Product/publication 引用 |
| 目标平台资料 | Listing Record | 不可变目标资料版本、用户补全与确切输入/规则/批准引用 |
| 适配/发布批次控制 | 拟新增 `internal/listing/preparation` | item 与 target 的关系、期望版本、阶段和现有 owner 回执引用 |
| Proposal/批准/应用 | Product Review、Asset | 原 owner 扩展已批准字段范围时必须保持原 version/receipt 语义 |
| AI预算、计量、结算、恢复 | 现有 Agent / Commercial | batch 只持 run/quote/receipt ID，不另计费或重发 |
| Store身份/凭据/服务权益 | Store Center | 窄的 server-only 官方商品执行授权 Port |
| 外部执行与 UNKNOWN | Listing Submission | 唯一 intent/attempt/fence/send authority |
| 平台商品标识/远端状态 | Marketplace/Listing | 仅接收匹配提交/核实结果，保留渠道维度 |

调用链：Console → 同源 BFF → verified effective Organization / live module grant → Product collection / Listing preparation → Catalog、Review、Asset、Store 窄 Port → marketplace SHEIN rules / integration official adapter → Submission Kernel → 真正的平台回执 → Console 投影。

App 持有 composition 与跨 owner 调用，Domain 不直接访问 HTTP、GORM、provider SDK 或凭据。`preparation` 是这个用户流程的有界领域能力，不是重新建立 Listing Workspace、Task-first Product 或通用 Orchestrator。

## 5. 数据、版本与事务

新增 Product collection 与 Listing preparation/target-binding 表由显式 schema 初始化创建；不让 serving constructor 自动迁移。复用 Product 数据库容纳本批次 Product/Listing 的有界事实和 Submission 持久执行表，按表/列授予最小 serving 权限；Store/Asset/Agent/Commercial pool 继续独立，禁止跨库复制 owner 表。

当前 acquisition serving role 的 readiness 明确拒绝未准入表权限。启用本模块时，初始化 grant 与只读 readiness 必须在同一最小权限清单中显式包含 collection、preparation、Listing record 与 Submission 表；不能仅增加 GRANT 后跳过原 verifier，也不能改用 owner/superuser 连接。未启用模块保留原清单。Product 数据库的当前 app 注入按这一 enabled capability 配置选择一致的 verifier；部署配置缺 schema/权限时 fail closed，不在 startup 修复或迁移。

1. `CollectionBatch`：Organization、OwnerActorID、OwnerMemberID、BatchID、名称、创建时间、Revision、ArchivedAt。
2. `CollectionItem`：Organization、OwnerActorID、OwnerMemberID、ItemID、BatchID、ProductKey、SourcePublicationID、OriginalSnapshotVersion、来源方式、SourceOperationID（采集来源）、Revision、ArchivedAt。原始版本永久指向来源资料；不随优化结果漂移。
3. `Preparation`：Organization、PreparationID、原 collection revision 与选定成员快照、RuleID/Revision、Platform/Site、Actor、幂等键/输入 hash、Revision。
4. `PreparationItem`：PreparationID、ItemID、目标维度、CurrentRecordID、CatalogVersion、资产 inventory version/hash、Stage、缺失项与校验引用、AgentRun/Review/ApplyReceipt 引用、Revision。
5. 目标资料正文、诊断与输入 hash 由 Listing Record 的不可变版本保存，不能同时写一份可变 preparation payload。`preparation` 只引用它。
6. 外部状态仍来自 Submission attempt 与平台回执；`uploaded` 是投影，不新增一个可以独立签发成功的布尔字段。

批次新建/改名/成员移动/归档与操作回执在 Product 数据库共享事务内完成；按 Organization + OwnerActor-qualified ID 锁定相关批次及成员、核对 expected revision。同 key 同 payload 返回原回执，不同 payload 409；数据库 COMMIT 不确定以原 key 查证，不能伪装失败或自动另建请求。

转入适配在同一事务固定选定成员及源版本，批次之后移动、改名/归档不篡改已有 preparation。归档只隐藏列表项，不删除来源、target、批准、financial 或远端资产；不因归档取消已经发出的外部调用。自有商品创建把声明为用户提供的来源资料、Catalog publication 和 collection 引用放在同一已有 Product UoW；图片仍需 Asset 显式批准。

外部 owner 读取不能伪造跨库原子事务。Listing 资料固定不可变 Product/资产/规则/模板版本及 hash；创建与发送各自重新核对 live Organization/Store access、Store connection/service version 与 exact owner refs。漂移返回冲突/需重新校验，不自动切到最新版本。Store凭据只在 server-only执行授权 Port 内解密，不落入 Listing/Product payload、前端或日志。

### 5.1 选择、来源所有权与保存合同

现有 `PublishedAcquisitionReader` 是按 Organization + Actor 限定的单次采集回执读取，不是企业共享商品列表。本批次保留此访问范围：批次、成员、preparation 和其 operation 均保存 OwnerActorID / 原有效 OwnerMemberID；读、下载、移动、归档和 worker 读取都同时限定 Organization + OwnerActorID。企业管理员角色不会自动取得其他操作人的来源资料。没有本轮授权的企业共享功能不得通过 collection read 权限隐式开放。

新增 `CollectionSourceReader.ReadSource(ctx, AuthorizedSelection)`，返回精确 `ProductKey / PublicationID / OriginalVersion / SourceKind / SourceReceipt`。`AuthorizedSelection` 只能由 collection owner 核对当前身份或 durable execution authority 后创建，不能接受客户端自报的 ProductKey 作为读取权限。采集分支先核对原 Organization/Actor/OperationID，再使用精确 Catalog 版本；自有录入分支核对本 owner 的创建回执，不能伪造 acquisition operation。仅在所属来源已经授权后调用 Catalog 的窄 VersionedSnapshotReader。

当前 Product Agent / AI Workbench 的选择以 acquisition OperationID 为依据。接入本流程时新增显式的 collection selection 分支，消费上述已授权、版本化 source receipt；原 acquisition 选择分支保持其授权合同。不得用任意 ProductKey 绕过原 source owner，也不得创建假的 operation 来满足旧接口。

新采集发布在当前 Product UoW 内原子保存 publication、采集成功回执与默认 collection 成员引用；collection 写入由 app 注入同事务窄 writer。任一步失败均不签发采集成功。OperationID 派生稳定成员身份，重放返回原引用。已存在的当前采集结果只在用户显式选择其自己的 operation 后以幂等命令加入，不进行历史回填、迁移或扩大可读范围。自有录入使用同一 publication/collection UoW。

数据库约束包括 Organization + OwnerActor + command key 唯一、payload hash 比较、Organization/OwnerActor-qualified batch/member 外键与 revision CAS；移动只接受同 owner 的有效目标批次。原 SourcePublicationID / OriginalSnapshotVersion 不可修改；优化后的 EffectiveCatalogVersion、target record revision 单独保存。操作结果按同 scope 原 key 查询，UNKNOWN 不另建请求。

## 6. 状态与用户操作

`待适配/待补全/已适配/待审核/已上传` 是 item+target 的产品投影，不能给 Product 全局添加一个跨平台共享状态。一个商品在店铺 A 已上传，仍可在店铺 B 待补全。

| 事件 | 前置条件 | 持久化效果 / 失败 |
| --- | --- | --- |
| 转入待适配 | live权限、来源成员/版本存在 | preparation+固定输入；原来源不改 |
| 开始适配 | 批次、启用规则与US目标已明确 | 创建逐项目标资料；规则失败不写已适配 |
| 缺必填内容/批准 | deterministic诊断 | 待补全，保存具体字段/原因；无假默认值 |
| 手动补全 | expected item/record revision | 新不可变 target record，记录用户来源；再次校验 |
| AI补全/优化 | 已启用Agent、实际支持字段、quote/确认 | 引用既有run/候选/Review/Asset；不自动Apply或上传 |
| 完成适配 | exact inputs、当前规则校验全部满足 | 已适配；本地资料完整不等于远端已收取 |
| 智能体关闭→开始上传 | 当前Store/权益、资料版本已确认 | 直接经过同一Submit准入，不要求无关AI能力可用 |
| 智能体开启→开始重构 | 绑定Agent/模板revision和输入 | 候选到待审核；失败/UNKNOWN不能变已适配成功 |
| 修改/重新重构 | 当前review版本、原scope仍有效 | 原owner保存新revision/run；旧批准不覆盖新内容 |
| 通过 | 明确选中实际改动、owner批准/Apply回执 | 全部required receipts齐全后重新校验；过期/部分失败仍可见 |
| 发送/查询平台结果 | 下节Submit边界 | 逐项真实结果；response loss保留UNKNOWN |
| 再次发布 | 明确新Store/target、重新适配/验证 | 新target intent，原渠道记录不变 |

必须保留未开始、执行中、失败、结果待核实和不可用状态。批量操作记录固定item选择和逐项结果，不用全部成功掩盖部分失败，也不对成功项再次调用provider。

Product字段优化继续经 Product Review → Catalog 新版本；原始版本不删。目标特有的类目/定价/属性补全属于 Listing record。图片候选/批准继续由 Asset/ImageAgent处理。不得让新target资料直接篡改 canonical Product，也不得让原title-only审核回执批准描述/图片。

## 7. Agent与持久执行接线

复用当前 Agent配置、版本化模板、quoted invocation、预算确认、BusinessTask及其UNKNOWN/结算合同。只有真实启用且可处理所选字段的模板进入选择器；无route/余额/模板时给出真实原因，直接上传分支仍可使用。

批次执行使用仓库已有 Temporal SDK/worker边界的一个有界workflow，以 PreparationID+operation key 为稳定workflow identity；workflow只编排 owner命令，不成为商品/批准/平台事实源。每个item调用带原key的command；不可重复活动以领域持久回执核实，Temporal Activity retry不能绕过owner的UNKNOWN规则。

创建批次操作及prepared intent先在Product/Listing UoW中提交，再以相同workflow identity启动。响应或启动不确定时只核实相同identity；重复启动不得产生第二workflow。若数据库已提交而workflow未启动，读取/重试原operation可调用同一有界 `EnsureExecution` 恢复入口；未确认启动前UI显示待启动，不声称处理中。已有run/terminal回执仅投影，不重新执行。取消只作用于尚未claim的项；已发出的调用核实原结果。

本批次不新增Scheduler/Reconciler、通用outbox平台、批次runner或验收session工具。若现有Temporal/Agent合同不能满足某个required变更，先指出具体阻碍并局部修正设计，不建第二套运行时。

### 7.1 后台授权与稳定执行合同

请求内 `product/sourcing` 的 publication read proof 最长五秒，并绑定已认证身份及 request deadline。它不可以序列化给 Temporal。持久命令只保存经过 verified identity 核对的 Organization / Actor / Member、所需 permission purpose、selection receipt、expected versions、输入 hash 和 operation ID；不保存 JWT，也不在 worker 中制造 `AuthenticatedIdentity` 或延长 TokenExpiresAt。

新增 owner 窄口 `ExecutionAuthorizer.AuthorizeExecution(ctx, ExecutionSubject) -> AuthorizedExecution`，沿现有 ImageAgent.ExecutionAuthorizer 的模式。每个 activity 先加载原 durable command，再使用当前 live Organization member/role owner 核对原 Actor + Member 是否仍有效及所需 module grant；Store 副作用另核对当前成员店铺授权和服务。授权结果仅供当前有 deadline 的 activity 消费。原请求证明和 worker 执行证明使用不同入口，不能互相强制转换。撤权后尚未发送的操作记录 denied/suspended，已发送的操作继续由原 Submission owner 核实；核实不得触发新的写入。

需要读取来源、发起 Agent、应用 Review 的 worker 用例提供明确的 `AuthorizedExecution` 入口，仍调用原 owner 的权限、输入版本和回执规则。HTTP 入口保持现有 verified identity 规则。Agent 使用新 collection selection receipt 时由 collection/source owner 重新授权，不能把保存的 receipt 当永久权限。

`EnsureExecution(Organization, OwnerActor, OperationID)` 先读原 operation：terminal 只返回原结果；pending 使用固定 `supply/<Organization>/<OperationID>` workflow ID；already-started 只核实同 workflow；启动不确定保留 pending/start-unknown 并核实同 ID。worker 从数据库取固定 item membership，按 page size 内部分页，逐项 command key 为原 operation + item + action。请求超时、worker 重启和 Activity retry 不改变身份、不重复 terminal 项、不重发 UNKNOWN 项。取消只阻止未 claim 项。该恢复入口只恢复已有命令，不创建另一套 scheduler。

### 7.2 优化、批准与资料版本绑定

首版选择器只开放实际已启用且支持当前字段的 ProductTitleAgent / ImageAgent 及其版本化模板。标题继续由当前 title-review-v1 / Review.Apply 处理；图片由 Asset owner 的候选和显式批准处理。描述、POD 或其他字段在没有实际已准入 owner/模板时显示不可用，可由用户手动补全 target record。不得将原型模板示例伪造为服务，也不得用 title 回执批准整份资料。

每个优化请求和批准消费绑定以下向量：Organization + OwnerActor/Member、CollectionItemID、OriginalPublicationID/Version、EffectiveCatalogVersion、Platform/Site/Store、TargetRecordID/Revision、RuleRevision、TemplateID/Revision、Candidate/RunID、Review/ApplyReceipt 或 AssetInventoryVersion/Hash。已有 owner 的 receipt 不存储全部 target 维度时，preparation 保存 target-to-receipt 的不可变绑定，并核对该 owner receipt 自身的 exact source/version/field；不能声称原回执包含它没有核对的维度。

Title Apply 成功后核对 receipt 指向的 ProductVersion，再生成绑定该版本的新 target record；Asset 仅引用批准 inventory 的确切版本与 hash。每个被修改字段都必须有匹配它的 owner receipt 或明确用户手动资料来源。缺回执、部分 Apply 失败、原字段/规则/模板版本漂移时仍为待审核/待补全或冲突，不能上传。用户修改生成新不可变 record，并使旧校验及绑定失效；重新校验后才可开始上传。title-only 批准不覆盖描述、图片或法律事实。

## 8. SHEIN官方上传与UNKNOWN

官方资料查证于2026-10-08：

- [商品发布/编辑](https://open.sheincorp.com/documents/apidoc/detail/3000888)：有正式商品创建接口，返回业务结果和SPU/SKC/SKU标识。新建与编辑的标识要求不同，不能把重新发布当作编辑原渠道。
- [平台商品查询说明](https://open.sheincorp.com/documents/apidoc/detail/3000196-1000001)：待审核/审核失败商品可能不出现在已审核商品查询中。因此“列表查不到”不能证明没有发生上传。
- [官方API目录](https://open.sheincorp.com/documents/apidoc/1000001)：发布权限、站点、末级类目/属性规范、图片转换、提交与审核查询分别是平台合同。具体payload必须按对应官方版本构建，不能直接发送旧seller后台DTO。

`internal/integration/shein`扩展server-only的有界商品adapter，复用现有签名与受控origin/HTTP transport。platform rules owner固定US/en目标、官方可发布站点和店铺资质、类目/属性/图片/价格/sku等规范的版本化快照。沿既有纯规则，不能自动AI猜出品牌授权、产地、证书等业务事实；缺失则待补全。

新 `StoreProductAccess` Port由Store owner提供live成员访问、有效服务周期、Store platform/site、connection revision和短生命周期的merchant-bound执行能力。Listing/BFF不能接受浏览器提供openKeyId/secret、supplier identity或任意endpoint。凭据缺失/撤销/漂移fail closed；新业务请求不得调用旧登录profile或seller网页发布runtime。

商品提交 target fence 以 `Organization + Platform + Store + SubjectID` 固定。`SubjectID = "product-us-" + SHA256(canonical ProductKey + length-delimited Site)`，同一 canonical 商品在同 Store/Site 的所有批次、资料版本与 operation 都使用这个稳定 SubjectID。TargetRecordID/Revision、operation 和 action 只绑定 intent key / exact payload fingerprint，不能进入商品 SubjectID 或因更改资料绕开尚未解决的 UNKNOWN fence。用当前 canonical fingerprint 与 provider execution key，不自行添加未被 SHEIN 合同承诺的幂等 header。

1. 重新核对live访问、Store服务/connection、exact资料/批准/规则，建立Submission intent。
2. 只有现有内核首次committed SendPermit允许发送；重放、COMMIT-unknown或claim过期无第二permit。
3. 需要对平台产生写入的图片处理也必须标出其effect与safe-retry合同；不把重复注册/上传藏在“纯适配”里。没有官方安全重放证据时用现有execution内核记录独立image-effect key/结果，UNKNOWN不重发。最终商品payload只引用已确认的图片结果。
4. 商品提交HTTP和业务成功、具体标识、匹配intent/fingerprint及provider证据由adapter严格确认。200、空响应、缺标识或未知错误不能直接作为uploaded或definitive-no-effect。
5. 完整成功响应原样受限保存必要标识与trace/hash，Submission原子finalize；前端显示“已上传/平台审核中”，不声称已上架或审核通过。
6. 超时/响应丢失/发送后取消/lease过期保持UNKNOWN；只由原Submission核实。同SKU或已审核列表无匹配不构成no-effect证明。
7. 有qualified读回证据才adopt成功；没有SPU/无法唯一匹配则保留UNKNOWN，沿既有人工授权/evidence contract处理，不能自动重发。
8. 真实确定的无副作用失败可以让用户修正资料后建立新intent；任何结果不确定的同target attempt未解决前，target fence继续阻止新的重复写入。

平台发布权限、证书、库存、类目规范及连接资质可能随店铺变化，上传前按当前Store和官方合同重查；不拿原型SHEIN-US示例ID或静态全局类目表冒充current owner事实。

### 8.1 Store → SendPermit → adapter 的窄合同

`StoreProductAccess.Authorize(ctx, ExecutionSubject, Target, ExpectedConnectionRevision)` 接收 server 取出的 Organization/Actor/Member、StoreID、SHEIN/US 和原 operation 身份，返回不含密钥的 `MerchantBinding`（StoreID、platform/site、connection/application revision、supplier identity hash、service validity）及进程内有 deadline 的执行句柄。Store owner 使用当前 live member grant、有效服务、已验证官方连接和凭据版本产生句柄。秘密仅由 Store 私有执行实现解密并交给 integration 签名；不返回给 Product、Listing、BFF、Temporal payload 或日志，不缓存过期权限。

顺序固定为：worker reauth → 精确 source/record/approval/rule 检查 → Store Authorize → 建立/读取 Submission intent → 原内核 claim/committed SendPermit → Store 发送前再核对同 MerchantBinding → adapter 单次调用 → 原内核 finalize。发送前连接/授权漂移拒绝执行。若已取得 SendPermit 后才发现授权变化或进程在 wire-send 前退出，由于现有内核没有 local-no-dispatch evidence kind，保守进入 UNKNOWN，不能制造 provider_response 或自行宣布无副作用；沿现有 qualified readback/manual resolution contract 处理原 attempt。批准回执、权限或连接漂移均不得换 payload 后重用旧 permit。

图片调用使用独立 `image-us-` SubjectID namespace，按 Store/Site + exact source-image hash + effect action 规范化摘要；不得随 operation/record 变化，也不共用商品 subject，避免同一批次图片处理与随后商品提交互相阻塞。图片 intent 绑定确切输入/provider规则版本，重复批次复用原 effect 身份和成功结果。平台已确认的 image result 与原输入绑定，最终商品 payload 只消费这些结果。没有官方 safe-repeat 证明的 image-effect 也使用同 Submission 内核和 fence；UNKNOWN 先核实原 effect，不由 Temporal 自动重发。纯本地图片格式校验无 provider mutation。

adapter 返回三种明确结果：`ConfirmedSuccess`（business success、必要 SPU/SKC/SKU 标识、trace、exact payload correlation 与证据 hash）；`DefinitiveNoEffect`（官方明确拒绝且可证明未发生写入的已列举业务结果）；`OutcomeUnknown`（其余情况，包括 transport error、缺标识、未识别 code、响应丢失、发送后取消）。只有现有内核验证的 evidence 能转移状态；200 或 query 列表缺失都不能作为证据捷径。初版不把未知业务错误归入无副作用失败。

官方 payload 在 marketplace SHEIN owner 定义独立 DTO 与 deterministic validation：发布站点/语言、末级 category、类目属性及销售规格、SKU/价格/库存、品牌/产地等必需事实、已确认平台图片引用。规则与类目规范由官方只读接口或版本化 fixture 提供；缺必需事实保持待补全，不能复用旧 seller DTO 或自动补造合规事实。正式请求由当前 official adapter 编码、签名和发送；实现阶段对实际 DTO/response 做 fixture 合同测试，真实调用仍 NOT_RUN。

## 9. HTTP、权限、资源与恢复

拟新增同源BFF/current-application路径：`/api/v1/workbench/product-collections`（批次/商品及导入/下载）、`/api/v1/workbench/supply-preparations`（转入、适配、补全、优化、审核投影）、`/api/v1/workbench/listing-submissions`（提交/同intent核实/读取）。handler仅验证/dispatch，Domain决定事实。

新增权限沿现有authz、enterprise module catalog和角色管理：商品collection read/manage、supply preparation read/manage、listing submit。已有角色的当前权益不被新功能覆盖；不把菜单可见或Product采集写权限自动解释为Store发布权限。API来源Organization/Actor来自verified identity；所有operation、item、target、record、run和receipt查询含Organization，返回not-found而非泄漏外企资源。

候选界面在isSwitching/selectionRequired/撤权时取消读请求、清空scope内选择与详情。服务端不接受客户端Organization当授权依据。读/下载/批量command范围都在服务端过滤，不在一页数据上计算总量。

初版边界：列表page size≤100、keyword≤80UTF-8 bytes、cursor稳定排序绑定查询与Organization/OwnerActor；普通命令≤2MiB并有deadline。选择整个批次时事务固定完整成员快照，worker 内部按100项分页处理并展示逐项进度，不要求用户手动拆批次，也不静默截断选择。Excel导入只允许声明格式及有界行数/单元格，拒绝公式执行/宏/外部链接，导出CSV转义公式前缀；具体行数/文件大小由已有请求与导入资源配置约束并在UI明确显示。资源约束不改变用户所选完整批次。

长任务POST返回持久operation引用，读接口deadline≤10s；慢provider在既有worker中运行。费用仍按现有quoted owner逐项结算；适配/批次管理不自行发明收费。worker资源上限及部署配置在本批次正常runtime接线中明确，缺必需依赖constructor fail closed；内存仅用于测试。

没有durable operation时失败可更正后重试；durable key一旦进入UNKNOWN，前端保存key并提供原operation核实。服务重启恢复数据库回执与同workflow identity；不依赖localStorage制造成功。归档与stop/restart不删除数据；destroy另行授权。

## 10. Legacy decision与复用

Legacy decision: **EXTRACT | RETIRE**。

- Reusable behavior：已合格的SHEIN纯规则、不可变资料、validated source映射、官方签名transport、Submission内核、当前Console/BFF组件。
- Current owner：Product collection/Catalog/Sourcing、Listing Record/preparation/submission、Marketplace SHEIN、Integration SHEIN、当前App。
- Cutover/deletion condition：本路径零root ListingKit/旧Workspace/Task/tenantbridge/compatibility/profile依赖；只删除此次确实切换的旧caller，不把全仓物理退休作为额外前置。

复用成熟xlsx/parser、Temporal、GORM与现有React表单/表格组件；实现前只查所需正式文档及已安装依赖，不造新Excel解析器、UI框架、队列或审批系统。

## 11. 验证与交付顺序

同一个主要PR连续完成：商品批次与正常入口 → target资料/适配及补全 → 可选Agent审核 → 官方上传/核实与逐项结果 → 正常运行时和用户交接。小片是提交和必要自检边界，不机械拆PR。

TDD覆盖当前修改的不变量：同key冲突/回放与COMMIT-unknown、跨Organization/撤权、不可变来源与target版本漂移、批次成员移动/归档、source图片未批准、缺必填字段、关闭Agent直接路径、审核/Apply后再验证、单次SendPermit与部分失败/UNKNOWN核实、成功后原渠道保留。复用现有真实PG、provider fixture、BFF/component与architecture guard；不开发专门验收平台。

独立检查仅执行本批次高风险边界准入和最终交付检查；普通修复复核相关增量。最终候选稳定后运行必需CI，以准确HEAD记录；merge/main CI与部署、实际provider运行和用户验收分别取证。

实现完成交出访问入口、正常启动/登录操作、数据owner/保存及stop/restart说明。fixture的已上传只证明fixture transport，不等于SHEIN真实上传；开发者不标产品验收PASS，不自行关闭#605。

## 12. Architecture Admission

当前结论：**IMPLEMENTATION_READY / FROZEN**。2026-10-08 独立 reviewer `/root/supply_architecture_review` 完成 R2 并给出显式开工准入；准入时尚无生产代码/schema变更。

独立Reviewer应只核对本次用户Must和高风险边界，对finding按AGENTS分类；不得把全部旧父Issue关闭、全仓搬迁、全平台开放或新验收工具追加成前置。

R1 独立结论为 NOT_READY：要求补齐 canonical selection / worker authorization、逐字段批准绑定、Store → SendPermit → official/recovery 三处合同；该轮没有独立验证出代码 BLOCKER，也没有要求新增产品决定。R1 未逐行验证所有现有端口，不能当作实现或完整代码评审 PASS。

上述补齐分别在 §5.1 / §7.1、§7.2、§8.1。R2 实际核对 source/readproof 与 actor-qualified SQL、AI Workbench selection、Review.Apply 的 title-only 语义、Asset approval、ImageAgent worker live authorization、Store credential/member ports 和 SUB-K1 fence/permit/evidence。发现并已修正商品 SubjectID 必须跨批次/record/operation 稳定、图片使用独立 namespace 的防重合同。无剩余 BLOCKER，正常架构评审两轮收束。

以下保留 IMPLEMENTATION_TEST：StoreProductAccess 必须组合当前成员/发布权限/服务/connection 而非只读凭据；enabled capability 的 schema/grant/verifier 一致及同 workflow 恢复；官方 DTO/响应、逐字段批准、部分失败/成功重放和跨批次 UNKNOWN fence。实施通过对应测试收敛，不重新打开无 Blocker 的冻结设计。

真实密钥/店铺配置与付费调用不是架构准入前提；真实运行与用户验收保持 NOT_RUN。实现阶段严格消费本冻结合同，不临时发明第二事实源或授权恢复协议。设计准入不等于实现完成、真实上传或产品验收。
