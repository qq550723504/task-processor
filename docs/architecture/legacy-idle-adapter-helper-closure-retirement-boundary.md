> #301 / #937 **USER_CONFIRMED**：[五组闲置支持门面完整退休闭包](legacy-idle-support-facade-closure-retirement-boundary.md) §1–§7（含§6.1）已获用户确认，Reuse Existing Architecture / Ready与Issue/PR读回6386195192/6386203704先于源码修改。仅替代本批request-role辅助、HTTP builder/NewSimple和SetGlobalLogLevel及其原guard正向保活项，转为准确缺席/owner禁入。#934原九文件/114声明/八项旧日志禁入、有效LogManager/Retry/OpenAI/JSON与全部其余保留能力和检查保持；旧CI/复核/措辞BACKLOG不改写。只替代准确旧KEEP；以下完整原Git正文/既有顶注/数字/勾选/批准/历史证据逐字保留。workflow/完整inventory/依赖/保护与既有实例/数据/凭据/进程零改，不增试用前置，保留独立SHEIN登录与Store同步，#301 OPEN。

> **§5.2 / §6.1 准确补充：OpenAI logger AST 保活检查。** 原 `tests/integration_provider_logger_contract_test.go / TestOpenAILoggerConfigFieldsAreASTComplete` 的 `want` 表仅移除已整退的 CachedClientConfig、ResilientClientConfig 两项；ClientConfig/PoolConfig/ManagerConfig 的 Logger 类型、json:"-"、AST/遍历/过滤/错误处理及缺失拒绝全部保持。冻结矩阵遗漏的仅此函数，原构造扫描器/SSA规则/计数、生产代码、workflow/完整 inventory 和其它检查零改；原退休 guard 继续明确禁止两旧类型回流。以下冻结原文/历史完整保留，只对本补充所列保活行生效，无运行资源操作或新试用前置，#301 OPEN。

> #301 / #934 **USER_CONFIRMED / SOURCE_IMPLEMENTED**。用户已确认冻结设计3208f5371的 §1–§7（含 §6.1）；Reuse Existing Architecture / Ready 已先写入并读回 [#301 接单6385055853](https://github.com/qq550723504/shuomi-ai-commerce/issues/301#issuecomment-6385055853)及 [PR读回6385065287](https://github.com/qq550723504/shuomi-ai-commerce/pull/934#issuecomment-6385065287)。本批9整退+5精确混合+5旧依据窄注+3当前文档共22工件按原矩阵整批实施；无新 EXTRACT 或替代 owner。当前日志/Retry/AI/JSON/采集/独立 SHEIN 登录与 Store 同步保留，workflow/完整 inventory/依赖/保护及既有资源不动，不增试用前置，#301 OPEN。准确实现 HEAD/CI/最终复核留主要 PR，合并另授权。以下3208f5371冻结设计完整保留；其中设计阶段/待授权表述仅是当时阶段，删除/保留/§6.1矩阵未变。

# 旧闲置 adapter/helper 完整退休闭包边界

## 1. 用户结果、范围与 Design Basis

状态：**DESIGN_ONLY / AWAITING_USER_CONFIRMATION**。执行父目标为 #301，保持 OPEN。本轮用户批准先做四组完整设计，未批准正式源码实施、合并、关单或运行资源操作。设计、源码、PR CI、实际 merge SHA 的 push/main CI、真实运行与用户验收分别记录，不能互相代替。

使用者是维护当前应用和独立 SHEIN 登录/Store 同步的维护者。当前操作是继续退出没有当前消费者的旧内部设计；交付结果是一个可直接确认的完整退休矩阵，连专属测试、原检查、前批 KEEP 与直接文档尾部一起收齐，不按文件数或内部模块机械拆批。不新增用户操作、服务入口或内部试用前置。

**Design Basis: N/A（本阶段仅本文、Architecture README、Legacy Register 三份文档）**。未来实施选择 **Reuse Existing Architecture / Admission: NOT_READY**：复用下列已批准 Greenfield、Hard-Cut 与现有 logging/Retry/OpenAI/JSON owner，不新建或改变领域合同、事实 owner、权限、状态、事务、持久化、AI 准入/预算/计量或 retry/UNKNOWN/recovery。没有 UI/IA 改动，Figma N/A。四组虽跨目录，均为无当前消费者的旧支持封装，现有架构覆盖本批退休，不引入新机制或重复全局 Architecture Review。

正式实现必须在用户确认本文 §1–§7（含 §6.1）后，先在 #301/主要 PR 记录并读回 Reuse Existing Architecture / Ready，刷新实际消费者和 main 交集，再由唯一 Writer 继续同一 PR。文档 CI 不授予源码实施准入。若发现真实当前消费者、外部可观察契约或必须改变上述边界，停止对应部分、报告用户并重新分类，不临时加兼容路径。

全批未来涉及 **22 工件：9 整文件 RETIRE + 5 精确混合文件 + 5 旧依据窄顶注 + 3 当前文档**。设计阶段只修改最后三份当前文档，其余十九工件仅列未来矩阵，尚未实施。

Out of Scope：workflow、完整 inventory（CURRENT/历史/计数/ceilings/parser/检查逻辑）、依赖/锁、保护规则及 §6.1 表外检查零改；不 tidy/升级、引入依赖、迁包、wrapper/alias、fallback、双读双写、旧状态兼容、缓存迁移/清理或新 runner/验收平台。不读取既有凭据，不操作既有实例、数据、缓存、日志、配置文件、进程或 provider；不建临时数据库/Redis，不执行业务命令。部署、环境处置和产品验收未授权；#29 不重开，#301 保持 OPEN。

## 2. 权威、基线与前批 KEEP 替代

依据：

- [Greenfield 产品决定](../product/greenfield-no-legacy-migration.md)：空业务数据和当前模型，不迁移旧状态，不通过历史 KEEP 恢复旧架构。
- [Legacy Hard-Cut](../refactoring/legacy-hard-cut-policy.md) 与 [Legacy Register](../refactoring/legacy-register.md)：只有 EXTRACT 或 RETIRE，有效行为归当前 owner，无 Compatibility 类别。
- [项目边界](project-boundaries.md)、[目标架构](project-target-architecture.md) 与仓库 AGENTS.md 的 Architecture First / Delivery Batch / 找到根因原则：一个完整结果一个主要 PR，不新建第二缓存、限流/熔断或日志事实 owner。
- 用户明确保留当前采集、原 Amazon 浏览器/debug、图像、有效规则、共享合同、独立 SHEIN 登录与 Store 同步；不把源码退休当真实资源清理权限。

调查基线为实际 main **aa5c0f8e58df153d0306ba7bfcaefa1cdadfc730**，tree **ee1305fc57627d1667b459fa192744f70a84db71**。相对 [#301 前次只读核对6384576781](https://github.com/qq550723504/shuomi-ai-commerce/issues/301#issuecomment-6384576781) 的 main3cef78692，仅新增 #933 当前商品采集材料改动，四组目标源码没有变化。本次重新解析全部 **3282 个受追踪 Go 文件**，包含测试、build-tag 文件和嵌套模块；解码 import 字面量，识别 alias/selector，并查同包声明、调用/测试与字符串/文档引用。不能把简单文本命中、测试表项或包 import 当生产装配，也不能把 import 为零等同全包无用。接单/唯一 Writer/分支与上一当前段逐字归档见 [6384715692](https://github.com/qq550723504/shuomi-ai-commerce/issues/301#issuecomment-6384715692)。

以下是未来用户确认本设计后才生效的准确替代，原文与历史批准不是错误，也不整体撤销：

| 前批依据 | 当时保留含义 | 本批准确替代 | 继续有效的边界 |
| --- | --- | --- | --- |
| [#868 数字值合同](legacy-numeric-value-contract-closure-retirement-boundary.md) | 保留 LoggerHelper 其余方法、通用字段、context/trace/request | §5.1 两 logging 源文件及专属测试整退；原八项数字日志禁入转入 §6.1 的 owner 检查 | 实际日志 singleton、配置、hook/writer 和当前身份/trace 合同不变 |
| [#880 Redis/proxy/shared](legacy-redis-proxy-shared-closure-retirement-boundary.md) | shared/resilience 广泛 KEEP | 只退出 breaker/ratelimit 及两专属测试，不退包根 | Retry/RetryConfig/RetryAttempt、ptr、实际 DB/Redis 与当前 owner 保留 |
| [#893 logger facade](legacy-logger-facade-closure-retirement-boundary.md) | helpers.go 必须可读且禁数字 Task/Store helper | 文件必须缺席；数字日志禁入准确迁入现有 AST guard，不 missing-skip | 旧 core/logger 根禁入和所有其它身份/日志断言保持 |
| [#914 idle support](legacy-idle-support-contract-closure-retirement-boundary.md) | 因旧 Amazon API 消费而 KEEP UnmarshalBytes | #926 已移除最后 API 消费者，本批只退该函数 | UnmarshalString/CleanLLMResponse、严格草稿 JSON、hash/读取拒绝保持 |
| [Phase2 原计划](../superpowers/plans/2026-08-30-internal-target-architecture-phase2.md) | logging 全导出及 resilience 三原语；两个 decorator 配置也有显式 Logger | 仅替代上述已失去当前消费者的 helper/context/limiter/breaker/decorator 保活含义 | 当前日志配置/实例与 ClientConfig/PoolConfig/ManagerConfig 显式 Logger、provider 中立端口仍有效 |

[旧开源替换评估](open-source-replacement-evaluation.md) 引用的 Amazon API/taskstatus 等是历史评估，不是重新建设共享 limiter/breaker 的当前需求，本批不改该历史记录。成熟 rate/gobreaker/backoff/Logrus/SDK 及锁文件保留，符合“已有开源实现不重复造轮子”；没有当前用户需求的封装直接 RETIRE，不再造替代层。

## 3. 实际消费者与 contract → implementation → injection → consumer

| 组 | 当前事实与旧候选消费者 | 保留的实际生产路径 / owner |
| --- | --- | --- |
| platform/logging helper/context | 外部生产 import 仅 core/config、loader_builder 和 pkg/appenv 使用配置/全局 logger；候选符号只在两个源文件自用及 helpers_test 中消费。同包 manager/rotating_writer/level_split_hook 不调用候选 | appenv 初始化 → InitGlobalLogger/LogManager → core/config/current runtime GetGlobalLogger；LogConfig/LevelFileConfig、singleton、级别/hook/writer 仍属原 logging owner |
| shared/resilience limiter/breaker | 只有本文件自用和对应专属测试；两个真实生产消费者用的均为 Retry/RetryConfig | imageagent/temporal/activities_artifacts_v3.go、listingsubscription/usage_ledger_gorm.go → 原 Retry；RetryAttempt/取消/退避/预算语义保持 |
| OpenAI CachedClient/ResilientClient | 全仓 52 条 decoded OpenAI import 记录；包外只在 provider logger 合同测试引用两个 config 类型，同包只有 TestDecoratingConstructorsUseExplicitLogger 构造。未发现生产构造、接口注入或工厂返回这两种 wrapper，无相关 dot/blank import | client_manager.go NewManager/resolveClientWithSelection → *Client，GetClient/GetImageClient → contextualChatClient/contextualImageClient → 当前 resolver、RequestPool/BaseClient、真实 text/image/provider 路径 |
| jsonx.UnmarshalBytes | 原 Amazon API 退出后，无生产/测试调用，只有原保活 guard 和历史设计；不因此删除整个 jsonx | publishing/shein/title_resolution.go → UnmarshalString/CleanLLMResponse；当前草稿 JSON/严格解码/规则保持原 owner |

TestProductionProviderConstructorInventoryIsComplete 与 scanProviderConstructors 中的名字是检查元数据，不是运行工厂，不因退出 decorator 就改扫描器或三个真实构造计数。logger_test 中静态/动态 Manager 的显式 Logger 传播仍是当前有效断言。

**架构问题与处置**：未装配的 decorator 保留了另一套 cache/Get-Set-Delete、手写 breaker/retry 和 stats/Close 接口，容易被新代码误当当前 AI 安全执行入口；通用 logging/context 也与当前身份/trace owner 并列存在。本批从根上删除闲置 owner，不把它们接到当前调用链，不合并或重写真正的 RequestPool 限流、AI 预算/计费/UNKNOWN 和当前身份链。不存在需要从这些旧文件抽取的新有效行为；有效能力已经由当前 owner 实现。

## 4. owner、持久化与副作用边界

### 4.1 当前合同不变

| 边界 | 旧候选的状态/副作用 | 本批处置与唯一当前 owner |
| --- | --- | --- |
| 日志 | helper 可包装操作、字段/进度/采样与 context 值；不负责当前身份认证 | 退闲置入口，LogManager 的全局实例、文件/hook/级别/关闭保留；不读/删日志，不替换当前 authidentity/aiidentity/trace/request 合同 |
| resilience | limiter 与 breaker 是进程内令牌/计数/状态；专属测试只验证无消费者封装 | 没有新 owner/状态机迁移；Retry 保留，当前其它域的 x/time/rate、外部 gate、退避/UNKNOWN 不动 |
| CachedClient | Cache 接口可 Get/Set/Delete，key 默认 openai:chat，TTL/Enable/Disable/ClearCache 属旧 wrapper；代码本身不能证明持久后端不存在 | 未发现当前 Cache 实现注入此 wrapper。既有实例/键/记录是否存在为 UNKNOWN，不查询/flush/删除/迁移。当前配置缓存、dynamicClients/有效 route 缓存是不同 owner，完整保留 |
| ResilientClient | 自有 breaker 状态、最大重试、超时、统计和底层 client Close；若被重新接线会改变调用/重试边界 | 无当前注入，不把状态转交 RequestPool/业务 owner；不运行构造器/Close/provider。真实 RequestPool/BaseClient 的限流/取消/重试和所有计费/幂等/retry/UNKNOWN 保持 |
| JSON | UnmarshalBytes 是无状态解析；不拥有草稿或数据库事实 | 只删除闲置函数，不改 UnmarshalString 的泛型/错误包装/JSON 格式，不改草稿字段/版本/fixture/hash/零值槽位或持久化 owner |

不新增 schema、迁移、记录、缓存、事务、恢复入口、credential 读取、provider 调用或权限/租户转换。全新安装/current product 的唯一事实 owner 不变；源码无消费者与运行资源不存在是两个不同结论。即使存在历史缓存/日志或旧进程，本文也不授予读取、迁移、停止或删除权限。

### 4.2 必须逐项保留

- logging 的 manager.go/manager_test.go、rotating_writer.go、level_split_hook.go、LevelFileConfig、LogManager/LogConfig/DefaultLogConfig、全局初始化/获取/级别/关闭及其实际调用与有效测试。其它当前 trace/request/identity 仍归原 owner。
- shared/resilience/retry.go、retry_test.go、Retry/RetryConfig/RetryAttempt 与原行为；shared/ptr 等其它基础层。整个 resilience 根、测试包结构及原共享 import 边界继续存在。
- OpenAI Client/ClientConfig、RequestPool/PoolConfig、BaseClient、**pool.go 内同名 RateLimiter**、Manager/ManagerConfig、Logger/AdaptLogrus、中立 config resolver、有效 route/credential 选择、配置缓存/dynamicClients、contextual text/image client、真实 image pipeline，及 Eino/Temporal/预算/结算/final gate/取消/UNKNOWN。#932 绝对 deadline 修复和原强断言不改。不能按通用名字 CircuitBreaker/RateLimiter/Cache/FromContext 全仓禁入。
- jsonx.UnmarshalString/CleanLLMResponse 的完整原声明/import/错误行为；当前严格草稿 decoder、持久化 JSON/版本/规范化/hash/SourceIdentity 四个零值槽位与读取拒绝。
- 当前采集、**原 Amazon 浏览器/extractor 的 goroutine.NewProcessor**、debug 入口、有效 timeout/httpclient/appenv、图像解码 JPEG/PNG/WebP 注册、纯规则、Store observation/official 及独立 SHEIN 登录/Store 同步。不能由本批 import/名字调查推导删除这些能力。

## 5. 删除、保留与抽取矩阵

### 5.1 九个整文件 RETIRE

以下路径逐一缺席，不保留空壳/dummy/alias，也不复制到新目录：

```text
internal/platform/logging/context.go
internal/platform/logging/helpers.go
internal/platform/logging/helpers_test.go
internal/shared/resilience/breaker.go
internal/shared/resilience/breaker_test.go
internal/shared/resilience/ratelimit.go
internal/shared/resilience/ratelimit_test.go
internal/integration/openai/cached_client.go
internal/integration/openai/resilient_client.go
```

helpers/breaker/ratelimit 的专属测试/benchmark 与无消费者旧实现同退，不把旧 API 测试当架构权威；原 manager/Retry/Client/Pool 的有效业务、安全、取消、日志与并发测试保留。

### 5.2 五个精确混合工件

| 文件 | 唯一允许变化 | 必须保留 |
| --- | --- | --- |
| internal/pkg/jsonx/unmarshal.go | 删除 UnmarshalBytes 的注释、完整泛型函数和自身分隔空白 | UnmarshalString、imports 与其它原字节/行为不变 |
| internal/integration/openai/logger_test.go | 删除 TestDecoratingConstructorsUseExplicitLogger；删除仅该测试使用的 time import | recorder/helpers、nil-safe/字段转发、静态与动态 Manager/Client/Pool 显式日志测试逐字保持 |
| tests/integration_provider_logger_contract_test.go | 仅 TestOpenAIConfigsExposeExplicitNonSerializedLogger 的 config 表删除 CachedClientConfig、ResilientClientConfig 两条 literal | ClientConfig/PoolConfig/ManagerConfig 的 Logger 类型与 json:"-" 检查及 grsai/gemini 断言保持；所有其它函数/helper/global/import 逐字保持 |
| tests/listingkit_backend_retirement_test.go | 仅 §6.1 指定的两个原函数体，精确转换 absence/owner/KEEP | 其它函数/helper/global/import 逐字保持，所有现有身份/JSON/清理/AI/图像/worker/Store 检查保持 |
| internal/shared/resilience/doc.go | 只将包说明从 retry/limiter/breaker 改为实际保留的 retry/backoff 用途；去掉 limiter/breaker 实现说明 | package 声明、Retry 源码与有效行为不变；不改 go.mod/go.sum 或实际开源组件 |

### 5.3 五个旧依据与三个当前文档

只在以下五份旧依据前加一段准确 superseding 注记，指回本文 §2/§5/§6.1。**完整原正文（含已有顶注）、数字/勾选/批准/历史证据逐字保留为 Git 原 blob 后缀**，不改旧计数或将旧结果重写为新 PASS：

```text
docs/architecture/legacy-numeric-value-contract-closure-retirement-boundary.md
docs/architecture/legacy-redis-proxy-shared-closure-retirement-boundary.md
docs/architecture/legacy-logger-facade-closure-retirement-boundary.md
docs/architecture/legacy-idle-support-contract-closure-retirement-boundary.md
docs/superpowers/plans/2026-08-30-internal-target-architecture-phase2.md
```

本文、docs/architecture/README.md、docs/refactoring/legacy-register.md 是三个当前文档；两个索引仅增加准确当前项，完整既有 Git blob 保留为后缀。#929 对 Retry/Resolved/context 等保留含义没有冲突，旧开源评估和 phase2-runtime-inventory/module-target-mapping 的历史/现役包根映射不改。不存在“顺带清掉全部旧文档”的授权。

```text
Legacy decision: RETIRE
Reusable behavior: 无新增 EXTRACT；有效日志、Retry、AI 执行与 JSON 已由当前 owner 保留
Current owner: platform/logging 的实际 LogManager；shared/resilience.Retry；当前 OpenAI Client/RequestPool/Manager；jsonx.UnmarshalString/当前草稿 owner
Cutover/deletion condition: 用户确认 §1–§7（含 §6.1）、Ready 读回、刷新实际消费者无冲突；九文件、五混合、五直接旧依据和三当前文档整批完成，无运行资源处置
```

## 6. 验证与验收边界

设计阶段只做只读源码/AST/消费者映射、矩阵/相对引用/历史后缀校验和原适用文档检查；不为文档制造 RED，不运行这些旧 wrapper 的测试证明其有用，不连接任何既有环境。原 CI 对准确设计 HEAD 的 completed/SUCCESS 与自动复核状态在主要 PR 如实分列。

未来实施复用原退休设施，先按 §6.1 更新两个原 guard/配置表，使待退工件仍在时真实 RED，再整批删除至 GREEN；不是新增退场检查器。删除单纯闲置内部实现不制造业务 TDD；原当前规则/安全测试继续保留。必要自检包括准确原 guard/文档/导入边界，logging manager/Retry/OpenAI logger+Client/Pool/路由相关已有测试，SHEIN title/worker/Store 受影响的原回归，以及当前应用、独立登录 worker、ImageAgent Temporal worker、原 Amazon debug 的真实依赖/编译。只编译命令，不执行初始化/迁移/安装/登录/provider。

不默认重复完整 CI、不新建故障注入平台/runner/PG/Redis/专项验收环境；稳定候选跑原 CI。形成最终候选前再同步一次实际 main，只验证真实组合风险；未变代码证据可明确复用，不能冒充新 SHA 直接执行结果。原 CI 真实失败先查 sibling paths/根因并分类；矩阵外修订须精确说明，不放宽检查、跳过缺文件或改保护规则。

### 6.1 原检查完整转换矩阵（确认实施后的准确授权边界）

| 原工件 / 原函数 | 唯一转换 | 保持的有效检查 |
| --- | --- | --- |
| listingkit_backend_retirement_test.go / TestLegacyRuntimeSkeletonAndIdleConfigClosureIsRetired | 在原 os.Lstat 列表追加 §5.1 九路径；原 retiredSupport 表新增下文三 owner 精确声明，jsonx 表追加 UnmarshalBytes；retained 表只移除 jsonx.UnmarshalBytes，追加下文正向 owner；原 owner filter 仅加入 shared/resilience 和 integration/openai | 其它全部旧禁入/正向字段、config/kernel/identity/decoder、读取/解析/decoded import/错误处理与遍历设施不变 |
| 同文件 / TestNumericContextAndValueContractsAreRetired | 原 map 仅删除 helpers.go 的“必须读文件”项，改为明确 os.Lstat 缺席；原八项数字日志禁入转入上一函数的 logging owner 精确表 | assertLegacyLoggerFacadeRetired 原调用、listingruntime 缺席、SourceIdentity 字符串/四零槽位/规范化/hash/读取拒绝及其它检查保持；不能 missing-skip |
| integration_provider_logger_contract_test.go / TestOpenAIConfigsExposeExplicitNonSerializedLogger | 只删两个退休 config literal，见 §5.2 | 三当前 config、Logger 类型/json:"-"、grsai/gemini 断言不变 |
| openai/logger_test.go / TestDecoratingConstructorsUseExplicitLogger | 专属测试及唯一 time import 随 wrapper 退场 | 其它当前 logger 传播/adapter/动态 resolver 测试及 helper 原样保持 |
| scanProviderConstructors、TestProductionProviderConstructorInventoryIsComplete；shared/import/resilience guards；其它检查、workflow、完整 inventory | **零改**；扫描器仍可识别旧构造器名，三条当前生产构造计数保持 | 不改计数、ceilings、扫描范围/解码/错误处理或任何 CI 条件，不 blanket 禁止保留包根 |

在原 owner-qualified AST 表中按原 receiver.name 逻辑逐一列名。原 slices.Contains 不会自动包含某类型的全部方法，所以以下方法不得仅凭 type 缺席代替。类型字段随其完整类型退场，不在其它 owner 禁用同名字段：

- **internal/platform/logging**：contextKey、loggerContextKey、traceIDKey、requestIDKey；WithLogger、FromContext、WithTraceID、GetTraceID、WithRequestID、GetRequestID、WithFields、WithField；FieldComponent、FieldPlatform、FieldProductID、FieldTraceID、FieldRequestID、FieldDurationMs、FieldRetryCount、FieldErrorCode、FieldErrorType、FieldOperation、FieldStatus、FieldUserID、FieldSessionID；LoggerHelper、NewLoggerHelper、LoggerHelper.LogOperation、LoggerHelper.LogOperationWithResult、LoggerHelper.LogProgress、LoggerHelper.LogRetry、LoggerHelper.LogAPICall、LoggerHelper.LogCacheHit、LoggerHelper.LogDatabaseQuery、LoggerHelper.LogStateChange、LoggerHelper.LogMetric；WithComponent、WithPlatform、ShouldLog。原八项继续禁止：LoggerHelper.LogTaskStart、LoggerHelper.LogTaskComplete、LoggerHelper.LogTaskFailed、WithTaskContext、WithStoreContext、FieldTaskID、FieldTenantID、FieldStoreID。
- **internal/shared/resilience**：ErrCircuitOpen、ErrTooManyRequests、CircuitState、StateClosed、StateHalfOpen、StateOpen、CircuitBreakerCounts、CircuitBreakerConfig、CircuitBreaker、NewCircuitBreaker、CircuitBreaker.Execute、CircuitBreaker.State、CircuitBreaker.Counts、convertCounts、convertState；RateLimiter、NewRateLimiter、RateLimiter.Wait、RateLimiter.Allow。
- **internal/integration/openai**：CachedClient、Cache、CachedClientConfig、NewCachedClient；CachedClient.CreateChatCompletion、CachedClient.generateCacheKey、CachedClient.Generate、CachedClient.AnalyzeImage、CachedClient.GetDefaultModel、CachedClient.SupportsAsyncImageGeneration、CachedClient.SubmitImageGeneration、CachedClient.SubmitImageEdit、CachedClient.QueryImageGeneration、CachedClient.ClearCache、CachedClient.SetTTL、CachedClient.Enable、CachedClient.Disable、CachedClient.IsEnabled、CachedClient.GetStats、CachedClient.Close；ResilientClient、ResilientClientConfig、NewResilientClient、CircuitBreakerConfig、CircuitState、StateClosed、StateOpen、StateHalfOpen、CircuitBreaker、Counts、DefaultCircuitBreakerConfig、NewCircuitBreaker、stateString；CircuitBreaker.Execute、CircuitBreaker.beforeRequest、CircuitBreaker.afterRequest、CircuitBreaker.onSuccess、CircuitBreaker.onFailure、CircuitBreaker.setState、CircuitBreaker.State、CircuitBreaker.Counts、CircuitBreaker.Reset、CircuitBreaker.OnStateChange；ResilientClient.CreateChatCompletion、ResilientClient.Generate、ResilientClient.AnalyzeImage、ResilientClient.GetDefaultModel、ResilientClient.SupportsAsyncImageGeneration、ResilientClient.SubmitImageGeneration、ResilientClient.SubmitImageEdit、ResilientClient.QueryImageGeneration、ResilientClient.GetCircuitBreakerState、ResilientClient.GetCircuitBreakerCounts、ResilientClient.ResetCircuitBreaker、ResilientClient.GetStats、ResilientClient.Close。
- **internal/pkg/jsonx**：在原九项禁入上仅追加泛型 UnmarshalBytes，保留原 FuncDecl 泛型解析，不改其它旧禁入。

原 retained 表中 logging 的 LogConfig/DefaultLogConfig 继续强制，精确追加 LogManager、NewLogManager、InitGlobalLogger、GetGlobalLogManager、GetGlobalLogger、SetGlobalLogLevel、LevelFileConfig；新增 resilience 的 Retry/RetryConfig/RetryAttempt，OpenAI 的 Client/ClientConfig/RequestPool/PoolConfig/BaseClient/RateLimiter/Manager/ManagerConfig/Logger/AdaptLogrus。jsonx 保留 UnmarshalString/CleanLLMResponse；其它原 retained 表逐字保持。该原函数用 .go 源文件扫描声明，正向名字存在不是行为等价证明，保留文件原字节、原有效测试和实际依赖/编译共同提供证据。

除该表明确的五混合文件与九整退文件，其它生产/测试/配置零改。没有新 helper/guard 文件、缺失包跳过、forbidden-import 豁免或弱化安全语义。若原 CI 暴露表外真实尾部，先报告具体根因、原语义与最小准确修订，不从“收齐闭包”推导无限授权。

## 7. 交付、停止点与授权

### 7.1 设计交付

唯一原 Writer/主要分支完成三文档，必要文档检查与原 CI 对准确设计 HEAD 绑定；#301 当前段只记录本批状态，历史后缀从“以下保留 #726 已合并批次与历史证据，不扩大本轮权限。”起逐字保持，旧当前段先归档。PR 列出本批完整 diff、设计依据、现有证据和仍未执行的阶段，交用户确认。不在设计 PR 中夹带生产删除、测试转换或旧依据顶注实施。

### 7.2 未来实施与最终检查

用户确认后先记录/读回 Ready，由原 Writer 在同一主要 PR 完成22工件，原 guard RED→GREEN、原保留行为回归/编译和原 CI；不逐小步重新请示。没有新增权限/状态/事实/副作用边界，不默认重开全局架构。最终交付检查集中核对真实完整 diff、§5/§6.1、保留文件和实际依赖/有效证据；有具体 finding 先分类，修复后仅复核该增量，不重复整轮或把理论 hardening 升级为新前置。

源码完成转 In Review，说明“实现完成，待独立验证/用户决定”。合并必须另获用户准确 HEAD 授权并正常 expected-head 合并；之后只以实际 merge SHA 匹配 event=push/head_branch=main 且整 run completed/SUCCESS 的 CI 收尾，不用 PR CI 或单个 Required Gate 代替。#301 不因本有界批完成而关闭。

### 7.3 复核与运行权限

本轮仅文档 N/A、未来为复用现有架构，不追加全局 Independent Architecture 检查、通用验证系统或人工试用前置。使用仓库已有自动复核时如实记录准确 HEAD 和状态，启动失败/无结果不是 PASS。未经用户本轮授权不默认新增 AI 互验线程；既往批次授权不扩展至本批。需要替代独立复核时在具体候选/既有证据形成后说明原因并申请准确范围，不默认重复触发 GitHub 评审。

真实登录、Store 同步、provider、部署、运行验收、用户产品验收均 **NOT_RUN**；既有实例/数据/凭据/进程与历史 cache/log 的读取/迁移/删除/停止均 **NOT_AUTHORIZED**。当前能力与独立 SHEIN 登录/Store 同步保留，不增加内部试用前置。任何源码/CI/AI 复核结果均不等于用户验收或运行发布授权。
