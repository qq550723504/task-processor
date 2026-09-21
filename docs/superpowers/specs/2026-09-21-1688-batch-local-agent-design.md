# 1688 批量/后台采集（本地代理执行器）设计

- 状态：**待评审 v2**（未批准前不进入实现）
- 基准：`origin/main` = `4942901cb`
- 相关 Issue：#398、#399、#396（需按 §11 更新）
- 前置实测证据：§1.4
- v2 变更：针对 PR #443 评审 4 条 finding 逐条修正，见 §14

## 1. 目标与范围

### 1.1 使用者与场景

需要在**不逐个打开 1688 页面的前提下**采集多个商品的操作人员：把一批 1688 商品链接交给系统，由本地执行器后台依次抓取，在应用内逐条看到「成功 / 失败 / 待核实」。

### 1.2 本次可见交付

```
在采集页粘贴多行 1688 链接 → 提交
  → 应用内出现 N 条采集操作，状态实时可见
  → 本地执行器后台依次执行（用户不需要停在任何页面上）
  → 每条的最终结果可在应用内查看
```

### 1.3 本次不做

- 不做定时/周期性重采（只做一次性批量）
- 不做多账号池、不做代理池、不做账号轮换
- 服务端不持久化 profile 引用或任何 1688 凭据（见 §3 D3）
- 不改插件路径（③）的任何行为
- 不恢复 ①（服务端匿名裸 HTTP）—— 已证明结构性不可行
- 不引入后台调度器、TTL 或 key GC（见 §1.5）
- 不做 `connectionStatus` 状态机扩展（#396 保持 PAUSED，见 §11）

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

两个附带实现事实：

- Chrome 153 profile 交给 Chromium 144 会**启动即崩**，元凶是 `Default/Sync Data`；⇒ **profile 必须由最终运行的那个浏览器创建**，不能后期迁移。
- 1688 路径的 `SetFingerprint` **从未被调用**（只有 amazon/shein/sds 登录调用）⇒ 1688 的浏览器身份 = 二进制 + profile，登录与自动化天然同身份。

**关键耗时**：单次真实抓取实测 **34.745 秒**。这个数字决定了 §3 D2 的认领方案。

### 1.5 已记录的既有设计决定（必须遵守）

`docs/engineering/src2b-public-acquisition.md`（`src2b-acquisition-v1`，#398）已经记录：

1. > This is anonymous public acquisition, **not Browser Capture, Source Account management, a Connection, or a login flow.**

   ⇒ 本设计是登录流程，**不属于该 contract**，必须作为**独立 producer** 显式准入，不能挂进 `public_acquisition/v1`。

2. > Only expired acquiring operations without a prepared command may obtain a new fenced **GET** lease. **There is no background scheduler, fallback, rebase, TTL or key GC.**

   ⇒ `acquiring` 租约是**服务端 GET 租约**（8 秒 GET / 20 秒操作 deadline / 30 秒租约），**不是**给浏览器长任务用的。
   ⇒ **不得新增租约续期 API、后台调度器、TTL 或 key GC。** 本设计因此**不采用**「为浏览器长任务延长 acquiring 租约」的方案（见 §3 D2）。

3. > Limits: **256 retained operations per organization**, 32 active operations, ... 30-second GET lease.

   ⇒ 256 是**按组织保留、无 GC** 的容量，且与 ①/③ 共用（见 §8）。

## 2. 现状盘点（在 `4942901cb` 上复核）

### 2.1 可直接复用（不要重建）

| 能力 | 位置 | 复用方式 |
|---|---|---|
| 采集操作状态机 + 幂等 + fence + 恢复 | `internal/product/sourcing/acquisition_operation.go` | 直接复用；批量 = N 个 operation |
| 浏览器采集 ingress（`StartPrepared` 恢复语义） | `internal/app/httpapi/browser_capture_application.go` | **作为本设计的 ingress 模板** |
| 扫码登录自动化 + profile runtime | `internal/sheinlogin/`、`internal/sdslogin/api.go:42 ManualLogin` | 抽取其模式 |
| 浏览器配置 / profile 根目录 | `internal/crawler/shared/browser/`、`platforms.alibaba1688.profileRootDir` | 直接复用 |
| 设备登记与身份 | `internal/localagent/deviceauth`（OAuth 设备码）、`localagent.Actor{TenantID,UserID}` | 直接复用 |

### 2.2 缺口

| 缺口 | 证据 | 影响 |
|---|---|---|
| **localagent 的 job 是纯内存** | `internal/localagent/service.go:61-65`（`jobs map[string]*record`） | 重启即丢，不适合批量 |
| **localagent 与采集操作是两套事实** | `Job`/`JobState` vs `AcquisitionOperation` | 第二事实源，违反 PD-GREENFIELD |
| **采集操作没有 list 路由** | 仅有 by-key / by-id 读单条 | 批量无法在应用内查看进度 |
| **local-agent 无任何前端** | `grep -l local-agent web/` 为空 | 用户看不到 |
| **local-agent 走匿名** | `internal/localagent/runner.go:79` → `Process(ctx,url)` | 就是 0/8 的那条路 |
| **采集页"贴链接"入口走 ①** | `acquisition-page.tsx:120` → 服务端裸 HTTP | 已证不可行 |
| **local-agent 路由不在当前应用白名单** | `grep local-agent internal/app/httpapi/current_application.go` 为空；模块由 `composition_modules.go:26` 的另一条组装路径提供 | 不能直接复用，需新增独立路由 |

### 2.3 legacy 账号辅助路径（不得依赖）

legacy crawler 里已存在「public → account-assisted 回退」：`internal/crawler/alibaba1688/worker_processor.go:60-110`（`IsAccountFallbackEligible` + `resolveAccountProfile` + `lockAccountProfile` + `ProcessWithAccountProfile`），挂在 `crawler_service.go:109` 的 worker pool，仅由 `composition_builder.go:118` 的 **legacy 组合**装配（`platforms.alibaba1688.enabled` 默认 `false`）。

按 Hard-Cut，这套代码**不是当前 owner**，新链路不得依赖或包装。本设计只 `EXTRACT` 其中仍正确的行为（**按 profile 串行加锁**），其余 `RETIRE`。

## 3. 核心设计决策

### D1：批量 = N 个既有采集操作，不新建队列实体

提交一批 N 个链接 = 创建 N 个 `AcquisitionOperation`。**不新增**批量实体、不新增 localagent 的 Job 事实源。

- 幂等作用域 = **`(organization_id, actor_id, idempotency_key)`**（`repository.go:338` 的 `read()` 与 `Start` 的存在性检查都按这三列）。
- **注意**：本设计**不提供**「同组织跨操作人去重」。同一组织两个操作人贴同一链接会产生两行——这是既有作用域决定，本设计不改动。
  - 影响：可能重复抓取同一商品（重复外部请求，非数据损坏；发布侧按 `crawler:1688:<offerID>` 归一到同一商品）。
  - 运维约束：批量由**单一操作人**提交，即可避免。
- 部分失败语义天然成立：一条失败不影响其它条。
- 容量沿用 `MaxAcquisitionOperations=256`（**终身额度，无 GC，见 §8.1**）/ `MaxActiveAcquisitionOperations=32`。
  - 注意：预建 `acquiring` 行会**立即计入 32 活跃额度** ⇒ 单批条数机械上不能超过 32（本设计取 10）。

> 备选「新增 `BatchJob` 聚合根」被否：会引入第二事实源、第二套状态机、第二套恢复协议。

### D2：执行器复用既有 fence 认领，**不新增续期 API**（F1 / F4 修正）

#### 既有认领语义（实测确认）

| 方法 | 语义 | 证据 |
|---|---|---|
| `Start` | 行不存在 ⇒ INSERT `state='acquiring'`, `fence=1`, `lease=now+30s` | `repository.go:168` |
| `Start` | 行存在且 `state='acquiring' AND command IS NULL AND lease_until<now()` ⇒ **`fence=fence+1`, `lease=now+30s`**（原子认领，外层 `pg_advisory_xact_lock(org)`） | `repository.go:145-152` |
| `Claim` | **只在 `state='prepared'` 时** `prepared→publishing`；不校验租约 | `repository.go:286-289` |
| `Prepare` | `acquiring→prepared`，**要求 `fence` 匹配且 `lease_until>now()`** | `repository.go:257-268` |
| `StartPrepared` | 行不存在 ⇒ 直接 INSERT 为 `state='prepared'`（携带 command） | `repository.go:224` |
| `StartPrepared` | 行存在且 `acquiring AND command IS NULL AND lease_until<now()` ⇒ 同一事务内**直接写 command 并转 `prepared`** | `repository.go:204` |

⇒ **`acquiring` 的原子认领入口是 `Start`，不是 `Claim`。**（评审 F1 仅看 `Claim` 得出「无原子入口」，此处更正。）

#### 但 34.7 秒的抓取撞上 30 秒租约 —— 这是真问题（F4）

`Prepare` 要求 `lease_until>now()`（`repository.go:263`），而 `Prepare` 在抓取**之后**调用。34.7s > 30s ⇒ `Start → 抓取 → Prepare` 必然 `ErrAcquisitionFence`。

#### 解决方案：把既有的**过期重认领**当作 fence 心跳（零新 API，零契约变更）

**心跳的 owner = 新的服务端 ingress handler（在 `internal/app/productsourcing` 当前 owner 内），不是执行器。** 执行器不感知租约，只负责「列出可认领行 → 抓取 → POST 证据」。

既有服务端编排（`internal/app/productsourcing/acquisition.go:45-96`）：

```
Start(45) → Acquire  ← 整体包在 AcquisitionTimeout=20s 里
          → Prepare(86) → Claim(96) → Publish → Finish(282)
```

① 路径下抓取是服务端 GET（≤20s），天然落在 30s 租约内（这就是 §1.5 第 2 条的来源）。

本设计的新 ingress handler 在 **映射证据之前**先重新 `Start` 一次，把已过期的租约换成新 fence：

```
t=0     应用提交 → handler Start(key)      → 行 acquiring, fence=1, lease=t+30s（进度可见）
t=0..35 执行器抓取（浏览器 34.7s）           ← 租约在 t=30 过期，这是允许的；不提供排他性
t=35    执行器 POST 证据
t=35    handler Start(key)                 → 租约已过期 ⇒ fence=2, lease=t+65s（原子重认领）
t=35    handler MapAcquisitionEvidence → Prepare(fence=2) → prepared
t=35    handler Claim → publishing → Publish → Finish → published
```

- **完全使用既有方法**，不新增 `Renew`、不新增调度器、不新增 TTL/GC —— 与 §1.5 已记录的决定一致。
- 重认领次数有界：受 120 秒操作 deadline 约束 ⇒ 最多 4 次。
- 抓取 ≤30 秒时，重认领会 `claim=false`（租约未过期）⇒ 继续用同一 fence 即可，无需特判。
- 认领者必须始终使用**最近一次 `Start` 返回的 `fence`**。

> 语义澄清：**预建 `acquiring` 行提供的是「进度可见」，不是「抓取排他」。** 30 秒租约不可能覆盖分钟级抓取，所以排除权由 §10-3 的「单组织单执行器」约束承担。

#### 发现（discovery）

新增 list 路由（D5）作为唯一发现入口，过滤条件与认领前置条件**逐字对齐**：

```
state = 'acquiring' AND command IS NULL AND lease_until < now()
```

即**只列出可被合法认领的行**。这保证「看到就能认领」，不会出现在活跃租约上的争抢。

#### 恢复行为

| 场景 | 结果 |
|---|---|
| 执行器抓取中崩溃 | 租约 30 秒后自然过期 ⇒ 该行重新出现在 list 中，可被再次认领 |
| 执行器在 `Prepare` 前被杀 | 同上（`command IS NULL`） |
| `Prepare` 后崩溃 | 行已在 `prepared`，走既有 `Claim`/`Finish` 或 verify 恢复路径 |
| 两个执行器同时认领 | `pg_advisory_xact_lock(org)` + `fence` 保证只有一方 `RowsAffected=1`；另一方 `claim=false` |
| 抓取中途被他人抢到（租约过期窗口） | 落败方 `Prepare` 拿到 `ErrAcquisitionFence` ⇒ **必须丢弃本地结果、不得重试发布** |

> **明确约束**：本设计假定**每个组织同一时间只有一个执行器**。多执行器会把这个租约过期窗口放大成重复抓取（重复外部请求，触发风控）。多执行器需要独立设计，见 §10-3。

#### 被否方案（记录原因）

- **为浏览器长任务延长 acquiring 租约 / 新增续期 API**：直接推翻 §1.5 第 2 条已记录的决定（"no TTL"、"GET lease"）。**不做。**
- **执行器持本地队列、抓完才 `StartPrepared`**（③ 的形状）：不需任何 DB 前置行，但**应用在抓取开始前看不到任何待办**，丢掉批量最核心的「进度可见」。作为 §10 的备选保留。

### D3：profile 引用由执行器本地持有，服务端不持久化（F2 修正）

**原 v1 错误**：写成「沿用 `sourceaccount.SourceAccount.ProfileRef` 的既有字段语义」。该 owner 已被 `docs/refactoring/legacy-register.md:117` 标为 `EXTRACT, then RETIRE`，且该条目要求行为须在「**a current owner ... are named**」之后才可保留。把它的字段带进新链路违反 Hard-Cut。

**v2 定义**：新增当前 owner 的契约 `LocalCollectorProfile`，**完全位于执行器本地**：

| 项 | 归属 |
|---|---|
| `profileRef`（本机 profile 名） | 执行器本地配置文件 |
| `profileDir`（本机目录绝对路径） | 执行器本地配置文件 |
| 组织绑定 | **既有**设备登记身份（`localagent.Actor.TenantID`，来自 OAuth 设备码）；服务端**不新增**任何字段 |
| 1688 凭据（cookie / token / 密码） | **仅存在本机 profile 目录**；服务端永不接收、永不持久化 |

- 服务端**不新增** profile 表、字段或路由。跨租户检查由既有的身份/租户作用域承担（执行器只能提交其 token 所属租户的操作）。
- **已知约束**：同一执行器服务其登记身份所属的那个租户。一个执行器跨多个组织使用时，会以该租户的 1688 账号为其它组织抓取——这是归因问题（非数据泄露，商品为公网公开数据），本设计**明确接受并记录**。
- 升级路径（若将来确需组织级多 profile）：作为 **BACKLOG**，需独立设计，不在本次范围。

### D4：登录由运维在本机扫码一次，失效走显式重登

沿用 `sdslogin` 的 `ManualLogin` 模式（`internal/sdslogin/api.go:42`）：

1. 执行器提供 `login` 子命令，用**与采集完全相同的启动配置**打开可见窗口（§1.4 已证明这保证同身份）；
2. 人工扫码；
3. 执行器在**同一会话内**做一次真实提取自检，成功才写入 profile 并标记可用；
4. 采集时若检测到被重定向到登录页 ⇒ 该条失败为 `CHALLENGE`，**不自动重登**，等待人工重新扫码。

**不做自动登录、不做验证码代解。**

### D5：新增一个 list 路由，作为批量进度的唯一读入口

`GET /api/v1/workbench/sourcing/1688/local-agent-operations`

- 返回当前 Organization 下**可认领**的操作（过滤条件见 D2）与近期终态操作摘要（`id`/`state`/`failureCode`/`sourceURL`/`leaseUntil`）。
- 授权：`PermissionProductSourcingWrite` + `OrganizationAccessPolicyLiveWrite`。
  （本仓库**没有** `OrganizationAccessPolicyLiveRead`；只读路由也走 `LiveWrite`，先例 `internal/app/httpapi/account_audit.go:40`。）
- 分页复用 `internal/app/accountaudit/projection.go` 的 cursor 模式，**不新造分页协议**。
- 返回体**不含**证据正文（沿用 `EnvelopeSummary` 的有界摘要思路）。

> **代价提示（唯一触碰公共契约处）**：当前应用路由集是**精确白名单**，由 `validateCurrentApplicationRoutesInternal`（`current_application.go:338`）按 per-feature 开关逐项比对。新增此路由必须新增一个 flag 并**贯穿整条 wrapper 链**（`validateCurrentApplicationRoutesWithBrowserFeatures` → `...Internal` 及全部调用点）。

### D6：幂等键由服务端派生

- 单条 `idempotency_key` = `(规范化 sourceURL)` 的确定性派生，作用域为 `(organization, actor)`。
- 作用域内重复提交同一链接 ⇒ `Replayed`，不产生第二条操作。
- 同 key 不同载荷 ⇒ `ErrAcquisitionConflict`（沿用 `repository.go:143` 既有语义：比较 `Source`/`Fingerprint`/`CaptureSHA256`）。

## 4. 不变量

1. **幂等作用域 = `(organization, actor, key)`**；同一操作人的同一链接只对应一条操作。（**不声称**组织级去重，见 D1。）
2. **一次执行只有一个有效 fence**：任何写操作必须带最近一次 `Start` 返回的 `fence`；过期即 `ErrAcquisitionFence`。
3. **证据不跨租户**：服务端只按既有身份/租户作用域读写。
4. **服务端无 1688 凭据，也无 profile 引用**：不得新增任何 cookie/token/profile 持久化字段。
5. **失败不伪造成功**：未知结果必须是 `outcome_unknown`，不得降级为 `failed` 或 `published`。
6. **不新增第二事实源**：批量进度只能由采集操作派生。
7. **不新增租约续期 API、调度器、TTL 或 key GC**（§1.5）。

## 5. 状态、前置条件与持久化效果

复用既有状态机（`acquisition_operation.go:16-20`）：`acquiring → prepared → publishing → published`，失败 `failed`，不可判定 `outcome_unknown`。

完整时序见 §3 D2。持久化效果：只有 `product_acquisition_operations` 行 + publication receipt。**本设计不新增持久化表。**

## 6. 失败、重试、重启、取消、并发

见 §3 D2 的恢复表。补充：

| 场景 | 结果 |
|---|---|
| 提交响应丢失 | 用原 key `ByKey` / verify 核实（既有语义） |
| 被重定向登录页 | `CHALLENGE`，需人工重登；不自动重试 |
| 用户取消 | **本次必须实现**（§8.2：不做会造成终身额度无界泄漏）。取消仅限未认领（`acquiring` 且 `command IS NULL`）的行；已 `prepared` 的行走既有 `Finish(…, AcquisitionFailed, "CANCELLED")` |
| 并发执行器 | 见 §3 D2，「单组织单执行器」为明确约束 |

## 7. 授权与租户

- 所有路由：`AuthPolicyVerifiedIdentity` + `OrganizationAccessPolicyLiveWrite`（与 ③ 一致）。
- 执行器身份：沿用 `internal/localagent/deviceauth`（OAuth 设备码）与 `localagent.Actor{TenantID,UserID}`。
- **不引入**新的权限常量。
- profile 不进服务端，故无服务端 profile 校验；租户隔离由既有身份作用域承担（D3）。

## 8. 边界

| 项 | 取值 | 来源 |
|---|---|---|
| 单条 envelope | 2 MiB | `MaxEncodedEnvelopeBytes` |
| 命令体 | 2 MiB | `MaxAcquisitionCommandBytes` |
| **保留操作总数/组织（终身额度，无 GC）** | **256**，与 ①/③ 共用 | `MaxAcquisitionOperations` |
| 活跃操作数 | 32 | `MaxActiveAcquisitionOperations` |
| acquiring 租约 | **30 秒**（GET 租约，不可续期；靠过期重认领） | `AcquisitionLease` |
| 操作 deadline | 20 秒（服务端）/ 平台 `timeout` 120 秒（浏览器） | 既有 |
| 批量提交条数 | **10（第一轮，见 §8.1）** | 本设计 |
| 单次抓取上限 | 需 ≤ 120 秒，否则重认领次数超界 | 设计约束 |

### 8.1 终身额度：批量必须面对的硬上限

`MaxAcquisitionOperations = 256` 不是一个软阈值，而是**终身额度**：

- 容量检查是 `SELECT count(*) ... WHERE organization_id=?`，**不带任何状态过滤**（`repository.go:161-165`）⇒ 已 `published` 的行**照样计数**。
- 整个包**没有任何 `DELETE` 路径**（已 grep 确认），且 `docs/engineering/src2b-public-acquisition.md` 明确写了 *"no ... TTL or key GC"*。
- 256 是**编译期常量**，无配置项（`acquisition_operation.go:12`）。
- 只有 **INSERT** 消耗额度；同一 key 重复提交走 replay，不再消耗。⇒ **额度 = 该组织累计可采集的不同商品数，永久。**

算一下：**256 ÷ 10 = 25 批**，而且 ①/③ 共用同一额度、③ 是单条消耗。⇒ **一个组织累计采满 256 个商品后，采集能力永久失效，且没有任何恢复手段。**

这在“批量导入”这个功能上是致命的：功能的目标就是导入很多商品。三个选项：

| 选项 | 代价 |
|---|---|
| **(a) 接受并把剩余额度显式展示** | 零契约变更；UI 显示“剩余 N 条”，接近上限时提前告知 |
| (b) 提高 256 常量 | 改动一行，但这是 `src2b-public-acquisition.md` 已记录的契约上限 ⇒ 需同步修订该文档 |
| (c) 为终态行加保留期/GC | **推翻**已记录的 "no TTL/key GC" ⇒ 需独立评审 |

**建议先 (a)**：零契约变更、不多建平台，由真实使用触发 (b)/(c)。但必须在 UI 里真实展示，不能静默失败。

### 8.2 额度泄漏：为什么取消语义不能省

预建 `acquiring` 行的设计（§3 D2）在**提交时就消耗终身额度**。结合“无取消语义、无 GC”：

- 一个打错的链接、一个已下架的商品 ⇒ 该行永久停在 `acquiring`，**永久占用 1/256**。
- 一批 10 条全部无效 ⇒ 一次损失 10/256。

⇒ §10-2 “取消语义”从“可选”升为 **必须做**；否则这是一个无界泄漏。

（备选的“执行器持本地队列 + 抓完才 `StartPrepared`”**没有**这个泄漏，因为它只在抓取成功后才建行。代价是提交后在应用内看不到待办。若取消语义不实现，这个备选反而更安全——见 §10-2。）

> **对照 ①/③**：256 是三者共用。② 是唯一会“批量预建行”的 producer，所以额度问题在 ② 上才暴露。

## 9. 验证证据（实现时按此验收）

| 不变量 | 验证方式 |
|---|---|
| 幂等（操作人作用域） | 同操作人同链接提交两次 ⇒ `Replayed`，行数不增 |
| **跨操作人不重复的边界** | 同组织两个操作人提交同一链接 ⇒ **产生两行**（记录为已知行为，非缺陷） |
| 租约心跳 | 构造 >30 秒抓取 ⇒ 重认领后 `Prepare` 成功；不重认领则 `ErrAcquisitionFence` |
| 崩溃恢复 | 抓取中 kill 执行器 ⇒ 30 秒后重新出现在 list 中并可被认领 |
| 响应丢失 | 提交成功后丢弃响应 ⇒ `ByKey` 读回 `published` |
| 并发 fence | 两执行器同时认领 ⇒ 仅一方 `RowsAffected=1`；落败方 `Prepare` 得 `ErrAcquisitionFence` |
| 跨租户 | 用 Org B 身份访问 Org A 的操作 ⇒ 404/403 |
| 无凭据/无 profile 持久化 | 扫描持久化层，无新增 cookie/token/profile 字段 |
| 容量 | 达到 256 终身额度后新提交返回 `ErrAcquisitionCapacity`；UI 显示剩余额度 |
| 取消不泄漏额度 | 取消未认领行后，该链接可重新提交（不出现“永久占 1/256”） |
| 真实批量 | 3 条真实链接端到端，全绿并在应用内可见 |

## 10. 待决项（评审时必须给出结论）

1. ~~批量上限~~ **已定：单批 10 条**（见 §8.1）。理由：风控只在 5 条上验证过，10 是 2 倍的下一步；10 × ~40 秒 ≈ 7 分钟；且给 32 活跃额度和 256 终身额度都留出余量。做成常量 `maxLocalAgentBatchSize`，实测通过后再提。
2. **取消**语义——**必须做**（见 §8.2：不做会造成终身额度的无界泄漏），不再是可选项。
3. **多执行器**是否允许？（当前设计假定单组织单执行器；多执行器需要独立的租约/去重设计）
4. **登录态探测**：周期存活检查，还是一条失败即提示重登？
5. **执行器是否允许租户跨组织使用**（D3 已记录为「接受」；如需禁止则需服务端 profile/组织绑定字段，属 BACKLOG）

## 11. 冲突引用（需由协调方更新）

本设计与既有决定存在**直接冲突**；历史证据保留，**不写成 PASS**：

- **Issue #396（PAUSED）** 原本写着：
  > 匿名公开商品采集**不得依赖**本 Issue、SourceAccount、connectionStatus、**历史 profile/session** 或旧 SourceAccount owner。

  **2026-09-21 用户明确推翻该前提**：1688 采集**不再要求匿名**，账号态（持久化 profile）是允许的优先路径。处理方式：
  - 被推翻的**仅是“必须匿名”这一条前提**。本设计仍然**不依赖** #396 本身、`SourceAccount`、`connectionStatus` 或旧 SourceAccount owner——只依赖**独立的本机 profile**（§3 D3）。
  - #396 自身范围仍**保持 PAUSED**；其历史证据与结论保留。
  - 该决定**不授权**真实环境的数据访问/迁移/删除，也**不授权**使用共享凭据或他人账号。
- **`src2b-acquisition-v1`（#398 契约文档）** 排除登录流程的边界**不变**。本设计**不并入**该 contract，而是作为**独立 producer** 准入；需在 #398 记录该新 producer 的存在与边界（§1.5 第 1 条）。
- **`src2b-public-acquisition.md`** 的 "no background scheduler, fallback, rebase, TTL or key GC" 继续遵守（§3 D2）；但其中 "256 retained" 对 ② 构成终身上限，按 §8.1 处理。

## 12. Legacy 处置

```text
Legacy decision: EXTRACT
Reusable behavior: public → account-assisted 回退选择、按 (tenant, account) 串行加锁
Current owner: internal/product/sourcing（操作与证据）+ 新的本地执行器 ingress
Cutover/deletion condition: 新链路端到端可用后，删除 internal/crawler/alibaba1688 的
                            worker 装配（composition_builder.go:118）与旧 sourceaccount 边
```

- `internal/localagent` 的内存 `Job` 事实源按 **RETIRE** 处理：批量进度改由采集操作派生（D1/D5），其唯一消费者迁移完成后删除，**不做兼容层**。
- `internal/sourceaccount.SourceAccount.ProfileRef` **不被本设计引用**（F2 修正）。

## 13. 实现切片（每片可独立验证、可回滚）

1. **S1**：采集操作 list 路由 + 取消路由（D5，含精确白名单 flag）+ 分页复用 + 剩余额度展示 —— 独立可验证，无新事实源
2. **S2**：执行器 ingress（D2 认领/心跳/恢复）+ 幂等派生（D6）
3. **S3**：账号态执行器（D3/D4 + legacy 行为 EXTRACT）
4. **S4**：采集页批量入口 + 进度列表 UI（含单批 10 条校验）
5. **S5**：端到端真实批量验证（§9 末行）

每片默认是实现/提交边界，不单独开 PR；最终交付一个主要 PR。

## 14. v2 修正记录（对应 PR #443 评审）

| Finding | 评审结论 | 核实结果 | v2 处理 |
|---|---|---|---|
| **F1** `Claim` 无法认领 `acquiring` | BLOCKER | **部分成立**：「`Claim` 只处理 `prepared`」属实（`repository.go:286`）；但「无原子入口」**不成立**——`Start`（`repository.go:145-152`）就是原子认领入口。**但底下的洞是真的**：见 F4。 | §3 D2 补齐**发现 + 认领 + 恢复**的完整定义 |
| **F2** profile 引用挂在已 RETIRE 的 owner | BLOCKER | **成立**（`legacy-register.md:117`） | §3 D3 改为执行器本地 `LocalCollectorProfile`，服务端零持久化 |
| **F3** 幂等只在 actor 作用域 | BLOCKER | **成立**（`repository.go:338` 按 org+actor 过滤） | §3 D1 / §4-1 修正不变量表述；分类下调为 `IMPLEMENTATION_TEST`（不构成数据损坏） |
| **F4** 30 秒租约、无续期、无过期字段 | BLOCKER | **成立且最严重**：`AcquisitionLease=30s`、`Prepare` 要求 `lease_until>now()`（`repository.go:263`）、实测抓取 34.7s、`count(*)` 无状态过滤 ⇒ 256 为终身额度 | §3 D2 用**过期重认领当 fence 心跳**（零新 API）；§1.5/§8 补齐租约与容量事实；**明确不采用**租约续期方案 |
