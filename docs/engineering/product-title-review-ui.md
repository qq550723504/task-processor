# 任务中心标题提案待确认（#345）

Authority: Issue #345 当前正文及 N1-C 启动评论、#343/#344、#36、#298、
`docs/product/final-ui-ia-authority.md`。同任务窄准入第一轮独立只读
review_ui_admission：纯展示/布局/无 wire 依赖交互 IMPLEMENTATION_READY。
无架构 BLOCKER；pending 布局模式、未知写结果、写回调生命周期及合同依赖归为
IMPLEMENTATION_TEST，随实现验证收敛。#344 交接前不做依赖字段的接线。

## 边界与产品映射

仅 `/workbench/ai/tasks/pending` 投影真实 Product Review pending/accepted。
保留统一 Console、TaskCenterLayout、Card/Button、语义 token 与已有企业 provider。
不创建 BusinessTask，不改 Product 状态机、授权、事务、schema、client 或 BFF。
预计 12–20 个范围文件、少于 1,000 行生产 TS/CSS；达到根 AGENTS 阈值重新拆片。
浏览器状态与已存在的持久写边界构成架构敏感修改，本文件限定 UI 消费合同。

Must：精确版本/真实 diff/evidence/history/receipt，人工接受/编辑/拒绝，
接受后独立确认 Apply，编辑撤销旧批准，不覆盖冲突，不自动重放，作用域隔离。
Should：长文本、键盘焦点、390 折列和可读错误。
Out of Scope：生成入口、模型调用、图片/其它字段、发布/同步 Listing、第二恢复引擎、
生产/IAM 验收、相邻 owner 的实现。沿 #333 已接受同人提案和审批、CachedRead +
LiveWrite；不证明任意自然语言真实性，不扩大 Threat Model。

## Figma → 组件

2026-09-06 UTC 已完整加载 figma-design-to-code，并调用
`get_design_context(fileKey=tg48P46SSXl6TBy9lZwg63,nodeId=427:3005,skillNames=figma-design-to-code)`。
返回 React/Tailwind 参考代码、上下文和截图；随后 get_screenshot 获取原生 1440×900。
主设计依据为 design context，截图用于核对；Figma 未修改。

| Frame / 节点 | 消费与批准增量 |
| --- | --- |
| 427:3005 / 427:3006 | 既有 Console Shell、240 侧栏/72 顶栏，不复制 Logo/导航 |
| 560:425 左列、564:363 右侧详情 | 复用 task-center.module.css 的 758/356 双栏、22 间距、14 圆角 |
| 564:372 等待你确认 | pending=待审核；accepted=已接受，待应用；不使用原稿假进度 |
| 564:376 建议区 | 真实标题 before/after 与原提案/依据/质量/未解决问题 |
| 573:389 待确认入口 | 连接已登记路由；不新增顶层审核中心 |

标题 diff、编辑和确认对话框是 #345 批准的交互补充，不能声称原稿已画全。
不使用原稿示例任务、统计、通知、Store 关系。390 是工程响应式，不是移动终稿。
本片复用已有组件/token，无新增图标需求；若后续使用导出图标必须保存确切原资产字节。

## UI 消费与失败矩阵

权威 owner：Product Review 管理 proposal/revision/decisions/receipt；Catalog 唯一商品
writer，原 UoW 保证 Apply 原子性；#344 管理全部 wire/严格解码/HTTP 与错误语义。
UI 没有数据库事务或本地持久账本。直接消费 #344 已推送提交
`c95b27679bd4cd2c1e4aed06da0290fd05586273` 的
`src/lib/api/product-title-review.ts`、`product-title-review-client.ts`。
通过正常 merge 消费依赖，不复制 DTO/client；Go 合同来自 #343
`3d59d669c5eda511c0724f66042bb54c2803be06`。组件样例只用于交互测试，真实
浏览器运行使用 #344 唯一 `product-title-review-fixture.mjs`。

| 事件/前置条件 | 浏览器效果 | 权威副作用及验证 |
| --- | --- | --- |
| 有效 scope 初次/手动刷新 | GET actionable 与所选详情，缓存不跨 scope，版本保留字符串 | GET 不写；实际 BFF/PG 验证 |
| pending 点击接受/拒绝 | 固定本次 key/proposal/exact revision/payload；同步锁定重复点击 | 仅一次 typed client POST；按后端新 revision 更新，不自行加一 |
| 编辑 pending/accepted | 进入编辑即隐藏 Apply；保存后等待后端 pending 新 revision | 不继承旧批准，不把保存当接受；红绿组件测试 |
| accepted 点击 Apply | 对话框固定当前 revision、商品和基线，仅标题；第二次用户确认才 POST | 只使用本次 accepted revision；Catalog 新版本与 receipt 来自返回 |
| exact revision/base 冲突 | 清掉可操作旧详情，提示重新读取，不换 base/强行覆盖 | 冲突没有自动写；组件/真实链验证 |
| POST 后超时/取消/非法响应 | 结果待核实，保留同一意图；只允许显式核实 | 不声称回滚；同 key 原 payload 核实依 #344，无 effect/retry POST |
| 不确定操作期间 GET 当前结果 | 展示权威状态/receipt，不从缺少 receipt 推断未提交 | 不自动生成新 key 或重试 |
| 已应用/拒绝 | 详情可读终态，动作停止，重新 GET 集合排除终态 | 不改原 Listing/平台状态；明确本次未操作平台 |
| 切企业/用户/角色、退出、撤权、卸载 | scope key 重挂载；清列表/选择/编辑/意图/敏感结果，abort 等待 | 迟到响应检查 mounted/generation，不流入新 scope；取消不撤销提交 |
| 403/上下文失败 | 隐藏敏感数据，显式恢复上下文后重新授权读取 | 不依据旧 admin/成功结果恢复权限 |
| 集合 GET 与 POST 交错失败 | 普通集合错误只替换左列；保留独立详情及同 key 核实意图 | 权限拒绝则清空整个投影并释放页面锁；延迟集合失败测试 |
| 未配置/空/网络/非法响应 | 分别显示 unavailable/empty/error，安全固定文案 | 无假数据或原始错误；不渲染任意 evidence 链接 |

每个用户写意图只存于当前 scope 的内存；rerender/focus/reconnect/retry/effect 不 POST。
异步锁防双击，关闭/取消未知写操作不解释为业务撤销。刷新/重启不恢复旧 POST，重新
授权读取。原 Go 10s 与 #344 总 deadline/body/响应字节上限原样复用，不新增重试 owner。
分页沿 #343 opaque cursor/20 默认/UUID ASC，不声称时间序、全局数量或跨页快照。
URL proposal_id 只定位，实际 GET 授权；合法终态可展示，不可读明确错误。

## 验证与交付

TDD 先失败：pending 入口/coverage、列表与详情、accept 不 Apply、edit 失效、
exact revision/冲突、双击、未知写显式核实、scope/撤权/卸载迟到响应。
定向组件与导航/原已完成/诊断/provider 回归，typecheck/lint/build。
真实浏览器从任务中心进入，使用 #344 专属 fixture 经实际 BFF→Go→隔离 PG，提案
经真实 Publisher/Proposer/POST 创建；外部身份/生成器替身明确。记录非 title 与原 Listing
不变、重建后新版本/receipt。1440 对照、390 长文本、焦点/dialog/axe 证据。
最终 SHA 的 CI/独立完整切片复核在 PR 维护；SKIP/NOT_RUN 不等于 PASS。

真实浏览器脚本分别执行主流程及交错场景：`scripts/product-title-review-ui.mjs`、
`scripts/product-title-review-ui-lifecycle.mjs`。后者仅延迟交付实际 BFF 响应以制造
取消/跨 scope/卸载交错，不替换响应 body，也不绕过实际 Go 授权和提交。
`restart` 控制是同一个 Go 进程内重建 application，不能称为进程重启。

浏览器验收捕获的对比度问题来自把背景色 token 当成文字颜色；状态文字使用
`--warning-foreground` / `--success-foreground`，保持现有主题定义。原生 dialog
允许浏览器 chrome 获得焦点，页面背景控件保持 inert；验证 Tab 回到 modal、
Escape 不发 POST、关闭后恢复触发按钮焦点。

## 本地复现

需要包含上述两个依赖的 checkout、Node/Go/Docker，先在 `web/listingkit-ui`
执行 `pnpm install --frozen-lockfile`。启动独立服务：

```text
node scripts/product-title-review-fixture.mjs --serve
```

使用启动器打印的私有 manifest 路径，在另一个终端执行（仍位于同一前端目录）：

```text
node scripts/product-title-review-ui.mjs <private-fixture.json> <evidence-directory>
node scripts/product-title-review-ui-lifecycle.mjs <private-fixture.json> <evidence-directory>
```

必须新建 fixture 才可完整重跑主流程；脚本会真实改变隔离提案状态。生命周期脚本
应在主流程之后运行，使用同一个隔离数据库验证原已完成/诊断仍可读。
正常停止是在 manifest 的 `controlDirectory` 新建空文件 `stop-fixture`，等待启动器
退出 0 并检查 `cleanup.json` 的 `portReleased`、`goExit` 与 `containerStopped`。
不要上传 manifest/session/cookie/raw trace。仅提交脱敏观察、截图和精确源码版本证据；
最终运行/CI/独立复核绑定的 SHA 在 PR 维护。

证据目录中的旧 `regression-completed-*` 是历史只读回归；Product 主链与生命周期
报告及 `regression-diagnostic-after-apply-*` 才来自本片真实组合。外部身份发行、
grant provider、CandidateGenerator 是明确的受控替身，真实 IAM/生产/付费模型不在验收范围。
Legacy decision: N/A，复用当前 Console owner；不消费 RETIRE 的 Task-first/Workspace。
