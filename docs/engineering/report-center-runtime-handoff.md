# 个人报告 v1 接线交接

Execution: #628; shared composition owner: #619 / chat
01a11f74-2565-78e0-9118-4fe1ff53e726. Design:
`docs/architecture/ai-workbench-reports-v1.md` (IMPLEMENTATION_READY).

## 当前可消费接口

- `internal/integration/persistence/reportcenter.InstallSchema(db)`：使用私密
  owner 连接在**独立、空的报告数据库**原生安装 `report_center`。重复安装失败，
  不执行迁移。不能在 serving startup 或任何已有业务数据库上调用。
- 初始化负责人单独创建受限 LOGIN 角色与凭据，撤销数据库 CREATE / public schema
  CREATE，再调用 `GrantRuntime(db, role)`。只授予 SELECT / INSERT 和
  `favorites.favorite/updated_at` 的 UPDATE；不向 runtime 授予 owner membership。
  凭据遵循现有私密配置交接，不放入 Issue / PR。
- `internal/app/reportcenter.New(ctx, Dependencies{DB, Resolver, Policy, Reviews,
  Records})`：DB 是报告独立受限 pool；Resolver / Policy 使用当前 Workbench
  Organization resolver 与 Casbin。启动先验证 schema 和权限，缺配置不构造服务。
- Reviews 是当前请求身份授权的 `review.Service.Get` 窄 reader，不能注入固定
  Agent execution scope reader；Records 是当前 `record.Reader.ReadOfflinePackage`。
  两者可缺省，只阻对应来源保存，已有报告读取与收藏不依赖模型或原来源可用。
- `internal/reportcenter/httpapi.BuildRoutes(handler)` / `ValidateDescriptor`：
  6 条 `/api/v1/workbench/reports` 路线，read 是 CachedRead，write 是 LiveWrite，
  身份必须经过现有 CurrentIdentity middleware；不能裸挂 Gin handler 绕过边界。

## 正常程序接线

1. handler descriptors 已接入现有 currentapplication/module 准入，精确验证
   `reporthttp.ValidateDescriptor`；注入上述原 owner readers，不另建服务器或数据库事实源。
2. 已登记 `workbench.report.read` / `workbench.report.manage` 和 reports 模块。
   按当前企业角色/自定义角色合同授予；报告权限不隐含标题 Agent、Listing、模型权限。
   当前有效企业 context 的 permissions 必须返回真实授权，不由前端自行猜测。
3. 原生 `cmd/report-center-schema-init` 与 normal startup 已接入独立 Report DB。
   为新安装分配受限 pool；不改变或清理已有 retained runtime/volume。
4. 共享 `console-navigation.ts` 的 `ai/reports` 依实际配置启用，并登记
   `recent` / `favorites` / `all`。未接线时保持明确未开放，不填示例报告数量。

接线由用户已授权的共享 Writer 在同一 PR #630 完成。功能实现只使用同源
`/api/report-center/reports` BFF，复用 `LISTINGKIT_SERVICE_API_BASE=/api/v1`
现有 origin 配置、Auth.js 与企业选择 cookie；BFF 不要求新的 provider 配置。

正常 composition 注入已挂载 ProductAgent application 的请求身份 `review.Service.Get`，
保留其 live request capability；不使用 Agent execution 的固定 scope。未挂载该 owner 时，
标题来源不可用，但历史报告仍可查阅和收藏。当前正常 SupplyChain 的 TargetRecord 不是
`record.Reader.ReadOfflinePackage`；离线 SHEIN reader 只在独立显式试用应用中提供，
因此正常程序 **SHEIN_RECORD 新保存暂不可用**，不会暗中挂载旧试用应用或编造 adapter。
feature-local factory 可消费独立准入的原 SHEIN reader，已有该类型快照照常可读。

## 新安装与正常启动

适用于已批准的新安装，未在31544保留实例执行。单独建立空的 `reports` 数据库，
预创建 `report_center_runtime` LOGIN（NOSUPERUSER/NOCREATEDB/NOCREATEROLE/
NOREPLICATION/NOBYPASSRLS，无任何 owner membership），通过现有私密凭据流程交接。
数据库管理者先撤销 PUBLIC / runtime 的数据库 CREATE 和 public schema CREATE；
runtime 只有 CONNECT 和本 installer 的 report_center 受限 grants。
owner DSN 使用绝对路径私密常规文件，literal 127.0.0.1，dbname=reports；不是 runtime 凭据。

```powershell
go run ./cmd/report-center-schema-init -dsn-file 'C:\private\reports-owner.private.dsn' -confirm-database reports -install-empty-schema
```

命令用单连接、30秒截止、事务 advisory lock 确认数据库没有业务表、视图或序列；
原生 InstallSchema 和 GrantRuntime 同事务。grant 失败回滚；已有 report schema 或任何其他
业务对象拒绝，重复执行不是迁移。命令不生成凭据，也不在 serving startup 调用。

在现有完整私密 current-application manifest 中新增以下字段（密码由私密配置替换）：

```json
"reportCenter": {
  "database": {
    "host": "127.0.0.1", "port": 15432,
    "database": "reports", "user": "report_center_runtime",
    "password": "<private-runtime-password>", "maxConnections": 4
  }
}
```

Report DB 与所有已配置 owner 数据库必须不同；池不可复用。正常 serving 只 VerifySchema，
不建表、seed 或迁移；失败时关闭自己已打开的 pool，不关闭其他 owner 的别名 pool。

```powershell
go run ./cmd/current-application -config 'C:\private\current-application.json'
```

UI 沿用现有 `LISTINGKIT_SERVICE_API_BASE=<current application origin>/api/v1`，
只有后端 Report DB 成功接线的安装才设置 `LISTINGKIT_REPORT_CENTER_ENABLED=true`，
然后按现有 UI 正常启动方式运行。未设置时导航明确未启用。入口是该 UI origin 下的
`/workbench/ai/reports`；无额外 provider 配置，不因为 reports 模块授予原来源权限。

## 用户操作路径（接线完成后）

入口：`/workbench/ai/reports`，以及 `/recent`、`/favorites`、`/all`。

1. 正常登录，选择有报告查看权限的企业；有 manage 权限时点击“保存报告”。
2. 选择本人待处理标题审核或业务任务中的标题结果，点击“保存此版本”。SHEIN
   资料与诊断须原 offline owner 已显式准入，本次正常程序尚未挂载。已 Apply / 已拒绝的标题审核可在“已有审核详情”
   粘贴原详情链接或 ID，读取当前详情并检查版本后保存。来源在提交前变化会拒绝保存。
3. 在报告列表打开详情，检查保存时间、来源 ID / 精确版本与历史状态，收藏或
   取消收藏；下载 JSON / UTF-8 文本，或打开原业务来源继续操作。
4. 写响应丢失时点击“恢复原请求”，使用浏览器 sessionStorage 中原 key / 原 body；
   不重新分配请求。切换企业/用户会清空显示；回到原身份/企业才能恢复原请求。

报告长期保存在 Report DB；收藏是个人元数据。v1 没有删除/自动过期接口。
报告池应纳入现有数据库保存、备份、正常 stop/start 流程。下载是用户主动导出的
历史内容，不会重执行模型、Apply、诊断或平台副作用。

## 已有验证与限制

- 有界领域 / 来源 adapter / HTTP / 同源 BFF / 页面交互测试已执行；真实可销毁
  PostgreSQL fixture 验证受限 LOGIN、并发唯一性、旧收藏回放、响应丢失恢复与重开读回。
- 正常 config/lifecycle、六条精确路由、原请求 capability、模块权限和条件导航已接线；
  可销毁 PostgreSQL 验证 native composition 的受限角色查阅/收藏、无来源可读与隔离，
  initializer 验证 grant 失败回滚、成功安装、重复/非空拒绝。GET body 在读取前明确拒绝。
- 这些是开发证据。真实环境部署、真实浏览器端到端试用、用户验收均为
  **NOT_RUN**；不宣称目前“我的报告”已对用户开放。
- 标题审核是本次正常程序可注入来源；SHEIN 商品资料/保存时诊断的 feature-local
  reader 合同已实现，但正常 owner 未挂载。市场分析、经营分析、
  团队共享、报告编辑、Office / PDF / ZIP 格式生成尚未开放。
- report schema 安装、启动准入及保存不访问历史环境、不进行 paid provider 调用。

必要本地检查：

```powershell
go test ./internal/reportcenter/... ./internal/app/reportcenter
$env:TESTCONTAINERS_RYUK_DISABLED='true' # 仅本机 disposable fixture 的 Ryuk 故障时使用
go test -tags=integration ./internal/integration/persistence/reportcenter ./internal/app/reportcenter
Set-Location web/listingkit-ui
pnpm.cmd test src/components/workbench/reports/report-page.test.tsx src/lib/server/report-center-proxy.test.ts src/lib/api/report-center.test.ts
pnpm.cmd typecheck
```
