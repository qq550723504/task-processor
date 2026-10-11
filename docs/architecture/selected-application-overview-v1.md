# 优选申请总览只读汇总 v1

Status: **IMPLEMENTATION_READY / FROZEN**. Execution: #619; primary PR: #631.

独立只读架构复核（2026-10-11）：IMPLEMENTATION_READY，无 BLOCKER；评审绑定
起始 HEAD `9fac2dcd6ca61982214655305ce9fa13d1b9dd20` 与提案 SHA256
`31CDF33D2B5EDAF587BF84876648DF5760D95B91091B7294B3F1644F2BC8A87B`。
R1（Collection org/actor 事实隔离、MemberID 授权/密封上下文及申请精确三字段）和
R2（跨页全数、稳定前三、2000预算/并发失败不返回部分）均为 IMPLEMENTATION_TEST，
在当前实现和测试中收敛，不新增 schema 或扩大治理体系。R1 文案已按实际 owner 修正。

## Product outcome and authority

2026-10-11 用户明确要求 `/workbench/supply/applications` 对齐 Figma，并选择
“完整接入真实统计和商品预览，先补齐只读接口设计”。本轮交付一个真实成员总览：
发起申请、查看申请记录、四项指标、申请条件/流程/服务说明、最多三件商品预览。
继续一个 Writer、现有 worktree/分支/PR，不增加开发线程。

Design Basis: Independent Architecture，因为新增只读领域汇总合同。
复用 IMPLEMENTATION_READY `supply-market-v1.md` 及当前 Collection/Catalog/Review
owner；本轮不改变事实 owner、授权模型或写入语义。
UI authority: `final-ui-ia-authority.md`，Figma `tg48P46SSXl6TBy9lZwg63` / `31:463`，
深色 frame `429:2507` / content `996:359`，浅色 `429:2712` / `996:451`。
本轮完整 design context/截图已读取；其中数字和商品仅为原型示例，不能用于正式数据。
用户新范围取代此前菜单修复中“父页统计/商品预览 Out of Scope”，历史证据保留。

## Scope and non-goals

Scope: member-only GET overview、原 owner 真实申请统计与已优化自有商品预览、
Figma 总览布局/主题/响应式、原三个叶入口、当前保留本地实例交付。
Out of Scope: 申请表单拆步骤、商品/供货/资质业务规则变更、申请/审核/发布写入、
新表/迁移、支付/连接器/provider、全局 UI shell、真实数据变更、合并/关单/生产。
“可申请商品”指可进入原申请表单的基础候选：本人当前有效自有商品 + 现有
`Service.Choice` 返回 `optimized=true`。库存/生产能力、资质、披露及最终提交的
资格仍在原表单和 `submit_selected` 核验；总览不提前声称申请一定成功。

## Fact owners and path

`GET /api/v1/workbench/supply-market/application-overview`（无参数/请求体）
→ 当前 native route / binder / authorizer → SupplyMarket Service →
1. SupplyMarket repository 对 `supply_market_records` 按精确 scope 与 kind=selected
   进行数据库聚合；审核中 = SUBMITTED + EVALUATING，补充 = SUPPLEMENT_REQUIRED，
   已通过 = APPROVED。不混入其他 kind 或结束页总数，不从当前分页推导。
2. 已注入的 Collection Service `ListItems(ctx,"",Query)` 枚举当前本人自有商品；
   原 Collection 负责 archive、batch 与 org/actor 事实过滤；MemberID 属于当前
   授权/密封选择上下文，不将其臆造为 Collection 表中不存在的 owner 列。
3. 对枚举项调用原 `Service.Choice`，保留 sealed Collection selection、精确原始
   publication/version 与 Catalog/Review Apply lineage 校验，不用 SQL 猜测优化事实。
4. 计入 optimized 的完整数量，预览按 Collection 原排序取前三件；预览只输出
   item ID、当前 Choice 商品 title/首图、Collection batch name、固定事实来源 own。
   batch name 用 Collection 新增薄只读 `ReadBatch` 调原 repository 已有 ReadBatch；
   不跨 owner 直接读 Collection/Catalog/Review 表，不造第二套商品或资格事实。
→ 已有 same-origin BFF / expected-user/org /严格 Zod schema → scoped React Query
→ root overview。progress/records/admin/form/command 路径保持既有消费者。

响应：`{eligibleProducts, reviewing, supplementRequired, approved, products:[
{itemId,title,thumbnailUrl,groupName,source:"own"}]}`。计数为非负安全整数；products 最多3，
ID/文字/HTTPS图片沿用当前合同边界；不输出 source refs、sealed token、资质或价格。
返回 immutable 数据投影，无 mutation intent，不能提交申请。发起按钮仍去原 `/new`，
记录按钮去 `/records`，申请进度由既有三级菜单进入。

## Authorization and tenant invariants

native descriptor 复用 Choice 的 current identity + cached organization read +
PermissionApply + 10s deadline。Service 开始与响应前复核市场 read/apply、Collection
read/manage，四次授权所得 scope 必须完全一致，并绑定当前身份 org/actor/member；
不接受客户端 scope 或商品 ID 作为汇总参数，不提供 admin 汇总或跨企业结果。
每次 Choice 保留现有授权复核；每个分页/预览 batch 仍由 Collection 授权。
拒绝身份未就绪、换企业过程、授权撤销或 scope 漂移；返回结果前再次核验。
BFF沿原 session/expected-user/org/feature gate 和 route allowlist，overview GET 使用
原 apply permission；绝不放宽原 read/manage/tenant 边界。权限失败不当作空列表或0。
React Query key含 user/org，无 keepPreviousData，MarketBoundary防换企业展示旧事实。

## Consistency, bounds and errors

纯只读投影，无事务提交、状态机、幂等键、outbox、恢复 owner 或外部 mutation。
申请三项计数是同一条聚合 SQL；商品统计经完整 cursor 遍历才能返回数字；最多扫描
2000件当前本人自有商品，每页50，Choice复用 errgroup、并发最多4，总请求10s。
预检 total / 实际枚举量超预算、重复 ID/cursor、当前项 revision 与 Choice 不一致、
单项核验失败、超时/取消、缺依赖均终止整次读取，不返回局部计数或假0。
该预算是请求资源保护，非新的商品配额；超预算显示明确读取失败和既有 /new 入口。
不做浏览器全库 fan-out、后台缓存/定时更新/资格物化；今后实测瓶颈再补 owner 批量读取。
自有商品和申请状态是当前只读视图，非跨 owner 的同一时间点快照；并发变化可能需刷新。
分页游标/Choice revision冲突显示读取失败，GET可安全重试；提交仍重新查原资格。
预览最多三张当前真实图；不使用 Figma 色块或示例商品作为运行数据。
BFF原响应2MiB/超时/strict JSON/no-store保持；HTTP GET body和未知query先拒绝，
所有错误走既有错误码。失败时四项指标显示不可用、商品区显示读错和重试，不能显示空。
加载中用加载文本；完整成功但0商品显示真实空态；正常非空由API数据驱动。

## Legacy decision

N/A：仅当前 SupplyMarket/Collection/Catalog/Review owner 和原生装配。
不依赖 RETIRE legacy、不迁移旧数据、不新增兼容/fallback/双读双写。

## Implementation and verification

架构复核达到 IMPLEMENTATION_READY 后才修改生产代码。
TDD: 数据库聚合跨 org/actor/member/kind 与混合 stage、数量超分页；Service 优化资格、
全量跨页计数/前三预览、未优化排除、依赖/预算/取消/漂移拒绝、响应前撤权；
route/BFF拒绝body/query/admin和错误schema；UI真实指标/图片、空态/失败不假0、
原叶/admin查询不变。复用现有 Testcontainers 独立DB及测试，不建设验收工具。
必要Go相关包、前端相关测试/typecheck/lint、准确候选生产构建；冻结源码/镜像绑定。
当前本地Compose只替换必要backend/UI，保留所有卷和登录；Edge桌面/手机截图及按钮。
最后一次独立增量交付复核；用户实际产品试用/业务写入/provider/payment保持 NOT_RUN。
