# SHEIN 本地资料：暂停挂载的实现资产

Refs #328 / PR #330。当前产品前置是 #331 共享框架验收和正式入口映射；API 配置不构成入口批准。

## 当前行为

- `/workbench` 不展示平台专属 SHEIN 卡片。
- `/workbench/shein-records` 进入 Next.js not-found，不挂载列表、不请求集合，即使配置 `SHEIN_RECORDS_API_ORIGIN`。
- 已交付的诊断深链仍可使用原授权流程；撤下指向暂停列表的返回链接。
- #327 的集合合同、typed client、BFF 与授权保持独立。列表组件、分页和请求隔离测试保留，待正式投影接入；不新增开关或开发旁路来启用旧入口。

## 保留的行为与 owner

列表仅呈现已授权 metadata；版本保持字符串，不猜测标题、Store 或发布状态，不逐行请求诊断。每页20条，最多保存50个游标位置，刷新回首屏。用户/企业/角色变化、退出、撤权和卸载取消请求并清空旧状态，迟到响应不能覆盖新作用域。失败与空集合分开。

#327 拥有 schema/client/BFF/Go/共享 fixture；#331 拥有 Shell、导航与模板。本片保留列表内容和局部请求生命周期，最终挂载须按 #328 当前正文恢复，不能自动重启旧工作台路径。

Legacy decision: RETIRE
Reusable behavior: 列表分页、精确版本、请求隔离与原诊断行为及其测试。
Current owner: 共享 Listing/Marketplace capability；最终 UI 投影等待 #331 映射。
Cutover/deletion condition: 旧卡片和返回链接已移除，旧列表路由不可用；不增加兼容路径。

## 验证当前入口关闭

组件测试：在 `web/listingkit-ui` 运行 `pnpm test src/app/workbench/shein-records src/components/workbench/shein-records src/components/workbench/shein-diagnostic`。

实际浏览器复用 [隔离 fixture](shein-diagnostic-bff-fixture.md)：

1. 仓库根运行 `node web/listingkit-ui/scripts/shein-diagnostic-fixture.mjs --serve`。
2. 在前端目录设置 `ISSUE328_FIXTURE_MANIFEST` 为本次输出的 manifest 绝对路径，然后执行 `pnpm exec playwright test --config playwright.shein-records.config.ts`。
3. 当前用例验证1440/390px工作台无旧入口、直达列表显示404、无集合请求；fixture 已配置真实 BFF/Go/隔离 PostgreSQL，因此不是以缺少后端配置冒充产品门禁。
4. 测试结束后在 manifest 的 `controlDirectory` 写入空文件 `stop-fixture`，等待 launcher 正常核对业务表内容/xmin并清理自有容器。不要公开会话cookie或清理其他资源。

既有诊断浏览器测试仍验证其独立功能。真实 ZITADEL、生产来源与正式产品验收不在此验证范围。

## 历史检查点

原工作台→列表→诊断→返回的脚本由当前入口关闭用例替代；旧实现和测试可从 PR 修复前提交 `7c6cf76f97a3cf8ea794a79a8ce385aece3e1906` 查阅，不作为当前可运行路径或最终 IA 验收。

保留的[桌面工作台](evidence/issue328/workbench-1440.png)、[桌面列表](evidence/issue328/list-viewport-1440.png)、[窄屏工作台](evidence/issue328/workbench-390.png)、[窄屏列表](evidence/issue328/list-viewport-390.png)仅为历史证据，不代表当前 UI。最终 SHA、CI 与评审结果在 PR 维护。
