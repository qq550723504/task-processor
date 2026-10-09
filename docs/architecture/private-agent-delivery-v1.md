# 指定企业私有智能体交付 V1

Status: 原手工输入基线 APPROVED / IMPLEMENTATION_READY；下述平台草稿范围修订 DRAFT / PENDING_ARCHITECTURE_REVIEW (2026-10-09). Design Basis: Independent Architecture. Execution: #611, primary PR #617. 修订准入前不修改正式生产路径。

## 当前产品决定：上传前的平台草稿质检

用户于本会话 2026-10-09 指出应选择草稿产品，随后明确选择“上传前的平台草稿：检查待补全／已适配的商品资料”。本节替代下方原手工输入范围；下方保留为已发生的设计、实现及隔离试用历史，不恢复为当前要求。

### 用户结果和范围

当前企业获授权运营人员进入指定企业的私有智能体，选择自己的供应链批次、SHEIN 美国站店铺及待补全/已适配草稿，直接取得可保存、可追溯的资料质检报告。用户不用重新填写名称、材质、尺寸或规格。报告记录草稿 ID/revision、source/store、商品版本、草稿内容/规则/资产 hash、草稿保存时间和检查时间，以及保存时确定性校验的问题；修改后的草稿不能把旧报告当作新版本结果。

只复用当前唯一支持的平台 SHEIN 美国站。报告投影 `TargetRecord.Result.Issues` 与 `ReadyForUpload`，不另造必填/规格/图片规则；明确“基于该草稿保存时规则”，不宣称重新查询了最新官方规则、验证了图片当前可访问性或取得平台发布许可。原上传 owner 的实时授权/规则/资产/fence/UNKNOWN 核对保持独立，报告零问题不替代上传批准。确定性固定代码能力仍不调用模型、不计费、不发布、不改商品，也不调用新的外部 API。

Out of Scope：批量执行、自动补全/Apply、图片处理、模型/provider、实时官方规则重新查询、上传门禁新依赖、其他平台、通用 Agent runtime、通用报告平台、交付版本升级/转移/撤销、旧系统兼容或数据迁移。使用者从现有供应链页补全和保存，再选新 revision 检查；不增加新的人工审批流程。

UI/Product Authority：上述用户明确决定；`docs/product/final-ui-ia-authority.md`，Figma `31:463` 当前智能市场企业 My Agents `4703:429` 与 `docs/architecture/my-supply-chain-shein-v1.md` §2/§6 的待补全/已适配投影。沿用现有 private detail、Console、供应链选择 API/组件与正常入口，不新增顶层菜单或旧商品中心。

### Owner、合同与注入

复用已批准 `my-supply-chain-shein-v1.md` §4/§5.1：Catalog 拥有商品；collection/preparation 保存当前 Organization + OwnerActor + Member 的来源授权；`internal/listing/record/target.TargetRecord` 拥有不可变平台草稿、revision、精确输入、保存时 goods 校验和 hash。待补全/已适配是 item+store 投影，不能给 Product 增加全局 draft 状态。专员和企业管理员均不能绕过来源 owner 读取另一操作人的草稿。

新增有界的只读 app Port：`DraftInspector.Inspect(ctx, scope, recordID, expectedRevision, requireHead)` 返回报告需要的引用/摘要/问题。实际 adapter 只消费现有 `supplychain.Application.ReadRecord`、`ReadTarget`，检查当前 authenticated identity、live supply.read、来源 owner、exact record/revision/target/source/store/hash 一致性。没有 SupplyChain runtime 时执行明确 unavailable，私有交付列表及历史读取不依赖其启动。`agentcustomization` 不注入 Product/Store/Asset SQL pool，不直接读其他 owner 表。

contract → implementation → injection → consumer：私有执行 command `{recordId, expectedRevision}` → customization Service/Repository 的有界 inspector callback → 当前 supplychain app 的授权精确草稿读取 → 保存时 goods 校验结果的报告投影 → customization 同库 immutable run → 既有 HTTP/BFF/private detail。当前 `current_application` 在 SupplyChain 建成后注入窄 Port，不增加第二个 SupplyChain constructor、Temporal worker、store 授权体系或 rule engine。

### 权限和持久化

私有 delivery 继续企业共享：agent.read 读取、agent.use 执行、原需求企业唯一目标。**草稿执行结果改为当前 Organization + Actor 私有读取**，即使企业内另一成员可使用同一 delivery，也不会获得他人的 draft/report。读取草稿报告还须经 inspector 当前 supply.read/来源授权；报告不是获得来源访问的替代凭证。撤权/切企业时前端清空草稿和报告，服务端不信任客户端发送的商品正文、校验结果、organization/actor。

继续使用现有 `quality_runs` 和唯一 `(organization_id,actor_id,key)`；不新增表、schema、Product 写事实或跨库事务。run 只保存窄的不可变草稿引用、草稿标题摘要、保存时 issue 列表和 ReadyForUpload，不复制整份 Product/平台 payload、凭据、图片内容或规则正文。引用与报告是观察结果，不成为 Listing 第二事实源。记录/问题总大小受当前 Target 2 MiB 上限及私有响应 2 MiB 上限约束；报告列表每页至多20，若完整投影超响应上限明确失败，不静默截断检查项。请求只含两个引用字段，最多1 KiB；问题最多4096条，字段/消息沿当前合同有界。

同 key 锁与完整指纹由 customization DB 唯一负责；指纹包括原 delivery、recordID、expectedRevision、固定定义/版本。原 key 同意图先查已保存结果，重新核对其原 record 的当前读取授权后返回原报告；不得因草稿 head 改变用同 key 改写报告。同 key 不同 record/revision 冲突。首次执行才要求所选 record 等于当前 head、revision 一致；不一致返回需重新选择/核实，不能自动读最新草稿。跨库只读不产生副作用：读失败/超时/撤权不写成功报告；计算/INSERT/COMMIT 任一步失败事务回滚或保留 UNKNOWN，客户端冻结原两个引用和 key 后核实。head 在检查中变化，报告依旧准确指向检查时的不可变 record，并明确不承担上传锁或授权。

报告读取只返回当前 actor 的候选；逐条读取原 record 授权，任一来源 unavailable/denied 时整页不返回受保护内容，不用缓存旧授权放行。当前身份 token、成员权限及来源归档按现有读取合同处理；报告不是永久访问授权。幂等回放同样须 current delivery 与源记录授权，cannot replay bypass。

### 未发布版本切换与 Legacy decision

本批尚未合并/上线；新发布固定代码版本 `product.quality.check/2.0.0` 表示平台草稿执行，客户端不能选择定义/版本。已发生的隔离试用 `1.0.0` delivery/report 保持 immutable，只作历史只读，不接受新的手工执行或把原交付静默改为2.0.0。新试用通过正常定制需求/专员交付发布2.0.0，不重写旧 delivery，不迁移或清理已有事实；仍每需求至多一个固定交付，不增加升级机制。

Legacy decision: RETIRE 手工表单和手工执行/checker生产路径；EXTRACT 当前私有交付、冻结命令、normal authorization、持久报告及既有 SupplyChain/Listing 平台校验。历史手工报告只保留不可变读取 DTO，不暴露旧执行入口、fallback 或双写。旧 ListingKit/Workspace/DRAFT-S1 constructor 不参与当前调用链。

### 必要验证和准入

Independent Architecture 的新增高风险边界仅为 source/report 授权及跨 owner 只读观察。本次复核只检查本节实际增量，原人工定制/单库发布有效证据继续复用。必须先获得明确 IMPLEMENTATION_READY，再修改正式代码。

TDD：原手工 payload 拒绝；客户端伪造正文/issue拒绝；当前 source/actor/org隔离与撤权；陈旧 record/revision 新执行拒绝、原 key 回放仍返回原报告；同 key 不同绑定冲突、并发一个结果、失败无成功报告；source unavailable 不泄露报告；不依赖实时外部规则且不写 Product/Store/Asset；正常装配与 BFF/选择/冻结 command。复用已有真实隔离 PG 与供应链 fixture/测试，不新增 runner/验收平台。当前独立本地实例没有 SupplyChain runtime，须如实保留无法选草稿的限制；仅在现有获准隔离范围内复用正式 native supply wiring 与受控 fixture，无真实平台、provider或生产授权。开发自检不签发用户验收。

## 产品结果与权威

用户本会话确认继续推进实际私有智能体交付，并在首个业务选择中授权“你想一个出来吧”。首个业务为商品资料质检：客户手动输入名称、材质、尺寸、规格与描述，生成缺失项、重复/矛盾值及修改建议的报告。首版是确定性资料检查，UI 明示检查规则与限制；没有模型生成、语义推断、行业合规判定或质量保证。报告不是商品正式事实，不修改商品，不发布，不调用外部系统，不收费。后续模型能力须另行按现有 governed AI 准入，不能伪造 Product Agent binding 或借用 ChatPlan operation。

Must：平台专员在定制单上交付代码注册的精确版本；目标企业从已保存需求取得；仅该企业有 agent.read/use 权限的当前成员能读取/执行；我的智能体出现真实私有入口；输入、规则版本、报告、执行 actor/time 真实保存，刷新/重启可读；同键同意图只有一个报告，同键不同意图冲突；人工交付说明与实际可执行交付分开。

Out of Scope：通用插件/Agent runtime、第三方代码/任意 prompt/tools、模型调用和凭据、付费/点数、自动联系、客户知识库、图片/文件输入、商品事实修改、版本升级/转移/单独撤销交付授权。V1 授权持续绑定原需求企业，成员撤权/企业访问撤销立即由既有 live authorization 生效；暂停整个本功能可移除 runtime 模块。不是个人账户私有，UI 明确当前企业。指定真实客户处理与用户验收未授权，使用独立测试企业。

UI Authority：docs/product/final-ui-ia-authority.md；当前 Figma tg48P46SSXl6TBy9lZwg63 页面31:463。2026-10-09 已读取 4703:429 我的智能体企业语义页面（含截图），复用已有 Console/Card/Button/Input/tokens；详情沿用详情页面结构、由本产品决定定义输入与报告，不引入新的顶层页面。定制进度沿用原批准设计，增加真实交付链接。

## Owner 与最小调用链

`agentcustomization` 拥有定制需求与交付聚合：人工事件、immutable delivery（固定定义/版本/目标企业）以及该交付产生的 immutable execution report。它不是 Product Catalog/AgentRun/agentconfig/AI ledger 的第二事实源。报告是交付执行结果，不是 canonical Product，不写已有商品库。

`internal/product/quality` 是无 IO 的当前 Product 资料检查能力，负责 typed input、规则与报告计算。复用标准库校验/集合；仅检查确定性缺失、重复规格键（相同/不同值）、名称或描述相同重复信息、尺寸未说明单位，不猜测事实。不存在值得引入通用规则引擎的当前复杂度，不再建报告平台。

contract → implementation → injection → consumer：quality.Check → agentcustomization.Service/Repository → 同库 SQL adapter → 当前 customization runtime module/正常 HTTP authorization → 已有同源 BFF → 我的智能体私有卡片/详情/报告；平台 progress 命令 → 单库事务发布并关联交付。

现有 `internal/agent` 的 Binding 与 Candidate 强绑定 Product publication/catalog/title proposal，不能用虚构 Product ID 运行人工资料报告。本批保留此边界，质检直接调用有界 read/compute 领域能力；不新增第二个通用 Agent runtime，也不扩展已有 Product title 模板。

## 交付与持久化

复用原独立 agent_customization DB/pool/受限 role/installer。新增 `deliveries(id uuid,request_id uuid UNIQUE FK,organization_id text,payload jsonb)`、`quality_runs(id uuid,delivery_id uuid FK,organization_id text,actor_id text,key uuid,fingerprint text,payload jsonb, UNIQUE(organization_id,actor_id,key))`，两表仅 SELECT/INSERT；索引支持按组织/交付 UUID cursor 有界分页。schema 安装明确执行，serving 构造只 VerifySchema，不 DDL，不初始化示例。

平台进度 Update 增加可选 `deliverQualityAgent=true`。只在目标阶段 DELIVERED 可用（从 DEVELOPING 或 DELIVERED 同阶段记录）；仍要求交付说明、原报价/线下确认和强 If-Match。固定代码定义 product.quality.check / 1.0.0，不接受客户端定义/版本/企业。发布 ID 绑定原请求及精确版本；每单至多一交付。需求锁事务同时写 delivery、更新阶段/版本、事件和原命令回执，全部成功或回滚。相同命令重放仍先取原回执；后续同阶段记录不会重复创建交付。

Request 只存 deliveryId 作为关联，详情读取 delivery owner；不会把旧人工 DELIVERED 自动转换成可用智能体。空安装没有交付。My Agents 私有列表只读 delivery 表的当前企业，独立于 Product title 依赖的加载/错误状态，避免 title runtime 未安装阻断本路径。详情与执行拒绝不同企业的 deliveryId；专员权限不产生企业执行权限。

执行没有 pending/running/unknown 新状态机：同步纯 CPU 规则在一数据库事务内算出结果并持久化 immutable run。输入上限 16KiB，字段字符上限固定（名称120、材质240、尺寸240、描述4000；规格最多20项，键80/值240），输出最多固定数量检查项。客户可保留空字段接受缺失诊断，整个空输入拒绝。每页20，UUID cursor；HTTP/BFF deadline 沿用30/35秒，GET 拒绝未读 body/未知 query，写严格 JSON/同源/UUID key。

完整执行幂等身份 `(organization, actor, key)`，fingerprint 包含 delivery、typed input、固定定义/版本。事务先 advisory lock 完整身份，再检查当前企业 delivery 与原报告。同意图返回原报告，不同资源/输入冲突。不存在外部发送或资金预留；失败回滚可重试同一 key，响应丢失读取/重放原报告。页面保留冻结原 org/user/输入/key 的小型 sessionStorage 命令，撤权/切企业卸载并 abort；刷新可核实原操作，不以新 key 自动重跑。任意输入仅 React 文本渲染，无 HTML 执行、网络或日志正文。

## 权限与验证

私有列表/详情/报告复用 CurrentIdentity + EffectiveOrganization + LiveWrite（含读取）及 agent.read，运行用 agent.use。不接收 org/actor/platform；HTTP route descriptor 明确仅 enterprise。平台发布仍通过 VerifiedRoles + platform-admin + OrgNone，无需推导企业上下文。BFF 延续 identity/org 期望头、正常企业 cookie、same-origin mutation、严格响应大小及身份一致性检查。

TDD：纯能力缺失/重复/矛盾/单位规则；真实隔离 PG 原子发布、并发同键、冲突/CAS、跨企业猜链接、报告读回与不可变权限；正常 HTTP middleware/BFF 路由/输入 bounds；前端读取及冻结命令恢复。沿用现有必要测试，不建 runner/通用验收平台。独立高风险准入检查后实现，最终完整路径检查一次。正常本地入口供用户操作，开发自检不等于用户验收。

Legacy decision: EXTRACT 合格 Console/当前 auth/BFF/SQL transaction/installer；RETIRE 对所有定制交付都展示可用的占位语义。无 legacy wrapper、旧数据迁移、双读双写；旧手工记录保持原人工事实。

## 准入

第1轮独立只读 Architecture Review（/root/architecture_review，设计提交 f56e9299666aa7be9991704d6dcbe114caed4805）明确 IMPLEMENTATION_READY，无 BLOCKER。IMPLEMENTATION_TEST：事务中插入交付后失败全部回滚、发布标志纳入 fingerprint、CAS/并发/同阶段不重复；正常 middleware/BFF 的跨企业/撤权/重放/平台边界；title 依赖缺失仍可用、报告停启读回、冻结命令及 installer/serving 权限。由本 Writer 在同一 PR 收敛，未满足 Must 不得合并。原人工设计仍为其范围的冻结基线，本文件仅覆盖新边界；实现者不签发用户验收。
