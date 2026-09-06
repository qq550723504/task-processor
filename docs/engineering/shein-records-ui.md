# 任务中心：本地资料准备工作记录

Refs #328 / PR #330。当前产品映射以 #328 正文和 R328-C1（issuecomment-5558848270）为准，替代旧独立 SHEIN 列表入口决定。#331 已合入，#329 已合入为 e0ba4afd4031ba7523ef7138e4055637ed7a19f8，对应 main CI 34029559189 成功。

## 当前交付边界

任务中心两条现有登记路径 `/workbench/ai/tasks`、`/workbench/ai/tasks/completed` 已消费统一 ConsolePage、ConsoleToolbar、ConsoleState、Shell 和主题变量。当前仅完成布局及无数据接线的组件切片：运行页面明确“暂未启用”，不会查询投影或使用运行时假数据。导航仍不把该能力登记为 connected。#340 shared exports 已明确 planned，但尚未交付可 import 代码；实际 API 接线和最终联调继续等待该 owner，不阻止本片组件实现。

展示组件使用 React 内容插槽，不定义第二份 wire schema、client 或 source→work projection。唯一合成成功 DTO 在 `task-center.test.tsx`，依据 #340 C340-H1，仅纠正 href；它不是接口或业务数据。列表选择、右侧详情、键盘焦点、空集合/未启用与覆盖范围可在组件层验证。

## Frame → 路由 / 组件 → 处置

本轮先加载 figma-design-to-code，再实际读取 design context 和截图。参考文件 `tg48P46SSXl6TBy9lZwg63`、页面 `31:463`，未修改 Figma。

| Frame | 当前映射 | 处置 |
| --- | --- | --- |
| [427:3005 任务中心](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=427-3005) | TaskCenterLayout / CompletedWorkResults / WorkResultDetail；上述两条任务中心路径 | 保留概览、状态切换、列表与右侧详情层级。仅消费本地资料准备完成记录，不伪造任务生命周期 |
| [429:5323 店铺商品](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=429-5323) | 本片不接该页面 | 原稿明确仅展示已发布商品；本地资料无 Store 关联，展示通用业务，不猜测店铺 |
| #331 已验收框架 | Console Shell / tokens / Page-Toolbar-State / 原企业上下文 | 复用；main 同步冲突保留 ConsoleOverview，不恢复旧空首页或平台卡片 |

1440×900 原稿：240px 侧栏、72px 顶栏、内容左右32px、概览88px、状态按钮38px、列表758px与详情356px、间隔22px；Noto Sans SC。局部 CSS module 使用已有语义变量，不新增主题框架。原稿低对比字色沿 Console 的 AA 适配；Toolbar 带标签/边框的通用容器比原稿状态行更高。390px 使用工程流式适配，不冒充已有 Figma 移动稿。

| 参考 / 运行证据 | 范围 |
| --- | --- |
| [任务中心原稿](evidence/issue328/r328/figma-task-center.png)、[店铺商品边界原稿](evidence/issue328/r328/figma-store-products.png) | 当前设计参考图，含设计示例数据，不是实际经营结果 |
| [1440未启用](evidence/issue328/r328/unavailable-1440.png)、[390未启用](evidence/issue328/r328/unavailable-390.png) | 本分支实际 Next 页面，由现有工作台点击任务中心进入；无投影API请求，axe无违规，无横向溢出 |
| 已完成数据双栏 / 真实投影→诊断→返回 | NOT_RUN，待 #340 实际 BFF/client/启动器交接；不得用上面的图或组件fixture替代 |

## 产品语义与隔离

- coverage 仅为本地资料准备完成记录，不是企业全部任务。空集合也不证明其他业务为空。
- “完成”只指本地资料创建已提交，不代表诊断通过或可发布。运行中、待确认、暂停、全局统计、今日完成数、Chat发起/搜索/筛选、AI建议均明确未接入，不用0或假进度替代。
- source_record_id 不改称 task_id，不制造 AgentRun / BusinessTask；正确诊断链接为 `/workbench/shein-records/{source_record_id}/diagnostic`。
- #340 独占 DTO、client、BFF 和对应共享fixture；#327 的 Go SQL/授权/cursor 仍是权威。前端不复制 projection 判定。
- 展示组件在结果页成员变化时清空选择，覆盖 A→移除A→重新出现A。后续数据请求层必须按身份/企业/角色/请求序号重挂载，取消请求并清空分页、列表及详情；生产 scope、分页、撤权、迟到响应和诊断返回尚待接线验证。
- `/workbench/shein-records` 独立列表继续404；原诊断深链有效。旧卡片及返回旧列表链接保持撤回。

Legacy decision: RETIRE
Reusable behavior: 原列表metadata精度、opaque cursor分页/刷新、取消/迟到响应和诊断授权测试作为提取依据。
Current owner: 任务中心内容消费#340只读产品投影；领域事实仍归Listing。
Cutover/deletion condition: 旧独立入口保持关闭，新布局不包旧页面、不增加兼容层。

## 当前验证与重现

前端目录运行：

```powershell
npm.cmd exec -- vitest run src/components/workbench/task-center src/app/workbench/shein-records/entry-boundary.test.tsx src/components/workbench/shein-diagnostic src/components/workbench/workspace-app-shell.test.tsx
npm.cmd run typecheck
npm.cmd run build
```

组件5项先红后绿；独立复核发现选择复活后补失败断言并修正。关联54项组件测试通过；局部lint、typecheck、build通过。原入口关闭2项与诊断9项浏览器回归通过。最终完整 SHA 的 CI/独立复核状态维护在 PR；这些不是新投影全链验收。

本切片实际未启用页面的浏览器重现（原诊断 fixture，尚无 #340 投影）：

1. 仓库根运行 `node web/listingkit-ui/scripts/shein-diagnostic-fixture.mjs --serve`。使用任务隔离 Go / PostgreSQL，不连接共享历史库。
2. 第二个 PowerShell 进入前端目录，将 `ISSUE328_FIXTURE_MANIFEST` 设为本次输出的 manifest 绝对路径，运行 `npm.cmd exec -- playwright test --config playwright.task-center.config.ts`。脚本用当前合成 session 从工作台点击任务中心与已完成，验证未启用、零投影请求、1440/390、axe及Escape归还焦点。
3. 此阶段测试不显示合成成功DTO，不对外宣称投影可用。数据双栏仅在组件测试渲染批准DTO，最终真实浏览器用例在#340交付后补齐。
4. 结束时在本次 manifest 的 `controlDirectory` 写空文件 `stop-fixture`，等待启动器正常核对内容/xmin并清理自有容器。

本轮 fixture 已停止，Go TestSheinDiagnosticBrowserFixture PASS（332.04s），启动器正常退出0并删除自有容器。没有仍可访问的本轮地址。manifest/session/cookie/敏感trace不提交；真实ZITADEL、生产环境、真实账号与业务数据 NOT_RUN。

## 历史检查点

旧7c6cf76的工作台卡片→独立列表截图和流程均为已撤回的历史证据；a085c784修正撤回入口。保留历史图不代表当前可访问能力。本次接续保持原分支/PR，无 reset、强推、合并、关单或部署授权。
