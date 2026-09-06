# 硕米统一 Console 基础

Refs #331。2026-09-06 实读 Figma `tg48P46SSXl6TBy9lZwg63` / `31:463`「硕米官网」：六个参考 Frame 的 design context + 截图，以及只读 Plugin API 的可见性、导航、尺寸、颜色、字体与 reactions。设计原稿未修改。

[FV1最终视觉验收：本轮参考/运行图、门禁修复前后、差异分类及源码SHA](console-fv1.md)。以下保留基础映射及可重复运行流程。

## Frame → 当前组件 / 路由

| 实读 Frame | 代码落点 | 保留 / 替换 |
| --- | --- | --- |
| [393:321 驾驶舱](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=393-321) | WorkspaceAppShell + ConsolePage；`/workbench` | 替换简化外观；概览卡片/双列布局，无假指标和曲线 |
| [427:2467 Chat](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=427-2467)、427:3005 任务中心 | console-navigation 静态登记；`/workbench/ai/chat` 代表卡片模板 | 一/二/三级展开；业务未接入明确标识，不运行假会话/任务 |
| [1560:359 我的店铺优化版](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=1560-359)、429:5323 店铺商品 | 既有 `/workbench/stores` + 列表工具区/卡片行 | 保留真实 filters/query/lifecycle/权限，替换旧表格外观；未有合同的统计/同步/店铺类型不伪造 |
| [432:4483 企业空间](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=432-4483)、425:976 用户菜单 | 顶部已有 OrganizationSwitcher + 账户下拉；账户导航三级登记 | 企业选择器整合在顶部账号附近，当前企业与代管始终可见；复用既有 logout URL，不新建 IAM/注销账户 |
| [411:323 浅色驾驶舱](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=411-323) | 同一组语义 CSS 变量的 light 值 | 默认深色，沿用 next-themes；390px 为工程响应式适配 |
| 已合入诊断深链 | `/workbench/shein-records/[record_id]/diagnostic` + 详情模板 | 只换外层页面结构；保留 action/digest/取消/错误恢复和授权 |

## 设计与交互合同

- 桌面 1440×900：侧栏240、顶栏72、品牌42×42、侧栏内距18、主内容28–32；一级菜单36高/40节距、二级26高/28节距、三级引导线；选中一级蓝色、二级青色。响应式以内容流布局实现，不固定900高。
- 当前10个一级：运营驾驶舱、AI工作台、供应市场、智能市场、工具市场、生态服务、数据服务、店铺中心、套餐与权益、我的账户。当前二/三级来自展开态实读；创业中心/顶层项目中心/旧商品中心等隐藏归档节点不登记。AI工作台内的二级项目中心仍可见，不能误删。
- 参考节点实际 raw fills 未绑定全局 BG 变量；采用实际可见 Frame 属性映射现有 background/card/primary/border 等语义变量，而非拿官网早期的紫蓝 BG 变量覆盖终稿。深色基底 #030711，内容 #070f1b / #07111e，卡片 #0d1a2c，边框 #1b314b，文字 #f3f7fc / #8fa6bf，强调青 #2dd4c5、蓝 #223b79。Noto Sans SC，标题26/32，正文13/14，导航12/11；低对比辅助字在保证层级的前提下达到AA。
- 一个静态 typed 登记源驱动导航层级、route匹配、选中和面包屑。未实现的项进入明确“未启用”的说明视图，无业务 fetch 或假成功；客服/通知不显示虚构计数，无接口时不可提交。现有Store链接只是入口，授权仍由现有页面/BFF确定。
- 展开/折叠与页面导航分开；键盘可达，移动导航可关闭、Escape关闭并返回触发焦点，切路由关闭。主题是已有本地偏好，不存储企业/报告/业务事实。
- 概览、列表、详情通过有限 React slots/组合共享，不引入页面Schema、插件注册平台或第二套控件库。现有Button/Select/Card继续消费同一 scoped 变量。

## 边界、切片与范围

本片不改变身份、租户、授权、BFF、schema、query keys、mutation/恢复协议或后端持久化。只读上下文及请求隔离继续由 WorkbenchContextProvider 和既有 hooks 持有。切企业/用户/退出/撤权的测试必须继续通过。选中/展开仅是当前浏览器展示状态，无副作用、重试或新事实源。

分两个实现提交：A 为共享 tokens/nav/Shell + 概览/Chat真实路由消费者；B 为既有 Store/诊断模板接入及验证。计划生产代码小于1500行；若超出30个语义相关源/测试文件再拆片。设计截图/导出资产单列，不扩展架构范围。窄复核后在同一任务继续实现。

文件 owner：`components/workbench/workspace-app-shell.tsx`、新增`components/workbench/console/*`、`lib/workbench/console-navigation.ts`、scoped CSS、`providers/theme-provider.tsx`必要默认参数、`application-frame.tsx`单一挂载、`app/workbench/page.tsx`与受登记约束的说明路由、StoreList/StoreTable和Diagnostic外层模板、相邻测试与本说明。不改 #327 API/schema/proxy/共享 fixture。现有非工作区 ListingKit legacy 路由仍在其原装配，本片不批量删除，亦不将其导航引入新工作区。

## #328 入口缺口

实读店铺商品只表达已经发布到店铺的平台商品；本地 Listing record 没有 Store 归属。当前六个终稿不能证明本地资料属于“店铺商品”或“我的报告”。#328 保持暂停，待协调方一次确认最终入口/模板映射；本片不新增首页卡片，不恢复 #330。

## 验证与交接

### 同视口设计对照

参考图为本任务从 Figma 导出的原图；运行图为本分支实际 Next 页面。桌面均为1440×900，窄屏 viewport 为390×844（概览/Chat/Store附全页截图）。以下链接不是正在运行的服务地址。

| 页面 | Figma原图 | 实际运行图 |
| --- | --- | --- |
| 驾驶舱深色 | [393:321](evidence/issue331/figma-393-321.png) | [1440](evidence/issue331/overview-1440.png)、[390](evidence/issue331/overview-390.png) |
| Chat | [427:2467](evidence/issue331/figma-427-2467.png) | [1440](evidence/issue331/chat-1440.png)、[390](evidence/issue331/chat-390.png) |
| 我的店铺 | [1560:359](evidence/issue331/figma-1560-359.png) | [1440](evidence/issue331/stores-1440.png)、[390](evidence/issue331/stores-390.png) |
| 驾驶舱浅色 | [411:323](evidence/issue331/figma-411-323.png) | [1440](evidence/issue331/overview-light-1440.png) |
| 既有诊断模板 | 无对应终稿，工程接入 | [1440](evidence/issue331/diagnostic-1440.png)、[390](evidence/issue331/diagnostic-390.png)、[390代管/错误](evidence/issue331/delegated-error-390.png) |

| 对照项 | 实现与差异处置 |
| --- | --- |
| Shell及内容宽度 | 同为240侧栏、72顶栏；主区x240，内容左右28–32；随视口流式布局。实际企业选择替代原稿装饰性英文位置，保留当前企业和代管安全信息。无企业/错误门禁仍由已有Shell逻辑先阻断。 |
| 导航层级和选中 | 10个可见一级、二级和Chat/任务/企业三级；以实际路径高亮，折叠按钮独立于链接。品牌及圆点使用Figma导出资产；隐藏历史一级不恢复。原稿最近会话的示例计数及示例选中，不冒充实际用户会话状态。 |
| 字体/色彩 | Noto Sans SC字体栈，Windows缺该字体时使用系统中文字体，存在字重/抗锯齿差异；标题26/32及导航层级一致。辅助文字/边框适度提高对比；浅色基于实际浅色Frame，强调文字加深以通过AA。 |
| 概览卡片/间距 | 指标112高、20间距；双列272/264高、18间距。指标行流式填满内容区，比原稿右侧额外留白更宽；不生成假GMV、趋势曲线、预警和待办。原稿业务筛选区替换为明确未接入说明。 |
| Chat卡片 | 三列等宽、18间距、588高；青/蓝/紫渐变及顶部短强调线。统计条替换为未启用状态；示例会话/计数/运行按钮不进入运行路径，三个操作明确禁用。 |
| Store卡片/工具区 | 复用当前真实筛选和分页，以卡片行呈现原表格真实字段、生命周期操作和详情入口；不添加原稿中无合同的同步/告警/店铺类型。筛选标签显式保留，因此工具栏比原稿56px高。四个指标仅投影现有response的总数/额度，不捏造业务统计。 |
| 390与交互 | 是工程响应式适配，非Figma移动稿；移动菜单为流式展开区，Escape在触发按钮及菜单内均关闭并归还焦点，切路由/浏览器历史关闭。所有视口无水平溢出，整页axe检查通过。 |

### 可重复运行、体验和停止

先在本分支仓库根目录使用现有隔离启动器；需要Node/pnpm、Go、Docker和可用的本地PostgreSQL镜像。它创建任务独占临时容器/数据库及动态回环端口，30分钟上限，不读取共享历史库。

```powershell
pnpm.cmd --dir web/listingkit-ui install --frozen-lockfile
node web/listingkit-ui/scripts/shein-diagnostic-fixture.mjs --serve
```

保持该终端运行。在第二个终端进入 `web/listingkit-ui`，只传入本次终端输出的manifest文件路径，不复制或公开其中的session/cookie：

```powershell
$env:ISSUE331_FIXTURE_MANIFEST = '本次 fixture.json 的绝对路径'
pnpm.cmd exec playwright test --config playwright.console.config.ts
# 逐步体验：Inspector逐步执行，浏览器从实际工作台经导航到Chat和Store。
pnpm.cmd exec playwright test --config playwright.console.config.ts --grep 'single Shell.*1440px' --headed --debug
# 既有诊断完整回归：真实刷新/摘要/动作/企业隔离及明确标注的故障注入。
$env:ISSUE324_FIXTURE_MANIFEST = $env:ISSUE331_FIXTURE_MANIFEST
pnpm.cmd exec playwright test --config playwright.shein-diagnostic.config.ts
```

浏览器由测试安全注入本次合成外部身份session，经现有Auth.js解密和页面门禁。未注入session的普通浏览器会进入原有登录；不添加登录绕过。可访问地址以本次manifest的origin为准，不使用历史端口。诊断仅在本片用内部深链做模板回归，不声称完成#328从入口发现资料的验收。

**数据层级**：概览和Chat没有业务请求；Store截图中的“隔离视觉验证店铺”为测试文件显式拦截的HTTP合同fixture，不是Store真实Go验收。诊断则是实际页面→BFF→Go→隔离PG；迟到响应测试只延迟真实BFF响应。外部身份签发和授权源为受控替身，真实ZITADEL登录及客户生产数据NOT_RUN。现有Store/provider/权限/缓存/撤权行为另由适用组件、真实client与BFF测试回归，不能把截图作为后端验收。

体验结束，在第二终端写停止信号并等待第一个终端正常退出：

```powershell
$run = Get-Content -LiteralPath $env:ISSUE331_FIXTURE_MANIFEST -Raw | ConvertFrom-Json
New-Item -ItemType File -Path (Join-Path $run.controlDirectory 'stop-fixture') -Force | Out-Null
# launcher退出后：Go日志尾部应有PASS；本次container应不存在。
Get-Content -LiteralPath (Join-Path $run.controlDirectory 'go.log') -Tail 2
docker ps -a --filter "id=$($run.containerId)" --format '{{.ID}}'
```

launcher停止自身Next/Go并移除自己的容器；Go核对业务表内容和行版本未变化。只清理本次资源，不打印完整manifest、不删除其他容器。体验反馈按“具体操作 → 观察结果 → 期望结果”记录，未经确认不增加业务功能。

### 自动验证与复核

`pnpm.cmd lint`、`pnpm.cmd typecheck`、`pnpm.cmd test`、`pnpm.cmd build`与默认`pnpm.cmd test:e2e`分别运行。新增静态导航/页面组合测试，既有Shell、Store、诊断及provider测试保持执行；受控浏览器单独配置，不因CI没有fixture就访问生产或偷偷换mock。

新增浏览器覆盖1440/390跨页面、三层选中/面包屑、浏览器历史、Escape两种焦点起点、主题持久化、axe、真实200→100→200及迟到响应。独立Reviewer代码与实际图片复核已发现并促成Escape和Chat强调色修复；最终SHA/CI/复核结论只记录PR，不让旧SHA证据冒充新HEAD。

后续模块只能消费上述静态导航、ConsolePage/Toolbar/State及既有primitives，不另起Shell或复制颜色。尚未模板接入的Store新建/详情、无企业说明及非工作区旧页面不宣称全面改版；#328仍等待入口归属决策，最终消费#327共享类型而非复制schema/client。
