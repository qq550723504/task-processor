# 任务中心：本地资料准备工作记录

Refs #328 / PR #330；当前产品映射以 #328 正文、R328-C1（issuecomment-5558848270）及 #340 C340-H1 为准，替代旧独立 SHEIN 列表入口。#331 与 #329 已合入 main。本片已组合 #340 实际提交 503703a3ceef6e1fd5e3f8dcd0864add60fc6360；其 PR #341 未合入时，仍是最终 main 组合前置。

## 已实现的用户路径

现有 AI工作台 → 任务中心 `/workbench/ai/tasks` → 已完成 `/workbench/ai/tasks/completed` → 选择真实返回的工作记录 → 右侧工作结果“查看诊断” → 既有诊断 → “返回任务中心”。返回后重新读取当前授权范围，不复用上次列表作为授权凭据。

两个页面复用 ConsolePage、ConsoleToolbar、ConsoleState、Shell、设计变量及企业 provider。任务中心与已完成项是实际连接页面；其余任务生命周期保持未接入。未配置现有受控来源时明确“暂未启用”，没有投影请求或运行时 fixture 回退。旧 `/workbench/shein-records` 独立入口继续404，诊断深链仍有效，不恢复旧首页卡片。

#340 拥有并实际提供 `completed-work.ts` 的共享 DTO/严格解析与 `completed-work-client.ts` 的 fetchCompletedWork / typed error，以及同源 projection BFF。UI只消费实际模块，不复制 wire schema、client、source→work判断；测试中的合成 DTO 显式标注，仅用于组件/错误测试。

## Frame → 路由 / 组件 → 处置

本轮先加载 figma-design-to-code，再实际读取 design context 和截图。Figma文件 `tg48P46SSXl6TBy9lZwg63`、页面 `31:463`，未修改原稿。

| Frame | 映射 | 保留 / 差异 |
| --- | --- | --- |
| [427:3005 任务中心](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=427-3005) | TaskCenterLayout / CompletedWorkResults / WorkResultDetail；两条现有任务中心路径 | 保留概览、状态切换、列表与右侧详情层级；仅显示真实本地资料准备完成记录 |
| [429:5323 店铺商品](https://www.figma.com/design/tg48P46SSXl6TBy9lZwg63?node-id=429-5323) | 本片不挂载店铺商品 | 原稿仅展示已发布商品；没有 Store 关联则通用业务，不猜店铺归属 |
| 已验收 #331 框架 | Console Shell / tokens / Page-Toolbar-State / 企业上下文 | 同步 main 保留 ConsoleOverview；任务中心导航最小接线，不重做导航/登录/主题 |

1440×900 原稿：240px侧栏、72px顶栏、内容左右32px、概览88px、状态按钮38px、列表758px与详情356px、间隔22px；Noto Sans SC。局部 CSS module 仅消费现有 Console tokens。主题、字体栈和对比度沿共享框架，不建立第二套变量。

| 对照区域 | 当前结果 / 分类 |
| --- | --- |
| Shell、导航、品牌、主题 | 复用已验收框架，任务中心/已完成高亮和面包屑来自现有登记；诊断仍是已有内部深链，不伪称对应新Figma终稿 |
| 概览与状态行 | 三个概览统计明确未接入；通用业务卡无虚构店铺。ConsoleToolbar容器、刷新和coverage说明使数据区起点低于原稿，属于必要模板/范围提示适配 |
| 列表与右侧详情 | 758/356比例、22间距；每页最多20条，列表滚动区域432px，避免批量记录使详情难以抵达。真实metadata、固定完成含义和重新授权入口替代原稿的假进度/AI建议。右侧内容比示例长，完整结果另附区域截图 |
| 390px | 工程响应式：概览两列、状态/操作折行、列表和详情纵向排列；选择后焦点进入详情，返回记录列表恢复原行焦点；不是Figma移动稿 |
| 未接入业务 | 运行中/待确认/暂停/全局统计/今日完成数/Chat/搜索/店铺筛选/AI建议明确不可用；不显示0、假进度或假成功 |

## 设计与运行截图

实际数据图绑定运行源码 `207282dfdc9a078de89a5c8893df1cc40cb9fc81`。后续若仅提交文档/PNG，不改变运行源码；最终HEAD及源码树关系在PR记录。参考图含Figma示例数字，不是实际经营数据。运行图为真实BFF→Go→隔离PG返回的本地记录。

| 参考 | 实际 |
| --- | --- |
| [任务中心原稿1440](evidence/issue328/r328/figma-task-center.png) | [数据双栏1440×900](evidence/issue328/r328/completed-1440.png)、[完整右侧结果](evidence/issue328/r328/result-detail-1440.png) |
| [店铺商品边界原稿](evidence/issue328/r328/figma-store-products.png) | 本片不接店铺商品、不制造发布状态 |
| 390无移动终稿 | [390响应式完整页面](evidence/issue328/r328/completed-390.png) |
| 共享浅色变量 | [浅色1440](evidence/issue328/r328/completed-light-1440.png) |
| 实际空范围 / 错误 | [企业100空集合](evidence/issue328/r328/empty-1440.png)、[撤权](evidence/issue328/r328/actual-revoked-1440.png)、[权限不足](evidence/issue328/r328/actual-store-1440.png)、[超时](evidence/issue328/r328/actual-slow-1440.png)、[依赖故障](evidence/issue328/r328/actual-unavailable-1440.png) |

[未启用1440](evidence/issue328/r328/unavailable-1440.png)、[未启用390](evidence/issue328/r328/unavailable-390.png)来自前一布局源码 e370d86f3b9ed33602198c0b98beb335c68a9717，仅证明当时未接线页面。当前无配置仍消费相同未启用布局，并有组件测试；不把旧截图当当前投影联调证据。

## 产品语义与隔离

- coverage仅为本地资料准备完成记录，不是企业全部任务；实际空集合也不证明其他业务为空。
- “完成”仅指本地资料创建已提交，不代表诊断通过或可发布。source_record_id不是task_id，无AgentRun、通用任务状态机、Store关系或第二事实源。
- 使用服务器opaque cursor，刷新回首屏，不自动遍历；历史位置最多50个。快照版本保持精确字符串，不逐行请求Package或诊断。
- 身份/企业/角色变化、切换中、退出/撤权销毁请求子树；分页/刷新使用新请求序号，取消旧请求并清空选择。A→移除A→再次A不会复活原选择。
- gcTime/staleTime为0，不自动后台重试或窗口刷新；失败与成功空集合分开，原始错误不透出。上下文恢复只在同一user/org/roles恢复成功后重读，迟到恢复不能启动旧scope。
- 诊断链接由#340严格绑定同一source UUID；打开时重新认证授权。记录出现在列表不授予未来访问权限；实际移除Listing权限后诊断403已验证。

Legacy decision: RETIRE
Reusable behavior: 原metadata精度、opaque cursor分页/刷新、取消/迟到响应与同scope错误恢复。
Current owner: 任务中心内容/请求子树消费#340只读投影；领域事实与授权仍归Listing/现有provider。
Cutover/deletion condition: 已接新任务中心，旧独立入口保持404；无消费者的旧列表组件、三个专用测试文件、测试fixture及入口测试的无效mock已删除。分页有界、卸载取消和迟到响应等有效行为由任务中心测试覆盖。#327/#340的API/schema/client/共享fixture不变；历史提交和截图保留为checkpoint。

## 验证证据

- 后续评审清理仅删除未挂载旧列表/专用测试fixture、补任务中心行为测试及文档；正式运行路径的组件和样式不变，以上设计截图仍适用。清理前后任务中心与入口边界测试均通过；清理后9文件68项测试、类型/lint/build及9项真实任务中心浏览器、2项旧入口404回归通过。本次隔离Go只读断言76.27s PASS，启动器正常退出0并删除自有容器；未重新运行的原诊断全套浏览器沿用前一源码证据，不冒充本轮重跑。
- 5项布局/选择TDD先红后绿；独立复核发现选择复活后补失败断言并修复。
- 16项实际client接线TDD先红后绿；补共享decoder负例、实际provider/切换器、上下文恢复测试。
- 当前关联14文件107项测试通过；涵盖shared provider/Shell/组织切换/诊断/新页面。局部eslint、typecheck、production build通过。
- 新页面9项实际浏览器验证：1440/390导航→投影→选择→诊断重新授权→返回；前进/后退；分页/刷新；200→100→200；延迟实际HTTP响应；Go撤权/Store-only/超时/不可用；选中后移除权限再打开诊断403。
- 页面axe（WCAG2/2.1 A/AA）无违规，无横向溢出；390 Escape关闭菜单并归还焦点。
- 原诊断9项浏览器回归（真实正常链与显式合成故障分开）及旧入口关闭2项通过。
- #340创建fixture使用真实Publisher/POST提交及同操作重放；本页面读其真实授权投影。外部身份与Auth.js会话签发为显式替身，Go/PG和BFF不是mock。真实ZITADEL、客户/生产环境、真实业务数据 NOT_RUN。
- 最终CI/独立复核绑定最终完整SHA，写PR。#341尚未合入时，不能声称已完成main组合验收。

## 可重复启动、访问与停止

在包含本片与#340实际提交的checkout运行，需要已有Node/pnpm、Go、Docker和本地PostgreSQL镜像；不读取共享历史库。

```powershell
node web/listingkit-ui/scripts/shein-diagnostic-fixture.mjs --serve --completed-work
```

使用本次输出的manifest路径，第二个终端进入 `web/listingkit-ui`：

```powershell
$env:ISSUE328_FIXTURE_MANIFEST = '<本次fixture.json绝对路径>'
npm.cmd exec -- playwright test --config playwright.task-center.config.ts
```

脚本以合成会话从工作台点击现有任务中心/已完成，再选择服务返回记录；不会手工拼接fixture UUID直达诊断代替流程。需要本机手动体验可在同一命令附 `--debug --grep 1440px` 打开Playwright Inspector与受控浏览器，按步骤执行；不要复制/公开session或cookie。manifest中的origin仅在这次启动器运行期间有效，本文不提供已停止地址。

正常清理：

```powershell
$fixtureInfo = Get-Content $env:ISSUE328_FIXTURE_MANIFEST -Raw | ConvertFrom-Json
New-Item -ItemType File -Path (Join-Path $fixtureInfo.controlDirectory 'stop-fixture') | Out-Null
```

等待启动器退出0，确认Go内容/xmin只读断言通过、自有容器已删除。所有本轮资源均正常停止；manifest、session、cookie及敏感trace不提交。最终清理耗时和证据见PR。

## 历史检查点

旧7c6cf76的工作台卡片→独立列表流程和原图片均已撤回；a085c784修正继续保留。#331框架已交付，不能再引用原Paused描述作为当前状态。本片未合并、关单、部署或操作真实账号/业务数据；父Issue保持open。
