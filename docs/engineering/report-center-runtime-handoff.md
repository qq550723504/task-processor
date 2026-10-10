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

## 公共装配 owner 待完成的接线

1. 将 handler descriptors 接入现有 currentapplication/module 准入，精确验证
   `reporthttp.ValidateDescriptor`；注入上述原 owner readers，不另建服务器或数据库事实源。
2. 登记 `workbench.report.read` / `workbench.report.manage` 和 reports 模块。
   按当前企业角色/自定义角色合同授予；报告权限不隐含标题 Agent、Listing、模型权限。
   当前有效企业 context 的 permissions 必须返回真实授权，不由前端自行猜测。
3. 在共享原生 initializer 与 normal startup 配置中接入独立 Report DB。
   为新安装分配受限 pool；不改变或清理已有 retained runtime/volume。
4. 共享 `console-navigation.ts` 的 `ai/reports` 从占位改为已接线状态，并登记
   `recent` / `favorites` / `all`。未接线时保持明确未开放，不填示例报告数量。

本批没有修改上述公共文件，也没有为其发明第二套组合。功能实现只使用同源
`/api/report-center/reports` BFF，复用 `LISTINGKIT_SERVICE_API_BASE=/api/v1`
现有 origin 配置、Auth.js 与企业选择 cookie；BFF 不要求新的 provider 配置。

## 用户操作路径（接线完成后）

入口：`/workbench/ai/reports`，以及 `/recent`、`/favorites`、`/all`。

1. 正常登录，选择有报告查看权限的企业；有 manage 权限时点击“保存报告”。
2. 选择本人待处理标题审核、业务任务中的标题结果，或本人已保存的 SHEIN
   资料与诊断，点击“保存此版本”。已 Apply / 已拒绝的标题审核可在“已有审核详情”
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
- 这些是开发证据。正常共享 runtime 接线、真实浏览器端到端试用、用户验收均为
  **NOT_RUN**；不宣称目前“我的报告”已对用户开放。
- 标题审核和 SHEIN 商品资料/保存时诊断是首版真实来源；市场分析、经营分析、
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
