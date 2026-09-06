# SHEIN 资料诊断页面与受控浏览器验收

Refs #324；BFF 与跨进程 fixture 由 #323 独占，见 [BFF 合同和启动说明](shein-diagnostic-bff-fixture.md)。最终验收 HEAD、CI 与独立复核记录在 PR，不以本说明代替当前 PR 状态。

## 页面与行为

内部深链为 `/workbench/shein-records/{record_id}/diagnostic`，由既有 ApplicationFrame / WorkspaceAppShell 挂载，不新增菜单。使用实际创建返回的资料 ID；这里没有资料列表、UUID 输入流程或店铺归属推断。

- 首次明确请求 `action=publish`，可切换到按本地草稿规则检查。
- “重新检查”读取当前内容；“复核同一内容”仅为当前动作携带当前报告的 expected_digest。内容变化失败后需明确点击“检查当前内容”，没有静默去掉摘要重试。
- 问题、警告、检查项目都直接来自服务端，可用鼠标或键盘展开字段路径与建议。不在前端重新计算规则。
- 未评估范围单列；无 blocker 不代表可以发布。资料只是硕米本地记录，没有保存到 SHEIN 或属于某店铺的含义。
- 切企业、用户、权限上下文及卸载会销毁请求子树并取消请求，结果不保留在持久缓存。每个组织/资料/action/本次请求有独立查询键。失败、刷新和切换时隐藏旧结果，迟到响应不能重新显示旧企业内容。
- 页面仅消费同源只读 client。无发布、Apply、自动生成、报告持久化、轮询或假数据 fallback。

## 可重复启动与访问

前提是此 worktree 已包含 #323 fixture 提交，已安装 Go、Docker、Node 和锁文件指定的前端依赖。

1. 从仓库根目录启动本次隔离资源：

   ```powershell
   node web/listingkit-ui/scripts/shein-diagnostic-fixture.mjs --serve
   ```

2. 记下 launcher 输出的 `manifest`、`origin` 和 `recordId`。每次端口、资料 ID、临时目录和合成会话均重新生成；不要复制旧运行的地址作为固定入口。页面地址是 `<origin>/workbench/shein-records/<recordId>/diagnostic`。

3. 在另一个终端，从 `web/listingkit-ui` 运行：

   ```powershell
   $env:ISSUE324_FIXTURE_MANIFEST = '填写本次输出的 fixture.json 绝对路径'
   pnpm exec playwright test --config playwright.shein-diagnostic.config.ts
   ```

   浏览器测试读取这个临时 manifest 的合成 session，进入真实页面门禁及企业 Shell。直接在未设置该合成 session 的浏览器打开地址会进入既有登录流程。无需也不得修改生产 auth/proxy。

4. 要逐步观看和操作同一验证流程，可加 `--headed --debug`；在 Playwright Inspector 中逐步执行至页面打开，然后检查动作选择、问题展开及企业切换。组织 `200` 有此记录，切换到 `100` 后应显示不可读取，不能保留原报告；切回后重新读取。

5. 停止时向本次 manifest 的 `controlDirectory` 写入 `stop-fixture` 空文件，或在 launcher 终端 Ctrl+C；等待正常退出。Go 会核对业务表内容与行版本未变化，launcher 只清理本次随机 schema/容器。不要清理其他数据库或容器。

本 fixture 的 Catalog Publisher、记录 POST、Go middleware/reader/evaluator、PostgreSQL、Next serverAuth 解密、页面门禁、实际 BFF 和 HTTP 转发均真实运行。输入产品是合成内容，外部身份签发、Verifier/grant source 是明确的测试替身。**真实 ZITADEL 登录、正式产品来源和生产开放：NOT_RUN / 未授权。**

## 验证分层

- `pnpm test src/components/workbench/shein-diagnostic`：组件、生命周期、真实 client/decoder 和真实企业 provider 测试；HTTP 响应为合成 fixture，不是跨进程验收。
- `pnpm exec playwright test --config playwright.shein-diagnostic.config.ts`：前四项使用实际 BFF/Go/隔离 PG，覆盖刷新、摘要、动作、企业、窄屏、Store-only 拒绝和 Go 超时。名称以 `synthetic browser response only` 开头的错误场景只证明浏览器展示，明确拦截诊断 HTTP，不冒充真实诊断。
- 默认 `pnpm test:e2e` 保留原公共首页和无障碍回归；受控业务浏览器测试使用独立配置，不会因普通 CI 缺少隔离 fixture 而静默转用生产环境。
- 完整前端检查：`pnpm lint`、`pnpm typecheck`、`pnpm test`、`pnpm test:e2e`、`pnpm build`。

浏览器截图由测试保存在仓库根 `.local/issue324-browser` 的相应测试目录。截图仅展示本次合成产品与受控身份下的真实服务端结果，不能视为真实登录或客户生产验收。PR 附桌面与窄屏截图、当次测试结果和独立复核证据。

## 产品与文件边界

Authority：[最终 UI / IA](../product/final-ui-ia-authority.md) 与 #324 批准的有界深链。本页沿用现有 Shell 视觉变量，不宣称逐像素还原 Figma 或完成整体最终 Shell。

#324 只拥有页面、`components/workbench/shein-diagnostic`、UI 测试/fixture 和本说明。#323 的 schema/client/proxy、专用 API route、Go 跨进程 fixture 与启动脚本通过依赖提交消费，修复仍归 #323 owner。测试替身从不进入客户运行路径。
