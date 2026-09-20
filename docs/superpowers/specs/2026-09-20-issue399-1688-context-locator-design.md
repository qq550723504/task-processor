# Issue #399 1688 `window.context` 定位规则设计（v1 提案）

日期：2026-09-20。依据：Issue #399；Parent SRC-2B2。
状态：**DESIGN_REVIEW_READY，待用户决定 D1–D5**。本文是设计提案，不改变任何已冻结合同，不表示已实施。
本轮只新增本文，未修改任何生产代码。

## 1. 结论

真实 1688 商品页的 `window.context` 赋值形状与两个通道的解析器假设不一致，导致：

- 通道③（浏览器插件，SRC-2B2）：`UNSUPPORTED_PAGE`，无法采集真实页面。
- 通道①（服务端匿名抓取，SRC-2B1）：先被 1688 反爬 `punish` 拦截，即使绕过也会因同一形状假设而失败。

这不是"某处正则写错了"，而是**同一条取数规则在两处各写一遍**，两处同时依赖一个已经不成立的页面假设，且两边的测试夹具都只覆盖了旧形状。

## 2. 事实与执行证据

### 2.1 真实页面形状（本次实测）

真实页面（`https://detail.1688.com/offer/932524015351.html`，已登录会话）的内联脚本为：

```js
\nwindow.contextPath = "/default";\nwindow.context=(function(b,d){var c=d.module||{};var e={};for(var a in c){if(typeof c[a]==="string"){e[a]=c[a]}}Object.assign(e,c[b]||{});Object.assign(d,{module:e});return d})(window.contextPath,{"result":{...},"panelConfig":...,"module":...,"version":"0.26.16"});
```

- 脚本长度 137679 字节；`window.context` 赋值偏移 34（即**不在脚本开头**）。
- 等号右边是 **IIFE 调用**，真正的 JSON 文档是它的**第 2 个实参**。
- `window.context` 全局对象求值后为 object，`result.data.Root.fields.dataJson.tempModel.offerId === "932524015351"`。

### 2.2 同一条正则、两处实现、两处执行失败

| 位置 | 代码 |
| --- | --- |
| 服务端 | `internal/integration/acquisition/a1688/public.go:33` `^\s*(?:window\.)?context\s*=\s*`，`public.go:300` `FindIndex(text)` |
| 插件 | `extensions/1688-capture/src/extractor.ts:19-21` 同一条正则，作用于 `substringData(0, 80)` |

对同一份真实页面文本执行：

| 输入 | 正则命中 | 等号右边是合法 JSON |
| --- | --- | --- |
| 真实 1688 页面 | **否** | — |
| 仓库现有夹具（Go / 插件同形） | 是 | 是 |

（`go run` 与 `node -e` 各自独立执行，结果一致。）

插件侧进一步用**已打包的 `dist/extractor.js`** 注入真实页面执行，返回 `{ok:false, code:"CAPTURE_REJECTED"}`，内部失败码 **`UNSUPPORTED_PAGE`**；页面上 26–32 个内联脚本中命中数 **0**。

两个独立成因，任一个都足以失败：

1. **锚定位置错误**：`^\s*` 要求 `context` 出现在脚本文本开头，实际前面还有 `window.contextPath = "/default";`。
2. **右边不是裸 JSON**：即使把赋值点找到，`(function(...){...})(...)` 不是 JSON 字面量，`jsonc-parser` / `encoding/json` 都会失败。

### 2.3 为什么"参考服务端源码"不足以修复插件

服务端 `publicJSON()` 的**取数引导**与插件一字不差，照抄会把同一缺陷搬过去。服务端可参考的只有**字段语义**：

| 事实 | 服务端 | 插件 |
| --- | --- | --- |
| title | `result.data.productTitle.fields.title` | 同 |
| images | `result.data.gallery.fields.offerImgList` | 同 |
| offerId | `result.data.Root.fields.dataJson.tempModel.offerId` | 同 |
| variants | `...dataJson.skuModel.{skuProps,skuInfoMap}` | 同 |
| attributes | `result.global.globalData.model.offerDetail.featureAttributes` | 同 |
| priceFacts | 遍历 `result.data` **所有 key** 取 `fields.finalPriceModel.tradeWithoutPromotion.offerPriceRanges` | 只取 **`data.price`** ← 不一致 |

代码无法复用：服务端 browser-capture 入口 `sourcing.ParseBrowserCapture` 只接受**结构化证据**（严格 wire 校验，`parserVersion` 必须等于常量 `1688-browser-dom/v1`），不接受原始 HTML/脚本；通道③ 是唯一持有页面的一方。

### 2.4 已漂移的四处（现状不是"两处一致"）

| # | 项 | 服务端 | 插件 |
| --- | --- | --- | --- |
| 1 | priceFacts 的 key 范围 | 所有 key | 硬编码 `price` |
| 2 | JSON 规模上界 | `nodes ≤ 4096`（`public.go:337`） | `nodes ≤ 16384`（`extractor.ts:34`） |
| 3 | 挑战页标题守卫 | 无 | 有（`验证码\|请登录\|captcha…`） |
| 4 | 夹具内容 | `public-product.html` | `tests/fixtures/product.html` + `capture-v1.json` |

真实页面实测 `result.data.price.fields.finalPriceModel.tradeWithoutPromotion.offerPriceRanges` **不是数组**，故插件在真实页面上即使定位成功也会报 `priceFacts` 缺失。

### 2.5 未验证项（不得据以实施）

- 真实页面价格实际挂在 `result.data` 的哪个 key 下（operator profile 现已被 1688 判为机器流量，每次导航都进验证码）。
- 真实页面 JSON 的节点数是否会超过服务端 `nodes ≤ 4096`。
- IIFE 形状是否在 1688 其他页面变体上稳定（仅 1 个真实样本）。
- 通道③ 在真实登录页面上能否端到端完成一次采集（被 2.2 的提取缺陷阻断，尚未观察到成功路径）。
- 仓库内无任何真实页面形状记录（`contextPath` 全仓 0 命中）。

## 3. 设计

### 3.1 定位算法（逐字语义，两处必须一致）

输入：单个**无 `src`** 的内联 `<script>` 文本 `text`。

1. **找赋值点**：从左到右找第一个 `context`，满足：
   - 匹配 `(?:window\s*\.\s*)?context\s*=\s*`；
   - 且 `context` 前一字符不是 `[A-Za-z0-9_$]`，也不是 `.`（排除 `foo.context=`）；
   - 该条件天然排除 `window.contextPath = ...`（`context` 与 `=` 之间必须是空白）。
2. **唯一性**：所有内联脚本中符合条件的赋值点必须**恰好 1 个**。0 个或 ≥2 个 → `UNSUPPORTED_PAGE`（保留现有"重复即拒绝"语义）。注意实现顺序：第 4 步的"RHS 之后只允许空白与一个 `;`"逐命中先行检查，因此第二个赋值点通常先以 `trailing` 被拒。两者都是 fail-closed 的 `UNSUPPORTED_PAGE`，无语义差别；实现时不必强求错误码分类。
3. **取 RHS**（去前导空白后）：
   - 以 `{` 开头 → **Case A（静态字面量）**：字符串/转义感知配平扫描出对象字面量；其后只允许空白和至多一个 `;`，否则 `UNSUPPORTED_PAGE`（保留现有"拒绝可执行后缀"语义）。
   - 以 `(` 开头 → **Case B（IIFE）**：配平扫描第一组括号（被调用表达式）；其后必须紧跟 `(`，否则 `UNSUPPORTED_PAGE`；配平扫描实参表，按顶层逗号切分；取其中**对象字面量**形式的实参，**必须恰好 1 个**，否则 `UNSUPPORTED_PAGE`。该对象即 payload。
   - 其他 → `UNSUPPORTED_PAGE`。
4. **RHS 之后**只允许空白和至多一个 `;`。

Case B 在真实页面上：实参为 `window.contextPath`（标识符，非对象）与 `{...}`（对象）→ 恰好 1 个对象实参。

### 3.2 必须保持的不变量

- 只解析脚本**文本**，任何情况下**不读取 `window.context` 全局**、不 `eval`/`Function`、不访问 cookie/storage/网络（插件的 `extractor.test.ts` 已对这些设置抛错 getter）。
- 严格 JSON：拒绝重复键；深度 ≤ 32；节点数与字节数有界；`offerId` 必须与来源一致。
- 任何不确定情形一律 **fail-closed** 为 `UNSUPPORTED_PAGE` / `ErrUnsupported`，绝不产出猜测事实。
- 服务端 `failFetch` 的既有映射（`SOURCE_UNAVAILABLE`）与插件 `CAPTURE_REJECTED` 语义不变。

### 3.3 参考实现验证（本次已执行，验证 §3.1 可判定性）

用一份一次性参考实现（不在仓库内，`/tmp/locator/ref.mjs`）对 13 个用例执行 §3.1：

| 用例 | 期望 | 实测 |
| --- | --- | --- |
| 真实页面形状（`contextPath` 前缀 + IIFE） | 接受 | `caseB`，payload 是合法 JSON |
| 仓库 Go 夹具（JSON 字面量） | 接受 | `caseA`，payload 是合法 JSON |
| 仓库插件夹具（JSON 字面量 + 缩进） | 接受 | `caseA` |
| 前置无关语句后再赋值 | 接受 | `caseA` |
| 无赋值 | 拒绝 | `no-match` |
| 重复赋值 | 拒绝 | `trailing`（见第 2 步说明） |
| 可执行后缀 `;window.location=...` | 拒绝 | `trailing` |
| IIFE 未被调用 | 拒绝 | `iife-not-called` |
| IIFE 有两个对象实参 | 拒绝 | `object-args=2` |
| IIFE 无对象实参 | 拒绝 | `object-args=0` |
| `mycontext =` | 拒绝 | `no-match` |
| `foo.context =` | 拒绝 | `no-match` |
| 仅 `window.contextPath =` | 拒绝 | `no-match` |

结论：§3.1 对真实形状与旧形状都能正确定位，且对全部反例 fail-closed。该参考实现只用于验证规格，不进入仓库、不构成实现。

### 3.4 备选方案

- **B1 收敛到一处实现（服务端解析）**：插件上传原始脚本/HTML，服务端解析。代价：新增上传契约；插件失去"只复制有界 JSON、不复制页面内容"的隐私属性；改动已冻结的 browser-capture wire。**不推荐**。
- **B2 共享代码（Go→JS/WASM）**：新增构建基础设施。**不推荐**（违反"复用成熟组件、不建平台"）。
- **B3 保留两处实现 + 共享一致性语料**：规则以本文为准，两处各自实现，但共用同一组夹具（尤其"真实形状"夹具），并断言两处对同一夹具产出的 evidence 语义一致。**推荐**。
- **A/B 无关**：反爬（`punish`）不是本设计能解决的，见 D1。

## 4. 待决定项与落实结果

用户已确认「按建议来」，以下为 **实际落实结果**（含实施中发现的事实修正）。

| # | 决定 | 建议 | 落实结果 |
| --- | --- | --- | --- |
| **D1** | 修哪些通道：只修③、或③+① | 只修③ | ✅ 已按只修③实施。`internal/integration/acquisition/a1688/public.go` 未改动，① 的反爬 + 定位缺陷作为产品级 finding 保留 |
| **D2** | 单实现还是两实现 | B3：两实现 + 共享夹具与一致性断言 | ✅ 已实施。夹具为**派生**关系（`product-real-shape.html` 由 `product.html` 生成，非手抄），并新增「两种形状产出逐字节相同 evidence」的断言 |
| **D3** | 是否升 `parserVersion` | 升 | ✅ 已升为 `1688-browser-dom/v2`，插件 / Go / web 三处字面量同步；**修正一处原设计事实错误**，见 §4.1 |
| **D4** | priceFacts key 范围是否对齐 | 对齐成遍历 `result.data` 所有 key | ✅ 已实施（按 key 排序保证确定性）。**修正**：子路径本就与服务端一致，差异只在 key 范围，见 §4.2 |
| **D5** | 服务端 `nodes ≤ 4096` 是否够 | 先实测真实页面节点数 | ⛔ `NOT_RUN`：实测被 1688 验证码拦截阻断（见 §4.3）。插件侧 `nodes ≤ 16384` **保持原值**，服务端 `4096` 未动 |

### 4.1 D3 事实修正（重要）

原设计称 `1688-browser-dom/v1` 是「冻结合同字面量」。**该说法不成立**：`git grep` 显示该字面量 **不存在于 `origin/main`**，只存在于未合并的 #400 / #406 分支。因此：

- 现在升级**不存在存量数据不兼容**问题（尚未有 v1 的持久化记录），代价比原估更低。
- 但升级**确实会改变规范载荷指纹**：`parserVersion` 参与 `json.Marshal(wire)`，golden `PayloadSHA256` 已由 `503afdca…` 变为 `a369a576…`（该值已用独立 Python `hashlib` 重算核对，未采信测试自身输出）。
- 代价：需在 #406 分支与 trial 接线分支各同步一次（已完成），并重建 trial 镜像。

### 4.2 D4 事实修正

原设计假设「子路径存在分歧」。**实际不分歧**：服务端 `FinalPriceModel.Trade` 的 json tag 就是 `tradeWithoutPromotion`，与插件相同；唯一差异是服务端遍历 `result.data` **所有** key 并排序，插件硬编码 `price`。因此本次只改 key 范围，不改子路径。

### 4.3 D5 阻断说明

真实页面在重复自动导航后被 1688 验证码标记（`punish?x5secdata=…`），无法再取得节点数。该测量**需用户提供可用会话后重做**；不得用夹具节点数替代。

### 4.4 D6 / D7：真实页面复测发现的两条新上限（用户已批准实施）

用户在验证码可读后重新提供会话，`§3.1` 定位规则在**真实页面**上命中正确（3 处 `context=` 命中里 2 处被正确排除，接受项位于脚本偏移 `34`，RHS 为真实 IIFE `(function(b,d){var c=d.module|…`）。但采集仍然失败，实测到两个与夹具无关的真实缺陷：

- **D6 — 载荷不是严格 JSON。** 真实载荷里 `"skuWeight":{6290953586037:0.3000,…}` 使用**未加引号的数字对象键**；`parseTree` 报 **90 个解析错误**（全部落在 `skuWeight`，偏移约 `87084`/`87104`）。仅在对象键位置补引号后 **错误 90 → 0**，标题/SKU/图库均可读，`roundTripOk: true`。
  - 处理：新增 `quoteBareNumericKeys()`，**只**在对象键位置给裸十进制数字补引号；字符串内容与其余 token 逐字复制；补引号后**仍执行**严格解析，因此注释、尾随逗号、重复键依然被拒（有对应用例）。
- **D7 — 节点预算过小。** 归一化后实测 **nodes = 16501**，超过插件原先的 `16384` 上限 ⇒ 即使解析成功也会判 `CAPTURE_TOO_LARGE`。
  - 处理：上限 `16384 → 65536`（约为实测值的 4 倍）；**2MB 字节上限仍是主要资源护栏**；服务端 `4096` 上限与通道①不在本次范围。

**parserVersion 仍为 v2**：D6/D7 只放宽输入接受范围，未改变任何字段语义，且当前不存在 v2 持久化数据。如需 v3 需用户另行决定。

D6/D7 之外的第三项限制在**服务端**（不在本设计范围内，如实记录）：真实载荷经 `MapAcquisitionEvidence` 展开后 `warnings = 275`、`missingFacts = 183`，其中 `warnings` 超过 `MaxSourceEnvelopeCollectionItems = 256`（触发时累计 `items = 475`，仍低于 `MaxSourceEnvelopeAggregateItems = 1024`，`stringBytes = 18375`），提交被 `413 SOURCE_TOO_LARGE` 拒绝。该上限属 server owner，需单独产品决定（见 §6）。

## 5. 不做什么

- 不改 1688 反爬 / 登录门槛，不使用任何真实 1688 账号或凭据。
- 不新增跨语言共享解析库、兼容层、fallback 或第二事实源。
- 不改已冻结的 browser-capture wire 字段（除 D3 的 `parserVersion` 字面量，且仅在决定升级时）。
- 不改 `validation.ts` 的应用来源校验（A1 已用配置解决）。
- 不引入新的验收平台、runner 或故障注入设施。
- 不迁移、不清理任何历史数据。

## 6. 验证结果

- 插件（#400）：`npm test` → **42/42 通过**（新增 4 个用例：真实形状拍摄、两形状 evidence 等价、模糊形状拒绝、非 `price` key 的价格识别）；`npm run typecheck`、`npm run lint` 均干净。
- 服务端（#406）：`go test ./internal/product/sourcing/...`、`go test ./internal/app/httpapi/...` 通过；`go build ./...` 干净。web 契约相关 **152/152 通过**。
- trial 接线分支：同步 v2 字面量与 golden 指纹后，`internal/product/sourcing` 测试通过。
- 一致性断言（D2）：同一份语义记录，两种页面形状（IIFE 形 / JSON 字面量形）产出**逐字节相同** evidence。两个 golden 当前带同一 `contentSHA256`（`69ea755f…`），但**分处两个分支、需各自维护**——这是已知成本，非共享库；不为此新建跨语言共享层。
- 端到端（真实页面）：**采集段已 PASS**。最终构建（`1688-browser-dom/v2`，含 D1–D7）在真实页面 `https://detail.1688.com/offer/932524015351.html` 上，经真实 Chrome + 真实 `Extensions.loadUnpacked` + 真实弹层按钮路径完成采集：弹层显示真实标题与“采集完成”，并列出未取得字段；交接页显示真标题、真 URL 与 `92 warnings; 92 missing facts`。
- 提交段（真实页面）：**FAIL，`413 SOURCE_TOO_LARGE`**，且明确“未创建 operation”。根因已用证据定位（见 §4.4 末段）：提交被接受的实测证据体为 21 812 字节、`variants=45`、`variantAttributes=90`、`missingFacts=92`、`warnings=92`（服务端展开后 `missingFacts=183`、`warnings=275`），触发服务端每集合上限 `MaxSourceEnvelopeCollectionItems=256`。
- 因此“真实产品结果 / 缺字段展示 / 刷新+重登回读”仍 **`NOT_RUN`**：当前不存在成功 operation 可回读。
- 回归证据：插件 `npm test` → **47/47**、`npm run typecheck`、`npm run lint` 干净（新增 5 个用例覆盖 D6/D7 与其拒绝面）。
- 浏览器级证据（层介于单测与真实页面之间，已执行）：将真实形状页面 `product-real-shape.html` 喂给**仓库已有的真实 Chrome + `Extensions.loadUnpacked` 冒烟路径**（`scripts/browser-smoke.mjs` 的等价副本，脚本放在 gitignored 的 `artifacts/` 下，**未进仓库**），使用 **A1 应用来源**构建的 `dist-fixture`：弹出层显示“采集完成”，交付载荷 `parserVersion = 1688-browser-dom/v2`，且除 `capturedAt`（实拍时间）外与 golden **逐字段相同**，`contentSHA256` 与 golden 一致 ⇒ 两种页面形状经真实注入路径产出同一语义记录。同路径下 JSON 字面量夹具（`product.html`）仍通过（`port-isolation-check.mjs`）。
- 该浏览器级证据**仍不能替代**真实 1688 页面验证：页面响应仍被替换为夹具，未经过真实反爬、登录会话与真实后端。

## 7. 影响面与归属

- 切片属于 #399（SRC-2B2）。插件侧实现位于 #400 分支 `codex/issue-399-1688-browser-capture`；服务端 browser-capture 后端位于 #406（当前 `CONFLICTING`）。
- 若采纳 D1"只修③"，则本切片不触碰 `internal/integration/acquisition/a1688/public.go` 的定位逻辑；仅新增 §4 记录的产品级 finding。
- Legacy decision：本设计不新增、不包装任何 legacy owner；涉及的服务端解析器 `internal/integration/acquisition/a1688` 是**当前 owner**（不在 legacy register 内）。
