# 服务端匿名浏览器采集（1688 public browser acquisition）架构设计

- Contract/Design ID: `src2b-public-browser-v1`
- Status: **DRAFT for Independent Architecture Review**
- Product Decision: `PD-1688-SERVER-PUBLIC-BROWSER-2026-09-26`
- Related: Issue #398（`src2b-acquisition-v1`）、#399（`browser_acquisition` producer）、#30
- Baseline: main `4e9a20081`
- Supersedes（仅列出的部分）: #398「不做绕过验证码/访问控制」；`PD-1688-BATCH-VERIFY-ON-CAPTCHA-2026-09-21`「不自动识别或绕过验证码」「不恢复服务端匿名裸 HTTP」

## 1. Product Outcome / Scope

**用户结果**：在 `工作台 → 供应 → 采集` 提交一个 1688 商品链接或 offer ID 后，服务端**不依赖任何 1688 登录态**，直接由真实浏览器取得公开商品页，并把 title/属性/SKU/价格/图片/描述映射进既有的 `SourceEnvelope → SRC-1 → Catalog` 链，用户随后可在采集详情页读到已发布商品。

**本次范围**

- 服务端 1688 匿名浏览器采集 provider（`PublicAcquirer` 实现），复用 fingerprint-chromium + Playwright。
- 自动检测并处理站点验证码/风控挑战页，使匿名采集在被挑战时仍有机会取得商品页。
- 沿用既有 `POST /verify`、`GET .../:operation_id`、`GET .../:operation_id/product` 与幂等/发布协议。
- 把旧浏览器采集能力 `EXTRACT` 到当前 owner。

**本次不做**

- 不使用、不持久化用户 1688 登录态、Cookie、密码、profile、session token；仍是匿名路径，不引入 SourceAccount/Connection。
- 不做多账号池、代理池、账号轮换、定时/周期重采、后台调度器。
- 不新建 Product/Catalog 第二事实源；不改 SRC-1/Catalog 事实规则。
- 不改浏览器插件 / Browser Capture / 本地执行器路线（#399 与批量设计继续有效）。
- 不做 UI/IA 变更（复用现有采集页与详情页）。

## 2. Product / Figma Authority

- 产品决定：`PD-1688-SERVER-PUBLIC-BROWSER-2026-09-26`（本设计的授权来源）。
- Figma：**N/A**。本设计不新增/改名页面、导航或交互，仅替换服务端 provider 实现；现有采集页（`/workbench/supply/acquisition`）不变。

## 3. Domain / Fact Owner

| 事实 | Owner | 本设计是否改变 |
| --- | --- | --- |
| untrusted acquisition evidence | `internal/product/sourcing`（`AcquisitionEvidence` 契约） | 否（仅新增一个 channel 取值） |
| SourceEnvelope / Normalize | `internal/product/sourcing` | 否 |
| 采集操作行（幂等/状态机） | `internal/integration/persistence/product/acquisition` | 否 |
| ProductSnapshot / Catalog version | `internal/product/catalog` | 否 |
| 采集应用编排与 HTTP | `internal/app/productsourcing` + `internal/app/httpapi` | 否（provider 注入点替换） |
| 浏览器采集叶能力（新增） | `internal/integration/acquisition/a1688` | **是**（新增 browser 子包） |

## 4. Contract → Implementation → Injection → Consumer

```
contract   sourcing.PublicAcquirer.Acquire(ctx, AcquisitionSource) (AcquisitionEvidence, error)
           sourcing.AcquisitionEvidence（含新增 Channel 语义 public_browser）
implementation
           internal/integration/acquisition/a1688/browser.Client   ← 新增（EXTRACT 自 legacy）
           复用 internal/crawler/shared/browser 的启动参数/反检测/验证码/提取器能力
injection  internal/app/httpapi/product_acquisition_application.go
           newCrawler1688HTTPModule 之外的 PublicAcquirer 构造点（当前 a1688.New()）
consumer   productsourcing.AcquisitionService.Acquire
           → MapAcquisitionEvidence(channel="public_browser")
           → PublicationIdentity → SRC-1 InternalProducer → Catalog
           → POST /verify / GET :operation_id / GET :operation_id/product
```

## 5. 关键设计决策

### D1. provider 采用「先取证据、再原子 StartPrepared」，不使用 `acquiring` GET 租约

现状（`acquisition.go`）：`Acquire` 先 `operations.Start` 建 `acquiring` 行（30s lease / 8s GET / 20s 操作 deadline），再 fetch，后 `Prepare`。
浏览器采集实测约 **19.6s**（冷启动 + 导航 + 提取），且挑战页处理可能更长，`acquiring` 的 30s 租约与 20s 操作 deadline 都不适合作为长任务栅栏。

⇒ 浏览器路径按 Browser Capture 的既有模式：先取 evidence 并 validate，再 `StartPrepared` 一次性原子写入 command。只有可确定性恢复的操作才有 durable 行；这与 `src2b-acquisition-v1` 的「没有 background scheduler / 租约续期 / TTL / key GC」零冲突。

### D2. 独立 provider 时间预算（**这是本设计最需要评审确认的一条**）

`sourcing.AcquisitionTimeout = 20s` 由 `AcquisitionService` 对**整条 Acquire**施加，实测浏览器路径 19.6s 已贴线，冷启动会直接超时并返回 `DEADLINE_EXCEEDED`。

⇒ 为浏览器 provider引入独立预算，`Acquire` 的 deadline 按 provider 类型选择：HTTP 路径保持 20s，浏览器路径使用 `BrowserAcquisitionTimeout`（建议 90s：覆盖浏览器冷启动 + 导航 + 最多一轮挑战处理 + 提取）。发布阶段（SRC-1/Catalog）仍受各自超时与幂等协议保护，不因该预算放大。

- 该预算必须**有界**，并与 HTTP 请求体读取超时、`MaxAcquisitionCommandBytes`、`MaxAcquisitionOperations` 的关系在实现中显式核对。
- 若评审认为不应按 provider 分叉 deadline，替代方案是在 provider 内做浏览器复用/预热把首次耗时压到 20s 内；但实测数据不支持该假设，故默认采纳独立预算。

### D3. 匿名、非持久化、零凭据的浏览器上下文

- 每个采集使用**临时、非持久化** user-data-dir，结束即删除；不使用 `config.Browser.UserDataDir` 的持久 profile。
- 不读取/写入任何 Cookie、storage、登录态；不调用账号 profile（`ProcessWithAccountProfile` 及其 `AccountProfileResolver`、`sourceaccount` 依赖 **不进入**新路径）。
- 复用 `NewPublicBrowserManager` 所表达的「干净非持久化 public 意图」，但实现必须落在当前 owner，且**不得**重新引入 public→account 回退（见 §9 Legacy）。

### D4. 验证码/挑战边界

- 允许自动处理站点验证码/风控挑战页（`PD-1688-SERVER-PUBLIC-BROWSER-2026-09-26`）。
- 处理能力 `EXTRACT` 自 legacy：验证码类型检测（滑块/点选/图片/文字/数学）+ 自动滑动轨迹。
- **不引入人工介入**：服务端无人工操作者。自动处理失败或超时 ⇒ 按 D5 如实失败，**不得**伪造成功、不得静默降级为「已成功」。
- 挑战处理受 D2 预算约束；同一采集的处理尝试次数有界（建议 ≤1 轮重试），不做无限重试。
- 真实挑战形态会变化，自动处理**不保证长期有效**；这是已接受风险的一部分（见 PD 文档），不是本设计的隐藏假设。

### D5. 失败 / unknown / 恢复

| 场景 | 结果 |
| --- | --- |
| 页面为挑战页且自动处理失败/超预算 | `ErrAcquisitionFailed` → HTTP 502 `SOURCE_UNAVAILABLE`（既有投影，前端已有清晰文案） |
| 取得证据但字段缺失 | 进入 `MissingFacts`/`Warnings`，不伪造字段（既有 mapper 行为） |
| 证据无法通过 `MapAcquisitionEvidence` | `ErrInvalidAcquisition` → 400 `INVALID_ACQUISITION` |
| 发布阶段 COMMIT 结果未知 | 沿用既有 `ErrAcquisitionUnknown` → 503 `OUTCOME_UNKNOWN`，`Verify`/`GET` 用**原 command** 核实，**不重新抓取换 publication ID** |
| 浏览器/驱动不可用 | `ErrAcquisitionUnavailable` → 503 `ACQUISITION_UNAVAILABLE` |
| deadline | `DEADLINE_EXCEEDED`（按 D2 预算） |

浏览器抓取**永不**直接决定已发布事实；只有 SRC-1 成功才算发布（既有不变量）。

### D6. 幂等与 publication identity

- 操作身份仍是 `(organization, actor, Idempotency-Key)`；等价 URL/offer ID 产生同一 SourceIdentity（既有 `Canonical1688Source`）。
- 同 key 同载荷重放；异载荷冲突。浏览器重抓**不**换 publication ID；确认提交结果未知时只 Verify/Read。
- 新 channel `public_browser` 进入 `MapAcquisitionEvidence` 的允许集合，并进入 `RawReference.Metadata["channel"]`；publication identity 计算方式不变。

### D7. 权限与租户边界

- 每次 acquire/verify/read 都按当前 verified identity + Effective Organization + live `product_sourcing.write` 重新授权（既有 `ContextAuthorizer` + `OrganizationAccessPolicyLive`）。
- 不新增权限常量、不新增路由、不新增认证路径。
- 浏览器进程本身不持有身份；Provider 不做授权判断，授权仍由 application 层在 provider 前后各一次执行（既有 `authorizeScope`）。

### D8. 资源与副作用边界

- **出网副作用**：新增服务端对 `detail.1688.com`（及同域跳转）的真实浏览器出网；必须只访问当前任务明确的 1688 公开商品资源。
- 并发：浏览器采集为稀有操作，需显式上限（建议单进程并发 ≤2，超出排队或 `ACQUISITION_CAPACITY`），避免把服务端资源打满。
- 响应/命令大小沿用 `MaxAcquisitionCommandBytes`（2 MiB）；提取字段数量沿用既有 `AcquisitionEvidence` 上限校验。
- 临时 profile 目录与浏览器产物在结束/失败时清理；不写入仓库目录。
- **出口 IP 风控**是已接受风险（PD 文档第 3 条），因此实现必须支持「被风控时快速、如实失败」，不得通过无限重试放大对 IP 的伤害。

## 6. 状态与持久化边界

- 不新增表、不新增列、不新增状态机。
- 浏览器路径只写既有 `product_acquisition_operations`（经 `StartPrepared`）+ publication receipt（SRC-1/Catalog owner 所有）。
- 首个 durable 副作用点仍是 `StartPrepared` 的 COMMIT；抓取失败不留下不可恢复行（与 browser capture 一致）。

## 7. 明确不改变

- 不改变状态机 `acquiring/prepared/publishing/published/failed` 的语义与迁移。
- 不改变 `AcquisitionOperationStore` / `InternalProducer` / Catalog 接口。
- 不改变既有 `public_http` 与 `browser_capture` channel 行为。
- 不改变幂等、重放、COMMIT unknown、恢复协议。
- 不改变授权、租户隔离与路由白名单。

## 8. 验证与验收方法

- provider 单测：挑战页 fixture、字段缺失 fixture、超预算 fixture、非预期 content-type、超大响应、驱动不可用。
- 真实链：current app → browser provider → `MapAcquisitionEvidence(public_browser)` → SRC-1 → Catalog → exact read-back，Product 不直接 seed。
- 幂等：同 key 重放、异载荷冲突、并发、响应丢失、SRC-1 commit unknown、cancel/deadline。
- 授权：Home A / Effective B、跨组织、撤权在 provider 前后正确拒绝。
- 资源：并发上限、临时目录清理、超预算快速失败。
- **真实 1688 网络验收**：由用户单独授权后执行；未授权则 `NOT_RUN`。受控 fixture 不得冒称真实平台 acceptance。
- 已知的匿名可靠性风险由 PD 记录为 `ACCEPTED_RISK`；实现只需如实投影结果。

## 9. Legacy decision

```text
Legacy decision: EXTRACT
Reusable behavior:
  - fingerprint-chromium 启动参数与反检测初始化脚本（shared/browser: launcher/launcher_context/fingerprint/random_config）
  - 浏览器生命周期管理（Install/LaunchPersistentContext/NewPage/Close）
  - 验证码检测与自动滑动（alibaba1688: captcha_*）
  - 页面导航/就绪/滚动（alibaba1688: page_operator）
  - 1688 DOM 结构化提取器（alibaba1688/extractor/*，20+ 提取器）
Current owner:
  internal/integration/acquisition/a1688（浏览器子包），输出仍为标准 AcquisitionEvidence
Cutover/deletion condition:
  新 provider 上线并验证后，internal/crawler/alibaba1688 的 Service/worker/API/account profile
  与 public→account 回退、internal/crawler/shared/browser 的旧调用方、internal/localagent 的
  匿名 runner 在无消费者后按独立任务 RETIRE；不建兼容层、不新增 tenantbridge consumer、
  不做旧数据/ID 迁移。
```

明确 `RETIRE`（不抽取、不保留、不引入）的部分：

- `internal/crawler/alibaba1688` 的 `Service`、worker pool、`APIService`、`/api/v1/crawl|tasks|stats` 路由。
- `AccountProfile` / `AccountProfileResolver` / `sourceaccount` 依赖 / public→account fallback。
- `tenantbridge` 相关消费与 ListingKit handoff 边。
- 旧 task/DTO/结果 Redis store 与 `CrawlerTask`/`CrawlerResult` 路径。

## 10. 未决 / 需评审确认

1. **D2 的 provider 时间预算**：是否接受按 provider 分叉 deadline（建议 90s），或要求把浏览器耗时压进 20s。
2. **D8 的并发上限**：建议 ≤2；需确认是否与部署形态（单进程/多副本）一致。
3. **验证码自动处理的失败语义**：确认「自动处理失败即如实 `SOURCE_UNAVAILABLE`，不做二次人工介入」。
4. **`public_browser` channel 命名**：需确认与 `src2b-acquisition-v1` 的 channel 语义扩展方式。
