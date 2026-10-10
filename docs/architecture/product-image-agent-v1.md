# 商品图片智能体 V1 — 完整图片能力设计

> Status: **IMPLEMENTATION_READY**。2026-10-09 独立 Architecture Review R2 已解除唯一设计 BLOCKER；用户随后确认本线程统一完成本批必要共享接口与启动接线，正式开工依赖已满足。
> Execution: [#612](https://github.com/qq550723504/task-processor/issues/612)；Parent: #137。
> 当前调查基线：main `d95807f13475d27c0b8691f044cca6320a540d06`，2026-10-09。
> Design Basis: **Independent Architecture**。评审证据与实施状态记在 #612 / 主要 PR #616；后续范围内实现沿本冻结合同推进。

## 1. 用户结果与当前决定

使用者是当前硕米内部试用企业中的商品运营人员。用户为商品配置整套主图与详情图规则，启动生成，查看逐图结果和整套进度，人工审核并保存可被现有商品详情和供应链消费的正式图片集合。

2026-10-09 用户明确：“要完整的，不要单图的，单图的只是验证模型能力而已。”本任务不再以 #487 的单白底图页面挂载作为交付终点。#487/#503/#525 已交付的派发、统一点数、恢复和批准能力保留为复用依据，不重写其历史完成或验收状态。

用户随后接受智能体粒度方案：**一个可复用图片智能体；每个商品一个独立 Run/Plan；每次图片生成一个 Slot/effect。** 这是已确认的产品粒度；具体领域合同见已完成独立架构准入的§7。

同日确认两项内容决定：**共用视觉素材仅共用原始素材，主图与详情图两组仍独立生成**；详情任务采用商品全貌、核心卖点、使用场景、结构功能、细节特写、规格尺寸、使用步骤、包装配件。每项可勾选；缺少真实依据时提示补充或取消该项，不编造规格、配件或功效。

用户进一步指出平台与类目图片要求的差异。完整计划必须区分通用内容模板与目标平台发布规则；不把“标准 8+8”写成各平台、各类目的必需张数或通用发布合同。

一个智能体定义可以服务多个商品；商品之间不共享 run、来源身份、预算、审批或恢复状态。单张图不会变成另一个智能体定义。整套协调不表示一次 provider 请求生成所有图片，也不授权额外规划模型调用。

## 2. Product / Figma Authority

- [当前 UI / IA Authority](../product/final-ui-ia-authority.md)，Figma `tg48P46SSXl6TBy9lZwg63` / page `31:463`。
- 2026-10-09 本 Writer 只读实查当前可见设计：`1998:539`、`1998:545` 完整 AI 重构图片设置；`2061:362` 背景；`2144:361` 语言；`4262:1294` 图片维护；`4908:429` 审核单图处理；`5198:429` 预览；`3395:389` 商品详情。
- `1998:545` 已取得当前截图，明确官方 8+8、自定义指令、共用视觉素材/分别生成、主图/详情图、背景和语言。当前可见状态只展开八个官方主图任务；详情八项以本会话用户明确决定为依据，不冒称已观察到对应 Figma 状态。
- “官方方案”是当前产品内容方案的标签，不能暗示其八项内容已由目标电商平台认证。共用视觉素材旁“减少重复生成”与当前用户决定冲突：仅共用原始素材，两组独立调用与计费；正式 UI 必须据此修正文案。
- 背景设置为统一自然语言背景要求，默认白色；语言为英语、中文、西班牙语、法语、德语、日语。
- Figma 的 token 计费文案与 #487 最新统一 AI 点数决定冲突时，沿领域计费决定：图片按实际生成张数与配置价格扣统一 `orgresource.ai_point`，真实 token 仅审计，不暗改成 token 消费。
- Figma 中“数量不限”不能取消当前请求大小、执行预算、时间和资源边界；自定义任务的有界提交规则须在完整设计中明确，不能先实现无限派发。

### 2.1 平台、类目规则与内容任务的关系

规则按 **平台 + 站点 + 末级类目/商品类型 + 适用商品属性/变体 + 店铺应用模式** 解析。不同类目可能共用规则，但不能因为同平台就假定要求相同。主图、轮播附图、规格/变体图、详情内容等位置及其名称由平台合同定义；当前 UI 的“主图/详情图”分组不得直接等同于所有平台的位置类型。

- 平台硬约束：需要哪些图片位置、允许/要求的数量、类型、尺寸/比例、格式/大小，以及有明确官方依据的背景、文字等内容限制。
- 内容任务模板：已确认主图八项与详情八项是可选内容意图，根据商品证据及目标规则筛选和映射；不是强制生成十六张。
- 企业风格/用户指令：背景、语言、构图与其他偏好，只在适用硬约束内生效。冲突在确认计划前显示，不通过提示词偷偷覆盖平台要求。

调用路径：精确目标与来源 → 当前 Marketplace/Store 规则 reader → 固定规则引用/摘要及适用条件 → 图片计划/报价确认 → 逐图生成及本地可确定校验 → 显式批准 → Listing 发布前沿原 owner 再校验。规则变化使原发布校验失效，不删除既有生成资产或自行触发付费重生成。未绑定目标的通用图片仍可作为通用素材生成与审核，但不声称已通过某个平台发布规则。

当前只读核对结果：

| 当前代码 | 已有能力与本轮接缝 |
| --- | --- |
| `internal/app/supplychain/rules.go` / `RuleReader.ReadTargetRules` | 读取当前店铺应用绑定、末级类目、类目填写规范、商品类型属性及关联规则；图片 consumer 应复用这条授权读取路径，不增加另一套规则抓取服务。 |
| `internal/integration/shein/official_goods.go` / `QueryProductFillStandards` | 官方只读查询携带 `category_id`；`picture_config_list` 返回该类目图片要求标志。它并未提供全部尺寸、背景、文字规则，不能把未知字段当不限。 |
| `internal/marketplace/shein/goods/images.go` | 当前 Marketplace owner 验证 SPU/SKC/SKU/站点详情位置、主图/方图/附图需要与允许条件、顺序、数量及真实尺寸；尺寸/类型约束有当前发布合同代码。图片计划应复用并版本绑定这些约束，不在 ImageAgent 再抄一套常量。 |
| `internal/marketplace/imagepolicy` / `internal/app/httpapi/image_agent_policy.go` | 内部质量阈值与场景默认策略；不是官方类目发布规则，存在该 profile 不代表该平台/类目已可发布。 |

本轮需要 Marketplace owner 提供计划可消费的窄图片要求投影，并与最终发布校验共享同一约束来源。官方动态标志与当前已核对的官方文档规则分别标明依据/版本；没有已核对规则或实际 adapter 的目标显示规则不可用，不伪造全平台支持，也不扩大成通用规则管理平台。

## 3. 已实现能力与真实缺口

| 已核对当前入口 | 可复用行为 / 限制 |
| --- | --- |
| `internal/imageagent/model.go` 的 Run/Plan/Slot | 已有商品运行、计划、多图片 slot 和有界并发；无须创建第二图片运行时。 |
| `internal/imageagent/generation_execution.go` | 当前正式经济准入只接受 Main、单来源、product/zz/default/general 与 render_source_white_background；不能仅复制十六次请求来实现完整能力。 |
| `internal/app/worker/imageagent/generation_provider.go` | 当前 GRSAI 同步 adapter、exact bytes、单次 dispatch、typed 成功观察与价格/凭据版本绑定可复用；能力工厂目前只构造白底行为。 |
| #487 设计 §9.5 A–F | 固定 image owner 的跨库 immutable intent/terminal proof、资源 operation fence、成员 UTC 月限与 UNKNOWN 处理已独立准入；完整图片扩展需要覆盖实际新增边界。 |
| `internal/imageagent/validation.go` / tools executor | 内部 SlotRole 与面向用户的主图/详情图分组不是同一概念；不能把所有轮播图都标为 Main 后全局放松原不变量。 |
| `internal/agentconfig` 与其准入文档 | 企业启用、精确模板/配置版本存在，但正式模板仅支持 title-config-v1；图片参数不能冒名写入标题模板。 |
| `web/.../acquisition/acquisition-page.tsx` | exact 商品详情与 ProductAgentPanel 已有；主图 hook 尚无视觉消费者。 |
| 当前 Supply / Asset 合同 | 来源批准、完整集合与 exact inventory 消费存在；完整生成图片必须进入该 owner，不能保存第二套 Listing 私有批准事实。 |

## 4. 建议粒度与完整操作路径

```text
企业图片智能体与版本化模板
  → 商品的 exact 来源/资料与有效配置
  → 一个商品级 Plan（主图、详情图、素材、顺序、背景、语言）
  → 用户确认实际计划与预算后开始
  → 现 Temporal 对各图片 Slot 执行 / 记录 / 恢复
  → 整套结果列表与逐图原图对照
  → 用户逐图处理，显式确定完整采用集合
  → Product Asset 不可变批准 / inventory
  → 商品详情与 Listing 按 exact inventory 消费
```

多商品批量入口复用当前用户业务任务/批次消费者，每件商品保持独立运行；不新增“管理所有智能体的智能体”。单图重新生成是整套运行中的显式修改意图，必须有新的准确版本/图片身份和预算确认；它不是 UNKNOWN 重试。旧成功结果及其费用事实保留。

## 5. Owner 与装配边界

| Contract → implementation → injection → consumer | 责任 |
| --- | --- |
| Catalog/Sourcing narrow source reader → 当前 source owner → 明确消费者注入 → 图片 Start / source resolver | 来源、actor 与准确商品版本；无 latest fallback。 |
| ImageAgent Plan/Run/Slot contracts → 当前 ImageAgent/Temporal → 当前 worker 能力注册 → 图片 consumer | 商品级协调、图片级唯一 effect 和恢复。 |
| 图片参数/企业启用合同 → 当前 agentconfig owner 的有界增量 → 注册元数据与模板 consumer | 配置不提供权限，不复制执行状态，不把标题模板变图片模板。 |
| 固定 image_generation_v1 intent/proof → 当前 orgresource integration/ledger → 原 owner reader → 每个实际图片 dispatch | 原统一点数/月限、准确价格和每次副作用结算。 |
| 图片候选/完整集合批准合同 → 当前 Product Asset owner → current consumer → Listing / 商品展示 | 未明确采用不进入 ApprovedAsset，旧批准/原 action 回执保留。 |
| safe projection → 当前 BFF/Console → 页面内消费者 | 真实 loaded/denied/unavailable/partial/unknown，不伪造分数或计量。 |

公共 runtime/启动装配由会话 `01a11f74-2565-78e0-9118-4fe1ff53e726` 唯一负责。本 Writer 提供明确能力/消费者/生命周期需求；未协调前不改共享导航、启动装配、Supply 或 agentconfig 公共写入合同。

## 6. 保持的安全、计费与恢复不变量

- 所有来源、配置、运行、slot、结果和批准均绑定 verified Organization、actor、canonical member、exact Catalog/source/config 版本；每次外部派发及读取/批准重验适用 live 权限。
- 每个真实图片 effect 使用原稳定 run/planRevision/slot/attempt 身份；同键异载荷拒绝。多图不共享一个可重复消费的 provider 许可。
- 复用现固定 image owner 经济事实。生成成功仍按该次价格扣点，后处理失败或用户不采用不释放；可信未生成才释放；UNKNOWN 保留原预留并核实，不能由重试按钮换键重发。
- 每次修改/显式重新生成需要新的准确意图；任何未核实旧 effect 不会被伪装成无效果或取消成功。
- 每张图片程序校验有效才可成为候选；程序校验不保证商品保真。保留商品事实、内容安全、SSRF、文件/资产边界，不增加 Extract 或默认模型 Review，不填假评分。
- 批准动作必须绑定所采用的准确候选集合、顺序/角色和当前版本；ACK 丢失沿原 action/payload/immutable receipt 核实。未保存成功不显示正式集合已应用。
- 部分生成失败不删除成功候选和已发生费用。授权撤销阻止新生成/读图/批准，原 terminal proof 的后台结算沿原 owner 恢复。

## 7. 执行合同候选

下文是本轮技术方案，供独立评审；新增名字是拟实现合同，不冒称 main 已存在。

### 7.1 商品级计划与图片级配方

一个 Run 固定一个商品来源版本与一个目标上下文；同一商品投向不同平台/站点/类目时创建各自明确的计划，复用同一个 ImageAgent 定义。重新生成是该商品后续的显式操作，可以创建新的 Run；不是给每张图创建 Agent。页面按商品展示本次整套运行与后续修改历史，不建立第二份商品图片任务 owner。

在现 Plan/Slot 内增加 typed `product-image-set-v1` 配方与集合元数据。`Placement.Group=carousel|detail`、组内顺序及可选 canonical variant identity 表达展示位置；现 `SlotRole` 表达内容用途，二者独立。新集合合同不要求全局恰好一个 `SlotRoleMain`；所选平台需要的首图/变体图位置由 Marketplace 要求验证。详情单独生成不插入假主图。原已准入单白底合同的不变量不得因一次全局宽松修改而被绕过。

| 固定输入 | 合同 |
| --- | --- |
| 商品/来源 | verified Organization、actor、canonical member；采集/collection selector receipt、OriginalPublicationID/Version、EffectiveCatalogVersion、CatalogManifest hash。读取由现 source owner 授权，不接受用户 URL 或自报 tenant。 |
| Agent 配置 | `product.image.agent` 的 code-owned definition/version；exact `image-config-v1` template ref、配置 snapshot ID/digest、activation epoch。模板只提供参数，不提供权限/凭据/Tool allowlist。 |
| 目标 | `generic` 或实际已接入的 platform/site/store/category/productType/variant/application binding；规则 owner snapshot/ref/digest、文档约束版本、图片位置要求。generic 内部 inventory target 继续使用现 `product` 维度。 |
| 集合配置 | content-template/custom 模式、main/detail 所选任务、shared-originals/separate-originals、对应授权来源 identity、背景、语言。shared-originals 为两组解析同一原始来源集合，但生成调用和结果身份各自独立。 |
| 单图配方 | 有界 task ID、自定义 brief、所需事实引用、用途/展示位置、所选原始图片有序 identity+实际 bytes digest、prompt version、最终请求内容 digest、provider/model/protocol/输出参数。 |
| 经济确认 | 每 slot 单张生成报价与价格版本；计划真实张数/调用数、总 AI 点数上限、当前成员月限观察与有效硬预算。token 仅在可信 provider 回执存在时作为审计观察。 |

内容任务需要事实时，计划构建器沿 exact Product evidence 引用值；尺寸、材料、配件、步骤、功效缺失返回具体待补全项。自然语言指令和图片中的文字均为不可信内容，不能变成身份、路由、授权或 provider 参数。V1 不新增规划 LLM；确定性构造配方，产品保真与不可程序判断的内容由现人工审核承担。

边界沿现提交上限：每 Run 最多 32 个生成 Slot；默认并发 4、部署硬上限 10，只有实际能力开放时允许配置。自定义任务提供增删，不展示“无限执行”。单 slot 最多 8 张原始参考图，每张不超过现部署 `MaxSourceBytes`，合集 bytes 不超过 16MiB；超过则计划待调整。background ≤1024 UTF-8 bytes、brief ≤2048 bytes，模板序列化 ≤64KiB、完整计划/投影沿既有 2MiB 上限；不得截断输入或把图片 bytes 放进 Temporal history/数据库 JSON。

### 7.2 企业配置与跨库准入

现 `agent_configuration` 仍是唯一企业启用、默认模板、版本和 start-config snapshot owner，继续位于 ProductAgentDB。新增图片 typed schema discriminator；title-config-v1 与 image-config-v1 的参数、验证及数据库 CHECK 分开，不把图片参数塞进标题字段或任意 JSON map。图片不带 KnowledgeBase、可编辑 AgentDefinition、模型路由或自定义凭据。图片 catalog 与 worker 构造都读同一个 code-owned registration，readiness 来自真实依赖，无付费探测。

拟增 `PrepareImageConfiguration` / `LoadImageConfiguration` 窄 port：在 config owner 的一个事务中校验企业启用、exact template、同作用域 request key 与输入指纹，存不可变图片配置 snapshot，返回 exact ref。metadata snapshot ≤8KiB，只固定 typed 参数/计划摘要与外部引用，不复制 provider secret、图片 bytes、财务状态或 Run 状态。新 image template payload 以独立有界 typed 列存入原 template revision owner；empty-install schema 更新，不迁移旧数据或运行时 AutoMigrate。

ImageAgent 在自己的本地事务初始化 Run、Plan、授权 Catalog、配置/规则/报价引用及初始投影。配置 snapshot 创建后 ImageAgent 初始化失败可留下不可变未消费 snapshot；原 key 查询/重放得到同一 snapshot，Run ID 从原组织/actor/source operation/request key 稳定派生。不能因响应丢失生成第二 Run，不能假称两个数据库共用事务。

图片企业停用沿当前已批准配置产品语义：**停止新的整套任务准入，已经准入的有界执行可完成**。不把一次跨库 live 读取声称为可线性排序的逐 Slot 停用门禁，不新增逐图片许可平台。配置 owner 拟增 `AdmitImageRun`：锁当前企业 Agent 行并核对 ENABLED/原 epoch、snapshot scope与hard ceilings，在同一原配置事务写入一次 `ImageRunAdmissionReceipt`，**该事务成功提交是整套任务准入点**。disable 先提交则新准入失败；原准入先提交则该 Run 可在原预算/deadline 内继续。UI 在停用确认中沿现语义说明这点；新的重生成必须新的整套准入。

准入回执是原 start snapshot 的 append-only/write-once关联记录，固定 snapshot ID/digest、Organization/actor/member、稳定 RunID/confirm actionID、source/input/plan/quote digests、definition/模板版本及原 admittedAt/deadline/硬预算；原 immutable snapshot payload/digest不改写，不复制 ImageAgent 状态。ImageDB 确认CAS与Temporal启动只消费原确切回执；配置事务ACK未知时先按原action读取回执，绝不换key或刷新原预算/deadline。配置准入成功、ImageDB确认/启动失败按原回执重放；已经cancelled/completed的Run不复活，过期回执不产生新生成、点数预留或 provider POST，但允许原稳定 workflow 按原期限保存超期结果。已有工作流阻止/失败/等待审核的进度不通过确认动作复活；已有成功与 UNKNOWN 仍由原 Slot/fact owner 判断，不能根据启动 ACK 未知统一标为未派发。确认CAS同时比较当前Run/plan/action摘要，不由配置回执覆盖已改变的ImageDB状态。此处仍是两个本地事务，不冒称跨库原子。

每次实际派发仍核原 actor/member 的 live 身份、image write 与当前 source权限、原 recipe/config引用、provider凭据/价格及当前资源月限；企业配置停用本身不撤销已获得整套准入的执行，也不删除候选或代替领域权限。领域授权/来源/凭据等重验失败走原 no-generation proof/预留释放；已取得唯一dispatch CAS的工作沿原 proof/recovery收束，不授权第二 POST。结果读取与 Asset批准保持原领域 live 权限，配置停用不新增审批门禁。这里不声称与外部IAM原子提交。

标题 Agent 现有同库 Claim/disable 语义保持不变；不能用图片的跨库端口替换标题已批准的事务入口。模板更新/归档保留已固定内容和历史读取；归档阻止新准备，企业停用/epoch控制新的整套准入；已准入Run的派发继续受原领域权限与冻结预算约束。

### 7.3 准备、确认与整套预算

`PrepareImageSet` 只做授权读取、规则解析、原始图片有界读取/真实尺寸/hash、确定性配方与无外部效果报价；经原 repository 初始化 `awaiting_plan_approval` Run。每张报价固定 provider/model/protocol、输出参数、priceVersion 与点数；总点数为选定 Slot 报价求和，不预扣全额，不把美元 costMicros 当 AI 点数。当前钱包/月限仅为观察，其他工作可能消耗额度，不能承诺预览即预留。

用户看到所选任务、两组原始素材、目标位置/尺寸、实际生成张数和总点数后，`ConfirmImagePlan(runID, expectedRevision, planDigest, quoteDigest, actionID)` 明确确认。服务重验 source/目标规则及价格，先取得§7.2原配置 owner事务准入回执，再在原 ImageAgent projection/command ingress 原子记录原action、准入回执引用与确认摘要，以原稳定workflow identity启动已有Temporal；启动/ACK不确定只查询/继续原action。原 admittedAt固定整套deadline，延迟初始化/重启不延长时间或预算。修改尚未确认的配置重新准备明确的新计划/request key并取消旧待确认计划；不静默消费旧报价，不向已确认Run塞新参数。报价或输入漂移要求重新准备确认，不派发。

每次 Slot dispatch 继续使用原 `GenerationIntent` → resource reservation → dispatch CAS → terminal proof/finalization。native images=1、model calls=1、operation=1；整套 budget 绑定 N 张/N 次，无自动 repair/额外 Review 调用。重生成另做一次明确预算确认，不能藏在本次次数内。价格版本不一致或当前资源/月限不足时，不派发该项及后续未开始项；已派发项完成/核实，成功候选及费用保留，页面显示实际已扣/预留/未派发项。共享素材没有共享生成折扣。

原成员 UTC 月限在每次原资源预留时核对。已预留 effect 的恢复沿 intent 固定原月份及 limit/price，不把旧 UNKNOWN 迁入新月份；跨月未派发的新 effect 在执行时核对当月实际限额，但仍受确认总点数上限约束。超时/取消不把已派发当作未生成。

### 7.4 Provider 适配与输入指纹

复用现 GRSAI HTTPS transport、`MaxAttempts=1`、`gpt-image-2.5`、`replyType=json`、成功观察先于下载、现 staging/immutable artifact/materialization/recovery。在 `internal/product/image` 定义窄 source-edit request/capability，复用现 integration adapter 编码；支持各用途 source edit，不走旧 Extract 或默认模型 Reviewer，不按假 Main 角色绕路。

2026-10-09 只读核对 [GRSAI 官方生成接口](https://qmy27nhsd9.apifox.cn/452409160e0)：images 接受 base64/URL，支持 prompt 与 aspectRatio；gpt-image-2.5 quality 为 auto，接口未给出参考图上限或可靠 token usage。V1 发送 worker 已验证并复制的 exact source bytes；不发送可变来源 URL，不切换异步接口或擅自改模型。最多 8 张是本地边界，不冒称 provider 支持量已实测。真实 provider 能力仍 NOT_RUN。

扩展原 GenerationIntent 的版本化 `InputDigest`，绑定有序来源 identity/bytes digest、完整最终 prompt、code prompt version、配方/placement、配置 snapshot、exact 目标/规则与报价/输出参数。`SourceDigest` 为有序 source bundle hash，worker 用已复制输入形成最终 request fingerprint，再创建 intent；保留原 intent identity 与 proof reader，不新增财务账本。相同 identity/key 不同 payload 在 intent 创建、worker、resource proof reader 和恢复入口统一拒绝。密钥只用原安全 credential reference/version，不进入指纹明文。

准备时输出目标参数必须在现已配置 renderer 支持范围内。若目标要求不能由当前 renderer 合法满足，显示具体能力缺口，不伪造尺寸、不放大图片冒充原生清晰度、不默换更贵模型。生成后用真实 bytes 验类型/大小/像素/目标尺寸及可确定硬约束；校验失败不是退款证据，仍按已发生成功 effect 结算。对于无法程序确定的背景、文字、商品保真，不把技术校验写成平台最终批准。

### 7.5 运行、部分失败、取消与重生成

复用现 Run 和逐 Slot effect 状态，不建立批次账本或第二任务状态机。新增产品集合的明确动作；service、Temporal 更新校验和 Asset publisher 使用相同资格函数，不能只在按钮层保护。

| 触发/观察 | 持久结果与允许行为 |
| --- | --- |
| 准备成功 | awaiting_plan_approval；无预留、无 POST；确认/修改/取消。 |
| 确认成功 | 原确认 action 持久后 planning/executing；刷新读取原 Run，不重 Start。 |
| 有成功、其余运行中 | 保存逐 Slot immutable candidates/proof；展示已成功结果，待后续项收束。 |
| 所有 effect 已结算，存在已知失败/未派发，至少一个可用候选 | 允许选择成功结果与已有正式/原始图片保存完整集合；不强制所有 Slot 成功，不把失败改为 accepted。Run 可进入 awaiting_final_approval，保留逐图失败原因。 |
| 任一 UNKNOWN / 成功后物化或财务恢复未完成 | Run 保持 blocked 与全部 RecoverableEffects；原 effect 核实/恢复，只 GET/materialize/finalize，不 POST。不开放整个 Run 的终结批准，也不使它因保存成功图而消失。成功候选可查看。 |
| 已知失败、无可用候选 | blocked；明确新生成请求或取消，费用按各原 proof；不显示生成成功。 |
| 用户取消 | 停止后续派发；等待已派发项确定/按原核实入口保留 UNKNOWN，再沿现 cancellation 协议收束；不删成功图，不退已发生成功费用。 |
| 用户对已知终态图片重新生成 | 新 action/request key、新 Run（可只含所选任务）、原商品/source/目标与 parent candidate/slot ref；重新确认报价。原成功/失败及费用不覆盖。 |
| 用户选择未知图片重新生成 | 拒绝；只核实原请求，不能借 parent ref 或新 key 抹掉该项 UNKNOWN。 |

重生成 consumer 从原 exact Run/slot/attempt 核对 parent ref；必须是已知终态且原费用已结算（provider 已成功但图片永久不可用时也要先结算）。新 Run 的未改图片不复制成新候选，不再次调用模型；原正式库存或本次已知成功候选用原身份明确选择。选择集合可引用本商品同 source version 的已知终态 parent/new Run 候选，不能将旧对象改写成新 plan identity。页面对比原始素材、旧结果和新结果，用户决定采用哪个。

V1 不新增“所有商品自动重试”、自动质量 repair 或跨商品 Agent。原来源操作/已准入 collection 消费者可逐商品启动上述路径；没有当前消费者的批量入口显示不可用，不借 legacy Task 桥补齐。

### 7.6 完整资产选择与回执

Product Asset 继续唯一持有 ApprovedAsset、immutable approval receipt 与当前 inventory head。拟增同 owner 的 typed `SelectImageSet` 命令/服务，复用原 repository transaction 与 action-key/payload hash；ImageAgent 的 `ApproveSelectedResults` 在所有 effect 收束后消费该服务，而不是先把全部候选批准、再过滤展示。

请求只接受 actionID、expected exact inventory action/hash（初次不存在需明确）、product/source binding、exact result/selection digest 与 ordered typed choices。choice 为 human-source identity、exact current-approved receipt identity 或 image-agent run/plan/slot/attempt/candidate identity；客户端不提供 URL/尺寸/计费结果。来源 reader、当前批准回执 reader及候选 reader各沿自己的 live 权限读取，候选必须 bytes/技术校验及原经济结算已完成，DurableAsset identity/hash 与 effect精确一致。完整集合最大 40 项，非空、无重复来源选择、同图身份不得重复批准进一套集合。

生成候选使用拟增 `ImageCandidateSelectionReader`：读当前同 Organization/actor/product/source 的确切 Run projection 与已知终态 effect，返回封装事实及结果摘要；只接受当前服务已固定的 typed target/rule context。generic 素材采用到具体平台时是新的显式 target approval action，保持原生成 provenance，按该 target 的要求核对；不伪造原 Run target、不静默跨平台读旧 inventory。不同 source/effective version 沿原 source selector/Apply receipt 做明确版本绑定，缺证据不 fallback。

新集合使用独立版本化 set result/selection digest：绑定 source/target/config/规则及plan revision、ordered Slot identities与真实状态（含已知失败/未派发）、可选候选的origin attempt与durable object/hash/settlement、最终ordered choices/位置及expected inventory head。旧ResultDigestV3要求全部Accepted且一个Main的合同不全局放宽。新配方/位置/摘要字段在原normalized Run/Plan/Slot行、projection/event的同一事务保存，禁止只写投影导致重启丢失合同。

在原 ApprovedAsset payload增加presentation group/order/optional variant identity，纳入canonical serializer/selection hash；origin identity仍是原Run/plan/slot/attempt或source provenance，不把展示位置当provider身份。重选沿现SelectionReceipt与新的selection AssetID保存原provenance/批准出处，不重复插入tenant下已占用的原生成AssetID；调整顺序/角色是新的显式选择action，旧记录与回执不可变。标题Apply后沿原exact source/effective binding再选择，不能宽松跨版本消费。Marketplace官方位置（SPU/SKC/SKU/type/sort等）作为独立typed要求/映射事实纳入目标摘要；不是carousel/detail的别名。同一批准资产可被Listing引用到多个合法位置，不复制批准库存行或额外生成。

一次事务校验 expected inventory head，写本次完整选择、immutable receipt，并替换 head；并发旧 head 返回冲突，不能由最后一次旧页面保存悄悄覆盖新集合。不自动合并所有历史批准。原图、先前正式图片与本次候选的最终采用状态都在保存前显示；用户必须明确提交全套选择。未选项不入库存；仅详情生成不要求假主图，是否能发布由实际目标必需位置决定。

跨ImageAgent/Asset终结沿现approval saga：持久原action/选择摘要 → Asset原子提交/读原receipt → ImageAgent completed。同key replay先按原payload/action返回原immutable receipt，不因当前head变化拒绝原成功回放。出版开始后，失败/超时或ACK未知不得被cancel/supersede；`ReadApprovalCommit=not_found`不是未提交证明，原事务可能仍在途后提交。只通过原action/hash回执恢复并收束；允许取消须有原owner确切no-commit结果，或在任何出版调用前已确定未开始，不能靠异常类型或一次查询空值推断。沿现command/effect-owner恢复，不新增关闭平台。必须修复`failedPendingActionCanBeSuperseded`的同根因路径并验证Asset提交/ACK丢失后cancel不越过原批准。

Asset已提交但ACK/Run finalization丢失只读原immutable receipt，不能沿当前head推断原action；同key异集合拒绝。在ACK核实前UI显示保存结果待核实，不显示已应用。Run completed表示本次生成/人工选择协议收束，不代表平台已上传或所有候选都被采用。

### 7.7 UI / HTTP consumer

入口沿当前采集商品详情与已准入 Supply 图片编辑，不新增独立 AI 工作台。设置沿 Figma 1998:545 的官方内容方案/自定义、主图/详情任务、共用原始素材/分别选择、背景、语言；目标存在时显示实际平台/类目要求及冲突。准备计划与点数确认、逐图进度、原始/结果对照、勾选采用与完整集合保存是完整功能的必要消费者，应用现 Console 表单/错误/确认模式，不增加人工验收 session。

拟有 feature-local `/images` source consumer：GET 原始候选/当前库存/可用配置；POST prepare；GET exact run；POST confirm、cancel、regenerate、approve-selection；原 recover/resume 沿现 effect/action 核实入口。所有 mutation 使用 verified Effective Organization/member、image read/write 与原 source/collection 权限；BFF 传 `X-Expected-User-ID` / `X-Expected-Organization-ID`、Idempotency-Key，schema strict，拒绝未知字段、请求未读体/大体。prepare/selection 请求 ≤128KiB，现通用投影≤2MiB，30s HTTP deadline；外部 generator deadline仍原≤5min、原 finalization grace。POST 不允许浏览器任意 source URL、member/month、price、model或任意规则 JSON。

页面保留 exact source operation/request/action 与 request fingerprint，原 scope 独立隔离；切企业/商品后迟到响应丢弃。刷新查原任务/原批准 action，只有用户明确新生成才换请求键。state 来自 owner：loading/empty/denied/unavailable、等待确认、逐图执行、部分失败、UNKNOWN、待审核、保存待核实、正式集合；无假进度、假分数/假扣点。人工文件替换复用当前 source media/collection 与 Asset 选择，不增加可绕过来源批准的图片上传 owner。

### 7.8 持久化、恢复与共享装配清单

| Owner / 本地事务 | 本轮有界增量与恢复 |
| --- | --- |
| agent_configuration 原 schema | typed image template revision/metadata start snapshot及write-once关联的ImageRunAdmissionReceipt；企业行锁与snapshot准入原事务/命令幂等；准入ACK未知读原action，原payload/deadline不改。标题原CHECK/Claim不得宽松替代。 |
| ImageAgent 原 DB/Repository | typed set recipe/元数据、配置/规则/报价refs，Prepare/Confirm ingress receipt、selected approval digest；Run/Plan/catalog/projection/event本地原子事务。无新运行库/队列。 |
| GenerationFact / SlotEffect 原 owner | InputDigest/source bundle、逐图报价；仍原意图、唯一dispatch CAS与proof reader，restart/retry只核实原 effect。 |
| orgresource 原数据库 | 原 reserve/finalize/cancel operation fence，统一 AI 点数和成员月限；只消费固定 image_generation_v1 exact proof，不增加消费账本。 |
| Product Asset 原 DB/UoW | 位置payload、exact expected-head CAS、selected完整集合与原审批回执；失ACK读原action，不跟latest。 |
| Store/Marketplace/Listing | 当前授权只读规则 port → 窄图片要求投影，共享发布约束版本与plan检查；无第二规则库，无新平台写/凭据路径。 |
| 当前 app/worker composition | 注入配置 reader、source/collection reader、规则 port、现资源/Artifact/Asset依赖、typed renderer，前后端均以实际依赖投影可用性。 |

Formal write ownership：2026-10-09 用户确认本线程统一完成本批必要修改。会话01a11f6e-a012-7110-a953-3f7d43dbe6ad为唯一Writer，覆盖ImageAgent/图片domain/feature-local consumer、agentconfig typed合同、Asset选择、Marketplace图片要求、Supply消费者与必要runtime/启动接线；替代原会话01a11f74-2565-78e0-9118-4fe1ff53e726在本批范围内的分配。沿同一分支/主要PR，不覆盖其他工作，不新增部署或真实数据/付费provider权限。

### 7.9 Must / Threat Model 与验证

Must 是整套参数→实际计划/报价确认→逐图运行→逐图可检查→明确采用完整集合→现商品/Listing读取；包括来源/租户/权限、预算上限/成员月限、每 effect 单次外部请求、UNKNOWN不重发、 immutable选择及版本绑定、refresh/restart可核实。未接入平台不宣称可发布，不能以单白底fixture取代完整消费者。

Threat Model沿已有#487/#503及当前Source/Asset/Store边界：不可信浏览器/图片文字/自然语言、跨企业或actor ID/receipt伪造、权限撤销/配置与来源漂移、同key异载荷、并发与ACK丢失、provider模糊结果、不可逆计费和不安全远端文件。现文件/SSRF/凭据保护复用；不新增IAM、全局Saga、规则平台、安全扫描、验收runner或理论side-channel需求。没有新 Accepted Risk；Out of Scope沿§1/Issue，不调用付费provider，不做真实采集/平台写/部署。

验证只覆盖改变的用户行为与高风险边界：typed配置隔离/字节边界、官方规则动态标志与共享尺寸约束、source/target/recipe指纹冲突、未确认0POST、N独立生成与计量、价格/额度不足保留成功项、UNKNOWN读取恢复0POST、known部分失败可选择成功集合、未选失败不变accepted、明确重生成不重算未改图片、并发选择CAS/原actionACK核实、切scope迟到响应和完整页面路径。先写能捕获目标缺陷的相关测试再实现；复用现PG/Temporal/provider fixture/BFF/component工具，不建新验证平台。

本次架构独立检查审核实际新合同与代码接缝，最多两轮；达到IMPLEMENTATION_READY后冻结，只对新BLOCKER重开设计。最终独立检查一次核完整用户路径与实际diff/运行；TDD/CI/Reviewer不能自行签发产品验收。真实provider、真实平台和用户使用效果如实保留NOT_RUN至获准实际执行。

## 8. Legacy 与验证范围

### 8.1 实际启动阻碍：通用集合与平台装配解耦（IMPLEMENTATION_READY）

2026-10-10 在最终运行准备中发现可执行 BLOCKER：§4 允许无平台目标的通用素材生成与审核，但完整 ImageAgent 启动强制 Supply，Supply 又强制正式 SHEIN 应用。`TestFullImageGenericManifestNeedsCurrentOwnersWithoutOfficialPlatform` 在无 Supply/Store、具有当前 Product/config/resource/Image/Temporal 的 manifest 上实际 RED，失败原因为强制 Supply。受影响 Must 是通用商品整套参数→生成→人工选择→Asset 保存；阻塞层级为完整模式试用和正式交付，不涉及平台规则降低。

最小组合修正只消费当前合格 owner：

- `imageAgent.assetDatabase` 显式声明与 `imageAgent.database` 同一物理 Image/Asset owner 的独立 `supply_asset_runtime` pool，最多8连接；API、worker、Asset 角色仍分离。已有 Asset 初始化/授权/启动校验不变。
- Supply 是可选消费者。有 Supply 时其 Asset `DatabaseConfig` 必须与 image 的 Asset 配置完整一致，装配共用这一原窄 Asset pool；不开第二 owner/pool。无 Supply 时只打开 image 的该 Asset pool，不构造 Supply worker、官方应用或平台 adapter。
- Acquisition 通用来源直接复用当前有界 Product snapshot、Review AppliedPublicationLookup、作用域 publication reader、现 OrganizationExecutionAuthorizer 与原 source/actor/member 检查。配置 owner、ImageAgent、orgresource、Asset 调用及恢复协议不变；不从另一来源 fallback。
- 只有真实 Supply 已装配时才注入 Supply source 与官方 target/rule ports。无该 port 时平台来源、平台规则与批准明确拒绝；不能用 generic 冒充平台，不能削弱 Supply/Store 的原正式应用准入。
- SourceMedia 与同根手动替换依赖当前 Product/Collections、current IAM 和既有 immutable storage，不以 Supply/正式应用为前置。保持现文件、权限、SSRF、不可覆盖与真实字节读回要求。
- 不新增数据库、schema、状态、财务、准入回执、外部副作用、IAM 或通用启动/验收框架。不包装历史单图 overlay；此前无完整模式实例，使用当前明确 manifest，不新增旧配置兼容路径。

contract → implementation → injection → consumer：当前 source/Review/组织执行授权与 Asset contracts → HTTP 有界原 owner 构造 → full ImageAgent 注入；有 Supply 时注入其已批准 ports → 采集通用面板/原 Supply 面板与 worker。前后端仅按实际规则配置开放平台选择；generic 仍无发布合规声称。

验证限于本阻碍：先保留上述 RED，再验证无官方应用的 generic 配置/独立 Asset 角色与生命周期；错误角色、不同物理 owner、与其它 owner 混池拒绝；有 Supply 的单 Asset pool 与原 worker 保留；未接入平台来源/规则拒绝；当前来源授权/Asset 选择及 SourceMedia 不变量使用已有测试。其余冻结的状态/计费/UNKNOWN/恢复证据不重跑全局矩阵。2026-10-10 本增量经对应独立检查明确达到 IMPLEMENTATION_READY；仅对该启动 BLOCKER 进入正式装配修复，原冻结合同保持有效。

Legacy decision: **EXTRACT | RETIRE**。抽取当前合格计划/逐图 effect/文件校验/资源计费/回执行为到正确 owner；不 Wrap 已退休 root ListingKit、Task-first 或旧 Service，不增加双读/双写/第二事实源、旧数据迁移或 compatibility。

2026-10-09已按AGENTS完成两轮正常独立架构评审，技术设计收束为IMPLEMENTATION_READY。R1配置准入BLOCKER已由§7.2整套事务准入回执解除；R1的批准ACK/cancel、set digest/持久化、官方位置映射、Asset CAS/replay及多来源proof接缝为IMPLEMENTATION_TEST，正式实现与必要测试内收敛，不重开全局设计。正式实现使用本Delivery Batch一个Writer/分支/主要PR；复用现PG/Temporal/provider fixtures，不建设runner/验收平台。评审候选合同SHA256为417e65c9fdd0f1705d5e0a174fe63efc8647b78f36f532ffcc47035dcdb7179c；本次状态标记和R2提出的非阻塞停用文案清理不新增合同边界。

最终交付检查包括完整用户路径、准确diff/当前运行组合、权限/错误副作用/数据保存。架构独立review已完成；正式实现进度与开发检查证据记录在#612/PR#616，设计准入不代表完整用户链或产品验收。最终交付检查、付费provider、真实平台与用户验收仍**NOT_RUN**。无合并、关单、共享/生产部署或真实数据授权。
