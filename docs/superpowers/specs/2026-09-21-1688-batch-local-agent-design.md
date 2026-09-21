# 1688 批量导入（本地执行器驱动既有浏览器采集链路）设计

- 状态：**待评审 v3**（未批准前不进入实现）
- 基准：`origin/main` = `14df0e520`（含 #444 的 2048）
- 相关 Issue：#398、#399、#396（见 §12）
- v3 变更：用户 2026-09-21 选定**路线 B**，设计整体重写；v2 的「服务端预建待办行 + 过期重认领当心跳」方案**已废弃**，原因见 §2 与 §15

## 1. 目标与范围

### 1.1 使用者与场景

需要在**不逐个打开 1688 页面手工点击采集**的前提下导入多个商品的操作人员：把一批 1688 商品链接交给本地执行器，由它在后台依次驱动浏览器完成采集，用户不必停在任何一个页面上。

### 1.2 本次可见交付

```
本地执行器：给入一批 1688 链接（≤10）
  → 执行器无人值守依次：打开商品页 → 复用已上线的插件采集 → 交付到采集页
     → **由执行器代用户点击「Confirm and submit」**（§4 D1.2 的四条护栏）
  → 每条结果写入本地队列文件（执行器侧可见进度）
  → 成功发布的商品逐个出现在应用的商品目录中
```

**人工只在两处介入**（用户 2026-09-21 决定，§1.6）：

1. **登录**：1688 用二维码、应用用正常登录；执行器复用已登录 profile，不持有凭据。
2. **遇到验证码时**：整批暂停并提示，由用户处理后继续；**执行器不尝试识别或绕过验证码**（§4 D1.3）。

**明确不含**：

- 应用内的「待办列表 / 批量进度页」。待办进度只在**执行器本地**可见（§4 D2）。
- 无人值守**登录**与无人值守**验证码处理**（这两件仍由人做，见上面两点）。

### 1.3 本次不做

- 不做定时/周期性重采（只做一次性批量）
- 不做多账号池、代理池、账号轮换
- **不新增任何服务端路由**（与 v2 的关键差别：v2 的 D5 list 路由已撤销，见 §4 D4）
- **不新增服务端待办行**；服务端不持久化 profile 引用或任何 1688 凭据
- 不改 ③（插件采集）的任何现有行为
- 不恢复 ①（服务端匿名裸 HTTP）—— 已证结构性不可行
- 不引入后台调度器、TTL 或 key GC（§1.5 第 2 条）
- **不自动识别或绕过验证码**（遇到验证码即整批暂停、交用户处理，§4 D1.3）
- 不代替用户完成 1688 或应用的登录（只复用已登录 profile）
- **不以 `404` 作为「可重新提交」的依据**（与该键下「不证明失败」的既有语义可能冲突，见 §4 D2）
- 不做 `connectionStatus` 状态机扩展（#396 保持 PAUSED，见 §12）
- 不处理 `internal/localagent` 的去留（本设计不依赖它；去留按独立任务，见 §13）

### 1.4 前置实测结论（本设计的依据）

在 fingerprint-chromium 144 + 已登录 profile 上的实测（同一 IP 已被匿名请求污染的前提下）：

| 配置 | 次数 | 成功 |
|---|---|---|
| 匿名非持久化（早期时点） | 13 | 6 |
| 匿名非持久化（污染后同时点） | 8 | 0 |
| 匿名 + 全新持久化 profile | 3 | 0 |
| 匿名 + 已过验证码的设备态 profile | 2 | 0 |
| **登录 profile** | **5** | **5** |

结论：**匿名路径不能作为产品前提**；**登录 profile 在同样被污染的 IP 上 5/5 成功**。

三个附带事实：

- Chrome 153 profile 交给 Chromium 144 会**启动即崩**，元凶是 `Default/Sync Data` ⇒ **profile 必须由最终运行的那个浏览器创建**，不能后期迁移。
- 1688 路径的 `SetFingerprint` **从未被调用**（只有 amazon/shein/sds 登录调用）⇒ 1688 的浏览器身份 = 二进制 + profile，登录与自动化天然同身份。
- 单次真实抓取实测 **34.745 秒**。这个数字是 v2 设计崩掉的直接原因（30 秒租约 + `Prepare` 要求租约未过期），也是本路线**完全绕开租约**的理由。

### 1.5 已记录的既有设计决定（必须遵守）

`docs/engineering/src2b-public-acquisition.md`（`src2b-acquisition-v1`，#398）已经记录：

1. > This is anonymous public acquisition, **not Browser Capture, Source Account management, a Connection, or a login flow.**

   ⇒ 本设计是登录流程，**不属于该 contract**，必须作为**独立 producer** 显式准入，不能挂进 `public_acquisition/v1`。**本设计不新增 producer**：它原样复用 ③ 已准入的 `browser_acquisition` producer。

2. > Only expired acquiring operations without a prepared command may obtain a new fenced **GET** lease. **There is no background scheduler, fallback, rebase, TTL or key GC.**

   ⇒ `acquiring` 租约是**服务端 GET 租约**（8 秒 GET / 20 秒操作 deadline / 30 秒租约），**不是**给浏览器长任务用的。
   ⇒ 不得新增租约续期 API、后台调度器、TTL 或 key GC。**本设计完全不使用 `acquiring` 租约**（§4 D2），因此与这条决定零冲突——这是相对 v2 的实质改进。

3. > Limits: retained operations per organization, 32 active operations, ... 30-second GET lease.

   ⇒ 保留上限是**按组织、无 GC 的终身容量**，且与 ①/③ 共用；已由 #444 从 256 提到 **2048**（§9.1）。

### 1.6 本条产品决定（用户 2026-09-21，决策 ID `PD-1688-BATCH-VERIFY-ON-CAPTCHA-2026-09-21`）

> **批量提交不需要逐条人工确认；只有遇到验证码时才需要用户处理。**

其影响与边界：

- 执行器**可以代为触发提交**（§4 D1.2），批量因而是无人值守的。
- 人工介入只剩**登录**与**验证码**两处（§4 D1.3）。
- 这是一次**语义变更**：③ 契约里「逐条取得显式确认」变为「批量启动时一次确认 + 执行器逐条代触发」。应用侧代码路径不变（执行器点的是真按钮），但该 README 的措辞需要由 owner 同步更新，见 §12 第 2 条。
- **不自动放行验证码识别或绕过**：用户明确说「遇到验证码让用户处理」，因此执行器的职责是**检测并停下来交人**，不是自己解决。
- 本决定**不降低**任何一条安全护栏：§4 D1.2 的四条护栏（尤其 scope 固定）是把它做成安全实现的前提，不是可选项。

## 2. 为什么是路线 B（决定记录）

PR #443 第三轮评审提出 13 条，逐条对照真实代码核实后确认：v2 试图**把采集操作行当成待办队列**，而该表不是为队列设计的（详见 §15）。其中一条直接证伪了 v2 的核心机制。

因此列出三条路线交用户决定，用户于 2026-09-21 选定 **(B)**：

| | 内容 | 结论 |
|---|---|---|
| (A) | 为批量引入独立持久化队列 owner（`BatchJob`） | **否**：会引入第二事实源，需独立架构设计与评审；按「产品先行」当前投入不成立 |
| **(B)** | **执行器本地队列 + 复用既有 ③ 链路** | **选定** |
| (C) | 只交付已上线的 ③，批量等真实用户被阻塞再投入 | 否：忽略用户「要批量/后台导」的真实需求 |

**(B) 的关键性质：它不需要采集契约上的任何变更，也不需要新增服务端路由。**

原因：③ 已经是一条完整、已上线、已端到端验证的链路（扩展取 DOM → 采集页提交 → `StartPrepared` → 发布）。批量缺的**只是「谁来点 N 次」**。执行器驱动浏览器去点，就用不上 v2 引入的每一个新东西：

| v2 需要 | 路线 B |
|---|---|
| 服务端预建 `acquiring` 待办行 | **不需要**（抓完才由 ③ 建 `prepared` 行） |
| 发现谓词 + 生产者区分符 | **不需要**（没有待办行可发现） |
| 过期重认领当 fence 心跳 | **不需要**（无 `acquiring` 阶段，无 30 秒租约约束） |
| list 路由 + 分页 + 稳定排序位 | **不需要**（不做应用内待办视图） |
| `failed → acquiring` 重试迁移 | **不需要**（③ 每次采集用新 key，重试天然可行） |
| actor 委派语义 | **不需要**（提交由浏览器里的用户会话完成，actor 天然是本人） |
| 设备令牌 / 新认证路径 | **不需要**（见下） |

> **对先前推荐的修正**：我最初的 (B) 描述里写了「需给执行器一条能提交证据的认证路径（设备令牌）」。核实后这是**不必要的**，而且那条路更差：`internal/localagent/deviceauth` **故意拒绝 refresh token**（`client.go:133`，测试 `TestAuthorizeRejectsRefreshToken`），设备流拿到的是短时令牌，长批量必然中断。正确做法是**执行器不持有任何凭据**——它只驱动浏览器，提交由浏览器中的用户会话（BFF 代理附 `Authorization: Bearer`，`web/listingkit-ui/src/lib/server/workbench-proxy.ts:424`）完成。**凭据面为零新增。**

## 3. 现状盘点（在 `14df0e520` 上复核）

### 3.1 直接复用（不要重建）

| 能力 | 位置 | 复用方式 |
|---|---|---|
| 扩展取数（`1688-browser-dom/v2`） | `extensions/1688-capture/`（已上线） | **原样复用**，不重写解析 |
| 采集提交与发布 | `internal/app/httpapi/browser_capture_application.go`（4 条路由，已上线） | **原样复用**，不改 |
| 采集页 → 服务端提交 + 用户会话 | `web/listingkit-ui/src/app/capture/1688/capture-receiver.tsx`、`workbench-proxy.ts:309-315` | **原样复用** |
| 触发采集的扩展内部消息 | `extensions/1688-capture/src/background.ts:32`（`popup.capture`） | 执行器发同一条消息，绕开「必须聚焦窗口才能开 popup」的限制 |
| CDP 加载扩展 + 驱动 popup | `extensions/1688-capture/scripts/browser-smoke.mjs:78-101`（`Extensions.loadUnpacked` + `Runtime.evaluate{userGesture:true}`） | 作为驱动循环的**已证模板**（Node 参考实现） |
| Go 侧 CDP 能力 | `github.com/mxschmitt/playwright-go`：`Browser.NewBrowserCDPSession()`（`browser.go:125`）、`Context.NewCDPSession`（`browser_context.go:88`）；已有用法 `internal/sheinlogin/automation_network.go:52` | 执行器用 Go 实现，不新增运行时 |
| fingerprint-chromium 安装 | `cmd/fingerprint-browser-installer` | 直接复用 |

### 3.2 缺口（本设计要补的全部）

| 缺口 | 证据 | 本设计的补法 |
|---|---|---|
| **没有「驱动 N 次」的循环** | 采集一直是人工点 popup | §4 D1：执行器循环 |
| **既有链路是「单次人工点击」的有状态流程，不是无状态 API** | `Controller` 是 background 模块级单例；`capture()`/`handoff()` 在已完成后直接返回（`controller.ts:18/:24`），只有 `popup.new`（`background.ts:34`）重置 | §4 D1.1：每条之间的 reset 转换 |
| **提交动作需要人触发，而执行器要无人值守** | 契约 `README.md:124-128` 要求展示已核实身份/组织并取得显式确认；唯一提交入口是用户手点按钮 `capture-receiver.tsx:145` | §4 D1.2：执行器代为触发**真按钮**，用四条护栏（尤其 scope 固定）替代逐条人工确认（用户决定，§1.6） |
| **验证码/登录失效会中断采集** | 实测：风控下找不到商品 DOM；登录态会过期 | §4 D1.3：检测⇒整批暂停⇒交用户⇒原条目重做（**不**尝试识别/绕过验证码） |
| **没有本地待办队列** | 无 | §4 D2：执行器本地文件（**不是**服务端事实源） |
| **profile / 会话被并发使用** | 无锁 | §4 D3：profile 本地锁，单执行器 |

### 3.3 与 legacy 的关系

本路线**不依赖、不包装、不抽取** `internal/crawler/alibaba1688` 的任何代码：不调 `ProcessWithAccountProfile`、不调 `Process`、不引入 public→account 回退。浏览器由执行器直接以 profile 启动。

## 4. 核心设计决策

### D1：批量 = 执行器循环调用 N 次既有 ③ 采集

执行器对每条链接执行一次**与人工操作等价的** ③ 采集：导航到商品页 → 触发扩展采集 → 打开采集页（携带 handoff 参数）→ **用户确认并提交** → 等待终态。

- 不新增服务端实体、不新增状态机、不新增路由。
- 部分失败天然成立：一条失败不影响后续条。
- 单批上限 **10**（§11-1），实测通过后再提。

#### D1.1 每条之间的 reset（必须显式实现）

`Controller` 是 background 的**模块级单例**（`extensions/1688-capture/src/background.ts:25`），且：

- `capture()` 在已有 payload 时**直接返回**（`src/controller.ts:18`：`if (this.payload) return Promise.resolve()`）
- `handoff()` 在已交付时**直接返回**（`src/controller.ts:24`：`if (this.handed) return Promise.resolve()`）

⇒ 只循环发 `popup.capture` 会在第 2 条**重复第 1 条的结果**，批量根本不工作。唯一的 reset 是 `popup.new`（`background.ts:34`）。

⇒ 每条之间的顺序必须是：

```
读状态 → 采集 → 交付 → 等待终态被本地确认
  → 导航到第 N+1 条商品页 → reset → 下一轮采集
```

**边界论证**：`popup.new` 在 UI 上是**两步确认**（`fresh` → `new-confirm-button`，`src/popup.ts:39-41`），边界语为「新采集会建立另一笔操作。已交给应用的操作请先在原页面核实。」它要防的是**用户误触、丢掉一笔尚未核实的操作**。批量中该前提由**执行器先确认上一条已到终态**替代 ⇒ 发 reset 时边界条件已被**显式满足**，不是绕过。

**实现方式**：执行器**驱动 popup 页面模拟真实点击**，而不是直接发 `popup.new` 消息。理由：`background.ts:28` 只接受来源为 `popup.html` 且无 `tab` 的消息，所以两种方式都必须驱动 popup；模拟点击不依赖内部消息字符串契约，也更贴合「这是用户操作」的语义。

#### D1.2 提交动作由执行器代为触发（**用户 2026-09-21 决定**，含四条护栏）

用户决定「只有遇到验证码时才需要人处理」（§1.6），因此执行器必须**自行触发提交**，批量才是无人值守的。本设计据此把「提交」这个动作交给执行器，同时**用护栏替代原来由人逐条承担的那部分保证**。

**先说清楚这改变了什么。** 已准入的 ③ 契约写的是：

> Before any POST, the app must replace the initial fragment with `#operationKey=<original UUID>`, **show the currently verified user and Effective Organization, and obtain explicit confirmation** using the existing BFF context-drift protections.
> —— `extensions/1688-capture/README.md:124-128`

拆开看，其中三件事**完全不变**（因为执行器点的是真按钮，应用代码路径一行未改）：

| 契约要求 | 谁执行 | 是否改变 |
|---|---|---|
| 把 fragment 换成 `#operationKey=<original UUID>` | 应用（`capture-receiver.tsx:62`） | **不变** |
| 展示当前已核实用户与 Effective Organization | 应用（`capture-receiver.tsx:139-140`） | **不变** |
| 用既有 BFF 上下文漂移保护取得确认 | 应用（`capture-receiver.tsx:70-79`、`110-119`；唯一入口 `:145`） | **不变** |

**真正改变的是「显式确认由谁作出」**：从「用户逐条点击」变为「用户在批量启动时一次性确认，执行器按其意图逐条代为触发」。这是本设计里唯一一处**语义变更**，必须显式记录；§12 第 2 条要求该 README 的措辞由其 owner 同步更新。

**替代护栏（缺一不可）** —— 原来「人在每条提交前看一眼」现在由这四条承担：

| # | 护栏 | 具体做法 | 不这么做会怎样 |
|---|---|---|---|
| **1** | **scope 固定（最关键）** | 批量启动时记录当时的 `actor` + Effective Organization 作为**本批批准 scope**；每条点击**之前**重新读取页面当前展示的 scope，必须与批准 scope 逐字节一致；不一致 ⇒ **立刻停止整批、不提交**并提示用户 | 应用在点击时是按**页面当前**上下文提交的（`consent` 取自当前 scope），它并不知道本批是从哪个身份启动的；只有执行器知道 ⇒ 可能把商品归属到用户未批准的组织/账号 |
| **2** | **只点真按钮，不复制其逻辑** | 必须点击真实的 `Confirm and submit`（`capture-receiver.tsx:145`）并等待按钮可点（`disabled={!scoped \|\| view.busy}`）；**不得**直接调 `capture1688(...)` 或手拼 POST | 绕过应用侧的上下文漂移保护与 `reserved` 置位，等于自建第二条提交路径 |
| **3** | **页面的拒绝就是拒绝** | 若页面自身给出「未派发」终态（`capture-receiver.tsx:74-79` / `:111` / `:115`），**不得重试**，停下交人工核实（§4 D2.1） | 在不明确是否已提交的情况下反复重试 ⇒ 重复发布 |
| **4** | **绝不做身份动作** | 不登录、不切换账号/企业、不处理验证码、不伪造成功状态 | 把「错误 Consent / 错误身份所有权」变成真实风险（AGENTS.md 的 BLOCKER 情形） |

⇒ 人工成本从「一次完整采集」降为「批量启动时一次确认 + 仅验证码/登录时介入」。这才是本路线真正交付的东西。

#### D1.3 验证码与登录失效：整批暂停、交用户、原条目重做

用户 2026-09-21 决定：**验证码由用户处理**。执行器据此只实现「检测—暂停—交人—继续」，**不尝试识别或绕过**。

- **检测点**：导航到 1688 商品页之后、触发采集之前。要求**正向确认**这是真实商品页（能取到设计要求的字段）；凡不能正向确认，一律视为可疑，**不采集、不提交**。
- **已知信号**（来自本仓库实测，非推测）：匿名裸请求命中风控时返回 HTTP 200 但带 `Bxpunish`/`x5secdata` 且页面不含商品数据；浏览器侧表现为跳到登录/校验页而取不到商品 DOM。
- **精确签名留到 S1 用一次真实命中固定**，本设计不臆造选择器。已确定的只有判定方向：**「取不到就停下来问」**，而不是「猜一个阀值继续跑」。
- **暂停后的动作**：保持浏览器可见、输出明确提示（第几条、遇到什么、需要用户做什么）、等待用户处理完并按继续（或在一个有界等待内自动检测页面已恢复为真实商品页）；随后**把当前条目从导航一步重做**（重新导航 → 重新采集）。
- **为何重做是安全的**：当前条目此时一定处于 `queued`/`capturing`/`captured`，**从未进入 `submitting`**，因此 §4 D2.1 允许自动重采，不存在重复发布风险。
- **登录失效同构处理**：若 profile 的 1688 登录态失效，同样整批暂停，由用户扫码重登；执行器不持有、不输入凭据。

### D2：待办状态是**执行器本地**的，服务端只看到已提交的操作

- 本地队列（链接、序号、状态、**idempotency key**、**原始 scope（actor + Effective Organization）**、operationId、失败原因）持久化在**执行器本地文件**。
- **明确不是第二事实源**：它不记录任何发布结果，只记录「我打算做什么、做到哪一步」；权威发布结果仍在服务端（由 `by-key` / 采集页回读）。
- **必须记录 idempotency key，但它的语义是「回读句柄」，不是「重试凭据」**：③ 的 key 由扩展在交付时内部生成（`src/controller.ts:26` 的 `crypto.randomUUID()`），**执行器无法指定它，也无法复用它重新提交**（重建 payload 走 `handoff()` 必然生成新 key）。执行器驱动采集页时能从 URL fragment `#idempotencyKey=...` 读到它，**仅用于回读**。
- **必须同时记录原始 scope**（actor id + Effective Organization id，从采集页已展示的**服务端已核实身份**读回，`capture-receiver.tsx:139-140`）。原因：`ByKey` 按 `(organization_id, actor_id, idempotency_key)` 查询（`internal/integration/persistence/product/acquisition/repository.go:338`）⇒ **同一 key 在另一个账号/组织下会被故意隐藏**，`404` **不**等于「从未建行」。

#### 回读结果的分支表（**不含「404 ⇒ 自动重采集」**）

先核对当前 scope 与队列记录的原始 scope：

| 情况 | 含义 | 动作 |
|---|---|---|
| **scope 不一致**（重启后换了账号/组织） | 无法相信任何回读结果 | **整批停止**，提示人工用**原始账号与企业**重新登录后重试；**不自动做任何提交** |
| scope 一致 + `200` 终态（`published`/`failed`） | 已建行并结束 | 记录结果 |
| scope 一致 + `200` 非终态（`acquiring`/`prepared`/`publishing`） | 已建行但未结束（响应丢失 / 20 秒 deadline 中断） | 记 `outcome_unknown`，**停止该条**，提示人工核实；**不自动重试** |
| scope 一致 + **`404 ACQUISITION_NOT_FOUND`** | 仅表示「**当前**账号与企业下看不到该 key」 | **按既有实现语义处理：不证明失败，不自动重新提交**（`capture-receiver.tsx:121-123` 原文：*"No operation is visible for this key in the current account and enterprise. This does not prove a prior request failed. No new submission will be made."*）⇒ 记 `outcome_unknown`，提示人工核实 |
| 队列条目状态为 **`captured`**（已采集、**从未进入提交路径**，无外部副作用） | 重走流程是安全的 | 允许重新采集（新 key） |
| 队列条目状态为 **`submitting`**（可能已提交） | 不可判定是否已派发 | `outcome_unknown` + 人工核实；仅满足 §4 D2.1 的两个回退条件时才回退为可重采 |

- 只有**状态为 `captured`** 的条目允许自动重新采集（见 §4 D2.1 的状态机），**不是** `404`。
- ⇒ 记录 key 的价值：**不把「已成功提交」误判为「没提交过」而重复发布**。它**不**声称能自动推进或自动重试已提交的条目（那需要改契约或加路由，§1.3/§5 明确不做）。
- **如实标注的代价**：已提交但未确认终态的条目需要人工核实，批量会在此停下。这是有意选择——商品重复发布与跨 scope 归属错误是数据正确性问题，优先级高于自动化程度（命中 AGENTS 的 BLOCKER 条件：跨租户归属、重复且不可安全恢复的外部副作用）。
- **代价（已接受）**：应用内看不到「还剩几条」。

#### D2.1 提交意图必须**悲观预写**（否则「未提交」不可判定）

上一版把「队列中明确记为已采集但未提交」当成可重采的依据，但这个判定**在执行器侧拿不到**：真正的 POST 由**页面**发起——`capture-receiver.tsx:85-86` 在调 `capture1688(...)` 前一刻才置 `dispatched = true`，而该变量只存在于页面内；且在点击与 POST 之间还隔着 `await` 的上下文重校（`capture-receiver.tsx:70-79`）。⇒ 若执行器在「提交可能已发生」与「执行器把这个事实写进队列」之间被杀，重启后队列里仍写着「已采集未提交」，就会重新采集并可能**二次发布同一商品**。

> 注：§4 D1.2 把点击交给执行器后，情形**变好而非变差**：「key 与 scope 在提交前就已可知」现在成立了（见下表阶段②），于是预写可以发生在**点击之前**，不再存在「提交时 key/scope 尚未落盘」的窗口。但点击之后那一段（`await` 重校 → POST）仍不可观测，所以下面的保守规则**不变**。

⇒ 修正后的规则：

- 本地队列条目的状态机固定为：

```
queued → capturing → captured → submitting → (published | failed | outcome_unknown)
```

- **预写分两阶段（因为 key 与 scope 在 handoff 之前根本不存在）**。上一版写「在 `popup.handoff` 之前把 `submitting` **连同 key 与 scope** 一起落盘」是**不可执行的**：

  - key 由扩展在 `handoff()` **内部**生成（`controller.ts:26` 的 `crypto.randomUUID()`），发消息之前执行器拿不到它；
  - 已核实 scope 只展示在 handoff **创建出来的那个采集页**上（`capture-receiver.tsx:139-140`），之前也不存在；
  - 而本设计又禁止修改既有采集链路（§1.3）⇒ 不存在「先拿到 key/scope 再 handoff」的路径。

  ⇒ 修正为：

  | 阶段 | 时机 | 写入内容 | 可执行性依据 |
  |---|---|---|---|
  | **① 保守标记（仅标记）** | 发 `popup.handoff` **之前** | 仅 `submitting`，**无 key、无 scope** | 执行器写自己的文件，不依赖任何外部信息 |
  | **② 绑定 key 与 scope** | 采集页就绪、**执行器点击之前** | 从 URL 读 key；从页面展示读已核实身份/组织 | key：handoff hash 已含 `idempotencyKey`（`handoff.ts:54-56`），页面随后归一为 `#operationKey=`（`capture-receiver.tsx:62`，参见 `handoff.ts:60`）；scope：`capture-receiver.tsx:139-140` |

- **阶段① 的语义是「此条可能已被提交，永不自动重采」**。标记本身不携带任何身份信息，所以它**不会**被误用为「可重采」的依据——这是故意的。
- **只有 `captured` 状态（从未进入 `submitting`）才允许自动重新采集。**
- **阶段① 与阶段② 之间崩溃** ⇒ 状态为 `submitting` 且无 key/scope ⇒ 无法回读、无法校 scope ⇒ 直接 `outcome_unknown` + 人工核实。**不自动重采**。代价是一次人工核实；换来的是不可能重复发布。
- **阶段② 之后、点击返回之前崩溃** ⇒ 状态为 `submitting` 且 key/scope 已绑定 ⇒ 可用 by-key 回读：读到行就走正常终态处理；`404` **不单独构成可重采依据**（见下）。
- 从 `submitting` 回退到 `captured`（即可重采）**必须同时满足两个可观测条件**，缺一不可（这也意味着阶段② 必须已完成，否则不可能满足条件 2）：
  1. 执行器**读到了页面自己给出的「未派发」终态文案**——即 `capture-receiver.tsx:74-79` 的 *"No new request was dispatched"*，或 `:111` 的 *"Capture was rejected before admission. No operation was created"*，或 `:115` 的 *"Capture was not submitted"*；且
  2. `by-key` 在 **scope 校验一致**的前提下返回 `404 ACQUISITION_NOT_FOUND`。
- 其余任何情况（包括「不确定用户是否点过」「页面文案没读到」「scope 不一致」「阶段② 未完成」）⇒ 一律 `outcome_unknown`，**停下等人工核实，不自动重采**。

⇒ 用一句话概括本设计的恢复底线：**「不确定是否提交过」时，选择不提交。** 宁多一次人工核实，不多一次重复发布。

### D3：profile 与会话都在执行器本地，服务端零持久化

- 浏览器 profile（已登录 1688）+ 应用会话（已登录应用）都在**同一个本地 profile 目录**。
- 服务端**不新增**任何 cookie/token/profile 字段。
- profile 同一时间只能被一个执行器使用 ⇒ **本地锁**（profile 目录下的锁文件）。这是「单执行器」约束的实现方式，不再依赖服务端租约。

### D4（v2 的 D5 撤销）：**不新增 list 路由**

v2 的 D5（采集操作 list 路由 + 分页 + 精确白名单 flag）**撤销**。理由：它的唯一目的是驱动「应用内待办视图」，而本路线不做该视图；而它的谓词直接来自已被证伪的发现设计（§15）。

⇒ 本设计**不触碰**当前应用的精确路由白名单（`internal/app/httpapi/current_application.go:334-358`）。

### D5：重试与幂等沿用 ③ 的既有语义，不引入 URL 派生 key

- ③ 的幂等键是每次采集新建的 **`crypto.randomUUID()`**（`extensions/1688-capture/src/controller.ts:26`）。
- ⇒ 同一商品重试 = 新的一次采集 = 新的一行，**重试天然可行**（这正是 v2 的 D6 URL 派生 key 做不到的）。
- ⇒ 额度消耗 = **采集次数**（含重复与失败）——这是 ③ **已上线**的语义，本设计**不改**。
- v2 的 D6（服务端从规范化 URL 派生 key）**撤销**：采用它就必须同时解决「失败后永久无法重试」，那需要状态机变更。

## 5. 不变量

1. **服务端零凭据、零 profile 引用**：不得新增任何 cookie/token/profile 持久化字段。
2. **不新增服务端路由、不新增持久化表、不新增状态机**。
3. **证据不跨租户**：服务端只按既有身份/租户作用域读写（③ 既有行为）。
4. **失败不伪造成功**：未知结果必须是 `outcome_unknown`，不得降级为 `failed` 或 `published`。
5. **单执行器单 profile**：本地锁保证；同一 profile 被两个执行器并发使用是禁止状态。
6. **不新增调度器、TTL、key GC、租约续期**（§1.5 第 2 条）。
7. **不依赖 legacy crawler**：不得调用 `internal/crawler/alibaba1688` 的 public→account 回退。

## 6. 状态、前置条件与持久化效果

服务端状态机**完全不变**（`internal/product/sourcing/acquisition_operation.go:16-20`）：`prepared → publishing → published`，失败 `failed`。本路线不产生 `acquiring` 行。

持久化效果：

| 位置 | 内容 | owner |
|---|---|---|
| 服务端 | `product_acquisition_operations` 行（由 ③ 的 `StartPrepared` 建立）+ publication receipt | 既有 |
| **执行器本地** | 待办队列文件（链接 / 步数 / operationId） | 本设计新增（**非业务事实源**） |
| 执行器本地 | 浏览器 profile（含 1688 登录态与应用会话） | 本地 |

## 7. 失败、重试、重启、取消、并发

| 场景 | 结果 |
|---|---|
| 单条失败（下架 / 无效链接 / 挑战页） | 记录本地失败并**继续下一条**；不自动重试该条 |
| 被重定向到登录页 | **停止整批**并提示人工重新扫码。**不使用** `CHALLENGE` 作为持久化码——`validFailure`（`repository.go:444-450`）只接受 `SOURCE_UNAVAILABLE`/`INVALID_SOURCE`/`SOURCE_TOO_LARGE`/`PUBLICATION_CONFLICT`，未知码会被 `Finish` 以 `ErrInvalidAcquisition` 拒绝并使行停在非终态 |
| 应用会话过期 | 停止整批并提示人工在应用内重新登录（**不静默失败、不伪造成功**） |
| 提交响应丢失 | 用 `by-key` 回读确认（既有能力，判定见 §4 D2）；**本地队列必须已记录 key 与原始 scope** |
| 执行器进程被杀 | 重启读回本地队列；先校 scope：不一致 ⇒ 整批停止；一致 ⇒ 按 §4 D2 分支表处理；**凡属 `submitting` 的条目一律 `outcome_unknown`**，除非同时满足 §4 D2.1 的两个回退条件 |
| 重启后换了账号/组织 | **整批停止**，提示用原始账号与企业重新登录；不自动提交 |
| 上一条未确认即取第二条 | 禁止：reset 必须在**上一条已到终态**之后（§4 D1.1） |
| 用户取消 | **本地动作**（停止循环）。服务端不新增取消路由；v2 的「服务端取消」撤销。已提交但未终态的条目按上行处理 |
| 并发执行器 | 本地 profile 锁阻止；无服务端竞争面 |

## 8. 授权与租户

- 全部复用 ③ 既有边界：`AuthPolicyVerifiedIdentity` + `OrganizationAccessPolicyLiveWrite` + 权限 `product_sourcing.write`。
- **不引入**新权限常量、不引入设备令牌、不引入新认证路径。
- profile 不进服务端 ⇒ 无服务端 profile 校验；租户隔离由既有身份作用域承担。

## 9. 边界

| 项 | 取值 | 来源 |
|---|---|---|
| 单条 envelope | 2 MiB | `MaxEncodedEnvelopeBytes` |
| 命令体 | 2 MiB | `MaxAcquisitionCommandBytes` |
| **保留操作总数/组织（终身额度，无 GC）** | **2048**（#444 起），与 ①/③ 共用 | `MaxAcquisitionOperations` |
| 活跃操作数 | 32 | `MaxActiveAcquisitionOperations` |
| 单批条数 | **10** | 本设计 |
| 单条耗时 | 实测约 35 秒；10 条约 6 分钟 | §1.4 |

### 9.1 终身额度：一个**已经存在**的产品上限

`MaxAcquisitionOperations` 不是软阈值，而是**终身额度**：

- 容量检查是 `SELECT count(*) ... WHERE organization_id=?`（`repository.go:161-166`），**不带任何状态过滤** ⇒ 已 `published`、已 `failed` 的行**照样计数**。
- 全包**没有任何 `DELETE`**，契约文档也写明 *"There is no background scheduler, fallback, rebase, TTL or key GC"*。
- 它是**编译期常量**（`acquisition_operation.go:12`），#444 已由 256 提到 **2048**。
- 只有 INSERT 消耗额度；同 key 重复提交走 replay，不再消耗。

**关键：这不是批量引入的问题，而是已上线的 ③ 就存在的上限。**

③ 的幂等键是 `crypto.randomUUID()`（`controller.ts:26`），走 `StartPrepared`（`internal/app/productsourcing/browser_capture.go:113`），而 `read()` 按 `(org, actor, key)` 查找（`repository.go:338`）⇒ 随机 UUID 永远查不到已有行 ⇒ **每次采集都 INSERT 一行**。⇒ ③ 的额度 = **采集次数**（含重复采集同一商品与失败采集）。试用库实测 3 行中 2 行是 random-UUID 的 `failed` 行，即已永久消耗 2 个额度单位。

**这使 (a)「UI 显式展示剩余额度」对已上线功能就有价值**，不只是批量的问题。

### 9.2 取消**不能**释放额度

- `Finish` 接受 `acquiring → failed`（`repository.go:323`），但**本路线不产生 `acquiring` 行**，故该分支用不上。
- 无论何种方式终止，**`failed`/`published` 行仍被 `count(*)` 计入**。取消只改状态，**不释放额度**。
- 唯一能释放额度的是 `DELETE` 该行——契约文档明确排除了 key GC。
- ⇒ (c)（为终态行加保留期）为 **BACKLOG，需独立评审**。终态 key 仍承担幂等重放识别，而 ③ 的随机 UUID key 永不被重提 ⇒ (c) 的合理形式是「仅回收永不可能被重放的终态行」。

## 10. 验证证据（实现时按此验收）

| 不变量 | 验证方式 |
|---|---|
| 单条等价性 | 执行器驱动的单条采集结果与人工点 popup 的结果一致（同一 envelope 契约、同一发布结果） |
| **批量不重复第 1 条** | 3 条**不同**链接 ⇒ 3 个**不同** `offer_id`/`productKey`；若 reset 缺失会出现 3 条相同结果 |
| **reset 边界** | 上一条未到终态时执行器**不**发 reset；下一条不得在上一条未确认时开始 |
| **提交动作必须经真按钮** | 执行器点击的是真实 `Confirm and submit`（`capture-receiver.tsx:145`）；全程**不出现**对 `capture1688(...)` 的直接调用或手拼 POST（可验证：请求只来自页面） |
| **提交前 scope 固定（§1.6 的核心护栏）** | 人为在另一标签把企业切到**另一个**组织 ⇒ 交给执行器提交时**整批停止、无新 POST**；仅在页面当前 scope 与本批批准 scope 逐字节一致时才点击 |
| **验证码则暂停（§4 D1.3）** | 人为让商品页进入校验/风控态 ⇒ 执行器**不采集、不提交**，整批暂停并输出提示；用户处理后该条目被**重做**，且队列中不出现重复发布 |
| **scope 绑定** | 重启后用**另一个**账号/组织运行 ⇒ 整批停止并提示用原账号重登；**不得**因 `404` 而重新提交 |
| **404 不授权提交** | 人为构造 scope 一致 + `by-key` `404` ⇒ 执行器记 `outcome_unknown` 并停下，**无新 POST** |
| **提交意图悲观预写（阶段①）** | 在 `popup.handoff` 之前 kill 执行器/拔网线 ⇒ 重启后该条为 `submitting`（**无 key/scope**），**不**被当作可重采，无新 POST |
| **预写不含 key/scope** | 检查阶段① 落盘内容：只有 `submitting`，无 key、无 scope（证明不依赖 handoff 后才能得到的信息） |
| **阶段①→② 之间崩溃** | 在 handoff 返回后、阶段② 写入前 kill ⇒ 重启后该条为 `submitting` 且无 key ⇒ `outcome_unknown`，无新 POST |
| **仅 `captured` 可重采** | 正常跑完后队列中无 `submitting` 且未终态的条目被自动重采；人为制造 `submitting` ⇒ 停下 |
| **回退双条件** | 只有同时拿到页面「未派发」文案与 scope 一致的 `404` 才回退为可重采 |
| 批量隔离 | 10 条中第 3 条失败 ⇒ 其余 9 条仍到达终态，且第 3 条本地有失败原因 |
| 登录态失效 | profile 登出后运行 ⇒ **整批停止**并给出明确提示，不产生伪造成功 |
| 应用会话失效 | 应用会话过期后运行 ⇒ 整批停止并提示重新登录 |
| 崩溃恢复（已建行） | kill 执行器后重启 ⇒ 对已提交条目走 `by-key`，**不产生第二次发布** |
| 崩溃恢复（未建行） | 在**从未提交过**的条目上 kill ⇒ 重启后允许重采（依据是**本地确认未提交**，而非 `404`） |
| 非终态不自动重试 | 人为制造非终态条目 ⇒ 执行器记 `outcome_unknown` 并停下，不自动重试 |
| 单执行器 | 第二个执行器对同一 profile 启动 ⇒ 被本地锁拒绝 |
| 零新增面 | `git diff` 中无新增服务端路由、无新增持久化表、无新增权限常量、无 profile/凭据字段 |
| 无 legacy 依赖 | 全链路 grep 无 `internal/crawler/alibaba1688` 调用 |
| 跨租户 | 用 Org B 身份访问 Org A 的操作 ⇒ 404/403（③ 既有行为回归） |
| 维度 | 达到 2048 终身额度后新采集返回 429 `ACQUISITION_CAPACITY`（§9.1） |
| 真实批量 | 3 条真实链接端到端全绿，商品逐个出现在应用目录中 |

## 11. 待决项

1. ~~批量上限~~ **已定：单批 10 条**（风控只在 5 条上验证过；10 × ~35 秒 ≈ 6 分钟）。
2. ~~形状~~ **已定：路线 B**（§2）。
3. ~~是否提高额度常量~~ **已定并已合并：2048**（#444）。
4. **剩余额度怎么展示**（本设计唯一可能触碰契约的点）。三选一：
   - (a1) 在既有采集响应体上**追加**一个 `remainingOperations` 字段（附加式，但需同步 web 侧 schema）；
   - (a2) **不加字段**，只在 429 `ACQUISITION_CAPACITY` 时给出清晰提示；
   - (a3) 执行器本地按`采集次数`自行计数（不权威，仅提示）。
   > 建议 (a1)，但它是契约新增，需你确认。
5. **登录态探测**：整批开始前只探一次（失败即整批停止），还是每条前都探？
6. **多执行器**是否允许（当前设计假定单 profile 单执行器，由本地锁保证）？
7. ~~是否接受「逐条人工确认」作为批量形态~~ **已定（用户 2026-09-21）：不需要逐条人工确认，只有遇到验证码时才由用户处理**（§1.6，决策 ID `PD-1688-BATCH-VERIFY-ON-CAPTCHA-2026-09-21`）。已据此把提交动作交给执行器，并用 §4 D1.2 的四条护栏（尤其 scope 固定）替代原来的逐条人工确认；另新增 §4 D1.3 处理验证码与登录失效。

## 12. 冲突引用（需由协调方更新）

- **Issue #396（PAUSED）** 原文要求匿名采集**不得依赖历史 profile/session**。**2026-09-21 用户明确推翻该前提**：1688 采集不再要求匿名，账号态（持久化 profile）是允许的优先路径。记录：`#issuecomment-5758793730`（Decision ID `PD-1688-ACCOUNT-STATE-2026-09-21`）。
  - 被推翻的**仅是「必须匿名」这一条前提**。本设计仍**不依赖** #396 本身、`SourceAccount`、`connectionStatus` 或旧 SourceAccount owner，只依赖**独立的本机 profile**（§4 D3）。
  - #396 自身范围仍**保持 PAUSED**；历史证据与结论保留。
  - 该决定**不授权**真实环境的数据访问/迁移/删除，也**不授权**使用共享凭据或他人账号。
- **`src2b-acquisition-v1`（#398 契约文档）** 排除登录流程的边界**不变**。本设计**不并入**该 contract，也**不新增 producer**（原样复用 ③ 的 `browser_acquisition`）。
- **`src2b-public-acquisition.md`** 的 "no background scheduler, fallback, rebase, TTL or key GC" 继续遵守；**本设计不涉及 `acquiring` 租约**，故与该条零冲突。retained 上限已由 #444 由 256 提到 2048，终身语义不变。
- **`extensions/1688-capture/README.md:124-128`（异常：自本条决定起与实现语义不再逐字一致，需 owner 同步）**：该段要求 ③ 链路在**每次 POST 之前**向用户展示已核实身份/组织并取得**显式确认**。用户 2026-09-21 决定「只有遇验证码才需人处理」（§1.6）⇒ 确认的**时点**从「每条」变为「批量启动一次」，**动作**改为执行器代触发。应用侧代码路径**未变**（执行器点的是真按钮，§4 D1.2），但 README 的措辞已不足以描述实际语义，需由该文档 owner 同步更新或记为显式例外。
  - **本设计不自行改写该 README**（它是已准入的 contract 文本，不属本 PR 范围）；也不声称已获其 owner 批准。
  - 在 owner 更新前，实现（S1+）不得绕开该链路直接 POST（§4 D1.2 护栏 2）——即：**语义变了，能力边界没变**。

## 13. Legacy 处置

```text
Legacy decision: RETIRE
Reusable behavior: 无。本路线不抽取任何 legacy 行为——不调 ProcessWithAccountProfile、
                   不调 Process、不引入 public→account 回退；浏览器由执行器以 profile 直接启动。
Current owner: 无新增 owner。沿用已上线的 ③ 链路（internal/app/httpapi/browser_capture_application.go
               + extensions/1688-capture）作为唯一 owner。
Cutover/deletion condition: 本设计不执行删除。确认 internal/crawler/alibaba1688 的
               public→account 回退与 internal/localagent 的匿名 runner（internal/localagent/runner.go:79）
               无消费者后，按独立任务删除；不建兼容层。
```

- `internal/crawler/alibaba1688` 与 `internal/localagent` **不在本设计范围内**：本设计既不使用它们，也不负责删除它们。它们当前都不在当前应用组合中（`grep` 已确认）。
- 本设计**不新增** legacy 依赖，也不以「保持旧代码可用」为由新增任何 fallback。

## 14. 实现切片（每片可独立验证、可回滚）

1. **S1 — 单条驱动跑通**：Go 执行器用 `playwright-go` 启动 fingerprint-chromium + 加载扩展 + 发 `popup.capture` + 打开采集页，完成 1 条并回读终态。
   验证：§10 的「单条等价性」。
2. **S2 — 本地队列 + 批量**：≤10 条循环、**每条之间的 reset（§4 D1.1）**、本地队列文件（**含 key 持久化**，§4 D2）、失败隔离、崩溃恢复、登录态/会话失效即停。
   验证：§10 的「批量隔离 / 崩溃恢复 / 登录态失效 / 应用会话失效」。
3. **S3 — 单执行器锁**：本地 profile 锁。
   验证：§10 的「单执行器」。
4. **S4 — 剩余额度展示**（依赖 §11-4 的决定）。
5. **S5 — 文档**：#398 记录「复用 ③ producer、不新增 producer」；legacy register 记录 RETIRE 条目。

每片在实现层独立可验证，**不要求每片单独 PR**。

## 15. 评审发现处置记录

### 15.1 v2 的 4 条（第一/二轮）

| Finding | 结论 | 处置 |
|---|---|---|
| F1 `Claim` 无法认领 `acquiring` | 部分成立：`Claim` 只处理 `prepared`（`repository.go:286`）属实；「无原子入口」不成立（`Start` 即入口） | v2 补齐定义；**v3 后整个 `acquiring` 阶段不复存在，本条消解** |
| F2 profile 引用挂在已 RETIRE 的 owner | 成立（`legacy-register.md:117`） | v2 改为执行器本地 `LocalCollectorProfile`；v3 保持（§4 D3） |
| F3 幂等只在 actor 作用域 | 成立（`repository.go:338`） | v3 下 actor 天然是本人（提交由浏览器会话完成），本条消解 |
| F4 30 秒租约 vs 34.7 秒抓取 | 成立且最严重 | v2 用「过期重认领当心跳」；**该机制经第三轮核实为不可行**（§16 第 2 条）；v3 完全不使用 `acquiring`，本条消解 |

### 15.2 第三轮 13 条

| # | 评审内容 | 分类 | v3 处置 |
|---|---|---|---|
| 1 | 发现缺少生产者区分符 | BLOCKER | **消解**：无待办行可发现（§2） |
| 2 | 待办意图 → 浏览器证据无合法迁移 | BLOCKER | **消解**：不做两阶段 `Start`。v2 的心跳机制经核实必然 `ErrAcquisitionConflict`（§16） |
| 3 | 须保留提交人 actor | BLOCKER | **消解**：提交由浏览器内用户会话完成（`workbench-proxy.ts:424`），无需委派 |
| 4 | 终态行须释放容量 | BACKLOG（产品已决） | #444 提到 2048；真正释放为 (c)，需独立评审（§9.2） |
| 5 | 终态后须允许重试 | BLOCKER | **消解**：沿用 ③ 的每采集新 key，重试天然可行（§4 D5） |
| 6 | 须定义可取消迁移 | IMPLEMENTATION_TEST | **已修**：取消改为本地动作（§7） |
| 7 | `CHALLENGE` 不可持久化 | IMPLEMENTATION_TEST | **已修**：改用 `SOURCE_UNAVAILABLE`（§7） |
| 8 | 新提交操作须出现在进度读 | IMPLEMENTATION_TEST | **消解**：不做应用内进度读（§4 D2） |
| 9 | 分页需要稳定位置 | IMPLEMENTATION_TEST | **消解**：不新增 list 路由（§4 D4） |
| 10 | 不确定发布须投影为 `outcome_unknown` | IMPLEMENTATION_TEST | 保留为不变量（§5-4），由既有 HTTP 投影承担 |
| 11 | `prepared` 行重启后须可恢复 | IMPLEMENTATION_TEST | **v3 的回答经第四、五轮两次纠正**（§15.3/§15.4）。恢复 = **`by-key` 回读 + 按结果分支**（§4 D2）；已提交但非终态的条目**不自动重试**，停下等人工核实；`404` **不**作为可重采依据（§15.4） |
| 12 | 抽取计划须移除已退役回退 | IMPLEMENTATION_TEST | **已修**：§13 改为纯 `RETIRE`，且本设计不抽取任何 legacy 行为 |
| 13 | 须按 2048 校验容量 | IMPLEMENTATION_TEST | **已修**：§9.1 与 §10 已按 2048 |

### 15.3 第四轮 2 条（针对 v3）

| # | 评审内容 | 分类 | 处置 |
|---|---|---|---|
| 14 | **批量中必须在每条之间 reset 扩展 controller** | **BLOCKER（成立，已修）** | 核实成立且是硬缺陷：`Controller` 是 background 模块级单例（`background.ts:25`），`capture()` 在已有 payload 时直接返回（`controller.ts:18`），`handoff()` 在已交付时直接返回（`controller.ts:24`），唯一 reset 是 `popup.new`（`background.ts:34`）⇒ 我原来的 D1 循环会在第 2 条重复第 1 条的结果。**已新增 §4 D1.1**：定义每条之间的 reset 顺序、边界论证（`popup.new` 的两步确认所防的「误触丢操作」已由「执行器先确认上一条到终态」满足），以及实现方式（驱动 popup 页面模拟点击，而非直发内部消息） |
| 15 | **同 key 崩溃重试无法执行** | **IMPLEMENTATION_TEST（成立，已修）** | 核实成立，且揭穿了我 v3 第一版里的一个真错误：我写了「同 key 重试走 `StartPrepared` 的 `Claim`→`resolve`（`browser_capture.go:135-143`）」，但该路径**需要重新提交 payload**，而重建 payload 走 `handoff()` 必然生成**新随机 key**（`controller.ts:26`）⇒ 执行器**根本没有**「复用同一 key 重试」的能力；我引用的代码路径真实，但它不提供我声称的能力。**已改为 §4 D2 的回读分支表**：`200` 终态⇒记结果；`200` 非终态⇒`outcome_unknown` 并停下待人工核实（不自动重试，避免重复发布）；`404 ACQUISITION_NOT_FOUND`（`repository.go:348` → `product_acquisition_application.go:315`）⇒ 丢弃 key 安全重采集。§10 验证项已相应改为可执行的 4 条。（**本行后半段的 `404` 结论已在第五轮被第 17 条推翻，见 §15.4；此处置换同样需要原始 scope 校验**）|

### 15.4 第五轮 2 条（针对 v3.1）

| # | 评审内容 | 分类 | 处置 |
|---|---|---|---|
| 16 | **提交前的显式确认不能由执行器代劳** | **BLOCKER（成立，已修）** | 核实成立，且有契约原文：`extensions/1688-capture/README.md:124-128` 要求 *"show the currently verified user and Effective Organization, and obtain explicit confirmation"*，而唯一产品提交入口是用户手点的按钮（`capture-receiver.tsx:145`）。我原来的 D1 只写「等待终态」，**默认了执行器会去点提交**——那会替换掉保护 actor/组织归属的确认。**已新增 §4 D1.2**：执行器**一律不点**「Confirm and submit」，只做导航/采集/交付/回读；批量形态定为「执行器跑腿 + 用户逐条确认」；并明确**不自行放行**批量级一次性确认。§1.2/§1.3/§10/§11-7 已同步。（**本行结论已被用户 2026-09-21 的产品决定推翻**：不需要逐条确认，遇验证码才交人处理，见 §1.6 / §15.7 / §4 D1.2 重写版） |
| 17 | **`404` 恢复必须绑定原始 actor 与组织** | **BLOCKER（成立，已修，且是我的硬错误）** | 核实成立且命中 AGENTS 的 BLOCKER 条件（跨租户归属 + 重复且不可安全恢复的外部副作用）。`ByKey` 按 `(organization_id, actor_id, idempotency_key)` 查询（`internal/integration/persistence/product/acquisition/repository.go:338`）⇒ 同一 key 在另一个账号/组织下**被故意隐藏**，`404` **不**证明「从未建行」。更严重的是：**既有实现本来就写着这一点**——`capture-receiver.tsx:121-123` 原文是 *"This does not prove a prior request failed. **No new submission will be made.**"*。我的「`404` ⇒ 安全重采集」与该已实现行为**直接相反**。**已改为 §4 D2**：本地队列记录**原始 scope**，回读前先校 scope（不一致⇒整批停止）；`404` 归入 `outcome_unknown`，**只有「本地明确记为未提交」才允许重采**。§1.3/§7/§10/§11 已同步。（**本行末尾的「本地明确记为未提交」依据已在第六轮被推翻——执行器观测不到提交动作，见 §15.5 / §4 D2.1**） |

两条都是**真实缺陷**，其中第 17 条是**我引入的回归**：我引用真实代码得出错误行为结论，而仓库里已有的前端实现已按正确语义写成（并把该纠正直接告知用户）——我只读了后端 `repository.go:348`/`product_acquisition_application.go:315`，没读前端对该投影的**产品语义**。

### 15.5 第六轮 1 条（针对 v3.2）

| # | 评审内容 | 分类 | 处置 |
|---|---|---|---|
| 18 | **提交意图必须在允许重采之前持久化** | **BLOCKER（成立，已修）** | 核实成立，而且它指出我第五轮的修复**仍然不可判定**：我把「本地明确记为未提交」当作可重采依据，但提交是**页面**发起的（`capture-receiver.tsx:85-86` 在调 `capture1688(...)` 前一刻才置页面局部的 `dispatched = true`），执行器**根本观察不到**这个动作。⇒ 点击后、落盘前被杀 ⇒ 重启后队列里仍写着「未提交」⇒ 可能二次发布。**已新增 §4 D2.1**：条目状态机 `queued → capturing → captured → submitting → 终态`；**必须在发 `popup.handoff` 之前**（payload 变可见之前）悲观预写 `submitting` + key + scope；只有 `captured` 允许自动重采；从 `submitting` 回退必须**同时**满足「读到页面自己给出的未派发文案」（`capture-receiver.tsx:74-79`/`:111`/`:115`）与「scope 一致的 `404`」。§7/§10 已同步 |

这条是对我上一轮修复的**直接反驳**，且成立。我把「正确语义」理解对了，但把它**建立在一个执行器拿不到的观测点上**——两次修复的共同模式都是「引用了真实代码，却推断出该代码并不提供的能力」。§10 因此新增两项可执行的 fault-injection 验收（「提交意图悲观预写」「仅 `captured` 可重采」）。

### 15.6 第七轮 1 条（针对 v3.3）

| # | 评审内容 | 分类 | 处置 |
|---|---|---|---|
| 19 | **预写意图必须可执行** | **BLOCKER（成立，已修）** | 核实成立，而且它指出我第六轮的修复**自相矛盾**：我写「在 `popup.handoff` 之前把 `submitting` **连同 key 与 scope** 落盘」，但 (a) key 由扩展在 `handoff()` **内部**生成（`controller.ts:26`），发消息之前执行器拿不到；(b) 已核实 scope 只展示在 handoff **创建的那个采集页**上（`capture-receiver.tsx:139-140`）；而本设计又禁止修改采集链路（§1.3）⇒ 该写入**不可执行**。**已改为 §4 D2.1 的两阶段预写**：阶段①（handoff 前）只写 **`submitting` 标记**，无 key/无 scope——不依赖任何外部信息，因此可执行；阶段②（handoff 返回后）从 URL 与页面绑定 key/scope。并明确「阶段①↔② 之间崩溃 ⇒ `outcome_unknown`，不自动重采」。§10 新增三项验收（含「预写不含 key/scope」与「阶段①→② 之间崩溃」） |

第三轮连续指出我的修复建立在执行器拿不到的观测点上。共同模式已经很清楚：**我把「语义正确」误当成「可实现」**。§15.5/§15.6 因此各保留一条相同教训，作为后续 S1 实现时的检查项：任何「执行器必须先知道 X 才能安全行动」的规则，都必须先回答「X 在某时刻真的可观测吗」。

### 15.7 产品决定：提交不再逐条人工确认（用户 2026-09-21）

§11-7 原本是留给用户的产品问题。用户回答：

> **批量只有在遇到验证码时才需要用户处理。**

即：**逐条人工确认被取消**，而**验证码由用户处理**。记录为 `PD-1688-BATCH-VERIFY-ON-CAPTCHA-2026-09-21`（§1.6）。

本设计据此做了三件事（**不是**简单地删除护栏）：

1. **授权自动提交，同时补上替代护栏**（§4 D1.2 重写）。执行器可以代用户触发提交，但必须满足四条：**scope 固定**（最关键）、**只点真按钮**、**页面拒绝即拒绝**、**绝不做身份动作**。理由已写入：应用在点击时是按**页面当前**上下文提交的，它不知道本批从哪个身份启动；只有执行器知道 ⇒ 没有 scope 固定这一条，自动点击就会把商品归到用户未批准的组织。
2. **新增验证码/登录失效的处理**（§4 D1.3）：检测⇒整批暂停⇒交人⇒原条目重做；且**不尝试识别或绕过验证码**（用户明确说交给人处理）。精确信号留到 S1 用一次真实命中固定，不臆造选择器。
3. **如实记录语义变更并标记待同步文档**：§12 新增一条，指出 `README.md:124-128` 的措辞已不足以描述新语义，需其 owner 更新，且**本设计不自行改写该 contract 文本**。

**一个正面副作用**：因为提交已由执行器控制，`key` 与 `scope` 现在**在提交前就已可知**，所以 §4 D2.1 的阶段② 从「handoff 之后」前移到「**点击之前**」——原来担心的「提交时 key/scope 尚未落盘」窗口被消掉了。恢复底线不变：**「不确定是否提交过」时选择不提交。**

**没有放宽的项**：错误 Consent / 错误身份所有权仍是 BLOCKER 级禁止事项；执行器不得绕开采集页直接 POST；不得登录、切账号、处理验证码或伪造成功状态。

两条 findings 都是**真实缺陷**，已直接修正设计，无需产品决定。第 14 条之所以能拿下来，是因为它指出了「复用已上线链路」这一路线的一个隐藏前提：既有链路是**为单次人工点击设计的有状态流程**，不是无状态 API。这一点已写入 §3.2 缺口表。

## 16. v3 变更记录

1. **路线 B 由用户 2026-09-21 选定**（§2）。设计整体重写；v2 的服务端待办行、发现谓词、过期重认领心跳、list 路由、服务端取消、URL 派生 key 全部撤销。
2. **v2 核心机制被证伪**：`Start` 在复用既有行前比较 `Source`/`Fingerprint`/`CaptureSHA256`（`repository.go:143-145`）。v2 的 t=0 `Start` 写入公开式指纹 + 空 `CaptureSHA256`（`repository.go:168`），t=35 带浏览器指纹回来再 `Start` 必然 `ErrAcquisitionConflict`，拿不到新 fence ⇒ 心跳不成立。v3 因此完全不使用 `acquiring` 阶段。
3. **对先前推荐的修正**：设备令牌路径不必要且更差（`deviceauth` 拒绝 refresh token，`client.go:133`）；改为执行器零凭据、由浏览器会话提交（§2）。
4. **`CHALLENGE` 与取消语义修正**（§7）：`validFailure` 只接受 4 个码；取消是本地动作。
5. **§13 由 `EXTRACT | RETIRE` 改为纯 `RETIRE`**：本路线不抽取任何 legacy 行为（v2 曾错误地把被禁的 public→account 回退列入 `EXTRACT`）。
6. **新增 §4 D1.1（每条之间的 reset）**：第四轮评审指出 `Controller` 是单例且 `capture()`/`handoff()` 会直接返回，只循环发消息会重复第 1 条的结果。已定义 reset 顺序、边界论证与实现方式，并相应加入 §10 验证项。
7. **修正 D2 的 key 语义**：原写「同 key 重试」是错的——key 由扩展在交付时内部生成（`controller.ts:26`），执行器无法指定也无法复用；引用的 `StartPrepared` 恢复路径需要重新提交 payload，而重建 payload 必然生成新 key。已改为「`by-key` 回读 + 按结果分支」的判定表。
8. **新增 §4 D1.2（提交确认边界）**：第五轮评审指出执行器不能代替用户点「Confirm and submit」——契约要求每条 POST 前展示已核实身份与企业并取得显式确认（`README.md:124-128`，`capture-receiver.tsx:145`）。已明确执行器不点该按钮，批量形态为「执行器跑腿 + 用户逐条确认」。**（此项已被用户 2026-09-21 的 §1.6 决定推翻：提交改由执行器代触发，改用四条护栏；见 §15.7）**
9. **修正 D2 的 `404` 语义（我的回归）**：原写「`404` ⇒ 安全重采集」，与既有实现**直接相反**——`capture-receiver.tsx:121-123` 原文明确 "This does not prove a prior request failed. No new submission will be made."，且 `ByKey` 按 `(organization_id, actor_id, key)` 查询（`repository.go:338`）使 `404` 也可能是 scope 不符。已改为「记录原始 scope + 回读前校 scope + `404` 归 `outcome_unknown`」，仅「本地确认未提交」允许重采。
10. **新增 §4 D2.1（提交意图悲观预写）**：第六轮评审指出上一版的「本地确认未提交」不可判定——提交由页面发起（`capture-receiver.tsx:85-86`），执行器看不到。已定义条目状态机、在 `popup.handoff` 之前预写 `submitting`、以及回退为可重采的双条件。
11. **D2.1 改为两阶段预写**：第七轮评审指出「handoff 之前写 key+scope」不可执行（key 在 `handoff()` 内部生成 `controller.ts:26`，scope 在 handoff 创建的页面上）。阶段① 仅写 `submitting` 标记；阶段② 再绑定 key/scope。
12. **产品决定：取消逐条人工确认（§1.6 / §15.7）**。用户 2026-09-21：批量只在遇到验证码时才需人处理。据此重写 §4 D1.2（授权自动提交 + 四条替代护栏，其中 scope 固定最关键）、新增 §4 D1.3（验证码/登录失效⇒整批暂停交人⇒原条目重做）、§3.2 缺口表、§10 验收、§11-7、§12（README 待同步）。同时把 D2.1 阶段② 前移到点击之前，消掉「提交时 key/scope 未落盘」窗口。
