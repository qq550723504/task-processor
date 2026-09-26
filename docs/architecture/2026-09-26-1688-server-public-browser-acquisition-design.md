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
- **不把租户身份传入采集进程**：采集 RPC 不携带 org/actor/roles/租户 token（见 D13），因此 D7 的授权与 `product_sourcing.write` 判定**全部留在 `current-application`**，不因拆分而改变。
- 不做多账号池、代理池、账号轮换、定时/周期重采、后台调度器。
- **不新建队列/调度器/Saga/Admission Control 平台、不建第二事实源**；采集以**独立进程**（无凭据、RPC-only evidence 生产者，见 D13）同步被调，复用同一个二进制；**所有数据库（含 `product_acquisition`）仍只由 `current-application` 持有，采集进程不持任何库凭据/无库网络可达**；不改 SRC-1/Catalog 事实规则。
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
           ── 采集在独立进程/服务（D13）内运行，current-application 通过内部 RPC 同步调用
           ── 采集进程**无任何数据库凭据/无库网络可达**；所有库与操作行只在 current-application
injection  internal/app/httpapi/product_acquisition_application.go
           newCrawler1688HTTPModule 之外的 PublicAcquirer 构造点（当前 a1688.New()）
           → 改为指向采集服务内部 RPC 客户端
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

**先重放、后抓取（Codex finding #5）**：上述顺序若无条件执行，同 key 的 POST 在响应丢失后重试会**先启一次浏览器**，此时若 1688 被挑战/不可用，重试会在 provider 处失败，而**无法返回已可恢复的结果**——违反 D6 的同 key 重放保证，并对外部重复了一次副作用。

⇒ 浏览器路径**必须**在 acquisition 之前先走已授权的 `ByKey` 快速路径：

1. 授权 + `Canonical1688Source` + 规范化 key 成功后，先 `ByKey(scope, key)`。
2. 命中时**必须先执行 `sameAcquisition(request, op)`**（Codex finding #16）：它比对 `Fingerprint` 与 `Source`，不同商品**必须**返回 `ErrAcquisitionConflict`，不得直接把旧 offer 的已发布结果返回给新请求。只有比较通过才继续。
3. 命中后按状态处理（Codex finding #20）：
   - `publishing` / `published` / `failed` ⇒ 直接走既有 resolve/read，**不启浏览器**。
   - **`prepared` ⇒ 必须先 `operations.Claim` 再 resolve**。已核实 `AcquisitionService.resolve` 只接受 `publishing`/`published`，遇到 `prepared` 一律返回 `ErrAcquisitionUnknown`（`acquisition.go`）；且契约明文**没有后台调度器**。因此若把 `prepared` 直接丢给 resolve，每次同 key 重试都会返回 `OUTCOME_UNKNOWN`，**那条已持久化的 command 永远不会被推进**。这与既有 HTTP 路径一致：`Acquire` 在 `op.State == prepared` 时先 `Claim` 再 `resolve`。
4. **同 key 准入协调（Codex finding #10）**：`ByKey` 只能挡住“已有行”的情况。两个**首次并发**的同 key POST 会**同时**看到 not found 并各启一次浏览器，`StartPrepared` 只在两次出网抓取都发生后才仲裁。⇒ 需在 acquisition 之前对同 key 做**最小准入协调**（例如按 `(scope, key)` 的 singleflight/互斥，且多副本下仍以 `StartPrepared` 的原子性作为正确性依据）——协调只为**省掉重复的昂贵出网**，不作为幂等正确性的唯一依赖。
5. **抓取前的容量预检/准入（Codex finding #15 / #19）**：容量检查目前只在 `StartPrepared` 内（`repository.go` 的 `count.Total >= MaxAcquisitionOperations || count.Active >= MaxActiveAcquisitionOperations`），而 `ByKey` 对新 key 总是 not found ⇒ 组织已触顶时，每个新 key 都会**先启一次浏览器**、耗掉共享的浏览器/IP 预算，然后才拿到 `ACQUISITION_CAPACITY`；唯一 key 可以无限重复这一点。但它当前**没有可实现的合同路径**：唯一的容量计数是 `Repository.StartPrepared` 内的私有查询。⇒ 需在抓取前做**有界的容量预检/准入**（并对预检与抓取之间的竞态保持 `StartPrepared` 作为原子正确性栅栏），
   - **最小只读容量合同**：在 `AcquisitionOperationStore` 上新增**一个只读**方法（如 `CapacityAdmitted(ctx, scope) (bool, error)`，返回该 scope 是否还有 `MaxAcquisitionOperations` / `MaxActiveAcquisitionOperations` 额度），application 在抓取前调用；已触顶则直接返回既有 429 `ACQUISITION_CAPACITY`，**不启浏览器**。
   - 该预检是**优化与风控保护**，**不是**正确性来源：预检与抓取之间的竞态（以及多副本）仍由 `StartPrepared` 的原子容量检查兜住。
   - 这是 §7 中唯一的接口例外（只新增一个只读方法，不改现有方法语义）。
6. 覆盖：响应丢失后的同 key 重试、**并发首次同 key**、**同 key 异 offer**、**已触顶组织的新 key**、**`StartPrepared` 已提交但响应/进程丢失后停在 `prepared` 的恢复**。

**失败语义（与 HTTP 路径的差异，需评审确认）**：HTTP 路径用 `Acquire` 的 `Start` 建 `acquiring` 行，抓取失败会 `Finish(failed)`，同 key 之后**永久失败**（客户端须换新 key）。浏览器路径在 `StartPrepared` 之前失败**不建行**，因此同 key 可重试。这是**有意**的：浏览器失败多为可恢复的挑战/超时，允许重试比永久锁死更合理；但它改变了同一 `src2b-acquisition-v1` 契约下两条 provider 的可见失败行为，须在评审中显式确认（见 §11）。

### D2. 独立 provider 时间预算与三层 deadline（**这是本设计最需要评审确认的一条**）

`sourcing.AcquisitionTimeout = 20s` 由 `AcquisitionService` 对**整条 Acquire**施加，实测浏览器路径 19.6s 已贴线，冷启动会直接超时并返回 `DEADLINE_EXCEEDED`。

**评审已确认的补充（Codex finding #1，成立）**：仅按 provider 分叉 service deadline **不够**——HTTP 路由描述符本身也带 20s 上限：`productAcquisitionRoutes` 的每个 descriptor 设 `RequestTimeout: sourcing.AcquisitionTimeout` 并用 `WithRequestBodyReadTimeout(sourcing.AcquisitionTimeout, ...)` 包裹 handler。路由层的 parent context 会在 20s 取消整个请求，使任何更长的 provider 预算失效。

⇒ 必须显式定义**三层 deadline**，且在实现前分配各阶段预算：

| 层 | 当前值 | 浏览器路径要求 |
| --- | --- | --- |
| `WithRequestBodyReadTimeout`（**只管阻塞 body 读**） | 20s | **保持短值**（不放大）。它只用于中断卡住的 body 读取，不应承担整个操作预算 |
| route `RequestTimeout`（从请求进入就开始计时） | 20s | 总量 = body 读预算 + 浏览器预算 + 发布预算（含余量） |
| `AcquisitionService.Acquire` 的 `context.WithTimeout` | 20s（**包住整个方法**） | 改为：**外层服务预算 = provider 预算 + 发布阶段预算**；浏览器预算只施加于 provider **子 context**，**不得**让整个 `Acquire` 共用同一个到期即死的 child context（见下） |
| provider 内部（浏览器导航/验证码/提取） | 8s GET | 独立有界子 context，严格小于其上层 deadline，保证能及时如实失败 |
| 发布阶段（`Prepare`/`Claim`/`Publish`/exact read） | 受各自既有边界 | 不被浏览器预算放大；由外层服务预算单独保证 |

- **不可把 `WithRequestBodyReadTimeout` 扩成整个操作预算**（Codex finding #8）：那会削弱现有 slow-body 资源护栏；同时 route deadline 在 body 读之前就开始计时，若不把 body 读计入总量，合法 body 也可能在采集开始前就被取消。
- **发布必须在外层预算内（Codex finding #9）**：实测 `internal/app/productsourcing/acquisition.go:31` 的 `Acquire` 用**一个** child context 包住整个方法，而 `resolve`（`Claim`/`Publish`/exact read）用的就是它。若直接把 provider 预算设成 90s，导航+验证码就会吃掉全部预算，**采集成功但发布阶段拿到已到期的 context**；调大 route deadline 也救不了已取消的 child context。⇒ 必须**重构为「外层 = provider+发布 预算」+「provider 子 context = 浏览器预算」**两层。已核实：采集路由的 route 精确白名单只断言 method+path（`current_application.go:487-492`），**不含 `RequestTimeout`**（只有 `acquisition-main-image` 路由断言 30s），因此调整采集路由超时不会碰 guard。
- 该预算必须**有界**，并与 HTTP 请求体读取超时、`MaxAcquisitionCommandBytes`、`MaxAcquisitionOperations` 的关系在实现中显式核对。
- 若评审认为不应按 provider 分叉 deadline，替代方案是在 provider 内做浏览器复用/预热把首次耗时压到 20s 内；但实测数据不支持该假设，故默认采纳独立预算。
- **不得**仅改 service deadline 而不改路由描述符；否则当前设计的主路径（提交链接→拿到已发布商品）在冷启动/验证码场景下无法完成。

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
| **服务端生成的证据无法通过 `MapAcquisitionEvidence`**（页面形状变化、提取器漏字段等） | **按 provider/解析失败处理 ⇒ 502 `SOURCE_UNAVAILABLE`**，与 HTTP 路径 `failFetch` 一致。**不得**投影成 400 `INVALID_ACQUISITION`：400 表示**调用方请求非法**，而这里拒绝的是服务端自己产出的输出（Codex finding #6） |
| 调用方请求非法（URL/offer ID 不可规范化、key 非 UUID、body 畸形） | 400 `INVALID_ACQUISITION`（与 HTTP 路径一致） |
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

- **出网副作用**：新增服务端真实浏览器出网；其 host/子资源/重定向约束由 D12 强制，不是口头约定。
- 并发：浏览器采集为稀有操作，需显式上限（建议单进程并发 ≤2，超出排队或 `ACQUISITION_CAPACITY`），避免把服务端资源打满；在 D13 形态下该上限落在**采集容器**（可独立调），不与 API 进程共享。
- 响应/命令大小沿用 `MaxAcquisitionCommandBytes`（2 MiB）；提取字段数量沿用既有 `AcquisitionEvidence` 上限校验。
- **出网响应体也要有界（Codex finding #17）**：现有 `src2b-acquisition-v1` 已有“压缩与展开后各限 2 MiB”的 provider body 上限（`docs/engineering/src2b-public-acquisition.md`）。该上限**必须一并带到浏览器路径**：被允许的 1688/CDN origin 若返回超大文档/子资源，Chromium 会在页内证据上限生效**之前**就把它物化进内存，`MaxAcquisitionCommandBytes`（只管后续 publication command）管不到。⇒ 需在**浏览器/代理边界**定义导航与子资源的单响应上限与**累计下载上限**，超限即中止该次采集（不静默继续）。
- **物化前限长（Codex finding #13）**：恶意或变形页面可在 `variants`/`images`/`attributes` 子树送出超大数据，而 `page.Evaluate`（CDP）与内部 RPC 都会**先把它反序列化进内存**，之后 `MapAcquisitionEvidence`/`MaxAcquisitionCommandBytes` 才拒绝——那时已经晚了。⇒ 必须：
  - **页内取数时就限量**：在页面上下文里对每个集合施加条目数上限与累计字节上限（只截取有界子集，不整棵搬运），并把超限作为 `source_too_large` 类失败而非静默截断；
  - **RPC 侧在解码前限长**：用 `io.LimitReader`（复用 `httproute.WithRequestBodyReadTimeout` 思路）对响应体做硬上限，超限直接断开/报错，**不先 `json.Unmarshal` 整包**；
  - 验收：构造超大 variants/images/attributes 的 fixture，证明页内与 RPC 两道上限各自生效；并构造超大文档/子资源响应，证明浏览器侧响应/累计下载上限生效。
- 临时 profile 目录与浏览器产物在结束/失败时清理；不写入仓库目录。
- **出口 IP 风控**是已接受风险（PD 文档第 3 条），因此实现必须支持「被风控时快速、如实失败」，不得通过无限重试放大对 IP 的伤害。

### D9. 真实页面形状与解析策略（依据 #399 实测，避免重复已知缺陷）

`docs/superpowers/specs/2026-09-20-issue399-1688-context-locator-design.md` 已实测真实 1688 页面：

- `window.context` 的赋值右边是 **IIFE**（`(function(b,d){...})(window.contextPath,{...})`），**不是** JSON 字面量；
- 真实载荷含**裸数字对象键**（`skuWeight:{6290953586037:0.3}`），**不是严格 JSON**；
- 归一化后节点数实测 **16501**，超过现有服务端 `boundedJSONDepth` 的 `nodes ≤ 4096`。

⇒ 服务端浏览器 provider **不复制**插件的「解析脚本文本」路径，而是：**在已渲染页面中直接求值 `window.context`**（Playwright `page.Evaluate`），在页内只取出映射所需的有界子树，再在服务端复用**当前 owner 已有**的字段映射（`public.go` 的 `contextDocument` 字段路径）。理由：

- 我们控制了页面（已导航、已执行页面 JS），求值 `window.context` 与插件「不得 eval / 不读全局」的约束前提不同；插件约束来自其在用户浏览器中运行，不适用于服务端受控浏览器。
- 直接求值天然规避 IIFE 定位、裸键、脚本文本体积三个已实测缺陷，且无需新增跨语言解析库。
- 只取所需子树即可保持有界，不需要放宽服务端 `nodes ≤ 4096`。

`ContentSHA256` 定义为**被映射字段子树的规范化 JSON 摘要**（而非整页字节），保证同一商品的可重放摘要稳定。价格：真实页面价格 key 尚未验证（#399 §2.5 为 `NOT_RUN`），取不到时进入 `MissingFacts`，**不伪造**。`MaxSourceEnvelopeCollectionItems` 的重复项折叠（`foldSourceEnvelopeRepeats`）已是当前实现，413 风险已收敛。

### D10. 浏览器依赖边界（Legacy 关键决策）

`internal/crawler/shared/browser`（2385 行）被 `internal/crawler/*`、`sdslogin`、`sheinlogin` 等共用，属产品文档定义的**提取/退休区**（`internal/crawler`），不是新代码的合法依赖。

⇒ **新 owner 不 import `internal/crawler/*`**。采用：

- 新包 `internal/integration/acquisition/a1688/browser` 直接用 `github.com/mxschmitt/playwright-go` 持有**最小**启动/反检测/导航/验证码/求值能力；
- 只 `EXTRACT` 行为（启动参数/反检测 init script/验证码检测与滑动），不 wrap、不 import 旧包；
- 该决定与 `product-sourcing-handoff.md`「`internal/crawler` 是提取/退休区」一致。

替代方案（评审可要求）：把 `shared/browser` 提升为当前共享基础设施并更新全部调用方——范围远大于本切片，默认不做。

### D11. `public_browser` channel 契约变更必须覆盖持久化校验（Codex finding #2）

**评审确认（成立）**：`public_http` 在非测试代码里恰好有 3 个消费点，只改 `MapAcquisitionEvidence` 会让浏览器 command 在发布前被拒：

| # | 位置 | 当前 | 变更 |
| --- | --- | --- | --- |
| 1 | `internal/product/sourcing/acquisition.go` `MapAcquisitionEvidence` | `channel != "public_http" && channel != "browser_capture"` → 拒绝 | 允许 `public_browser` |
| 2 | `internal/integration/persistence/product/acquisition/repository.go` `validCommand` | 空 `CaptureSHA256` 分支要求 `metadata["channel"] == "public_http" && metadata["capture_sha256"] == ""` | 允许 `public_browser` 且仍要求 `capture_sha256 == ""` |
| 3 | `internal/app/productsourcing/acquisition.go` 公共抓取调用 | 传入 `"public_http"` | 浏览器路径传入 `"public_browser"` |

- `public_browser` 操作的 `CaptureSHA256` **保持为空**（内容无关指纹，见 D1），因此 `validCommand` 必须走空-`CaptureSHA256` 分支，不得把它当作 browser-capture 处理。
- 持久化 command 是 durable 事实；恢复/回读路径必须能验证 `public_browser`，否则重启后无法读回（`validCommand` 是 `StartPrepared` 的校验，且 command 会被持久化并在恢复时重校验）。
- producer 仍为 `public_acquisition/v1`，**不新增 producer kind**。

### D12. 出网安全边界（SSRF）——必须作用在**连接层**（Codex finding #3 / #4）

**评审确认（成立，且第二次复核指出了更强的约束）**：HTTP provider 对每个重定向重新校验，`httpimage.NewPublicImageHTTPClient` 在 dial 层拒绝私有/回环地址；而 legacy 浏览器代码**无任何请求拦截**。

**第一版设计（已废弃）**：仅用 Playwright `BrowserContext.Route`/`Page.Route` 校验“解析后的 IP”。**这不可执行**：route 回调发生在 Chromium **解析目的地之前**，playwright-go 只在 `Response.ServerAddr`（响应到达后）才暴露对端地址，因此无法把一次独立的 DNS 查询绑定到 Chromium 的 socket ⇒ **TOCTOU SSRF 绕过**（DNS rebinding / 私有地址均可绕过字符串校验）。

⇒ 边界必须落在**实际连接点**，而非请求回调：

1. **连接层强制**（唯一可信执行点）：由**采集进程的出网白名单**落实（D13）——该进程**不持有任何数据库凭据、且网络命名空间不放行内网 DB**，只需允许出站到已校验的公开 1688/CDN 地址；未列出的目的地（回环、链路本地、私网、保留、云元数据）在网络层不可达。**或**用 Chromium `--host-resolver-rules` 将允许 host 显式 pin 到已校验 IP、其余 `~NOTFOUND`，从根上消除 rebinding。二者**至少其一为强制**；路由拦截仅作为**额外的**纵深防御。因为浏览器与 DB 既不同进程也不同网络命名空间，同进程 SSRF 通道被结构性切断。
2. **纵深防御（仍需要，但不能单独依赖）**：`BrowserContext.Route`/`Page.Route` 按 **host/URL 字符串**拦截导航与子资源，只允许本任务明确的公开 1688/CDN origin（建议 `detail.1688.com`、`m.1688.com`、商品页必需的 `*.alicdn.com`），其余 abort；重定向逐跳重校验，fail-closed。
3. **覆盖面**：必须同时覆盖 service worker、WebSocket 及 `fetch`/`xhr` 等非文档请求，以及浏览器预加载/预连接；不得只拦主文档导航。
4. **运行时环境**：浏览器在**采集进程**（D13 独立网络命名空间）内运行；出站仅白名单，且**无内网 DB 可达性、无 DB 凭据**。
5. **部署门禁**：上述未在目标部署形态落地前，**不得**把该 provider 接到生产路由；必须同时验证「采集进程无法连到任何内网 DB」这一可观测事实。

### D13. 部署形态：独立采集进程（用户决定 2026-09-26）

**用户决定（“按建议”）**：采纳**独立部署采集进程**、**同步被调**；不引入队列/调度器/第二事实源。

**关键修正（Codex finding #12，成立）**：首版写“复用同一个 `product_acquisition` 库”是**错的**，且与 D12 自相矛盾——若采集进程持有库凭据，其网络命名空间就必须放行到数据库，于是同容器的 Chromium **继承该可达性**，D12「浏览器不得访问内网 DB」就不成立；而 URL 层 Playwright 拦截按 D12 已被降为**不可单独依赖**的控制，无法补洞。

⇒ 采集进程必须是**无凭据的 evidence 生产者（RPC-only）**：

| 位置 | 拥有 | 不拥有 |
| --- | --- | --- |
| `current-application` | 全部库（product/source/commercial/membership）、操作行、SRC-1/Catalog 发布、授权与租户校验 | 不启动浏览器 |
| 采集进程 | Chromium、页面取数、`AcquisitionEvidence` 构造 | **无任何数据库凭据与数据库网络可达性**；不写操作行；不做授权判断；不持有租户身份 |

- 这使 D12 **结构性成立**：采集进程的网络只需放行 1688/CDN 白名单出站，不需要放行内网 DB。
- **调用方认证（Codex finding #14，成立）**：“不出网/不持租户凭据”解决了**租户隔离**，但**不等于**只有 `current-application` 能调用。已核实：现有采集路由带 `AuthPolicyVerifiedIdentity` + `OrganizationAccessPolicyLiveWrite` + `Permission: product_sourcing.write`；而采集 RPC 入参刻意不携带这些（正确），若仅靠“内部网络可达”，则同网络内**任何**工作负载都能驱动 Chromium，绕过 D7 的授权门禁，并可任意消耗共享的浏览器/IP 预算（放大已接受的 IP 风控风险）。已核实**仓库当前没有**可复用的 mTLS / 服务身份机制。
  ⇒ 必须有**可执行的调用方准入**，二选一（实现前定稿）：
  - **网络准入**：采集 RPC 端口只绑定/只放行来自 `current-application` 所在网络命名空间/主机，且**不对其他 compose 服务与宿主机暴露**；或
  - **服务身份**：为 `current-application ↔ collector` 建立双向认证（mTLS 或独立的服务凭据），请求必须携带可验证的服务身份。
  两者均需**可观测验证**（未授权来源的调用被拒），并作为**部署门禁**：未落实则**不得**接生产路由。该控制是**服务间认证**，**不涉及**租户身份与用户凭据。
- RPC 契约最小化：入参仅 canonical source URL / offer ID（**不携带** org/actor/roles/token），出参为有界的 `AcquisitionEvidence`（含本路径的 `channel`/`parserVersion`/`contentSHA256`）。身份与授权仍在 `current-application` 内完成。
- 幂等/重放/恢复、`ByKey` 快速路径、`StartPrepared`、发布全部在 `current-application` 侧（既有操作行），**不因拆进程而改变任何状态机或事实 owner**。
- 同步被调，保持既有 `acquiring → prepared → publishing → published` 语义；不引入队列、调度器、TTL 或 key GC。
- 复用**同一个二进制**（不同启动参数/子命令），但**不共享数据库句柄与凭据**。
- 代价（已知并接受）：新增一个部署面（compose 服务、镜像、限额、升级/回滚）。`AGENTS.md` 禁止的是“预建平台/调度器”，本形态两者都不是；真实开新部署面仍需在实现 Issue 中明确，并单独授权部署。
- **D8 并发上限**落到采集进程/容器（可独立调）；**D12 连接层强制**即“采集进程出站白名单 + 无 DB 可达”。

## 6. 状态与持久化边界

- 不新增表、不新增列、不新增状态机。
- 浏览器路径只写既有 `product_acquisition_operations`（经 `StartPrepared`）+ publication receipt（SRC-1/Catalog owner 所有）。
- 首个 durable 副作用点仍是 `StartPrepared` 的 COMMIT；抓取失败不留下不可恢复行（与 browser capture 一致）。

## 7. 明确不改变

- 不改变状态机 `acquiring/prepared/publishing/published/failed` 的语义与迁移。
- 不改变 `AcquisitionOperationStore` 的**既有方法语义**与 `InternalProducer` / Catalog 接口。**唯一例外**：为 D1 的抓取前容量预检（finding #15/#19）新增**一个只读容量查询方法**（见 D1 第 5 条与本节下方说明），不改变 `Start`/`ByKey`/`Prepare`/`Claim`/`Finish`/`StartPrepared` 的任何现有语义。
- 不改变既有 `public_http` 与 `browser_capture` channel 行为。
- 不改变幂等、重放、COMMIT unknown、恢复协议。
- 不改变授权、租户隔离与路由白名单。

## 8. 验证与验收方法

- provider 单测：挑战页 fixture、字段缺失 fixture、超预算 fixture、非预期 content-type、超大响应、驱动不可用。
- **重放优先**：同 key POST 在响应丢失后重试、并发同 key 重试，**均不得启动浏览器**，直接由 `ByKey` 返回可恢复结果（finding #5/#10）；**同 key 换 offer 必须返回 `ErrAcquisitionConflict`**（finding #16）；**已触顶组织的新 key 不得先启浏览器**（finding #15/#19，依赖 §7 的只读容量合同已实现）；**停在 `prepared` 的操作必须被 `Claim` 推进而不是长期 `OUTCOME_UNKNOWN`**（finding #20）。
- **失败归因**：页面形状变化/提取器漏字段导致服务端证据被拒时，投影为 502 `SOURCE_UNAVAILABLE` 而非 400；仅调用方请求非法才 400（finding #6）。
- **出网边界（finding #4）**：证明连接层强制真实生效——本地构造指向回环/链路本地/私网/元数据地址的被允许 host，浏览器**必须无法建连**；并覆盖重定向、service worker、WebSocket、`fetch`/xhr、预连接；未落地则不得接生产路由。
- **调用方准入（finding #14）**：在部署形态中可观测地证明**只有 `current-application` 能调用采集 RPC**（未授权来源被拒），且该控制不引入租户身份/用户凭据。
- **凭据与可达性（finding #12）**：在部署形态中可观测地证明**采集进程无法连到任何内网 DB**，且不持有任何 DB 凭据（凭据/环境变量检查 + 网络探测）。
- **发布阶段预算（finding #9）**：构造 provider 用满自身预算但仍成功的场景，证明 `Claim`/`Publish`/exact read 仍在外层预算内完成，不因同一个 child context 提前到期。
- **物化前限长（finding #13）**：超大 `variants`/`images`/`attributes` fixture 证明页内上限与 RPC 解码前上限各自生效，且不先整包反序列化。
- **deadline（finding #8）**：slow-body 请求被短 body 读超时中断（不得因放大而失去护栏）；合法 body 在 route 总预算内完成采集与发布。
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

## 10. 评审 finding 记录（Codex Review，commit `2dfbc83059`）

| # | Finding | 分类 | 处置 |
| --- | --- | --- | --- |
| 1 | 只分叉 provider deadline 不够：路由 `RequestTimeout`/`WithRequestBodyReadTimeout` 仍 20s，parent context 会在 20s 取消，冷启动/验证码场景主路径无法完成 | **BLOCKER（成立，已修）** | 命中「核心 happy path 按当前设计无法完成」。已重写 D2：定义 route/service/provider 三层 deadline 与发布阶段预算 |
| 2 | 只把 `public_browser` 加进 `MapAcquisitionEvidence` 会让持久化 `validCommand` 以 `ErrInvalidAcquisition` 拒绝每个浏览器 command，且恢复路径无法校验 | **BLOCKER（成立，已修）** | 命中「核心 happy path 无法完成」。已新增 D11，枚举 3 个消费点并明确 `CaptureSHA256` 保持为空、不新增 producer |
| 3 | 浏览器默认跟随重定向/子资源，legacy 代码无请求拦截，可能请求回环/私网/元数据地址（SSRF） | **BLOCKER（成立，已修）** | 命中「新的跳系统安全边界」。已新增 D12：路由拦截 + 允许 origin 集 + 解析地址拒绝私网 + 逐跳重校验 + fail-closed |

三条均为**真实设计缺陷**，不需要新 Broad 产品决定；已直接修正设计。修正只改变设计文本，未修改生产代码。

### 10.1 第二轮增量复核（commit `6359b20d7`）

| # | Finding | 分类 | 处置 |
| --- | --- | --- | --- |
| 4 | 上一轮的 D12 仍不可执行：`BrowserContext.Route` 在 Chromium **解析目的地之前**运行，playwright-go 仅在响应后的 `Response.ServerAddr` 暴露对端地址，无法把独立 DNS 校验绑定到实际 socket ⇒ TOCTOU SSRF 绕过；且未覆盖 service worker / WebSocket | **BLOCKER（成立，已修）** | 命中「新的跳系统安全边界」且绕过显式出网控制。已重写 D12：**强制点必须在连接层**（出网代理/拨号/防火墙校验，或 `--host-resolver-rules` 将允许 host pin 到已校验 IP），路由拦截降为纵深防御；并要求隔离出网环境 + 部署门禁 |
| 5 | 同 key 重试会先启浏览器：响应丢失后重试未先查 durable 操作，可能在 provider 失败而无法返回可恢复结果，并重复外部副作用 | **IMPLEMENTATION_TEST（成立，已修）** | 影响 Must「幂等重放」。已在 D1 增加**先重放后抓取**：`ByKey` 快速路径，仅 confirmed not found 才抓取；并要求覆盖响应丢失 + 并发同 key 重试 |
| 6 | 服务端生成的证据被 `MapAcquisitionEvidence` 拒绝时投影为 400 `INVALID_ACQUISITION`，错误归因于调用方 | **IMPLEMENTATION_TEST（成立，已修）** | 影响 Must「如实报告失败」。已改 D5：服务端产出失败 ⇒ 502 `SOURCE_UNAVAILABLE`（与 HTTP 路径 `failFetch` 一致）；400 仅用于调用方请求非法 |
| 7 | PD 已标 ACTIVE，但被推翻的批量设计原文仍无 supersession 标记，可被发现并重新适用 | **BACKLOG（成立，已修）** | 仓库权威维护项。已在批量设计头部与 §1.2/§1.3 对应条款就地加 `SUPERSEDED` 标记并链接本 PD，保留历史文本；明确**其余设计继续有效** |
| 8 | `WithRequestBodyReadTimeout` 是阻塞 body 读护栏，扩成整个操作预算会削弱 slow-body 护栏；且 route deadline 在 body 读前开始计时 | **IMPLEMENTATION_TEST（成立，已修）** | 影响 Must「提交链接→拿到已发布商品」与资源边界。已改 D2：body 读超时保持短值，route 总预算 = body 读 + 浏览器 + 发布 |

### 10.2 第三/四轮增量复核（commits `040975c89`、`54acd01ec`）

| # | Finding | 分类 | 处置 |
| --- | --- | --- | --- |
| 9 | `Acquire` 用**一个** child context 包住整个方法；若直接给 provider 90s，导航+验证码吃满预算后**发布阶段拿到已到期 context**，调大 route deadline 也救不了；主路径在预算边界无法完成 | **BLOCKER（成立，已修）** | 命中「核心 happy path 无法完成」。已改 D2 为**两层预算**：外层 = provider + 发布，浏览器预算只施加于 provider **子 context**；并记明已核实采集路由白名单只断言 method+path、改超时不会碰 guard |
| 10 | `ByKey` 只能拦“已有行”；两个**首次并发**同 key POST 会同时看到 not found 并各启一次浏览器，`StartPrepared` 只在两次出网后才仲裁 | **IMPLEMENTATION_TEST（成立，已修）** | 影响 Must「幂等重放」与已接受的共享 IP 风险。已在 D1 增加**同 key 准入协调**（singleflight/互斥），并明确协调只为省掉重复出网，正确性仍依赖 `StartPrepared` 原子性；验收新增“并发首次同 key” |
| 11 | PD 承诺架构阶段修订，但 `docs/engineering/src2b-public-acquisition.md` 未更新，仍写死 `window.context`-only parser 与 20s/8s 预算 | **BACKLOG（成立，已修）** | 仓库权威维护。已在该文件头部加**定向暂停说明**：仅对浏览器 provider 暂停静态 HTML 专用条款（parser 形态、8s GET/20s operation deadline、单一 provider 措辞），列出替换项；**identity/证据语义/SRC-1/Catalog/幂等/重放/COMMIT-unknown/授权/上限/不登录前提均继续对两个 provider 生效** |
| 12 | D13 让采集进程持库 → 网络命名空间必须放行 DB → 同容器 Chromium 继承可达性 → **与 D12 自相矛盾**；URL 层拦截已被降为不可单独依赖，无法补洞 | **BLOCKER（成立，已修）** | 命中「新的跳系统安全边界」。已重写 D13：采集进程改为**无凭据 evidence 生产者（RPC-only）**——无任何 DB 凭据/无 DB 网络可达、不写操作行、不做授权；`current-application` 保留全部库、授权与发布。D12 由此**结构性成立**；并新增可观测门禁「采集进程无法连到任何内网 DB」 |
| 13 | 恶意页面可在页内造超大数据，CDP `page.Evaluate` 与 RPC 都会**先反序列化进内存**，之后才被 `MaxAcquisitionCommandBytes` 拒绝 | **IMPLEMENTATION_TEST（成立，已修）** | 影响采集可用性与资源边界。已在 D8 增加**物化前限长**：页内取数即对集合施加条目/字节上限（超限作 source_too_large 类失败）、RPC 侧 `io.LimitReader` 在解码前硬限长；§8 增加超大 fixture 验收 |

### 10.3 第五轮增量复核（commit `87b7aa178`）

| # | Finding | 分类 | 处置 |
| --- | --- | --- | --- |
| 14 | 采集 RPC **刻意不带凭据**，若仅靠“内部网络可达”，则同网络内**任何**工作负载都能驱动 Chromium，绕过 D7 的 `product_sourcing.write` 门禁并任意消耗共享浏览器/IP 预算 | **BLOCKER（成立，已修）** | 命中「绕过明确的访问控制」。已核实现有采集路由带 `AuthPolicyVerifiedIdentity` + `OrganizationAccessPolicyLiveWrite` + `product_sourcing.write`，且**仓库无可复用的 mTLS/服务身份机制**。已在 D13 要求**可执行的调用方准入**（网络准入：只对 `current-application` 命名空间/主机开放；或服务身份：mTLS/服务凭据）、要求**可观测验证**并列为**部署门禁**；并明确该控制是**服务间认证**，不引入租户身份/用户凭据 |
| 15 | 组织已达 2048/32 上限时，`ByKey` 对新 key 仍 not found ⇒ 每次都**先启浏览器**、耗掉共享 IP 预算，然后才返回 `ACQUISITION_CAPACITY`；唯一 key 可无限重复 | **IMPLEMENTATION_TEST（成立，已修）** | 命中资源上限 Must。已核实容量检查仅在 `StartPrepared` 内（`repository.go:165,221`）。已在 D1 增加**抓取前有界容量预检/准入**，并保持 `StartPrepared` 为原子正确性栅栏；§8 增加“已触顶组织的新 key”验收 |
| 16 | `ByKey` 快速路径直接从 resolve/read，**未做 `sameAcquisition`** ⇒ 同 key 换 offer 会拿到旧 offer 的已发布结果，而不是 `ErrAcquisitionConflict` | **IMPLEMENTATION_TEST（成立，已修）** | 命中同 key/异载荷幂等 Must。已核实 `sameAcquisition` 比对 `Fingerprint` 与 `Source`（`acquisition.go`）。已在 D1 要求**命中后先比较**再 resolve，并新增“同 key 异 offer”验收 |
| 17 | 被允许的 origin 返回超大文档/子资源时，Chromium 在页内证据上限生效**之前**就物化；`MaxAcquisitionCommandBytes` 只管后续 command | **IMPLEMENTATION_TEST（成立，已修）** | 命中采集可用性与资源边界。已核实 `src2b-acquisition-v1` 已有“压缩/展开各 2 MiB”上限。已在 D8 要求把该上限**带到浏览器路径**，并在浏览器/代理边界定义**单响应与累计下载上限**（超限中止采集，不静默继续）；§8 增加超大响应 fixture 验收 |

### 10.4 第六轮增量复核（commit `4f72b0752`）

| # | Finding | 分类 | 处置 |
| --- | --- | --- | --- |
| 18 | §1 范围声明仍写着采集进程“复用同一个 \`product_acquisition\` 库” ⇒ 实施者照做就会把库凭据与 Chromium 放进同一网络命名空间，**直接推翻 D12/D13**（即 #12 的旧病灶） | **BLOCKER（成立，已修）** | 命中「新的跳系统安全边界」。已把 §1 范围声明对齐 D13：**所有数据库仅由 \`current-application\` 持有，采集进程无任何库凭据/无库网络可达**；§4 调用链同步 |
| 19 | D1 要求的抓取前容量预检**没有可实现的合同路径**：唯一的容量计数是 \`Repository.StartPrepared\` 内的**私有**查询，而 §7 又声明 store 接口不变 | **IMPLEMENTATION_TEST（成立，已修）** | 命中资源上限 Must。已在 D1 定义**最小只读容量合同**（\`AcquisitionOperationStore\` 新增一个只读容量查询），并把 §7 改为“既有方法语义不变 + **唯一例外**是新增该只读方法”；预检明确为优化/风控而非正确性来源，原子栅栏仍是 \`StartPrepared\` |

### 10.5 第七轮增量复核（commit `e17865e79`）

| # | Finding | 分类 | 处置 |
| --- | --- | --- | --- |
| 20 | D1 快速路径把 \`prepared\` 与 \`publishing/published/failed\` 一视同仁地丢给 resolve，但 \`AcquisitionService.resolve\` **只接受 \`publishing\`/\`published\`** ⇒ \`StartPrepared\` 已提交而响应/进程丢失时，每次同 key 重试都返回 \`OUTCOME_UNKNOWN\`，且**没有调度器**会去推进那条已持久化的 command | **IMPLEMENTATION_TEST（成立，已修）** | 命中响应丢失恢复幂等 Must。已核实现有 HTTP \`Acquire\` 在 \`op.State == prepared\` 时先 \`Claim\` 再 \`resolve\`。已在 D1 改为**按状态分支**：\`prepared\` 必须先 \`operations.Claim\` 再 resolve；§8 增加“停在 \`prepared\` 的恢复”验收 |

## 11. 未决 / 需评审确认

1. **D2 的 provider 时间预算**：是否接受按 provider 分叉 deadline（建议 90s），或要求把浏览器耗时压进 20s。
2. **D8 的并发上限**：建议 ≤2；在 D13 形态下落于采集容器（可独立调），需确认是否与部署形态（单副本/多副本）一致。
3. **验证码自动处理的失败语义**：确认「自动处理失败即如实 `SOURCE_UNAVAILABLE`，不做二次人工介入」。
4. **`public_browser` channel 命名**：需确认与 `src2b-acquisition-v1` 的 channel 语义扩展方式（`MapAcquisitionEvidence` 允许集合）。
5. **D1 的失败行差异**：确认浏览器路径「预 admission 失败不建行、同 key 可重试」相对 HTTP 路径「失败建行、同 key 永久失败」的差异是否接受。
6. **D10 的依赖边界**：确认新 owner 自持最小浏览器实现（不 import `internal/crawler/*`），而不是把 `shared/browser` 提升为当前共享基础设施。
7. **D9 的 `ContentSHA256` 定义**：确认为「映射字段子树规范化 JSON 摘要」，而非整页字节。
8. **D12 的连接层强制方式与部署形态**：**已定（D13 独立采集容器 + 出站白名单）**；具体白名单 CIDR/域名与采集容器网络策略仍需在实现前定稿。
9. **D12 的允许 origin 集**：需产品/安全确认具体的 1688/CDN 允许列表。
10. **真实 1688 网络验收**：需用户单独授权；未授权保持 `NOT_RUN`。
11. **D13 的内部 RPC 契约与部署面**：采集服务只进内部网络、无 DB 凭据（finding #12 已定），需定义最小 RPC（请求/响应/错误/超时/**解码前限长**）；新增 compose 服务/镜像/限额属新部署面，需在实现 Issue 明确并单独授权部署。
12. **D2 的具体预算数值**：外层服务预算 = provider + 发布，建议值待定（需一次真实冷启动+挑战的实测来定，不能只靠夹具）。
13. **D13 的调用方准入方式（finding #14）**：网络准入 vs 服务身份（mTLS/服务凭据）二选一，依赖部署形态；**未落实即不得接生产路由**。
14. **D8 的浏览器响应/累计下载上限数值**：需与 `src2b-acquisition-v1` 的 2 MiB 语义对齐后定稿。
15. **§7 例外的接口形状（finding #19）**：只读容量方法的最终签名（建议 `CapacityAdmitted(ctx, scope) (bool, error)`）需在实现前定稿，并确认其不引入第二个容量事实源。
