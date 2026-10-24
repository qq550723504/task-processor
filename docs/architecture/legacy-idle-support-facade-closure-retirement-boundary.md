> #301 / #937 **SOURCE_IMPLEMENTED**。用户已确认冻结设计9d1f2cbe1的§1–§7（含§6.1）；Reuse Existing Architecture / Ready先写入并读回[接单6386195192](https://github.com/qq550723504/shuomi-ai-commerce/issues/301#issuecomment-6386195192)及[读回6386203704](https://github.com/qq550723504/shuomi-ai-commerce/pull/937#issuecomment-6386203704)。同一主要PR收齐3整退+8精确混合+6旧KEEP窄注+3当前文档，共20工件，无新生产EXTRACT/替代owner；DB两原测试仅改调用，全部有效安全断言保留。当前日志/Retry/AI/JSON/采集及独立SHEIN登录与Store同步保持，workflow/完整inventory/依赖/保护与既有资源不动，不增试用前置，#301 OPEN。准确实现HEAD/CI/最终复核留PR，合并另授权。以下9d1f2cbe1冻结设计完整保留；其设计阶段/待授权表述只作当时阶段记录，删除/保留矩阵与§6.1按下方获批准确例外执行，原冻结内容逐字保留。

> **当前 §6.1 准确例外（2026-10-25，用户已批准）**：原 CI40109581690 的 Architecture/Backend 及本地原聚合测试实测同一 RED：TestListingKitTaskBackendSourcesAreRetired 仍要求已退休的 BoolPtr。按[准确提案6386410534](https://github.com/qq550723504/shuomi-ai-commerce/pull/937#issuecomment-6386410534)与[授权6386578200](https://github.com/qq550723504/shuomi-ai-commerce/pull/937#issuecomment-6386578200)，仅将该函数 required 表的 internal/shared/ptr/pointer.go 一行由 `func BoolPtr(` 改为 `func IntPtr(` / `func Float32Ptr(` 两项必需。原读取/错误处理/算法、其它全部断言及生产代码不变；BoolPtr 仍由本批 owner-qualified guard 强制缺席。以下原 §5.2/§6.1 “其它检查保持”仅对这一行被准确替代，其余约束保持；本例外仅改原20工件内测试/设计两文件，workflow/完整inventory/依赖/保护与既有资源不动，不增试用前置，#301 OPEN。原设计9d1f2cbe1完整 Git blob继续逐字保留为后缀。

# 五组闲置支持门面完整退休闭包

## 1. 用户结果、范围与 Design Basis

状态：**DESIGN_ONLY / AWAITING_USER_CONFIRMATION**。Refs #301，父目标保持 OPEN。用户已同意先将五组一起做完整设计，尚未授权源码实施、合并、部署、关单或真实资源操作。设计、实现、PR CI、实际 merge SHA 的 push/main CI、运行与用户验收分别记录。

使用者为维护当前应用及独立 SHEIN 登录/Store 同步的维护者。当前操作是继续收齐已失去生产消费者、却被旧 KEEP/原保活检查留住的支持门面；交付可直接确认的删除/保留、owner 和原检查矩阵，避免新代码误接旧入口。不新增用户操作、产品功能或内部试用前置，不为文件数或内部模块拆小 PR。

**Design Basis: N/A（当前只改本文、Architecture README、Legacy Register 三份文档）；文档工作 Ready。** 未来实施为 **Reuse Existing Architecture / Admission: NOT_READY**：复用现有 HTTP、verified Identity、Context DB、指针和 LogManager owner，只退出无当前消费者的入口。DB 两项有效安全测试改接现有 `openDatabaseContext` 参数化接口，不新增生产测试接缝。当前领域、身份/授权、事实、持久化、状态、事务、预算/计费、retry/UNKNOWN/recovery 与外部副作用边界均不改变，无新生产 EXTRACT/替代 owner。没有 UI/IA 改动，Figma N/A；现有批准架构完整覆盖，不重复全局 Independent Architecture Review。

未来完整交付为 **20 工件：3 整文件 RETIRE + 8 精确混合文件 + 6 旧依据窄顶注 + 3 当前文档**，五组在同一主要 PR 收齐。当前设计阶段仅编辑最后三份文档，其余十七工件只是拟实施矩阵，尚未修改。

正式实现须用户确认 §1–§7（含 §6.1），先在 #301/主要 PR 记录并读回 Reuse Existing Architecture / Ready，刷新实际消费者与 main 交集，再由原唯一 Writer 在同一 PR 连续完成。文档 CI 不授予源码准入。若发现真实当前消费者、具体外部可观察契约或必须改变保留边界，停止相应部分并报告，不临时加兼容路径或把旧入口接回当前调用链。

Out of Scope：workflow、完整 inventory（CURRENT/历史/计数/ceilings/parser/准确性/检查逻辑）、依赖/锁、保护规则零改；不 tidy/升级、迁包、复制 helper、添加 alias/wrapper/fallback、双读双写/同步或新 runner/验证平台。不读取既有凭据，不操作既有实例、数据、配置、日志、缓存、进程或真实 provider，不运行 DB/HTTP constructor、旧业务命令，不建 PostgreSQL/Redis/浏览器实例。#29 不重开，#301 保持 OPEN；部署、运行、环境处置与产品验收均不在授权内。

## 2. 权威、调查基线与前批 KEEP 替代

依据为 [Greenfield 产品决定](../product/greenfield-no-legacy-migration.md)、[Hard-Cut Policy](../refactoring/legacy-hard-cut-policy.md)、[Legacy Register](../refactoring/legacy-register.md)、[项目边界](project-boundaries.md)、[目标架构](project-target-architecture.md)、[Issue 驱动规则](../engineering/issue-driven-development.md) 与用户本轮范围。旧 API/测试不是架构权威；有效行为保留当前 owner，无当前要求的内部门面 RETIRE，不为未来假设造替代层。现有标准库、GORM、Logrus、req/v3/SDK 及依赖锁保留，符合“有成熟开源实现不重复造轮子”。

实际 main 基线 **10edafe0888f287f79c46808877e3e767e50f1a1**，tree **7d8de66576565bb24ebab6ce8570386666c7f238**。相对 #934 合并21e1eaecd，#936 仅改四份当前采集扩展/说明文件，没有 Go 或本闭包交集。已在此 main 重新解析全部 **3273 个受追踪 Go 文件**（包含测试、build tags、嵌套模块），解码 import 字面量、识别 alias/selector，再检查同包调用、原测试和保活引用；五个目标 owner 无 dot/blank import。[接单6385885789](https://github.com/qq550723504/shuomi-ai-commerce/issues/301#issuecomment-6385885789)记录唯一 Writer、分支/worktree、范围与 #934 上一当前段逐字归档。开放 #931 依赖与 #692 Store 设计不接管，不重复派工。

下列替代仅在用户确认本设计及实施准入后生效；不整体撤销任何包级能力，也不改旧数字/勾选/历史结果：

| 旧依据 | 原保留含义 | 本批准确替代 | 持续有效 |
| --- | --- | --- | --- |
| [#915 宿主/图像辅助](legacy-host-image-helper-closure-retirement-boundary.md) | TEMU API 当时使用 httpclient.ClientConfig/Build/buildTLSConfig；NewSimple 广泛 KEEP | 旧平台消费者已随 #926 退出，现只退 builder/专属测试和闲置 NewSimple | 下载器 Client/Config/New/Do、IP 定位 NewSimpleWithTimeout、图像解码/当前 TLS/retry owner 保留 |
| [#921 任务身份/映射](legacy-task-identity-mapping-closure-retirement-boundary.md) | request_roles.go 及三个 role/request-user helper 完整 KEEP | 此独立辅助层没有当前生产/同包消费者，类型/三入口/normalizer 同退 | Identity 五字段、WithIdentity/FromContext 的 nil/trim 合同、当前授权和 JSON/hash/四零槽位保持 |
| [#880 Redis/proxy/shared](legacy-redis-proxy-shared-closure-retirement-boundary.md) | shared/ptr 与 database 的广泛保留 | 只退六个无消费者指针变体与两无 context DB 入口/专属 helper；原 DB 安全断言转当前接口 | IntPtr/Float32Ptr、Context DB、Open/OpenShared/Close/CloseShared/Goose/原配置、实际 Redis/其它共享层保留 |
| [#893 logger facade](legacy-logger-facade-closure-retirement-boundary.md) | platform/logging manager 与原测试全部 KEEP | 只退 SetGlobalLogLevel 全局便利别名及唯一专属测试 | LogManager.SetLevel/GetLevel、级别并发/锁/singleton/hook/writer 与所有其它测试保持 |
| [#934 idle adapter/helper](legacy-idle-adapter-helper-closure-retirement-boundary.md) | 新 guard 正向要求 role helper、HTTP builder/NewSimple、SetGlobalLogLevel | §6.1 将这些准确项由保活转缺席/owner-qualified 禁入 | #934 已退九文件/114声明/八项数字日志禁入、当前日志/Retry/OpenAI/JSON 及其它原检查保持；旧复核/失败/措辞 BACKLOG 历史不改 |
| [Phase2 原计划](../superpowers/plans/2026-08-30-internal-target-architecture-phase2.md) | 全量迁包/导出及非 context DB、全局日志 setter、指针 helpers 保留 | 仅替代本批19项声明和专属尾部的继续保活含义 | 当前包根/owner、有效导入与全部其它当前行为保持 |

旧 SHEIN 活动设计和2026-07-10计划中的 `promotionFloat64Ptr` / `promotionTestFloat64Ptr` 是已退休链的不同 owner 历史样例，不是本批 `shared/ptr.Float64Ptr` 消费者，不改这些历史文件。`phase2-runtime-inventory` 的“if still justified by callers”与本设计一致，完整 inventory/计数/映射根保持；不是借本批改盘点或清扫所有旧文档。

## 3. contract → implementation → injection → consumer

| 组 | 候选的实际使用 | 保留的生产路径与 owner |
| --- | --- | --- |
| HTTP builder 与 NewSimple | ClientConfig/Build/buildTLSConfig 只互相调用并被 builder_test 使用；NewSimple 无生产/同包调用。不存在当前构造注入 | shared/browser/chrome_downloader.go 用 httpclient.Client/Config/New；ip_location.go 用 NewSimpleWithTimeout。当前 SHEIN SessionTransport/SDK 各用原配置，不接入此 builder |
| AI request-role 辅助 | requestRolesContextKey、RequestUserIDFromContext、WithRequestRoles、RequestRolesFromContext、normalizeRequestRoles 仅此文件内相互引用及 guard 保活，无当前调用 | authidentity、ImageAgent/Temporal、worker quote/review、OpenAI identity 消费 Identity/WithIdentity/FromContext；真实用户/角色授权归原 authidentity/authz/当前应用，不移交给此辅助层 |
| 无 context DB 门面 | OpenExistingReadOnly/OpenExistingWritable 无生产调用；openExistingReadOnly 仅前一入口与原只读安全测试消费 | current-application、独立 SHEIN worker、ImageAgent、schema/维护命令、采集初始化使用 OpenExistingReadOnlyContext/OpenExistingWritableContext；Store/commercial 还用 OpenShared，既有 schema CLI 用 Open，均保持 |
| 六个 ptr 变体 | Int16Ptr/Int32Ptr/Int64Ptr/StringPtr/Float64Ptr/BoolPtr 无包外生产/同包调用；后三项仅专属测试使用 | integration/openai/client.go 实际消费 IntPtr/Float32Ptr。Tencent SDK 的 common.StringPtr/common.BoolPtr 是不同 owner，完整保留，不按名字禁入 |
| SetGlobalLogLevel | 只有 manager_test.go 的 TestSetGlobalLogLevel 调用；没有生产接线 | appenv → InitGlobalLogger/GetGlobalLogManager/GetGlobalLogger；LogManager 的实例 SetLevel/GetLevel 与原并发安全/字段/输出机制保持 |

不能仅从“无外部 selector”推导无用：FromBytesWithFormat 被现役 Inspect 同包消费；NamespaceForDirectory/InstallationNamespace 被安装隔离路径消费；Temporal.Options、LogManager 构造/默认配置、RetryAttempt、goroutine 的类型/执行方法也服务当前调用。它们均不在删除矩阵，不因与候选同目录而一起退。

**架构问题与处置**：旧 builder、未接线的角色 context、无 context DB 入口和全局便利 alias 在现 owner 旁继续暴露，前批广泛 KEEP 已不能证明它们是当前要求。根因处置是删除这些闲置入口和专属保活尾部，沿原 owner 保留真实能力；不重写请求/身份/DB/日志体系、不引入统一工厂或另一事实源。旧名称或 Package import 本身也不能证明是 Legacy，应以准确消费者与用户范围判断。

## 4. 持久化、副作用与安全不变量

| 边界 | 本批效果 | 必须不变 |
| --- | --- | --- |
| HTTP/TLS/retry | 未装配 builder 与闲置标准库便利入口退出，没有当前网络请求切换 | 当前 downloader/IP/session/provider 的超时、TLS、代理、headers、retry/UNKNOWN 与实际 constructor 源码不改；不发请求或读取 Cookie/凭据 |
| 身份与角色 | 删除无消费者的 context key/便捷函数，不新增身份承载或授权判断 | Identity 的五字段/WithIdentity/FromContext/nil/trim、认证/有效 Organization/角色/租户与计费归属不变；不动当前授权边界、序列化或日志身份 |
| DB | 删除旧无 context 公共入口及其专属 private helper，原安全断言只改接当前现成测试接口 | Context 入口的 DisableAutomaticPing/PingContext/关闭/取消/deadline，不建库/不隐式重试/只读会话或不强制只读、既有 Open 创建行为、共享池引用/Close/Goose/事务与当前 owner 全保留 |
| ptr | 删除六个无状态变体，不改变任何 wire 字段或数值 | IntPtr/Float32Ptr 原实现/调用、OpenAI request/JSON、草稿/hash/SourceIdentity 四零槽位与读取拒绝不变；不新增泛型替代或迁移 |
| logging | 只退未使用的全局 setter alias | LogManager.SetLevel/GetLevel、原锁/并发/singleton、level/config/hook/writer/输出/关闭不变；不清理或搬运日志 |

无新 schema、事实、记录、状态机、事务、恢复 owner、provider 调用、配置开关或权限模型。DB 现有实例/角色/数据、旧程序或手工调用是否存在为 **UNKNOWN**；静态无消费者不证明物理状态已退出。本设计不授予连接、试开 DB、迁移、删除、停机或凭据检查权限，不把这些操作作为内部试用前置。

### 4.1 原 DB 有效断言的保留方式

当前 `openDatabaseContext(ctx, cfg, options, opener, creator)` 已接受测试 opener，原 `postgres_test.go` 也已用它验证 deadline。未来只在两个现有测试内改调用：

- `TestOpenExistingReadOnlyDoesNotCreateOrRetryMissingDatabase`：原 `openExistingReadOnly(cfg, opener)` 改为 `openDatabaseContext(t.Context(), cfg, databaseOpenOptions{readOnly: true}, 原opener, nil)`；opener body、fixture、一次调用、错误必须非 nil、DSN 只读标记断言原样保持。
- `TestOpenExistingWritableDoesNotCreateOrForceReadOnly`：原 `openDatabase(cfg, databaseOpenOptions{createIfMissing: false}, opener, nil)` 改为同参数的 `openDatabaseContext(t.Context(), ...)`；所有 fixture、opener body、一次调用、错误与不强制只读断言保持。

不新增函数/测试 seam/global/import，不改或删除任何有效断言，不把缺表/缺库变为成功，不引入 retry/create 或“缺环境跳过”。原 deadline/关闭、Open 创建/重连、nil config、共享池最后引用关闭等其它测试完整保留；真实 PG 权限/public-facts/启动 context/事务安全检查与 CI 仍按原规则执行。本阶段不跑这些测试、不建立 DB。

## 5. 完整删除/保留矩阵

### 5.1 三整文件 RETIRE

```text
internal/pkg/httpclient/builder.go
internal/pkg/httpclient/builder_test.go
internal/shared/aiidentity/request_roles.go
```

整退 builder 的 ClientConfig/Build/buildTLSConfig、两项专属测试，以及 requestRolesContextKey/RequestUserIDFromContext/WithRequestRoles/RequestRolesFromContext/normalizeRequestRoles。不留空壳/dummy/alias/re-export，不迁包保活，无新生产 EXTRACT。当前消费者已有自己的 HTTP/身份 owner，不把旧测试转为要求重建闲置 API。

### 5.2 八精确混合文件

| 文件 | 计划改动 | 保留边界 |
| --- | --- | --- |
| internal/pkg/httpclient/httpclient.go | 仅退 NewSimple 的完整声明/专属注释及分隔空白 | Config/Client/New/Do/NewSimpleWithTimeout、imports/字段/超时/retry/日志行为保持 |
| internal/shared/ptr/pointer.go | 仅退 Int16Ptr/Int32Ptr/Int64Ptr/StringPtr/Float64Ptr/BoolPtr 六函数与各自注释 | IntPtr/Float32Ptr 原字节/类型/值语义保持；不整体禁 ptr 或外部 SDK 同名 helper |
| internal/shared/ptr/pointer_test.go | 仅退 TestStringPtr/TestFloat64Ptr/TestBoolPtr 三专属测试 | TestIntPtr/imports 原样，不制造替代 API 或新测试框架 |
| internal/platform/logging/manager.go | 仅退 SetGlobalLogLevel 声明与专属注释 | LogManager 实例方法、初始化/获取/默认配置、级别/锁/hook/writer/输出/关闭完整保持 |
| internal/platform/logging/manager_test.go | 仅退 TestSetGlobalLogLevel | 其余每个测试/benchmark/import 保持，尤其原 SetLevel/GetLevel/singleton/锁/懒初始化并发断言 |
| internal/platform/database/postgres.go | 仅退 openExistingReadOnly/OpenExistingReadOnly/OpenExistingWritable 三声明及专属注释；仅重写两个 Context 公共函数注释，去除对已退入口的描述并说明当前不建库/只读/有界合同 | 两 Context 函数生产 body 逐字不变；Open/OpenShared/Close/CloseShared、openDatabase/openDatabaseContext、DSN/driver/creator/共享引用/配置/池行为保持 |
| internal/platform/database/postgres_test.go | 仅 §4.1 两原测试的调用目标/新增显式 context 与原 options 参数 | 不重命名测试，不改其 opener/fixture/断言/错误路径；其它测试、helper、global、import 保持 |
| tests/listingkit_backend_retirement_test.go | 仅 §6.1 的一个原 guard 完成精确 absence/owner/保活转换 | 其它历史禁入/有效安全/字段/JSON/采集/worker/Store/并发检查、遍历/解析/错误处理保持 |

### 5.3 六旧依据与三当前文档

未来仅对以下六份旧依据增加准确 superseding 顶注，指回本文 §2/§5/§6.1，**完整原 Git blob（含已有顶注）、历史数字/计数/勾选/批准/证据逐字保留为后缀**：

```text
docs/architecture/legacy-host-image-helper-closure-retirement-boundary.md
docs/architecture/legacy-task-identity-mapping-closure-retirement-boundary.md
docs/architecture/legacy-redis-proxy-shared-closure-retirement-boundary.md
docs/architecture/legacy-logger-facade-closure-retirement-boundary.md
docs/architecture/legacy-idle-adapter-helper-closure-retirement-boundary.md
docs/superpowers/plans/2026-08-30-internal-target-architecture-phase2.md
```

本文、docs/architecture/README.md、docs/refactoring/legacy-register.md 为三当前文档；两个索引仅增加准确当前项，完整原 Git blob 保留为后缀。历史促销样例、完整 inventory/module 根映射、其它历史文档不改；§2 已说明与当前保留边界没有冲突的引用不扩大为清扫范围。

```text
Legacy decision: RETIRE
Reusable behavior: 无新生产 EXTRACT；原 DB 安全断言只改接现有 Context owner，当前有效 HTTP/Identity/DB/ptr/logging 已有原 owner
Current owner: 原 HTTP consumer/SDK；aiidentity.Identity + authidentity/authz；platform/database Context/共享池；ptr.IntPtr/Float32Ptr；LogManager
Cutover/deletion condition: 用户确认 §1–§7（含 §6.1）、Ready 读回、刷新实际消费者无冲突后，同一主要 PR 收齐3整退/8混合/6旧依据/3当前文档，无运行资源操作
```

## 6. 必要验证与验收边界

设计阶段仅只读消费者/声明/原检查/owner 映射、路径/相对引用/清单和历史后缀校验、原适用文档检查与稳定候选原 CI。不为文档制造 RED，不用旧 helper 自测证明其有用，不调用 constructor/业务入口/DB/provider，不建新验证工具。设计 CI/复核只证明准确文档候选，不替代未来实现、合并后 main CI、运行或用户验收。

未来实施先在原 guard 加精确缺席/禁入并确认 RED，再按矩阵删除至 GREEN；DB 原安全断言迁接与其它必要回归一起验证。复用原 SQLMock/现成内存 fixture，无新 PostgreSQL/Redis/容器/浏览器实例或专项故障矩阵。必要本地自检限原 guard/相关共享导入与文档合同、Context DB 两测试及原 deadline/共享池、LogManager 原行为、HTTP 当前消费者/Identity/ptr/OpenAI/SHEIN 保留回归和当前应用/独立登录/ImageAgent worker/原 Amazon debug 的真实依赖/编译，命令只编译不执行。稳定候选跑原 CI，不人工重复 CI 的完整相同套件。

形成最终候选前再同步一次实际 main，只处理真实组合问题；未变化代码可明确引用已有证据，不冒充新 SHA 的直接执行。review/CI finding 先按当前 Must 分类、核查 sibling paths；范围内同根必要修复在同一 PR 收敛，记录准确差异，不降低断言强度或把未执行写成 PASS。必须改变保留能力、产品/安全/事实/持久化/副作用边界、workflow/inventory 或扩大工件范围时，先报告用户，不从一次失败推导重建通用机制。

### 6.1 原检查完整转换矩阵

| 原检查 | 精确转换 | 不变部分 |
| --- | --- | --- |
| tests/listingkit_backend_retirement_test.go / TestLegacyRuntimeSkeletonAndIdleConfigClosureIsRetired | 原 os.Lstat 清单追加 §5.1 三路径；retiredSupport 表按下文准确 owner 声明追加；retained 表退对应保活项并加下文当前 owner；原 owner filter 只加入 shared/ptr 与 platform/database | 原全部其它 retired/retained 项、kernel/config/Identity字段、八项旧数字日志禁入、#934的114项禁入、解码/遍历/解析/错误处理及 JPEG/PNG/WebP 生产注册检查保持；不 missing-skip、不禁当前包 import |
| postgres_test.go / §4.1 两原函数 | 只将原调用改接现成 openDatabaseContext，显式 t.Context()/原 options/opener/nil creator | 原全部有效错误/调用次数/只读或不强制只读断言、fixture/closure 保留；原 deadline/共享池/创建行为等其它测试保持 |
| manager_test.go / TestSetGlobalLogLevel | 随已退 alias 删除专属测试 | LogManager.SetLevel/GetLevel 及所有其它原安全/并发/输出/benchmark 检查保持 |
| pointer_test.go / TestStringPtr、TestFloat64Ptr、TestBoolPtr | 随六变体删除三专属测试 | TestIntPtr 与现有 OpenAI request/ptr 实际行为回归保持 |
| builder_test.go / TestBuildClientDefaultsToStrictTLS、TestBuildClientAppliesRequestSettings | 专属测试与闲置 builder 整文件同退 | 当前 downloader/IP/session/provider 的已有实际 TLS/retry/timeout 检查与源码保持，不新建替代 builder |
| SharedPackageTargetsExist、shared/provider import/SSA/logger config、其它有效安全检查、CI/workflow/完整 inventory | 保持原入口、原逻辑与强度；ptr/resilience 等包根继续存在 | 不改扫描器/AST算法、CURRENT/计数/ceilings、构造清单或原 CI 条件，不新增豁免 |

在原 owner-qualified AST 表中逐一增加以下 **19 项声明**，不能全仓按名字或前缀禁入：

- `internal/pkg/httpclient`：在原禁入项上追加 ClientConfig、Build、buildTLSConfig、NewSimple。
- `internal/shared/aiidentity`：在原旧 Envelope/数字 scope 禁入项上追加 requestRolesContextKey、RequestUserIDFromContext、WithRequestRoles、RequestRolesFromContext、normalizeRequestRoles；不能禁当前 FromContext/Identity。
- `internal/platform/database`：新增 openExistingReadOnly、OpenExistingReadOnly、OpenExistingWritable；必须是完整精确标识符，不匹配仍有效的 *Context 入口。
- `internal/shared/ptr`：新增 Int16Ptr、Int32Ptr、Int64Ptr、StringPtr、Float64Ptr、BoolPtr；不触及外部 SDK common 同名函数。
- `internal/platform/logging`：在原 helper/context 禁入上仅追加 SetGlobalLogLevel。

原 retained 表的准确调整：aiidentity 只退三个 request/role helper 与 normalizeRequestRoles 四项，保留 Identity 五字段/WithIdentity/FromContext；httpclient 只退 NewSimple/ClientConfig/Build/buildTLSConfig，保留 Config/Client/New/Client.Do/NewSimpleWithTimeout；logging 只退 SetGlobalLogLevel，原当前项保留并正向要求 LogManager.SetLevel/LogManager.GetLevel。新增 ptr 的 IntPtr/Float32Ptr；database 的 OpenExistingReadOnlyContext/OpenExistingWritableContext/Open/OpenShared/Close/CloseShared/openDatabase/openDatabaseContext/databaseDSN。其它原 retained/retired 条目保持，原 FuncDecl receiver.name/GenDecl 精确比对算法与错误处理不改。

原测试正向名字存在不等于行为等价。最终还须比对表外/current 生产源码与原有效测试，特别是两 Context 函数 body、DB 安全断言、LogManager 其余方法/测试、IntPtr/Float32Ptr、HTTP NewSimpleWithTimeout 和 Identity/JSON/hash 合同，结合真实依赖/编译和原 CI 提供证据。若真实测试发现遗漏的同根引用，先分类并记录其与本矩阵的关系，在不改变保留合同/断言强度的范围内收敛；不能以“收齐尾部”为由删安全断言、跳过错误或扩大产品边界。

## 7. 交付与停止点

### 7.1 设计交付

本阶段只交三文档，提交/推送一个主要设计 PR，#301 当前段记录本批范围/Design Basis/状态，旧当前段先归档；历史后缀从“以下保留 #726 已合并批次与历史证据，不扩大本轮权限。”起逐字保持，不改旧勾选。必要文档自检、准确设计 HEAD 原 CI 与自动复核状态分别记录，不把无结论写成 PASS。文档完成后交用户确认，不夹带生产/测试/旧依据顶注的提前实施。

### 7.2 用户确认后连续实施

用户确认 §1–§7（含 §6.1）后先记录/读回 Ready，由唯一原 Writer 在同一 PR 收齐20工件、原 guard RED→GREEN、DB有效断言迁接、必要保留回归/依赖/编译与原 CI；范围内修复/提交/推送/维护连续完成，不逐小片重开 PR 或重复请示。没有新增高风险 owner/边界，不重复全局架构评审。

完整结果形成后做一次集中最终交付检查，核对真实完整 diff、实际调用与保留能力/有效证据；修复后只复核必要增量。实现完成转 In Review，明确待独立验证/用户决定，不签发用户产品验收。合并另获准确 HEAD 授权并正常 expected-head 执行，之后只以 actual merge SHA 匹配 push/main 的整 run completed/SUCCESS 确认，不用 PR CI/其它提交/单 Gate 代替。#301 不因本有界批结束而关闭。

### 7.3 复核与真实资源权限

当前文档 N/A、未来复用当前架构，不追加全局 Architecture Review、通用治理/验收系统、人工操作流程或内部试用前置。已有自动复核绑定实际 SHA，启动失败/无正式结果如实分列，不默认重复触发同 SHA 的运行中评审。**未经用户本轮授权不默认新增 AI 互验线程**；前批授权不扩展到本批。确需替代独立检查时，在具体候选与现有证据形成后说明原因和准确范围。

真实登录、Store 同步、provider、部署、运行、用户与生产验收均 **NOT_RUN**；既有实例/数据/凭据/进程/日志/缓存/配置的读取、迁移、清理或停止均 **NOT_AUTHORIZED**。当前日志、Retry、AI、JSON、采集、原浏览器/debug、图像以及独立 SHEIN 登录与 Store 同步保持。源码/CI/AI复核不等于运行交付或产品验收；本设计不扩展部署、关单或真实资源权限。
