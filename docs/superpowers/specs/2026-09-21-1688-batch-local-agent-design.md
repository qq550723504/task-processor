# 1688 批量/后台采集（本地代理执行器）设计

- 状态：**待评审**（未批准前不进入实现）
- 基准：`origin/main` = `4942901cb`
- 相关 Issue：#398、#399、#396（后续需按 §11 更新）
- 前置实测证据：本文件 §1.3

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
- 不做 1688 账号的租户级自助绑定（登录态由运维在本机建立，见 §4.3）
- 不改插件路径（③）的任何行为
- 不恢复 ①（服务端匿名裸 HTTP）—— 已证明结构性不可行
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

结论：**匿名路径不能作为产品前提**（可用性随 IP 声誉衰减到 0）；**登录 profile 在同样被污染的 IP 上 5/5 成功**。

两个附带的实现事实：

- Chrome 153 profile 交给 Chromium 144 会**启动即崩**，元凶是 `Default/Sync Data`；⇒ **profile 必须由最终运行的那个浏览器创建**，不能后期迁移。
- 1688 路径的 `SetFingerprint` **从未被调用**（只有 amazon/shein/sds 登录调用）⇒ 1688 的浏览器身份 = 二进制 + profile，登录与自动化天然同身份，不引入身份不一致。

## 2. 现状盘点（在 `4942901cb` 上复核）

### 2.1 可直接复用（不要重建）

| 能力 | 位置 | 复用方式 |
|---|---|---|
| 采集操作状态机 + 幂等 + 恢复 | `internal/product/sourcing/acquisition_operation.go`（`AcquisitionOperationStore`：`Start`/`ByKey`/`Prepare`/`Claim`/`Finish`） | 直接复用；批量 = N 个 operation |
| 浏览器采集 ingress（含 `StartPrepared` 恢复语义） | `internal/app/httpapi/browser_capture_application.go`（4 路由） | **作为本设计的 ingress 模板** |
| 账密登录自动化 + `WaitForLogin` + profile runtime | `internal/sheinlogin/`（`automation.go:121`、`automation_login_flow.go:261`、`profile_runtime.go`、`worker.go`） | 抽取其模式（不是包装） |
| 浏览器配置 / 代理 / 时区 | `internal/crawler/shared/browser/`（`launcher.go`、`launcher_context.go`） | 直接复用 |
| profile 根目录配置 | `platforms.alibaba1688.profileRootDir`（默认 `./.local/tmp/browser-profiles/1688-accounts`） | 直接复用 |
| 账号登记外壳 + UI | `internal/sourceaccountregistry` + `web/.../resources/source-accounts` | 只在需要用户自助绑定时才接 |

### 2.2 缺口（本设计要解决的）

| 缺口 | 证据 | 影响 |
|---|---|---|
| **localagent 的 job 是纯内存** | `internal/localagent/service.go:61-65`（`jobs map[string]*record`） | 重启即丢；再叠加 20 分钟 TTL，**不适合批量** |
| **localagent 与采集操作是两套事实** | localagent 有独立的 `Job`/`JobState`；采集操作有自己的 `AcquisitionOperation` | 第二事实源，违反 PD-GREENFIELD |
| **采集操作没有 list 路由** | 仅有 by-key / by-id 读单条 | 批量无法在应用内查看进度 |
| **local-agent 路由不在当前应用白名单** | `grep local-agent internal/app/httpapi/current_application.go` 为空；模块由 `composition_modules.go:26` 的另一条组装路径提供 | 不能直接复用，需新增独立路由 |
| **local-agent 无任何前端** | `grep -l local-agent web/` 为空 | 用户看不到 |
| **local-agent 走匿名** | `internal/localagent/runner.go:79` → `Process(ctx,url)` | 就是上表 0/8 的那条路 |
| **采集页的"贴链接"入口走 ①** | `acquisition-page.tsx:120` → 服务端裸 HTTP | 已证不可行 |
| **`ProcessWithAccountProfile` 无当前 owner 消费者** | 仅 `crawler_service.go:109` 的 legacy worker pool（见 §2.3） | 需要迁到当前 owner |

### 2.3 关于 legacy 账号辅助路径的说明（重要）

legacy crawler 里已经存在一套「public → account-assisted 回退」实现：

- `internal/crawler/alibaba1688/worker_processor.go:60-110`：先匿名 `Process`，失败且 `IsAccountFallbackEligible` 时回退到 `resolveAccountProfile` + `lockAccountProfile` + `ProcessWithAccountProfile`
- `internal/crawler/alibaba1688/crawler_service.go:109`：`Crawler1688Processor` 挂到 `worker.NewPoolWithConfig`
- 仅由 `composition_builder.go:118` 的 **legacy 组合**装配，受 `platforms.alibaba1688.enabled`（默认 `false`）门控

按仓库 Hard-Cut 规则，这套代码**不是当前 owner**，新链路不得依赖或包装它。本设计只 `EXTRACT` 其中仍然正确的行为（**profile 解析 + 按 profile 串行加锁 + 账号态执行**）到当前 owner，其余 `RETIRE`。

## 3. 核心设计决策

### D1：批量 = N 个既有采集操作，不新建队列实体

提交一批 N 个链接 = 创建 N 个 `AcquisitionOperation`（各自独立的 `Key`）。**不新增**批量实体、不新增 localagent 的 Job 事实源。

- 幂等单位 = 单条链接（沿用 `(Organization, Key)`）。
- 部分失败语义天然成立：一条失败不影响其它条。
- 容量沿用 `MaxAcquisitionOperations=256` / `MaxActiveAcquisitionOperations=32`；批量大小上限取其中的较小语义（待定，见 §10）。

**被否方案**：新建 `BatchJob` 聚合根 —— 会引入第二事实源、第二套状态机、第二套恢复协议，且没有它无法表达的信息。

### D2：本地代理作为采集操作的一个 ingress，`StartPrepared` 复用现有恢复语义

本地代理不拥有任何状态。它只做：

```
claim 一条待执行的操作 → 执行 → 回填证据
```

完全照搬 ③ 的 ingress 形状（`browser_capture_application.go` 的 4 路由），因为**它已经解决了同一个问题**（接收浏览器侧证据、支持响应丢失后的 `verify`/`by-key` 恢复）：

| ③ 现有 | 本设计 |
|---|---|
| `POST browser-captures` | `POST local-agent/1688-operations`（提交/继续一条） |
| `POST browser-captures/verify` | 同名语义（原 key 核实） |
| `GET .../by-key/:key` | 同名语义 |
| `GET .../:operation_id` | 同名语义 |
| — | **新增** `GET local-agent/1688-operations`（D5 列表） |

**关键点**：`AcquisitionOperationStore` 已含 `Claim`/`Finish`/`Fence`（`acquisition_operation.go:57-65`），**执行租约与原子性已存在**，本设计不需要新的并发控制机制。

### D3：登录态由本地持有，服务端不持久化任何 1688 凭据

- profile 目录留在执行器所在机器（`platforms.alibaba1688.profileRootDir` 之下）。
- 服务端**只存** `profileRef`（字符串引用），沿用 `sourceaccount.SourceAccount.ProfileRef` 的既有字段语义。
- 服务端**不接收** cookie / 密码 / token；沿用 `validation.ts` 的敏感名拒绝策略（`sensitiveName`）作为验收条件。

> 这是租户隔离的关键不变量：**服务端不得因本设计而获得跨租户读取本机 profile 的能力**。

### D4：登录由运维在本机扫码一次，失效走显式重登

沿用 `sdslogin` 的 `ManualLogin` 模式（`internal/sdslogin/api.go:42`）：

1. 执行器提供 `login` 子命令，用**与采集完全相同的启动配置**打开可见窗口（本设计 §1.4 已证明这保证同身份）；
2. 人工扫码；
3. 执行器在**同一会话内**做一次真实提取自检，成功才写入 profile 并标记可用；
4. 采集时若检测到被重定向到登录页 ⇒ 该条失败为 `CHALLENGE`，**不自动重登**，等待人工重新扫码。

**不做自动登录、不做验证码代解**（与既有约束一致）。

### D5：新增一个 list 路由，作为批量进度的唯一读入口

`GET /api/v1/workbench/sourcing/1688/local-agent-operations`，返回当前 Organization 下的操作摘要（`id`/`state`/`failureCode`/`sourceURL`）。

- 授权沿用既有：`PermissionProductSourcingWrite` + `OrganizationAccessPolicyLiveWrite`。
  （注意：本仓库**没有** `OrganizationAccessPolicyLiveRead`；只读路由同样沿用 `LiveWrite` 策略，先例见 `internal/app/httpapi/account_audit.go:40`。）
- **代价提示**：当前应用的路由集是**精确白名单**，由 `validateCurrentApplicationRoutesInternal`（`internal/app/httpapi/current_application.go:338`）按 per-feature 开关逐项比对。本设计的新路由必须新增一个 flag 并**贯穿整条 wrapper 链**（`validateCurrentApplicationRoutesWithBrowserFeatures` → `...Internal` 及其全部调用点），与之前为 browser 特性加 flag 的改动同型。这是本设计**唯一触碰公共路由契约**的地方。
- 另注：`internal/localagent/httpapi` 的 4 条路由**当前不在**这个白名单里（该模块由 `internal/app/httpapi/composition_modules.go:26` 的另一条组装路径提供，不进当前应用）。因此本设计**不复用**那 4 条路由，而是新增独立路径，避免触碰 legacy 组装。
- 分页与游标：复用 `internal/app/accountaudit/projection.go` 已有的 cursor 模式（`ReadFiltered` + `positionWire`），**不新造分页协议**。
- 返回体**不含**证据正文（沿用 `EnvelopeSummary` 的有界摘要思路），避免把 2 MiB envelope 灌进列表响应。

> 这是本设计**唯一新增的对外路由**。审批时请重点看它。

### D6：幂等键由服务端派生，不由客户端提供

- 单条 Key = `(sourceURL 规范化, organizationID)` 的确定性派生（沿用 `source_identity.go` 既有规范化）。
- 重复提交同一链接 = `Replayed`，不产生第二条操作。
- 同 key 不同载荷 ⇒ `ErrAcquisitionConflict`（沿用既有语义）。

**被否方案**：客户端生成 UUID 作为 Key —— 会让"用户重贴同一链接"产生重复操作，也正是批量场景最容易踩的坑。

## 4. 不变量

1. **单条链接幂等**：同一 Organization 下同一规范化 URL 只对应一条操作。
2. **一次执行只有一个持有者**：由 `Claim` + `Fence` 保证，租约过期可被重新 claim。
3. **证据不跨租户**：服务端只按 `Scope(Organization, Actor)` 读写；profile 无法被其他租户引用。
4. **服务端无 1688 凭据**：不得新增任何 cookie/token 持久化字段。
5. **失败不伪造成功**：未知结果必须是 `outcome_unknown`，不得降级为 `failed` 或 `published`。
6. **不新增第二事实源**：批量进度只能由采集操作派生。

## 5. 状态、前置条件与持久化效果

单条操作复用既有状态机（`acquisition_operation.go:21-27`）：

```
acquiring → prepared → publishing → published
                     ↘ failed
         ↘ outcome_unknown（仅当提交结果不可判定）
```

前置条件：
- 提交：已验证身份 + Effective Organization + `product_sourcing.write`
- claim：执行器为已登记设备（沿用 `deviceauth`），且持有有效租约
- finish：`Fence` 未过期且与 claim 时一致

持久化效果：只有 `AcquisitionOperation` + 其 publication receipt。**本设计不新增持久化表。**

## 6. 失败、重试、重启、取消、并发

| 场景 | 结果 |
|---|---|
| 执行器崩溃 | 租约到期，操作可被重新 claim（既有语义） |
| 提交响应丢失 | 用原 key `verify` / `by-key` 核实（既有语义） |
| 被重定向登录页 | `CHALLENGE`，需人工重登；不自动重试 |
| 单条失败 | 不影响同批其它条（D1） |
| 用户取消 | 不新增取消语义；未 claim 的操作随 TTL 过期（**待决**，见 §10） |
| 并发执行器 | `Fence` 保证只有最新 claim 能 `Finish`（既有语义） |

## 7. 授权与租户

- 所有路由：`AuthPolicyVerifiedIdentity` + `OrganizationAccessPolicyLiveWrite`（与 ③ 一致）。
- 执行器身份：沿用 `internal/localagent/deviceauth`（OAuth 设备码），权限 `local_agent.write`。
- **不引入**新的权限常量。
- profile 引用校验：执行时必须校验 `profileRef` 属于当前 Organization，否则拒绝（防止跨租户读取本机 profile）。

## 8. 边界

| 项 | 取值 | 来源 |
|---|---|---|
| 单条 envelope | 2 MiB | `MaxEncodedEnvelopeBytes` |
| 每次请求体 | `MaxAcquisitionCommandBytes` = 2 MiB | 既有 |
| 操作总数/租户 | 256 | 既有 |
| 活跃操作数 | 32 | 既有 |
| 批量提交条数 | **待定**，建议 ≤ 32（= 活跃上限） | §10 |
| 采集超时 | 20s（服务端）/ 平台 `timeout` 120s（浏览器） | 既有 |
| 执行器 job TTL | 沿用 `claimTTL` 15min / `jobTTL` 20min | 既有 |

## 9. 验证证据（实现时按此验收）

| 不变量 | 验证方式 |
|---|---|
| 单条幂等 | 同链接提交两次 → 第二次 `Replayed`，操作数不增 |
| 崩溃恢复 | 执行中 kill 执行器 → 租约过期后另一实例可 claim |
| 响应丢失 | 提交成功后丢弃响应 → `by-key` 能读回 `published` |
| 并发 fence | 两个执行器同时 claim → 旧 fence `Finish` 被拒 |
| 跨租户 | 用 Org B 身份访问 Org A 的 operation/profile → 403/404 |
| 无凭据 | 扫描持久化层，无新增 cookie/token 字段 |
| 真实批量 | 3 条真实链接端到端，全绿并在应用内可见 |

## 10. 待决项（评审时必须给出结论）

1. **批量上限**取多少？（建议 32）
2. **取消**语义要不要做？（不做则未执行的操作随 TTL 过期）
3. **profile 归属**：运维单一共享 profile（最小）vs 每 Organization 一个 profile（多租户）？这决定要不要接 `sourceaccountregistry` 的 UI。
4. **登录态探测**：是否需要周期性存活检查，还是一条失败即提示重登？
5. 是否允许同一 profile 并发？legacy 用 `lockAccountProfile` 串行；建议沿用串行。

## 11. 冲突引用（需由协调方更新）

本设计与既有决定存在**直接冲突**，必须由用户明确的阶段决定更新，不能由 Agent 自行放行：

- **Issue #396（PAUSED）** 明确写着：
  > 匿名公开商品采集**不得依赖**本 Issue、SourceAccount、connectionStatus、**历史 profile/session** 或旧 SourceAccount owner。

  本设计恰恰**依赖持久化 profile**。⇒ 需要用户明确「推翻 #396 的匿名前提，1688 采集改为账号态优先」，并更新该 Issue 的范围与冲突引用（历史证据保留，不写成 PASS）。
- 该决定**不授权**真实环境的数据访问/迁移/删除。

## 12. Legacy 处置

```text
Legacy decision: EXTRACT
Reusable behavior: public → account-assisted 回退选择、profile 解析、
                   按 (tenant, account) 串行加锁、账号态执行
Current owner: internal/product/sourcing（操作与证据）+ 新的本地执行器 ingress
Cutover/deletion condition: 当前 owner 的批量链路端到端可用后，
                            删除 internal/crawler/alibaba1688 的 worker 装配
                            （composition_builder.go:118）与旧 sourceaccount 边
```

`internal/localagent` 现有的内存 `Job` 事实源按 **RETIRE** 处理：批量进度改由采集操作派生（D1/D5），在其唯一消费者迁移完成后删除，不做兼容层。

## 13. 实现切片（每片可独立验证、可回滚）

1. **S1**：采集操作 list 路由（D5）+ 分页复用 —— 独立可验证，无新事实源
2. **S2**：执行器 ingress（D2，照搬 ③ 形状）+ 幂等派生（D6）
3. **S3**：账号态执行器（D3/D4 + legacy 行为 EXTRACT）
4. **S4**：采集页批量入口 + 进度列表 UI（D1 的前端）
5. **S5**：端到端真实批量验证（§9 最后一行）

每片默认是实现/提交边界，不单独开 PR；最终交付一个主要 PR。
