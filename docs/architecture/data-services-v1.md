# 数据市场与 API 管理首版

Refs [执行 Issue #621](https://github.com/qq550723504/task-processor/issues/621)，Parent #137。

- Product Decision：`PD-DATA-SERVICES-2026-10-09`，见 §1。
- Design Basis：**Independent Architecture**。
- Admission：**IMPLEMENTATION_READY**。本文先于生产实现；R1 于设计候选 `6921e70816dadb4f0f552cbd138e60fd94aeaec0` 完成，无 BLOCKER。共享窄合同 Writer 已由用户确认。
- Investigation baseline：`origin/main @ 2e40643f63a4314a33b0f59f21def10a0cd2ecc9`。
- 唯一 Writer：会话 `01a11f6e-8e8c-7783-9131-fd88e18394c4`，`codex/data-services-v1`。
- 公共 runtime / 启动组合 Writer：会话 `01a11f74-2565-78e0-9118-4fe1ff53e726` / #619；共享 owner 改动按 §13 协调。

## 1. 产品决定与用户结果

使用者是当前硕米内部试用成员及其获授权的客户系统。用户于 2026-10-09 明确决定：此前占位菜单进入设计和开发；本批仅数据市场、API 管理，“我的数据”复用 #605 / PR #606 的 ProductCollections。

用户在本会话明确选择：

1. Amazon 实时抓取本批支持**多站点、关键词、类目和 ASIN 完整输入**，不能缩减为仅美国站链接。
2. 实时抓取及 API **成功保存一条消费一个 DATA_ROW，5 分/条**，复用现有预付资源；不采用 Figma 示例的 2 分/条，不直接逐次扣钱包。
3. 数据集定制由平台专员**线下确认规格与报价**，系统记录进度并交付到“我的数据”。本批不建立在线定制收付款、退款、自动报价或定期调度。
4. API 密钥绑定**创建者和企业**；结果进入创建者的私有“我的数据”；撤权/离职后失效。不新增企业共享数据或服务身份。

用户可完成三条路径：

- 数据市场选 Amazon 站点和查询条件 → 确认最大数据条数和预付资源消耗 → 启动 → 查看逐项结果与实际保存的批次。
- 提交数据集定制需求 → 专员记录规格/报价确认和制作进度 → 上传明确交付数据 → 创建者查看同一原始数据批次、下载和交付记录。
- 创建限定 Amazon 数据能力的密钥 → 客户系统以同一用例创建抓取任务并读回结果 → 查看真实调用记录、额度及费用 → 禁用/撤销密钥。

Must：完整输入、真实持久化与权限/租户/创建者隔离、密钥保护、实时撤权、幂等、按成功保存计量、并发限额正确、UNKNOWN 保留和有界恢复。Should：沿 Figma 的信息层次与主题，给出清楚的剩余条件和字段缺失原因。

Out of Scope：其他数据平台、跨企业数据售卖/自动上架、企业共享、任意外部 URL、通用网关/插件/调度/验收平台、模型/付费 provider、Amazon 登录账号或 CAPTCHA 绕过、历史迁移/兼容、正式环境数据和部署、合并/关单。没有新增 Accepted Risk；真实 Amazon、保留运行实例和用户验收仍分别取证，不以 fixture 或架构评审代替。

## 2. 当前 Figma / Product Authority

依据 [final UI / IA](../product/final-ui-ia-authority.md)、[greenfield baseline](../product/greenfield-no-legacy-migration.md) 与 §1 本轮决定。2026-10-09 已只读查看 `tg48P46SSXl6TBy9lZwg63` / 页面 `31:463` 当前可见、非归档节点及两张内容截图：

| 投影 | 节点 | 本批消费 |
| --- | --- | --- |
| 数据服务概览 | `404:1451` / `411:1699` | 已有 Shell 归属，不另建顶层域入口 |
| 数据市场 | `430:323` / `430:525`，内容 `430:611` | 来源选择、实时抓取/数据集定制、交付到我的数据 |
| API 管理 | `430:837` / `430:1039`，内容 `430:1125` | 概览/密钥/调用记录/用量与费用、开发文档与密钥操作 |
| 我的数据 | `430:1865` / `430:2067` | 复用当前 Collection，保持原 actor-private 权限 |

Figma 未提供抓取配置、API 创建/编辑密钥或数据集定制的专属详细弹窗。工程补充使用已有表单、弹窗、列表/详情模板，不宣称已有对应终稿。平台/国家、价格和能力来自后端已配置准入；图中“已开通”“50,000”“¥1,000”“2 个密钥”不是初始事实或默认配额。未配置时显示真实不可用，空安装是零条真实记录。完成时间没有可靠估计时显示未提供估时，实际进度与耗时来自保存记录。

## 3. 当前代码、可复用能力与根因

| 能力 | 证据 | 处理 |
| --- | --- | --- |
| 原始资料/Catalog/Collection | `internal/product/sourcing/publication.go`、`catalog/repository.go`、`collection/*`；对应 persistence | 保持唯一 owner、精确不可变版本及 UoW |
| Source 发布事务 | `sourcingpersistence.NewTransactionWriter`、CatalogBridge | 应用注入当前 Product 同事务窄 writer，不能重新实现版本算法 |
| Amazon 解析、neutral mapping | `internal/crawler/amazon/extractor/*`、`result_validator.go`；`internal/integration/crawler/amazon/product_source.go` | 选择性抽取合法行为和回归到当前 acquisition adapter，不依赖旧 Source/Task/Service |
| 当前浏览器采集 | `internal/integration/acquisition/a1688/browser`，仓库 Playwright Go | 复用成熟 Playwright 与已存在的 cancellation/资源上限经验；Amazon 专属固定站点/parser，不复活原浏览器池控制面 |
| 当前身份/企业/角色 | WorkbenchContext、Casbin/native RoleModules；`ReadExactServiceProjectAuthorization` | Console 使用现有 CachedRead / LiveWrite；API/worker读取当前准确服务授权 |
| DATA_ROW 计量 | `orgresource.ConsumerChargePort` / owner proof / RecoverDue | 新 Amazon 消费者只登记窄 owner，复用 reserve/settle/recovery，不复制余额/账本 |
| 持久执行 | 当前 Temporal SDK/worker | 一个本流程 workflow；数据库为业务事实 owner，Temporal 不复制商品或费用 |

实际架构问题：

1. `integration/crawler/amazon/processor.go` 的 `LegacyCrawlSource` / `NewLegacySource` 包装旧 `internal/crawler/amazon`；`AmazonDefaultDomainResolver` 未知站点回退美国站。新请求不能依赖这个 wrapper/fallback。新 adapter采用显式站点表和 neutral DTO，抽取纯解析及有效校验行为，不使用数字 Tenant/Store 或旧 `SourceRequest` 作为授权。
2. 现有 acquisition 是明确的 1688 `Canonical1688Source` / OfferID / fingerprint / channel 合同。不能将 Amazon ASIN 塞入 OfferID、伪造 1688 operation 或改旧 channel 重放分类。本批 Amazon job/item 是新的有界 acquisition 事实，原 1688 状态、价格和 free channel 完全保留。
3. Collection 的 `AppendPublished` 名称、schema kind 与前端来源枚举当前写死 1688/acquisition/own。必须由 Collection owner 增加明确的 Amazon/定制批次引用入口与读取枚举/标签；当前精确 Catalog 版本读取已不依赖来源kind，直接复用，不另建source resolver。不在数据服务直接 INSERT Collection 表，不能假冒“自有商品”或伪造旧 acquisition receipt。此窄变化由 §13 明确所有权后才写。

## 4. Owner 与调用路径

| 事实/职责 | 唯一 owner | 最小新增 |
| --- | --- | --- |
| 用户/成员/企业角色与停用 | ZITADEL / 当前 Organization / Casbin | 不新增身份、角色或本地 membership 表 |
| API 凭据及限制 | 拟新增 `internal/dataservice` credential 子能力 | 数据能力专用凭据元数据、digest、版本、启停/撤销、限额；不是全站登录令牌 |
| 查询/发现/item执行与terminal proof | 拟新增 `internal/product/dataacquisition` | 固定输入、逐项状态/证据、来源 refs；不保存第二份 Catalog 正文 |
| 原始证据与 Product 版本 | 当前 SRC / Catalog | 登记 `amazon_data_v1` / `custom_dataset_v1` 明确 producer，沿 UoW 保存 |
| 商品批次与原始引用 | 当前 Product Collection | 新来源类型及有界批次写 Port，保持 org+actor+member |
| DATA_ROW 资源与扣费恢复 | 当前 Organization Resource | 拟登记 `amazon_data_v1` consumer，1 条/成功 item 的 owner intent/proof |
| 需求/规格/线下确认与交付记录 | 拟新增 `internal/dataservice` customization 子能力 | 原申请人 scope、阶段/不可变事件、一次交付引用；不拥有数据正文或付款事实 |
| 官方匿名抓取/发现与解析 | `internal/integration/acquisition/amazon` | fixed-site `Discover` / `Fetch`，仅输出不可信中立 evidence |
| 装配 | 新有界 `internal/app/dataservices`，公共 caller 由 #619 | 注入 ports/pools/auth/workflow；不在 app 积累业务状态转换 |

链路：Console/session BFF → 当前 verified effective Organization/Casbin → DataService → Product DataAcquisition → current SRC/Catalog/Collection UoW；Resource 窄 port 跨其独立 DB；Temporal 编排原 owner 命令。外部 API 只走 `DataAPIPrincipal` → 同一 DataAcquisition 用例，不注入伪造的全站 `AuthenticatedIdentity`。

DataService 的凭据、需求、job/item 与 quota reservations 放在当前 Product 专用数据库的明确 data-service tables。使用独立受限 pool；同物理 Product DB 才能用一个事务协调 item、quota、SRC/Catalog 和 Collection，不复制其他 owner 表。Resource DB 保持独立，使用既有 owner proof/恢复协议，不假造跨库事务。

## 5. 输入、站点、发现与成功定义

首版站点表消费现有有效 region/domain 行为：us、uk、de、fr、it、es、ca、jp、au、mx、br、in、ae、sa；只有实际启用的 provider/站点进入可选列表。未知站点、与 URL 不一致的站点、空站点全部拒绝，绝不默认美国。

请求为强类型 `Query{site, mode, keyword?, categoryNode?, asins?, limit, fields}`。mode 明确为 ASIN / keyword / category；关键词允许与类目过滤组合。类目为站点原生数字 browse node，由固定说明输入，不声称有完整分类目录。ASIN 为去重的 10 位大写字母数字；允许固定 Amazon HTTPS 商品链接经 canonicalization 得到 ASIN，不接受 URL 作为 provider 目的地。keyword最多200字符、node最多20位、ASIN最多200个、requested limit为1–200、fields最多32个已批准字段。拒绝 unknown字段/重复JSON keys/超大数字。

Search URL仅由adapter按固定site与URL编码构造，限定 `/s` 与受控page/node/query；product路径固定 `/dp/{asin}`。最多10个发现页，最多200个去重ASIN，达到请求数量停止；不从广告、recommended、第三方URL或详情变体继续无限遍历。空结果可成功结束但不消费任何DATA_ROW。原查询及发现集合在任何商品Fetch前持久化，重启不改用新搜索结果补齐旧请求。

公开匿名只读抓取；全新非持久浏览器context；禁止客户cookie、登录态、source account、密钥或内部请求头进入页面。挑战/不支持结构有明确原因，停止相应请求；不自动换代理或绕过挑战。站点域与必要静态资源使用显式白名单，导航/重定向每跳重验；请求层禁止私网/loopback/link-local/metadata和任意origin，禁用serviceworker、下载、WebSocket，TLS正常验证。运行owner必须配置网络出口和浏览器路径，缺配置fail closed。

Reuse现有标题/ASIN/主图/可售价格与破损变体校验；抽取后修正未知availability不能当false/缺价格可成功的问题。成功商品至少有匹配site/ASIN、非空title、合法主图和捕获时间；明确available时须有实际currency/price，明确unavailable可保留真实缺价，未知availability记录缺失且不能据此伪造“暂无报价”成功。所选可选字段缺失保留missing/warnings，不生成值。原数据保存完整已取得allowlist；请求fields只限制交付投影。成功是一份商品结果，变体数量不额外扣条数；同job中同site/ASIN只计一条，用户以新command明确再次抓取可产生新版本。

搜索网页HTML不作为业务正文保留；保存有界allowlist evidence、来源、捕获时间、parser version和内容hash。商品evidence不超过既有2MiB envelope/snapshot限制。

## 6. API 密钥、实时授权与生命周期

Console 创建/编辑/禁用/撤销密钥需当前 LiveWrite、本人data-api.manage及相应 data-market.use / collection.manage 模块权限。密钥只能绑定verified `{organization, actor, canonical member/authorization ID}`，客户端不能指定其他创建者、企业或权限。管理员不因角色而读取他人密钥secret、job或商品。密钥权限从本人当前权限取交集，只允许 `amazon.acquire` / `amazon.result.read`；不能用于登录、企业切换、Store/Agent/支付或第三方任意调用。

随机secret使用crypto/rand 32字节；public key ID与secret分离；数据库只存digest、masked suffix与元数据。验证constant-time，无密钥写入log、URL/query、localStorage、审计或操作receipt。只有首次成功创建HTTP响应有完整secret；后续list/read/replay不返回。创建COMMIT/响应未知先用原command key查meta，若secret未收到，UI明确提示撤销后重新创建，不能保存明文以方便重放。名称最多80字符，每actor最多20个有效密钥；过期时间显式输入、最长365天，UI不自动创建无限期secret。

状态 `ACTIVE → DISABLED → ACTIVE` 与 `ACTIVE/DISABLED → REVOKED`，revoked终态。更新受expected revision与原command key/hash约束；轮换为创建新key、用户显式撤销旧key，不偷偷重置原 key ID。digest不可重新绑定scope。

API 使用专属 `Authorization: DataKey <public-id>.<secret>`，HTTPS、no-store；不接受Cookie、query token或客户端org/actor header。先校验key状态/expiry/可选IP CIDR，再通过已有 `ReadExactServiceProjectAuthorization` 准确读取当前subject+project+org、member ID、active grant和native RoleModules。另用ZITADEL官方GetUserByID确认主体active；企业deny-only suspension overlay沿原owner。服务凭据仅server-side，不储存/refresh用户bearer token。依赖故障fail closed，grant重新创建为新member ID也不能使旧key复活。

所有API读取、新job、每个Fetch前及发布前均实时核对原grant和当前permission。worker冻结原actor/member，绝不使用专员或runtime service user充当商品owner。key的禁用/撤销在ProductDB锁定相同key row，与job admission和publication guard排序；撤销完成后不能创建新item/publication，已保存结果只由正常Console本人读取，原key无权读回。

IP白名单可选最多20条CIDR；只取socket peer或安装配置的trusted proxy链，不信任任意X-Forwarded-For。绑定的合法permission交集也参与请求hash，范围不能在重放时扩大。

## 7. Job、item、事务与 quota

最小事实：key/credential command receipts；job与固定query/hash/scope/funding/最大费用/price version/创建时间；immutable discovered item set；item state/evidence/Publication/Collection/reservation refs；每日/月度quota bucket与per-jobreservation；custom request/spec/events/delivery refs。scope-qualified PK/FK、command key唯一、payload hash、revision CAS与item fence必须在schema成立。

API限制由用户创建密钥时设置正数每日成功条数、月度费用上限；不采用Figma50,000/1,000为默认。UTC日/月窗口明确显示，费用按5分minor unit计算，整数overflow拒绝。Console创建job也必须明确本次条数与最大5分×条数费用并确认；API请求携带maximumRows及maximumCostFen，价格/上限不符拒绝。

job admission在单一Product事务锁key→quota day/month→job，原key/hash先读回；确认active key/version后预留requested maximumRows及最大minor-unit费用，使并发满足 `consumed + reserved + new ≤ limit`。窗口固定为job admission UTC日/月，跨日重启仍结算原bucket；修改限额不得把旧reservation搬到新窗口、不得抹掉消耗。减限额低于既有consumed+reserved时拒绝并展示当前约束。quota是限制事实，不是第二余额或扣款账本；只从实际Product terminal/Resource receipt投影消费。

发现集合写入同事务后，为每item派生稳定operation ID/command hash；未使用的requested quota释放。每商品在Fetch前将1 DATA_ROW intent（原scope/member/funding/rowID/queryhash/source/5分价格）持久化，调用Resource reserve，再在Product记录准确reservation；未确认reservation不Fetch。admin使用enterprise-unallocated，成员使用member-allocated，消费既有规则；funding选择冻结，权限降低不能重选资金来源。

所有写命令采用统一锁顺序 key→quota（day/month）→job→item，取消、撤销、改限额及发布不能逆序。成功item的一个Product UoW：按该顺序锁定fence与限制→重新核对scope/未取消→current SRC publication、Catalog bridge、Collection append同事务→exact publication receipt→item `SAVED` 与可验证成功proof。任一步失败回滚；不得先签发success再append Collection。此UoW是唯一成功来源，Collection只保存refs。提交未知不重新Fetch/重新发布另一key，核实原item/publication/collection receipt。

Resource `Reconcile`消费immutable terminal proof，将原reservation committed/released。已保存但结算未知展示 `SAVED / CHARGE_PENDING`，不称余额扣减完成；UI/API实际费用只计已commit owner receipts，pending金额单列且继续占quota。成功quota转consumed与终结proof在同一Product事务，Resource COMMIT未知时保留原reservation，不释放或再reserve。后台沿既有RecoverDue只核实相同owner，不新增第二reconciler。

未保存的item只有在本owner先写终止fence、清除所有有效claim、证明不能再发布后，才提供 `failed_fenced` proof/release。绝不按浏览器超时或worklow timeout直接释放并允许晚到publication。禁用/撤权只停止新效果，不妨碍原已保存结果的账本结算核实。

## 8. 状态与恢复

| 事实 | 事件/前置 | 持久效果 / 用户显示 |
| --- | --- | --- |
| Job | admit/query/limit confirmed | `PREPARED`，原scope、hash、quota；未启动不显示执行中 |
| Job | exact workflow EnsureExecution | `DISCOVERING/RUNNING`，同稳定workflow ID |
| Item | original resource reservation bound + live access | `FETCHING` with fence/lease |
| Item | valid evidence returned + current fence | `PREPARED_EVIDENCE`，首次固定payload；不会在恢复时换内容 |
| Item | Product UoW commit | `SAVED` with exact source/catalog/collection refs；费用可能pending |
| Item | fenced no publication / terminal invalid evidence | `FAILED`，原因及failed_fenced proof |
| Job | all items terminal, saved+failed true counts | `SUCCEEDED / PARTIAL / FAILED`，empty明示0结果；no fabricated success |
| Job | original actor explicit cancel | 停止未claim/未发布项，先fence再release；已保存项保留 |
| Key | disabled/revoked or original access gone | 不再启动或发布；已保存项只核实原费用；API读拒绝 |

Temporal workflow只调用原job/item的owner命令；不保存第二状态机。HTTP创建202返回job ID和原key的状态查询；创建retry/读取触发同一有界EnsureExecution以恢复已commit而未start的job。workflow ID固定org+actor+jobID；重复启动校验同identity，不再建新workflow。安装配置不丢失原namespace/workflow，重启worker恢复未完成项；本批不建通用scheduler/恢复平台。

发现与商品Fetch都是匿名只读，允许原job在有限attempt/deadline内恢复尚未保存的只读请求；provider返回后以item fence保证只有一个固定evidence可保存。Browser结果未知本身不意味着付费外部mutation，不能据此新发资源reservation。已持久evidence/publication绝不再次Fetch。job最长30分钟、单页/Fetch最长30秒、浏览器并发最多2、单企业active job最多8；输入200条上限与数量/时限同时适用，部分完成诚实保留。DB/UoW最长10秒、外部auth最长5秒，超限返回真实pending/partial/failure，不无限延长原deadline。

## 9. 定制需求与交付

需求只保存本人verified scope、Amazon站点、输入条件/用途、fields、数量(1–200/次有界交付)、时间范围、期望交付格式和文字要求，不收集第三方账号/凭据。一次性/定期更新是需求描述；定期服务由专员线下安排，本批没有自动周期执行。专员由当前 verified platform-admin权限进入独立非企业菜单路由；客户企业管理员不推导成platform specialist。

状态顺序 `SUBMITTED → EVALUATING → SPEC_CONFIRMED → PREPARING → DELIVERED`；未交付时可CLOSED，终态不可重开/覆写。spec确认记录具体范围、线下报价/确认说明、revision与actor/time；只证明已记录线下确认，不能宣称已付款。阶段与事件/command receipt同事务，精确expected revision。规格变化为新revision，交付必须绑定当前confirmed spec。

专员上传有界CSV/JSON/XLSX商品数据，复用现有Collection导入和Excelize的公式/外链/zip/行数防护，strict fields；不是用URL去拉文件。数据声明为 `specialist_declared`，附原交付需求和确认revision、站点/来源说明、原operator，不冒充runtime实际抓取或已批准素材。需求申请人/org/member由已存在原request读取，上传体没有任意target scope。

专员交付需要独立 `DeliveryAuthority` 窄Port：当前平台权限 + 原request/spec/未交付 + 原申请人当前grant/Collection访问有效。该authority不作为全站用户identity，不授予任意Product读取。DataAcquisition/SRC/Collection在同ProductDB一事务创建不可变declared source、Catalog版本与原创建者custom_dataset batch refs，同时写DELIVERED与delivery receipt。上传错误/COMMIT未知通过原delivery key核实，不能重复导入。`DELIVERED`必须有该same-transaction批次回执，不能仅凭专员点击“完成”。

定制没有本地paid标记、钱包/支付mutation，也不自动消费实时抓取DATA_ROW；费用展示为线下单独报价。下载消费现有Collection exact-version读取，CSV公式危险值输出按现有安全规则处理；不新增文件镜像或第二dataset正文owner。

## 10. HTTP / BFF / UI 与真实投影

专属Console routes `/workbench/data/market`、`/workbench/data/api`，平台定制处理 `/workbench/admin/data-customization`。沿现有Shell/tokens/page-template；shared nav最后由既定owner消费，不另造菜单系统。BFF按现有固定origin、server-only session、no redirects、deadline/JSON size验证；跨企业/actor/key/job切换取消pending请求，晚到响应不能更新另一个scope；未知mutation保留原key/冻结payload。

API固定 `/data-api/v1/amazon/jobs`（POST）、`/data-api/v1/amazon/jobs/{id}` / `results`（GET），仅data-key协议；Console管理API和platform routes使用正常session身份，descriptor不与全站auth替换混用。所有GET拒绝unread请求体，Console/外部POST最大64KiB；交付upload最多2MiB，最多200行且每envelope受2MiB owner约束；JSON/分页response最多2MiB/100条，大结果按exact itemcursor取，每次实时授权。errors无secret、原始HTTP body或内部provider响应。

API四tab的card/列表从scope-qualified DB job/key与Resource receipt聚合：有效密钥、当日成功保存、本月确认费用、明确时间窗的成功率；样本为0时比率未知，不显示0%成功。调用记录显示原query数量、saved/failed/pending、已确认费用与unknown；生成的job不宣称成功返回所有请求条数。80%费用阈值及余额/异常提示按真实limit/Resource状态显示本页告警；没有有效通知owner输入就不声称已发送提醒，不为了此提示建设新的通知系统。

开发文档包含实际路由、strong schema、站点/字段、auth、idempotency、202/poll/result、计量和撤销行为与curl example；所有example使用明确synthetic key，不包含真实凭据。空安装无自动样例、平台目录仍有实际configured readiness。URL/按钮只有对应准确route/auth/worker/schema/resource配置ready时开放。

## 11. Threat Model、约束与测试义务

本批保护：恶意客户输入/页面/交付文件、其他企业/创建者/旧member/key伪造、secret泄露、撤权和key撤销、跨scope迟到响应、并发quota超限、重复资源消费、Product COMMIT/Resource响应丢失与晚到publication。不新增理想全局IAM、统一限流平台、多出口抗封控、容量平台或新的人工验收机制。

TDD先捕获：未知site误选US；输入/URL越界；search重复/空页/数量/截止；secret重复展示/credential范围升级；grant重建和user停用；同key异payload；并发quota与跨UTC窗口；fenced late-save；publication/Collection原子回滚；Resource reserve/commit unknown；specialist错误目标/spec/replay。

用既有Go test/临时Postgres/Temporal测试环境/前端Vitest和现有浏览器工具，验证实际窄caller和consumer。只为本改动和冻结Must补充必要负例；不开发runner、验收session平台或工具的验证工具。稳定候选后一次必需CI，最终独立Reviewer检查真实diff/调用链与可运行交接；实现者不签发用户验收。

## 12. Legacy decision

```text
Legacy decision: EXTRACT | RETIRE
Reusable behavior: Amazon合法字段解析、明确站点、可用价格/图片/变体校验；当前neutral publication/Catalog/Collection与资源合同。
Current owner: Product DataAcquisition + integration/acquisition/amazon；SRC/Catalog/Collection；现有Organization/Resource。
Cutover/deletion condition: 新用户路径不导入LegacyCrawlSource/旧Service/Task/全局config。抽取后旧行为调用方指向正确owner或继续标为退休；不增加wrapper、fallback、dual facts或legacy compatibility。
```

仅改变代码行为/新空安装schema，不处理历史身份/商品/费用，禁止数据迁移/backfill。旧使用者的外部兼容义务若出现具体当前证据，按AGENTS向用户报告，不自动恢复兼容。

## 13. 文件所有权、共享变更与准入

本Writer拟独占新 `internal/dataservice`、`internal/product/dataacquisition`、`internal/integration/acquisition/amazon`、`internal/integration/dataserviceauth`、本批persistence/app module、专属页面/BFF/client/contracts和本文。独立Reviewer只读。

必须协调的窄既有owner变更：

1. Collection domain/persistence的来源类型、批次publication Port/Source reader与原UI下载标签；新source facts仍由Collection owner管。原#605/#606实际main已交付，但不能与供应链/图片Writer并发写相同合同。
2. `ledger/orgresource/consumer_charge.go`消费枚举/intent校验登记Amazon专属1-row consumer；既有余额/结算schema/状态不改。公共Resource map/RecoverDue注册由runtime owner注入，不能抢改其装配。
3. Product installer/runtime enabled grants必须与Collection新来源/data-service tables、同owner publish Port一致，startup仅verify不DDL；`authz/module_catalog.go`与native RoleModules消费data-market/data-api窄permission；console-navigation/WorkspaceAppShell/currentapplication/Compose/worker启动由 #619 writer负责。

用户于 2026-10-09 明确：本线程负责上述 Collection 与 Resource 两处窄合同，#619负责公共导航/runtime，已记录Issue #621及PR #623。公共runtime不准备或未合入时本模块保持准确的未接线状态，不以独立测试声明正常安装已开放。本批不会修改其他worktree或临时借未合并代码覆盖main。

准入要求：§1产品细节已确认；本设计R1（最多R2）独立检查分类后达到IMPLEMENTATION_READY；共享owner明确；Issue正文Ready才开始production/schema。高风险检查只围绕新增key授权、Product UoW、quota与资源proof、跨scope专员交付，不重审无关全局架构。

## 14. 运行交接和验收层级

正常安装沿当前current-application；显式schema-owner初始化新空ProductDB，受限serving pool只获声明表DML/SELECT与所需函数权限，拒绝owner/superuser/DDL及不相干Store/其他数据库权限。新增tables不能由servingAutoMigrate，不能偷偷修试用库。

配置注入同一ProductDB/ResourceDB、ZITADEL准确Project/服务目录凭据、RolePolicyReader、匿名Amazon浏览器/出口、Temporal地址/namespace和worker。RUN-1 start/stop保留named volumes；保留实例/URL由获授权公共运行owner准备，凭据私密交接，不在Issue/PR输出secret。

本批最终交出正常入口与步骤、数据保存位置、已配置站点/字段/限制与未开放项。开发测试/CI、fixture browser、真实Amazon、用户试用及production各自PASS/FAIL/SKIP/NOT_RUN，精确绑定候选，不能互相代替。合并、关单、共享/生产部署、真实数据/付费provider均NOT_AUTHORIZED。

## 15. 独立评审记录

R1：`/root/architecture_review`只读检查设计候选 `6921e70816dadb4f0f552cbd138e60fd94aeaec0`（blob `6eb54f1a8c6eaa82c2e3991d0e95d3092b437599`），结论IMPLEMENTATION_READY，无BLOCKER。共享owner协调已完成，Issue Ready后允许实施。R2不需要；冻结边界不变时只复核实现增量。

三项 finding 均为 IMPLEMENTATION_TEST，影响当前Must且必须在本批合并前收敛：①资金来源冻结后，每次新reserve/Fetch/publication仍检查当前资金授权；管理员降权不能继续企业未分配资金，已保存结果的原费用核实仍允许；②按§7统一锁顺序验证并发准入/改限额/撤销/发布/取消；③原Resource reserve已提交但绑定响应丢失时，按原operation ID核实同一reservation并恢复准确绑定，撤销后仅允许核实/fence/release，不再Fetch/publication或新reserve。测试、运行及用户验收在R1中均NOT_RUN。
