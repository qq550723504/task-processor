# Store Center 商品与订单只读首版接线

Issue #614 / PR #615；Design Basis 为 `store-center-platform-observations-v1.md` 的冻结基线。本文交给已指定公共启动 Writer（会话 `01a11f74-2565-78e0-9118-4fe1ff53e726`）。当前主要分支提供独立模块；公共启动、权限目录、导航、Store enabled preflight 仍由该 Writer 接线。本文不授予真实数据库安装、平台读取、部署或合并。

## 注入与生命周期

1. 复用当前 Store 专用数据库的 serving pool，不开第二数据库连接或 schema owner pool 供业务运行。仅在授权的新空实例显式初始化时调用 `sheinobservations.Install(ctx, schemaOwnerDB)` 与 `GrantRuntime(ctx, schemaOwnerDB, servingRole)`；已经存在 schema 时拒绝迁移。
2. 所有当前 Store / Supply / resource 的 schema preflight 同时消费相同的 observation-enabled 清单。`search_path` 仍固定 `public`；新增四张表在 `shein_observations`，都是 schema-qualified 查询。运行角色只有该 schema 的 USAGE，commands 的 SELECT/INSERT，其他三表再加 UPDATE；没有 owner、CREATE、DELETE、TRUNCATE、REFERENCES、TRIGGER。不可只放宽新模块 verifier 而让旧 Store verifier 拒绝新增授权。
3. `sheinobservations.NewRepository(ctx, servingDB)` 验证观察 schema 与权限；constructor 不安装 schema、不 AutoMigrate。缺表、角色过权或未启用时不要创建可用模块。
4. `storeobservationsapp.Authorization` 注入当前 exact ZITADEL client、service token supplier、ProjectID、`*authz.ListingKitAuthorizer` 和组织状态 checker。原 MemberID 始终来自 verified request 或持久命令，worker 只复核当前 exact grant。
5. 从当前 Store member-scoped repository 和已配置三类 official application registry 调用 `storecenterapp.NewOfficialObservationAccess(storeRepo, authorization, registry)`。同一 `OfficialClient` 实现独立 `OfficialObservationProvider`；不要求或借用 Publish/Transform 授权。
6. 构造 `observations.Service{Repository:repo, Access:storeobservationsapp.Access{Authorization:authorization, Official:officialAccess}, Directory:storeobservationsapp.Directory{Stores:currentMemberScopedStoreRepo}}`。注入 `storeobservationsruntime.Starter{Client:existingTemporalClient}`；Temporal client 的 dial 仍归现有 platform owner。
7. `storeobservationsruntime.NewWorker(existingTemporalClient, service)` 注册模块本地 workflow 和逐页活动。启动成功后 readiness callback 才为 true；关闭时停止 worker。task queue `store-observations-current`；workflow `StoreObservationsV1`，固定 ID `store-observations/<org>/<syncId>`。不另建 runner、scheduler 或恢复平台。
8. `storeobservationsapp.Application{Service:service, Ready:workerReadyCallback}`；`storeobservationshttp.Routes(application, currentIdentityBinder)` 合入正常 current HTTP descriptors。binder 使用当前 effective org / user / member；客户端不能提交 scope。`Available()` 同时要求 repository、access、directory、starter、ready callback。

## 权限与 Console

公共 authz owner 把 `store-products` / `store-orders` 模块与四项权限纳入当前模块目录、role policies、Workbench display permissions。read 同时需要 `workbench.store.read`；sync 再需要对应独立权限，不能用 supply/publish 权限代替。

- `workbench.store.products.read`
- `workbench.store.products.sync`
- `workbench.store.orders.read`
- `workbench.store.orders.sync`

Console 导航正常入口为 `/workbench/store-products` 和 `/workbench/store-orders`。页面及独立 BFF 已在此分支；导航 owner 只在模块 readiness 与权限配置完成后将入口由 pending 接通。根布局的现有 Console shell 不需要另建。BFF 使用当前 `LISTINGKIT_SERVICE_API_BASE`、服务端 session token、所选企业 cookie、Expected Organization/User headers，以及现有同源保护；没有新的平台 URL 或密钥配置。

HTTP 基础路径 `/api/v1/workbench/store-observations`，products / orders 各有 capabilities、列表、POST syncs、commands/:key、syncs/:id、POST syncs/:id/ensure 和 scope-qualified detail；订单另有 packages/:packageId/track。服务端只允许 SHEIN 官方已确认的商品/订单/物流读取 endpoint，没有发货、售后、资金或商品写接口。

## 用户操作与保存

在正常工作台选择企业，确认有 Store read、对应模块 read/sync 及店铺成员授权。店铺须 active，服务有效，官方连接 verified，当前应用与凭据可用。进入商品或订单页，选择店铺或全部授权店铺后点同步；可查看各店铺进度、明确部分结果、商品分层详情、订单当前详情与实际包裹的物流轨迹。

订单默认取得最近30天，自运营/半托管应用支持消费者订单；全托管明确不支持，不以采购单替代。订单已发货/待揽收等平台状态保持原值。未知库存、时间、价格或审核中状态显示未提供。发货/售后使用固定 `https://sellerhub.shein.com/` 新窗口并可复制订单号，不自动执行操作。

父操作和全部子同步、页数据/checkpoint、最终完成/head 都保存在 Store 专用 DB 的 observation schema。页面会话存储只保留本企业/用户的操作 key 和无凭据输入，用于丢失响应后的原操作查询/重试；不是业务事实源。重启通过 SQL checkpoint 和同一个 Temporal ID 恢复，只有明确的新同步才分配新操作 key。保留 named volumes 的正常 stop/restart 不删除观察；本文没有 destroy 或清理授权。

## 验证边界

开发自检覆盖官方 signed fixture、隐私 allowlist、原成员/撤权、跨 scope、固定回执、真实临时 PostgreSQL 事务/CAS/head 顺序/分页统计、Temporal testsuite、有界 HTTP、BFF 和页面丢失响应重试。临时数据库与测试 fixture 不是商家数据，也不代表平台/产品验收。

公共 runtime/schema-enabled preflight/正常菜单组合、实际浏览器画面对比、真实平台读取和用户试用在接线完成前均为 **NOT_RUN**。本 Writer没有修改公共启动、authz、导航或共享数据库；不得据单元测试、PR CI 或独立 AI 评审声明可上线。
