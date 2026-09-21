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
  → 执行器后台依次：打开商品页 → 复用已上线的插件采集 → 交付到采集页
  → 用户当看到已核实身份与企业后逐条点「Confirm and submit」
  → 每条结果写入本地队列文件（执行器侧可见进度）
  → 成功发布的商品逐个出现在应用的商品目录中
```

**明确不含**：

- 应用内的「待办列表 / 批量进度页」。待办进度只在**执行器本地**可见（§4 D2）。这是本路线相对路线 A 主动放弃的东西，理由见 §2。
- **无人值守提交**。按已准入的 ③ 契约（`extensions/1688-capture/README.md:124-128`）与唯一产品提交入口（`capture-receiver.tsx:145`），**每条 POST 都由人确认**；执行器只做跑腿（§4 D1.2）。

### 1.3 本次不做

- 不做定时/周期性重采（只做一次性批量）
- 不做多账号池、代理池、账号轮换
- **不新增任何服务端路由**（与 v2 的关键差别：v2 的 D5 list 路由已撤销，见 §4 D4）
- **不新增服务端待办行**；服务端不持久化 profile 引用或任何 1688 凭据
- 不改 ③（插件采集）的任何现有行为
- 不恢复 ①（服务端匿名裸 HTTP）—— 已证结构性不可行
- 不引入后台调度器、TTL 或 key GC（§1.5 第 2 条）
- **不做批量级一次性确认**（不自行放行「替代逐条确认」的新准入；如需零点击自动化，属新产品决定，须单独提报，见 §4 D1.2）
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
| **提交需要逐条真实确认，执行器不能代劳** | 契约 `README.md:124-128` 要求展示已核实身份/组织并取得显式确认；唯一提交入口是用户手点按钮 `capture-receiver.tsx:145` | §4 D1.2：执行器只跑腿，人逐条确认 |
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

#### D1.2 提交前的确认边界（**本设计最重要的合规约束**）

已准入的 ③ 契约要的是**每一条提交前都向用户展示已核实身份与企业并取得显式确认**：

> Before any POST, the app must replace the initial fragment with `#operationKey=<original UUID>`, **show the currently verified user and Effective Organization, and obtain explicit confirmation** using the existing BFF context-drift protections.
> —— `extensions/1688-capture/README.md:124-128`

且产品上唯一的提交触发器是**用户手动点的按钮**（`capture-receiver.tsx:145`：`onClick={() => void run(true)}`，按钮文字「Confirm and submit」）。

⇒ **执行器一律不自行点击该按钮。** 本路线的边界是：

| 执行器做 | 执行器**不**做 |
|---|---|
| 导航、触发采集、交付、**打开采集页并停留在确认前** | **不**点「Confirm and submit」 |
| 回读终态 | 不代替用户确认身份/组织 |
| 本地排队与进度 | 不创建一个「批量级一次性确认」来替代逐条确认 |

⇒ 批量因此是**「执行器跑腿 + 用户按确认」**：执行器自动完成 90% 的机械动作（开页、采集、交付），人的动作降为「看到身份/组织 → 点确认」。每条的人工成本约一次点击，而不是一次完整采集。

**为什么不做「批量级一次性确认」**：它需要新准入（新契约 + 独立评审），且它会**降低**而不是保持现有保证——现有确认锁住的是「**这一笔**操作将要归属的 actor 与组织」；一次性确认无法在每条提交时重新校验上下文漂移（`capture-receiver.tsx:110-119` 处理的正是「上下文变了就不提交」）。**本设计不自行放行这一点**；若将来要自动化到零点击，属于新产品决定，需单独提报。

⇒ **代价（已如实计入）**：批量不是无人值守。人必须在场逐条确认。它仍然解决了真实痛点：不再需要人工逐个打开商品页、点采集、再点导入。

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
| 队列中**明确记为「已采集但未提交」**（从未点过提交，无外部副作用） | 重走流程是安全的 | 允许重新采集（新 key） |

- 只有**最后一档**允许自动重新采集，且它的依据是**「本地确认未曾提交」**，**不是** `404`。
- ⇒ 记录 key 的价值：**不把「已成功提交」误判为「没提交过」而重复发布**。它**不**声称能自动推进或自动重试已提交的条目（那需要改契约或加路由，§1.3/§5 明确不做）。
- **如实标注的代价**：已提交但未确认终态的条目需要人工核实，批量会在此停下。这是有意选择——商品重复发布与跨 scope 归属错误是数据正确性问题，优先级高于自动化程度（命中 AGENTS 的 BLOCKER 条件：跨租户归属、重复且不可安全恢复的外部副作用）。
- **代价（已接受）**：应用内看不到「还剩几条」。

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
| 执行器进程被杀 | 重启读回本地队列；先校 scope：不一致 ⇒ 整批停止；一致 ⇒ 按 §4 D2 分支表处理（**`404` 不等于可重采集**） |
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
| **确认边界** | 任何 POST 都由用户手动点击触发；执行器从未自行点击「Confirm and submit」（可验证：全程无对 `run(true)` 按钮的自动点击） |
| **scope 绑定** | 重启后用**另一个**账号/组织运行 ⇒ 整批停止并提示用原账号重登；**不得**因 `404` 而重新提交 |
| **404 不授权提交** | 人为构造 scope 一致 + `by-key` `404` ⇒ 执行器记 `outcome_unknown` 并停下，**无新 POST** |
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
7. **是否接受「逐条人工确认」作为批量形态**（§4 D1.2）。若不可接受，则本路线的可达上限就是「执行器跑腿 + 人工确认」，而「完全无人值守」需新的产品决定与独立准入。

## 12. 冲突引用（需由协调方更新）

- **Issue #396（PAUSED）** 原文要求匿名采集**不得依赖历史 profile/session**。**2026-09-21 用户明确推翻该前提**：1688 采集不再要求匿名，账号态（持久化 profile）是允许的优先路径。记录：`#issuecomment-5758793730`（Decision ID `PD-1688-ACCOUNT-STATE-2026-09-21`）。
  - 被推翻的**仅是「必须匿名」这一条前提**。本设计仍**不依赖** #396 本身、`SourceAccount`、`connectionStatus` 或旧 SourceAccount owner，只依赖**独立的本机 profile**（§4 D3）。
  - #396 自身范围仍**保持 PAUSED**；历史证据与结论保留。
  - 该决定**不授权**真实环境的数据访问/迁移/删除，也**不授权**使用共享凭据或他人账号。
- **`src2b-acquisition-v1`（#398 契约文档）** 排除登录流程的边界**不变**。本设计**不并入**该 contract，也**不新增 producer**（原样复用 ③ 的 `browser_acquisition`）。
- **`src2b-public-acquisition.md`** 的 "no background scheduler, fallback, rebase, TTL or key GC" 继续遵守；**本设计不涉及 `acquiring` 租约**，故与该条零冲突。retained 上限已由 #444 由 256 提到 2048，终身语义不变。

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
| 16 | **提交前的显式确认不能由执行器代劳** | **BLOCKER（成立，已修）** | 核实成立，且有契约原文：`extensions/1688-capture/README.md:124-128` 要求 *"show the currently verified user and Effective Organization, and obtain explicit confirmation"*，而唯一产品提交入口是用户手点的按钮（`capture-receiver.tsx:145`）。我原来的 D1 只写「等待终态」，**默认了执行器会去点提交**——那会替换掉保护 actor/组织归属的确认。**已新增 §4 D1.2**：执行器**一律不点**「Confirm and submit」，只做导航/采集/交付/回读；批量形态定为「执行器跑腿 + 用户逐条确认」；并明确**不自行放行**批量级一次性确认。§1.2/§1.3/§10/§11-7 已同步 |
| 17 | **`404` 恢复必须绑定原始 actor 与组织** | **BLOCKER（成立，已修，且是我的硬错误）** | 核实成立且命中 AGENTS 的 BLOCKER 条件（跨租户归属 + 重复且不可安全恢复的外部副作用）。`ByKey` 按 `(organization_id, actor_id, idempotency_key)` 查询（`internal/integration/persistence/product/acquisition/repository.go:338`）⇒ 同一 key 在另一个账号/组织下**被故意隐藏**，`404` **不**证明「从未建行」。更严重的是：**既有实现本来就写着这一点**——`capture-receiver.tsx:121-123` 原文是 *"This does not prove a prior request failed. **No new submission will be made.**"*。我的「`404` ⇒ 安全重采集」与该已实现行为**直接相反**。**已改为 §4 D2**：本地队列记录**原始 scope**，回读前先校 scope（不一致⇒整批停止）；`404` 归入 `outcome_unknown`，**只有「本地明确记为未提交」才允许重采**。§1.3/§7/§10/§11 已同步 |

两条都是**真实缺陷**，其中第 17 条是**我引入的回归**：我引用真实代码得出错误行为结论，而仓库里已有的前端实现已按正确语义写成（并把该纠正直接告知用户）——我只读了后端 `repository.go:348`/`product_acquisition_application.go:315`，没读前端对该投影的**产品语义**。

两条 findings 都是**真实缺陷**，已直接修正设计，无需产品决定。第 14 条之所以能拿下来，是因为它指出了「复用已上线链路」这一路线的一个隐藏前提：既有链路是**为单次人工点击设计的有状态流程**，不是无状态 API。这一点已写入 §3.2 缺口表。

## 16. v3 变更记录

1. **路线 B 由用户 2026-09-21 选定**（§2）。设计整体重写；v2 的服务端待办行、发现谓词、过期重认领心跳、list 路由、服务端取消、URL 派生 key 全部撤销。
2. **v2 核心机制被证伪**：`Start` 在复用既有行前比较 `Source`/`Fingerprint`/`CaptureSHA256`（`repository.go:143-145`）。v2 的 t=0 `Start` 写入公开式指纹 + 空 `CaptureSHA256`（`repository.go:168`），t=35 带浏览器指纹回来再 `Start` 必然 `ErrAcquisitionConflict`，拿不到新 fence ⇒ 心跳不成立。v3 因此完全不使用 `acquiring` 阶段。
3. **对先前推荐的修正**：设备令牌路径不必要且更差（`deviceauth` 拒绝 refresh token，`client.go:133`）；改为执行器零凭据、由浏览器会话提交（§2）。
4. **`CHALLENGE` 与取消语义修正**（§7）：`validFailure` 只接受 4 个码；取消是本地动作。
5. **§13 由 `EXTRACT | RETIRE` 改为纯 `RETIRE`**：本路线不抽取任何 legacy 行为（v2 曾错误地把被禁的 public→account 回退列入 `EXTRACT`）。
6. **新增 §4 D1.1（每条之间的 reset）**：第四轮评审指出 `Controller` 是单例且 `capture()`/`handoff()` 会直接返回，只循环发消息会重复第 1 条的结果。已定义 reset 顺序、边界论证与实现方式，并相应加入 §10 验证项。
7. **修正 D2 的 key 语义**：原写「同 key 重试」是错的——key 由扩展在交付时内部生成（`controller.ts:26`），执行器无法指定也无法复用；引用的 `StartPrepared` 恢复路径需要重新提交 payload，而重建 payload 必然生成新 key。已改为「`by-key` 回读 + 按结果分支」的判定表。
8. **新增 §4 D1.2（提交确认边界）**：第五轮评审指出执行器不能代替用户点「Confirm and submit」——契约要求每条 POST 前展示已核实身份与企业并取得显式确认（`README.md:124-128`，`capture-receiver.tsx:145`）。已明确执行器不点该按钮，批量形态为「执行器跑腿 + 用户逐条确认」。
9. **修正 D2 的 `404` 语义（我的回归）**：原写「`404` ⇒ 安全重采集」，与既有实现**直接相反**——`capture-receiver.tsx:121-123` 原文明确 "This does not prove a prior request failed. No new submission will be made."，且 `ByKey` 按 `(organization_id, actor_id, key)` 查询（`repository.go:338`）使 `404` 也可能是 scope 不符。已改为「记录原始 scope + 回读前校 scope + `404` 归 `outcome_unknown`」，仅「本地确认未提交」允许重采。
