# 店铺中心：平台商品与订单物流只读首版

Refs [#614](https://github.com/qq550723504/task-processor/issues/614)、#137。

- Product Decision: **PD-STORE-CENTER-READONLY-COMMERCE-2026-10-09**。
- Design Basis: **Independent Architecture**。
- Admission Status: **IMPLEMENTATION_READY / FROZEN**；独立准入见§12，共享路径仍须完成§9 owner协调。
- 调查基线：`main @ d95807f13475d27c0b8691f044cca6320a540d06`。
- 一个 Writer、一个主要分支：`codex/store-products-orders-v1`。

## 1. 用户结果与产品边界

硕米内部试用的获授权运营人员，从店铺中心查看 SHEIN 美国站店铺的已发布商品、实际价格/库存/站点状态，以及消费者订单、商品明细、物流和平台异常。用户于 2026-10-09 明确选择：**同步订单，查看详情、物流和异常；发货/售后到平台处理**。

Must：单店或全部当前授权店铺手动同步；按当前权限读取已保存的平台观察；列表、筛选和详情能区分未同步、正在同步、覆盖不完整、失败、未连接和不支持；刷新/进程重启后保留取得的观察与原同步进度。订单默认最近 30 天，能够按订单号、商品名称、店铺、平台状态查询该范围。统计依据完整的已保存查询集，不能从一页推算；覆盖不足时明确“不完整”，不能称平台总数或显示假 0。平台返回的数据缺项必须保留未知。

Should：操作显示每个店铺的具体同步结果、范围和观察时间；详情提供复制订单号和已验证的官方卖家平台链接。

Out of Scope：系统内发货、批量发货、拆包、导出地址、面单、售后处理、退款/资金；调价、库存写入、上下架；Agent 写操作；全托管采购订单；其他平台；webhook/定时同步；历史数据迁移；通用 ERP、同步框架、Scheduler/Reconciler 或验收平台。真实平台读取、真实业务数据、部署、合并和用户验收均须适用的单独运行授权。

Threat Model 的 Must：当前 Effective Organization、真实原成员/module grant 和 Store member grant；防跨企业/未授权店铺读取，撤权后不继续取得或展示数据；不泄漏凭据或隐私正文；客户端不能通过任意运单查别人的物流；平台不支持模式不猜测；不发出任何履约/商品 mutation。沿用现有 IAM、连接和服务准入，不扩大为新权限体系。不声明额外 Accepted Risk；现阶段不实现 webhook 或自动库存对账，平台观察自然具有取得时间且不是实时锁定快照。

## 2. Authority 与已核实差异

引用 [final UI / IA](../product/final-ui-ia-authority.md)、[greenfield baseline](../product/greenfield-no-legacy-migration.md)、[Store current application](store-center-current-application-v1.md) 和 [我的供应链 SHEIN](my-supply-chain-shein-v1.md)。2026-10-09 Figma 实查：`tg48P46SSXl6TBy9lZwg63` / `31:463`，以下均当前可见且未归档：

| 页面 | 深/浅色节点 | 消费语义 |
| --- | --- | --- |
| 店铺商品 | `429:5323` / `429:5525` | 手动同步、店铺/状态筛选、商品/价格/库存/状态/同步时间/详情 |
| 订单履约 | `429:5863` / `429:6065` | 手动同步、订单/商品/店铺/状态/最近30天、订单详情/物流/异常 |

保留原型布局、主题、表格与筛选结构。用户当前决定替代图上“去发货/批量发货/售后” mutation，改为“到 SHEIN 处理”；不伪造系统内操作。原型的示例金额、时限、数量和店铺不是真实事实。

**已核实架构问题**：现有 `MerchantHandle.QuerySPU` 是 publish purpose 的提交回读，既没有商品列表/售价/库存合同，又携带 mutation 方法。旧 `internal/listingkit/sheinsync` 使用数字 tenant/store、混合同步和旧 owner，不能包装接入；Commercial/Ecoservices fulfillment 也不是平台消费者订单。本批必须用 Marketplace 的独立只读 Port，不能借 publish 或 Supply 权限读取这些数据。

## 3. 官方合同与归一化

官方文档核对于 2026-10-09，只读浏览未使用商家凭据：

| 能力 | 官方 API / 文档 | 约束 |
| --- | --- | --- |
| 商品综合查询 | POST `/open-api/goods/searchProduct`，[3001634](https://open.sheincorp.com/zh/documents/apidoc/detail/3001634) | 三类应用；每页最多10 SPU；仅发布且审过商品；`meta.count` 为 SPU 数 |
| 消费者订单列表 | POST `/open-api/order/order-list`，[3001083](https://open.sheincorp.com/documents/apidoc/detail/3001083) | 仅自运营/半托管；每页最多30；UTC+8；单窗口最多48小时/10000条 |
| 订单详情 | POST `/open-api/order/order-detail`，[3001563](https://open.sheincorp.com/zh/documents/apidoc/detail/3001563) | 每次最多30订单；以 `salesSite` 核对US；金额/时间/包裹/异常从实际字段取得 |
| 物流轨迹 | GET `/open-api/gsp/logistics-track`，[3001784](https://open.sheincorp.com/zh/documents/apidoc/detail/3001784) | 仅自运营/半托管；当前订单号与其实际包裹号或运单号；不接受任意运单 |

商品保存 SPU→SKC→SKU 的实际层级和平台 identity；查询行是 US SKC，详情展示 SKU。站点状态来自 `skcSiteShelfStatusList{subSite,status}`，不能把 SPU/SKC 全局上架状态猜成 US 状态。没有 US 证据保留站点未知，不伪装已发布到 US。`priceList{site,currency,basePrice,specialPrice}` 为自运营售价；`costList{cost,currency}` 为半/全托管供货价，分别标注。`specialPrice=0` 表示未设促销价。仓库库存保留各 `warehouseId/inventoryNum`；空列表是未维护，不能显示0。缺币种/价格不求和、不转换汇率。标题优先实际英文项；图片只使用合格 HTTPS URL，不代理任意图片地址。

商品 `updateTime` 筛选只反映部分资料变更，不覆盖价格/库存。本批手动同步全量分页，不按该字段增量或认为列表缺项等于删除/下架。接口无法提供审核中/未通过的完整集合，审核中卡/筛选显示“平台接口未提供”，不拿 Submission/Draft 补数。商品显示 `observedAt`（取得时间），不冒充平台更新时间。卡片“全部商品”注明“本次已同步 US SKC”；SPU 总数单独保留源 metadata，不能混同。

订单保存列表的 `orderNo/orderStatus/orderCreateTime/orderUpdateTime`，再按最多30个实际列表订单读取详情；详情集合必须恰好匹配所请求订单、无重复，否则页面失败而不推进。仅 `salesSite=shein-us` 进入US结果，其他站点不保存正文；缺 site 标为该页覆盖不完整。详情 allowlist：状态/stockMode/orderType/orderTag/unProcessReason、商品 identity/标题/图片/单件goodsId、包裹/运单/carrier/packageLabel、`orderCurrency/productTotalPrice`、半托管 `saleCurrency/totalCostPrice`、实际订单/更新时间及 `needDeliveryTime/requestHandoverTime/expectedCollectTime`。供货价不能冒充订单金额；不含地址、联系方式、支付单/渠道号、支付方式、定制文本或 raw response。商品每件 `goodsId` 不同，不猜数量或合并换货成员；保留换货标识并只读显示。

订单状态保持原枚举1..9，未知的新值保留 raw code 并显示“未识别”，不能降为待发货或成功。“异常”来自平台 `unProcessReason`、问题标签、包裹丢失/滞留和报损/拒收状态，逐项解释；不是本地新售后状态机。“运输中”注明平台已发货/待揽收，轨迹未提供时不猜物流状态。发货时限使用当前 `needDeliveryTime`（废弃的 `requestDeliveryTime` 不使用）；缺失则不可用，不从订单时间加48小时。今日数量按UTC+8订单下单时间，在已同步30天范围内统计并注明覆盖。

## 4. owner、Port 与注入路径

| 事实/能力 | 唯一 owner / 实现 |
| --- | --- |
| 平台商品、消费者订单、物流状态 | SHEIN；本地只记录有时间的观察，不进行业务状态转换 |
| Store身份、成员grant、连接、凭据、服务权益 | `internal/storecenter` 及当前 Store App registry |
| 平台观察、固定同步范围/游标/结果 | 新 `internal/marketplace/shein/observations`，纯Go模型/用例/本地Port |
| SQL事务/查询 | `internal/integration/persistence/sheinobservations`，注入显式 pool，不自开数据库 |
| 签名、HTTP、平台DTO解析 | 现有 `internal/integration/shein/OfficialClient`，新增独立只读adapter，复用有界transport |
| live IAM/module授权、server-only句柄、组合 | 新 `internal/app/storeobservations`；沿既有 current member owner；不承接平台规则 |
| HTTP/UI | 模块本地HTTP descriptor、Console/BFF，复用现有 verified identity/同源/错误合同 |
| 后台逐页执行 | 现有Temporal SDK，模块本地有界workflow/activity；不拥有平台或观察事实 |

调用链：Console → 同源BFF → CurrentIdentity/effective Organization/live module permission → observations usecase → Store server-only read handle → 官方 readonly adapter → normalized allowlist → observation repository共享事务 → live再次授权后 Console projection。Domain 不 import Gin/GORM/provider SDK/App；App 仅授权与组合，归一化/覆盖和查询规则归 Marketplace owner。

新增独立 `ObservationSubject/Authorizer/Reader`（或者抽取既有material的内部共同读取，但不包装旧Service）。purpose 固定 `store_products_read` / `store_orders_read`，不扩展原 supply publish/image/rules 的权限。只读句柄仅暴露 Products/Orders/OrderDetails/Track；不含 Publish/Transform。复用 Store 内部合法 material 读取不变量和 registry保护；provider采用独立 readonly interface，不能以现有 mutable interface assertion 作为准入。

每个短活动取得最长10秒、受request/activity deadline约束的非序列化handle。每次平台调用重新检查当前成员/module、Store member grant、active记录、active且未过期服务、verified connected attempt、应用模式/版本及私密凭据hash。校验不成功拒绝；已有 Supply 目的在其原 authorizer 下保持原权限。凭据只在当前调用前解密，不进入工作流、DB观察、API、日志或错误。

permission沿当前authz点分命名，最小四项 `workbench.store.products.read/sync`、`workbench.store.orders.read/sync`；sync同时须read和Store read，普通read也须Store read及当前store grant。两个module独立可授予；订单不依赖Supply/Agent权限。原请求绑定 verified org/actor/member；worker从原持久命令取同scope，用当前 exact IAM重新查原成员，不制造JWT或延长request proof。平台管理员必须具有当前企业授权，既有tenant admin仅沿Store owner允许的admin规则。

## 5. 最小持久化与事务

复用当前 Store 的专用数据库，新增明确的 observation schema（`shein_observations`），不把观察写入Product/Catalog或复制Store身份/凭据表。观察repo只访问本schema，Store仍由自身repository访问。schema-owner初始化和serving role/readiness最小清单同步；constructor不AutoMigrate，不以owner连接serve。当前Store verifier有严格表/权限准入，接线owner须显式调整enabled capability合同，缺schema/grant fail closed。

同一owner下四类最小表，父命令回执显式存储，不能放在进程内存或以假的StoreID代替：

1. `syncs`：Organization/Store/SyncID、原Actor/Member、kind、幂等command key/hash、固定source binding、固定时间窗、status、checkpoint revision/page/window、取得/完成时间、安全错误码、coverage note。唯一(org,actor,key)，同key不同store/kind/range/hash冲突；same key永远查询原同步，不因重试创建新身份。
2. `records`：Organization/Store/SyncID/kind/platformID、allowlisted typed projection、observedAt；复合唯一/FK均带org+store+sync。平台商品按SPU保存内部完整层级，订单按orderNo；记录来源binding由sync固定，不保存credential、raw json或敏感字段。
3. `heads`：Organization/Store/kind→最近完整遍历generation、revision；固定外键不跨scope。失败/部分结果保留原head；当前sync部分结果独立可查看且显式不完整，不能覆写完整head。
4. `commands`：Organization/原Actor/Member/用户key、payload hash、kind、固定时间窗、实际StoreID→child SyncID集合；唯一(org,actor,key)。begin在同事务保存父回执及全部child syncs，single-store同样走该回执，避免切店或同key重试重选集合。父行不伪造StoreID，也不成为新的任务/平台事实源。

begin命令和同步事实在一事务提交，再以稳定 `store-observations/<org>/<syncId>` workflow ID调用已有Temporal Start/Describe。启动响应丢失只核实同ID，DB事实pending，不称running。显式重试/读取原sync通过 `EnsureExecution` 恢复同ID，terminal只读；不另建scheduler/outbox平台。原scope key碰撞返回409；数据库COMMIT不确定查询同key回执。

逐页先读取当前checkpoint，重新授权到原binding，再只读provider；验证响应、allowlist/US归一化；返回后重新核对原binding/live权限。在单DB事务锁sync行、比较checkpoint revision，将本页记录与下个checkpoint原子保存；CAS失败丢弃旧页并读当前进度。retry不按旧游标覆盖新数据。每页输入与安全摘要有稳定hash；同页重复成员/内容冲突标coverage不完整，不伪造成功。每个observedAt是实际响应取得时间。

完成发布在同一DB事务锁sync与head，将末页/checkpoint、terminal状态和head原子提交。begin事务在(org,store,kind)内分配单调generation序号；候选binding必须重新匹配当前live Store，同binding的head只提升较新generation，较旧sync晚到可保留自己的完整结果但不倒退head。旧head binding失效时，允许当前新binding的完整generation替换；旧binding的晚到sync不得提升head。partial/failed/suspended绝不提升head，任何页的coverage不完整标记sticky持久保存，后续成功页不能清除。

没有跨Store/IAM/观察库原子假设：短proof不是永久访问权；即使撤权前页已提交，也不会成为后续读权限。每次读head/partial/详情/同步状态都重新授权当前请求，并核对org/store和source binding。supplier/application/connection变化不读取旧source观察，不自动迁移到新商家；重新同步后形成新head。记录名称变更可造成binding过期，安全拒绝并提示重新同步，不在本批发明宽松binding兼容。

## 6. 同步、覆盖、失败和恢复

状态只管理本地取得过程：`pending → running → completed | partial | failed | suspended`，不管理订单履约。terminal不可自动再发；新“同步”使用新用户操作key；重试原失败操作只读原结果，引导明确重新同步。取消未开始同步可terminal，已开始只停止未来页；已取得结果仍保留且不称完整。

商品从page1以10 SPU/页全量扫描。记下首个meta.count，结束检查页序、计数、重复identity及metadata漂移；到达边界才标completed，否则partial。官方无原子快照保证，即使遍历完成也仅表示在取得时间跨度内完成遍历，不能据列表消失推断删除/下架；与前一head不做删除同步、库存对账或平台回写。统计完全由同一generation中取得的US SKC集合计算，显示遍历范围/时间和源SPU count。

订单固定结束时间、起点最近30天或用户明确范围，拆为不超过48小时的UTC+8 created/down-issued查询窗口，每窗口30/page；固定`queryType=1/queryOrderType=4`且不传orderStatus，避免默认漏认证仓。边界采用相邻窗口共享秒并按orderNo去重，不能跳过边界记录。每页详情校验后保存US记录。created范围不等于updated范围，不把订单更新时间作为筛选下单时间。每个窗口计数上限10000；可按时间二分缩窄窗口直到1秒。若同1秒仍达上限/无法证实完整，terminal partial显示具体区间与原因，不能跨过继续宣布全量。保存dedupe不会把未得到的订单判取消。

每个activity最多一个平台列表页和一批对应详情，deadline≤30秒，外部每call≤5秒，响应≤2MiB/页，最多10 SPU或30订单；嵌套成员、字符串、字段和DB单记录也有硬上限。workflow串行分页，不在全部店铺并行轰炸provider；用Temporal retry/backoff处理临时读取错误，无平台mutation UNKNOWN/resend语义。有限重试耗尽保存failed，授权明确撤销/connection漂移保存suspended，不把IAM outage当永久撤权。

一个sync最多5000商品页/30000订单页、最长24小时，达到限额为partial并显示范围；这是明确的资源保护，不能自动扩大权限/预建海量容量平台。长workflow在小批页数后ContinueAsNew，只传org/syncId，全部进度仍在SQL。暂停/崩溃/请求超时/response lost用原identity+checkpoint继续只读，没有生产内存runner或新恢复平台。

“全部店铺”只从当前成员Store目录分页取得真实授权店铺，再为每店铺相同用户操作创建固定子同步；child key稳定派生自父用户key+StoreID+kind，载荷固定选定集合与时间范围。父命令回执在begin事务固定该集合；同key重试查原集合，不重新选店或替换MemberID。逐店结果独立，不用一个success掩盖unsupported/failed。读取跨店列表先求当前授权店铺集合，再按该集合读取各head；某店被撤权立即从结果/统计排除。未同步或部分店铺让聚合coverage为incomplete，已知部分可以显示但不叫全库总量。不支持全托管订单在该店明确unsupported，其余店不受阻。

## 7. 详情、物流和平台处理

商品详情只从当前授权source generation、scope-qualifiedSPU读取；US site、SKU、售价/供货价和逐仓库存不丢维度。订单详情从当前授权同步记录的orderNo重新进行只读详情查询，校验US/site和订单identity后展示最新观察；没有该scope内记录不能用任意orderNo穿透查询。详情失败保留上次取得时间并明确陈旧，不能冒充新值。

物流请求只接受授权orderNo和其当前详情中的packageNo选择；服务端重新取当前详情并核对package成员，使用packageNo（不要求waybill已生成）或其中实际waybill调用Track。客户端传入waybill/returnOrderNo/任意URL拒绝。response缺轨迹显示“平台尚未提供”，不显示已送达。Track只allowlist carrier/waybill与description/nodeCode/timeMillis；不保存用户地址或物流请求raw body。返回后再次授权，binding漂移丢弃结果。

“到SHEIN处理”使用官方文档已确认的 `https://sellerhub.shein.com/`，新窗口noopener，显示订单号供复制；未经验证不构造特定订单深链、携带credential或自动登录/跳转执行。此按钮不能签发已发货/售后成功。

## 8. HTTP、BFF与Console

模块路由在 `/api/v1/workbench/store-observations` 下，包含capabilities、同步begin/by-key/status/ensure、goods/orders分页与scope-qualified详情、物流只读查询。每条descriptor消费CurrentIdentity+live permission；请求org/actor/member来自bind，不取客户端body，StoreID必须canonical UUID。读拒绝未消费body；同步有界严格JSON/idempotencykey/同源保护。每条query有显式白名单、长度、唯一参数；cursor绑定org/store/kind/filter/generation并在服务端验证，不能作为权限。page≤50，响应≤2MiB，外部/DB/request均deadline。有租户/店铺deny统一404或既有permission拒绝合同，不泄漏外店是否存在。

新增独立frontend contract/BFF，复用当前access token、Expected Organization/User headers、CSRF/同源及严格upstream JSON/schema/bounds机制；避免继续扩展大型workbench-proxy中的平台分支。固定upstreamorigin/path，任何body credential/URL拒绝；所有回应no-store，凭据错误统一安全码。有效企业key参与react-query，企业切换取消旧request并清理原列表/详情；晚到response还须scope一致才消费。

Console正常入口 `/workbench/store-products`、`/workbench/store-orders`，复用WorkspaceAppShell、ConsolePage、主题与当前Store选择器。图上未支持的审核中/售后mutation明确标注；未知数显示“—/未提供”，真实空集合且完整才0。支持desktop/narrow，详情有关闭/键盘focus和明确错误重试；同步按钮只在当前sync权限与运行capability允许时启用。不以authz菜单available或API存在代替真实runtime readiness。

## 9. 共享Writer与接线交付

2026-10-09 用户明确指示本线程继续推进，以下公共接线改由本会话`01a11f6e-956c-73f1-9f8a-f025734c58d1`统一写入，在原主要PR #615内完成；替代原启动线程分工。只调整Writer归属，不改变冻结的产品/领域/权限/持久化设计，也不写其他线程worktree。

| 共享边界 | 所需最小变更 / owner |
| --- | --- |
| Store private material与registry | 独立只读Port，共同material不变量/保护；不改变Supply purpose；待协调Store合同owner |
| 官方transport allowlist | 为新增只读endpoint放行POST/GET精确方法；待协调integration owner，不修改Publish实现 |
| authz module与permission默认策略 | 两module及四permission；本Writer沿当前角色合同接线 |
| Console导航 | 两个正常入口由pending改为connected；本Writer接线 |
| Store显式schema/readiness与runtime/Temporal | 新观察schema/最小privilege/module注入/workflow注册；本Writer统一接线；原Store、Supply、resource构造的Store preflight均须消费一致enabled清单，固定public search_path；新repo使用schema-qualified表；平台Temporal client不以无关Supply业务启用为前置 |

运行组合依赖Storepool、member directory/authorizer、三类OfficialApplicationRegistry、observerrepo与Temporalclient。schema只用于已授权新空实例；缺观察表/配置/worker时capability unavailable，不回退legacy或用fixture作生产值。后续真实试用保留volume，stop/restart与destroy分离。此文档不授予真实schema执行、读取商家数据或部署。

## 10. Legacy决定

```text
Legacy decision: RETIRE
Reusable behavior: 官方签名、bounded transport与当前Store授权来自合格current owner；旧sheinsync不接入。
Current owner: Marketplace SHEIN observations / current Store / integration official adapter。
Cutover/deletion condition: 本首版不消费旧同步、数字Tenant/Store、ListingKit facade或旧Product task；不做历史迁移或兼容。
```

已有 `internal/listing/README.md` 的task/workspace迁移说法与当前greenfield/hard-cut决定不符，不能作为准入依据；不因此在本批插入无关文档治理。

## 11. 验证与交接

业务实现按TDD先捕获实际不变量：未知库存/价格与站点不推断、同key载荷冲突、窗口/分页去重与溢出覆盖、错误/重复详情不推进、checkpoint旧响应CAS、head保留、跨org/store/source读拒绝、撤权/原worker成员、物流package成员。复用当前fixture transport、真实隔离Postgres和Temporal testsuite，不新建runner/故障平台。验证商品/订单/物流allowlist与实际官方request fields，大小/deadline/redirect/credential错误不外泄。

必要开发自检：受影响Go包/边界guard、BFF/contract/UI focused tests、typecheck，以及最终候选必需CI。高风险独立Architecture Review在正式编码前；完整用户路径形成后集中独立差异检查，finding按AGENTS分类，修复仅复核对应增量。不把fixture/本地测试/CI当真实平台或用户验收。

交接应含主要PR、实际入口/启动命令、当前enterprise/Store权限步骤、手动同步与详情/物流操作、观察保存在Store专用DB的新schema及stop/restart保存方式、provider应用模式限制和NOT_RUN。运行接线、真实平台读取和内部试用由已指定owner与用户适用授权完成；没有运行实例不声称现在已可用。

## 12. Architecture Review

2026-10-09 第1轮只读独立Reviewer `/root/store_observations_architecture_review` 核对设计候选`826e6ebba`、实际Store material/registry/current_schema、授权及官方文档，结论 **IMPLEMENTATION_READY**，无BLOCKER。§5/6/7/9补入其最小澄清；不是新增全局设计或第三轮评审。

| Finding | 当前Must/后果 | Classification / Action |
| --- | --- | --- |
| 完成与head并发发布 | 重启恢复、旧页不得覆写新结果 | IMPLEMENTATION_TEST：锁sync/head，terminal/checkpoint/head原子提交；单调序号与CAS；partial标记sticky |
| 全部店铺子key | 多店同步与同key重试 | IMPLEMENTATION_TEST：父回执固定集合；child key确定派生；不重选store/member |
| 官方列表默认仓库范围与物流package | 完整消费者订单/合法未生成运单物流 | IMPLEMENTATION_TEST：queryType1/queryOrderType4/no status；窗口覆盖验证；packageNo即可，不强制waybill |
| 新schema与已有Store verifier | 运行组合不能被新增权限整体拒绝 | IMPLEMENTATION_TEST：所有Store/Supply/resource preflight一致enabled最小清单，保留未启用时严格拒绝 |

原成员撤权、只读句柄/source binding、私密allowlist和物流成员边界准入成立。实现验证撤销sync权限后worker停止、相同Actor新MemberID不能接管旧命令。2026-10-09 reviewer对`404e0e831`仅复核上述澄清增量，维持IMPLEMENTATION_READY/FROZEN；父commands回执与child同事务成立，§5另明确新当前binding能够替换失效旧head且旧binding晚到不能提升。评审不替代共享owner协调，也不替代真实平台/运行组合/用户验收；这些均NOT_RUN。
