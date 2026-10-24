> #301 / #937 **USER_CONFIRMED**：[五组闲置支持门面完整退休闭包](legacy-idle-support-facade-closure-retirement-boundary.md) §1–§7（含§6.1）已获用户确认，Reuse Existing Architecture / Ready与Issue/PR读回6386195192/6386203704先于源码修改。仅替代request_roles.go及requestRolesContextKey/三个request-role便捷函数/normalizer的KEEP；该层无当前生产消费者，整文件退出。Identity五字段、WithIdentity/FromContext/nil/trim、当前认证授权、JSON/hash/SourceIdentity四零槽位和读取拒绝保持。只替代准确旧KEEP；以下完整原Git正文/既有顶注/数字/勾选/批准/历史证据逐字保留。workflow/完整inventory/依赖/保护与既有实例/数据/凭据/进程零改，不增试用前置，保留独立SHEIN登录与Store同步，#301 OPEN。

> #301 / #923 **SUPERSEDED，仅旧租户 Prompt 闭包**：用户已确认[完整退休设计 §1–§7（含 §4.2/§6.1）](legacy-tenant-prompt-closure-retirement-boundary.md)。本文涉及旧 `internal/prompt`、`internal/promptmgmt`、`shared/tenantctx`、五 YAML 及其现役 KEEP/复用建议仅在该准确矩阵内被退休决定替代；如涉及镜像打包，仅退出两条 `prompts` COPY。当前 AI/图像/采集、身份/有效规则、独立 SHEIN 登录与 Store 同步及其它 KEEP 不变。停止全新 schema 创建旧 Prompt 表，不改 Goose 登记或已有表/行，不新增替代 Store/迁移/兼容。以下原正文、历史数字/勾选/证据完整保留，不扩大实施、合并或真实资源权限。

# #301 旧任务身份与数字映射合同完整退休闭包设计

Refs [#301](https://github.com/qq550723504/shuomi-ai-commerce/issues/301)。#301 保持 OPEN，#29 不重开。

- Status: **USER_CONFIRMED / SOURCE_IMPLEMENTED**。用户确认冻结设计 `ad8f8f3c421b3a16c77badc209d08cdfbc94b2a8` §1–§7（含 §6.1），同一 #921 按准确矩阵整批实施；准确源码 HEAD、必要自检、CI、实际源码复核与权限留主要 PR，不把源码完成当运行/用户验收。
- Design Basis：设计阶段三文档 **N/A**；本轮 **Independent Architecture / IMPLEMENTATION_READY**。[用户确认/实施接单6379938077](https://github.com/qq550723504/shuomi-ai-commerce/issues/301#issuecomment-6379938077)及 Issue/PR Ready 读回先于源码修改。设计原 CI 与授权独立只读设计复核 PASS，冻结矩阵/保留边界不扩展，无新身份机制。
- 基线：main **c6e47a9060b12cacc367d10a671efb0ae4b97fc4**，tree **584d8b9d01503c29e388580a092941e20cca86a0**。#917 的准确 push/main CI40059797762 SUCCESS 仅为前批证据。
- 唯一 Writer 沿原独占 worktree，分支 `codex/issue301-task-identity-mapping-closure-design`；[接单及前批当前段逐字归档6379721944](https://github.com/qq550723504/shuomi-ai-commerce/issues/301#issuecomment-6379721944)。一个主要 PR 覆盖两组完整闭包，不按工件数机械拆批。

## 1. 用户结果、范围与产品依据

使用者是维护当前 AI/图像、采集及共享合同的维护者。结果是旧 Task 的执行信封/租户辅助与最后数字刊登映射类型完整退出，现役身份、隔离、JSON 和任务恢复继续由当前 owner 执行。没有当前核心用户操作因此阻塞；不以父目标清零为由新增平台、迁移、验收工具或内部试用前置。

Scope：`shared/aiidentity` 的 ExecutionEnvelope、PersistedExecutionEnvelope、旧 absent/partial/present 分类、capture/restore/context/校验与 Task 租户辅助，以及两份专属测试；`listingruntime` 最后 ProductImportMapping；原检查、显式 KEEP 和直接文档尾部同批收齐。设计阶段只新增本文并前置两个索引；拟实施准确范围见 §5/§6.1。

Authority 为用户本轮先设计、确认后实施的决定、[Greenfield 决定](../product/greenfield-no-legacy-migration.md)、[Hard-Cut Policy](../refactoring/legacy-hard-cut-policy.md)、[Legacy Register](../refactoring/legacy-register.md) 与 [Issue 驱动规则](../engineering/issue-driven-development.md)。当前身份归 authidentity/各领域执行授权，当前 ImageAgent Organization 身份见 [现行隔离边界](../engineering/imageagent-organization-scope.md)及[当前产品合同](product-image-agent-v1.md)；Commercetool 仍按[现有基础合同](../superpowers/specs/2026-08-31-commerce-tool-foundation-design.md)消费可信上下文。UI/IA/Figma N/A，无新用户流程。

Must：完整闭包；当前身份/跨组织拒绝/撤权/角色、AI 凭据选择、ImageAgent/Commercetool 当前执行与恢复不变；当前 JSON/版本/规范化/hash/读取拒绝、SourceIdentity 四零值槽位及全部采集、图像、Store、独立 SHEIN 登录与 Store 同步保留。Should：复用当前 owner、标准库、原 AST/import/dependency 检查与成熟组件，无替代框架。Out of Scope：任何当前身份/权限/状态/事务/事实 owner 变更、schema/迁移/配置/依赖整理、平台 API/Prompt/localagent 退休、旧数据兼容/fallback/双读双写、真实环境操作。Accepted Risks 无新增；真实环境状态 UNKNOWN，运行/产品验收 NOT_RUN。

Threat Model 限误删现役同名身份合同、把已死的旧无租户放行分支移植到当前授权、把旧 GORM/JSON 声明误认作当前持久化 owner，以及为保活历史测试重新引入旧任务。不得把其它领域同名 ExecutionEnvelope、TenantID 或映射字段列为全局禁词。

## 2. Design Basis、根因与旧 KEEP 替代

根因是旧任务/属性最后消费者已分批退休，早期为保护当时运行链而保留的共享协议和断言继续要求空壳存在。包级当前消费者不能证明其中旧声明仍有效；反过来，当前包无 importer 也不能自行撤销明确 KEEP。按 **RETIRE** 退出本轮准确旧协议，现役行为继续复用原 owner，**无 EXTRACT、新 wrapper、第二事实源或替代信封**。

拟实施选 Independent Architecture：涉及旧身份/持久化公共值协议的整体删除、#868/#912 等显式 KEEP 的替代及 31 个范围工件，须先独立核对真实消费者和不变边界；不把“零 importer”当作准入。没有新的运行机制，不因此重开全局 IAM、恢复或数据架构。

| 前批/旧依据 | 经本轮确认后仅替代的对象 | 继续有效 |
| --- | --- | --- |
| #822、#880 对 shared/aiidentity/原身份测试的整体保留 | 旧 envelope/tenant_scope 两实现与两专属测试 | context.go/context_test.go、request_roles.go、authidentity 与当前各领域授权 |
| 2026-08-22 旧信封及 ProductEnrich 完整性 spec/plan | 旧 Task row 嵌入、旧来源白名单、无 scope 放行、旧恢复/分类和实施步骤 | 当前已验证身份/隔离要求；不在现役 owner 复建旧协议 |
| #740/#824/#831/#867/#868/#912 及早期刊登依据 | ProductImportMapping 完整类型与其原文件必须存在的 KEEP；#868 映射编码用途已随 #912 两属性退出 | SourceIdentity 四 JSON 零槽位及原 trim/规范化/hash/拒绝；当前 opaque metadata 不全局删字段 |
| #854 原收口表、module-target-mapping 和两个 TEMU 参考 | 仅 listingruntime/旧映射仍属当前依赖或推荐实现的准确含义 | 原历史正文/计数/证据，当前 Sourcing/Collection/Catalog/Listing/Store 与其它参考 |

§5.3 的 22 份直接依据只加准确 superseding 顶注，原全部正文逐字后缀保留；用户确认本轮设计之前，旧 KEEP 不因这份提案自动失效。

## 3. 实际消费者：contract → implementation → injection → consumer

核对全部 tracked Go，含 tests/build tags、导入别名、包内引用及 `hack/debug` 独立模块；区分声明、自调用、测试调用与实际注入。

| 对象 | 实际消费者/接线 | 决定 |
| --- | --- | --- |
| ExecutionEnvelope/PersistedExecutionEnvelope、版本/错误/分类、context key、capture/restore/校验/转换/helpers | 仅 envelope.go 内部链、envelope_test.go 和 tenant_scope.go/test；没有现役模型嵌入、serializer、repository、schema 注册、生产/debug 调用 | 四文件闭包随下一行整退；不转交旧来源白名单或错误协议 |
| TenantIDFromContext/NormalizePersistedTenantID/TenantMatchesContext/TenantCanCreateTask（aiidentity owner） | 只有 tenant_scope.go 内部调用和 tenant_scope_test.go；旧 empty scope 放行不是当前 Organization 授权实现 | 实现和四专属测试整退，不搬入当前认证 |
| listingruntime.ProductImportMapping | types.go 唯一生产声明；全仓没有 importer 或类型消费；原产品属性编码 caller 已在 #912 退出 | 最后类型及 Go 包整退；不清理真实表/行 |
| aiidentity.Identity/WithIdentity/FromContext | authidentity/authenticated_identity.go；app/worker/imageagent 的 generation_quote/review_invocation；imageagent/service、temporal/activities、temporal/organization_scope；integration/openai/identity.go | 实现、字段、归一化/nil 语义及注入链全部 KEEP |
| request_roles.go | 原公开 request-role 合同，与上述 context 同包 | 全文件 KEEP；不因本轮另做消费者清扫 |
| 当前持久化执行与授权 | ImageAgent 的 ExecutionIdentity/RunScope/Organization authorizer/repository/Temporal；Commercetool 自有 ExecutionEnvelope/Principal/permission；当前 AuthenticatedIdentity/effective Organization/member/roles | KEEP 同名但不同 owner 的合同及所有拒绝/事务/恢复行为 |

现役链保持 `verified identity → authidentity.WithAuthenticatedIdentity → aiidentity.WithIdentity/FromContext → 当前 ImageAgent/AI adapter`；Organization 运行在原仓库读取并校验 run/member/BusinessTask/版本后，才恢复当前上下文。当前 Tool envelope 由 Commercetool 构造，不依赖旧 shared envelope。原 SHEIN CLI/bootstrap/worker 与 Store 的 binding/attempt/claim/fence/UNKNOWN 仍由 sheinlogin/storecenter 所有。

## 4. 持久化、JSON、权限及副作用 owner

| 边界 | 删除与保留结果 |
| --- | --- |
| 旧 GORM 六列 execution_identity_version/tenant/user/trace/source_platform/source_task_type | 只是待退类型的历史 tag，当前模型无嵌入/注册/读写；删除 Go 声明不执行 DROP/ALTER、迁移、数据查询、清理或旧任务恢复。实际对象/在途状态 UNKNOWN，归原运行/数据 owner |
| 旧 ProductImportMapping | 不持有当前 repository/事务/JSON writer；旧数字 ID/tenant/Store/owner/价格字段随整类型退出，不投影到当前事实，不猜测旧库已空 |
| 当前数据合同 | SourceIdentity 原四零值 JSON 槽位、六 Source*、规范化/trim/发布拒绝、EnvelopeJSON/SnapshotJSON/InputHash/PublicationIdentity/lineage，当前草稿/预览/严格解码/版本/clone 全不变；不加全局未知字段丢弃/禁止，不处理已有记录 |
| 当前身份/恢复 | authidentity/effective Organization/member/roles、ImageAgent/Tool/AICapability 的现有准入、预算/计量/幂等、live authorization、取消/重试/响应丢失/恢复保持；旧 empty-scope/absent envelope 分支不成为当前 fallback |
| SHEIN/Store、采集/平台 | 继续原 owner，所有登录、Store sync、fence/UNKNOWN、来源获取、API/规则不改，不作任何真实调用或接管 |

没有新表/持久化事实/队列/缓存/文件格式、权限边界或不可重复副作用。源码删除不改变当前任一 commit/retry/UNKNOWN/recovery owner；无当前序列化调用则不为旧类型删除升级当前版本。若发现具体当前外部可观察兼容义务或真实消费者，停止对应删除、报告证据并重新确定边界；不得自行新增兼容、迁移或放宽准入。

## 5. 完整删除、保留与文档矩阵

### 5.1 五个整退工件

| 文件 | RETIRE |
| --- | --- |
| internal/shared/aiidentity/envelope.go | 全部旧信封/持久化/版本/错误/状态/context/转换/规范化/来源白名单 |
| internal/shared/aiidentity/envelope_test.go | 仅验证上述旧 Task 协议的六个测试；不改现役 context/authidentity/Organization 拒绝测试 |
| internal/shared/aiidentity/tenant_scope.go | 旧 Task/历史无 scope 判定四函数及专属 imports/comments |
| internal/shared/aiidentity/tenant_scope_test.go | 上述四函数专属测试；不把其旧无 scope 放行正例搬入当前 owner |
| internal/listingruntime/types.go | 最后 ProductImportMapping 完整类型/字段；整 Go 包退出，无替代 DTO |

aiidentity 的准确 owner-qualified 禁入声明为：

```text
CurrentEnvelopeVersion ErrMissingIdentity ErrIdentityIntegrity
ExecutionEnvelope PersistedExecutionEnvelope PersistedEnvelopeState
PersistedEnvelopeAbsent PersistedEnvelopePartial PersistedEnvelopePresent envelopeContextKey
ExecutionEnvelope.Validate CaptureExecutionEnvelope WithExecutionEnvelope
ExecutionEnvelopeFromContext RestoreExecutionEnvelope EnsureExecutionEnvelopeContext
PersistedExecutionEnvelopeFrom PersistedExecutionEnvelope.State
PersistedExecutionEnvelope.ExecutionEnvelope PersistedExecutionEnvelope.executionEnvelope
normalizePersistedEnvelope normalizeEnvelope validSource
TenantIDFromContext NormalizePersistedTenantID TenantMatchesContext TenantCanCreateTask
```

### 5.2 KEEP 与选择性抽取结论

**无新抽取代码**。`shared/aiidentity/context.go`、`context_test.go`、`request_roles.go` 完整 KEEP；Identity 五字段 AgentRunID/TenantID/UserID/BusinessTaskID/TraceID、WithIdentity/FromContext 与 nil/trim 语义不变。RequestUserIDFromContext/WithRequestRoles/RequestRolesFromContext/normalizeRequestRoles 原样。authidentity、当前 ImageAgent/Temporal/Tool、OpenAI identity/client、SourceIdentity/codec/hash、当前规则/采集/图像、平台 API、Prompt/localagent、独立 SHEIN 登录和 Store 同步所有源码与测试零改。旧规范化仅服务被退协议，不复制到现役身份或引入新全局 nil-safe/tenant helper。

### 5.3 二十二份直接依据的窄顶注

用户确认后已按下列准确文档范围同批补齐窄顶注；各文件只添加指向本文的 superseding 前缀，原完整正文逐字后缀保留，不能重写旧计数、历史勾选、批准或追修步骤。

| 组 | 文件（相对 docs/） | 仅替代 |
| --- | --- | --- |
| 旧信封/完整性 spec/plan（4） | superpowers/specs/2026-08-22-ai-execution-identity-envelope-design.md；superpowers/plans/2026-08-22-ai-execution-identity-envelope.md；superpowers/specs/2026-08-22-productenrich-governed-execution-integrity-design.md；superpowers/plans/2026-08-22-productenrich-governed-execution-integrity.md | 旧 shared envelope/Task tenant/六列/续跑方案；不是当前执行 owner。历史 plan 的派工/迁移要求不构成本轮授权 |
| 旧 shared KEEP（2） | architecture/legacy-amazon-listing-closure-retirement-boundary.md；architecture/legacy-redis-proxy-shared-closure-retirement-boundary.md | 仅本轮四文件/旧协议保留；当前 aiidentity context/roles 与有效身份合同继续 KEEP |
| 数字映射直接 KEEP（12） | architecture/legacy-auto-pricing-retirement-boundary.md；architecture/legacy-listing-cli-retirement-boundary.md；architecture/legacy-numeric-acquisition-tail-closure-retirement-boundary.md；architecture/legacy-numeric-listing-support-closure-retirement-boundary.md；architecture/legacy-numeric-store-management-closure-retirement-boundary.md；architecture/legacy-numeric-task-persistence-retirement-boundary.md；architecture/legacy-numeric-value-contract-closure-retirement-boundary.md；architecture/legacy-shein-policy-contract-closure-retirement-boundary.md；architecture/legacy-shein-listing-closure-retirement-boundary.md；architecture/legacy-temu-listing-closure-retirement-boundary.md；architecture/legacy-temu-listing-submit-retirement-boundary.md；architecture/legacy-temu-preflight-retirement-boundary.md | 仅 ProductImportMapping/本文件必须存在的准确 KEEP；其余平台/当前 JSON/SourceIdentity 四槽位、owner/历史保持 |
| 原收口与参考/映射索引（4） | architecture/legacy-retirement-parent-closure-boundary.md；architecture/temu-architecture-patterns.md；architecture/temu-pipeline-stages.md；refactoring/module-target-mapping.md | 仅 listingruntime/旧映射仍为现役依赖或推荐复用形式的含义，current context/roles 及其它映射保留 |

当前 Architecture README/Legacy Register 新条目只维护本批状态/链接；其余历史原完整后缀保留。external-client-boundary-inventory、Commerce Tool spec 和旧 ProductImage scene plan 的 aiidentity 引用只指当前上下文，不含本轮旧协议义务，不改这些文件。只保留旧名称的历史证据不要求批量清扫。

## 6. 实施顺序与必要验证

设计原 CI、一次适用独立设计复核、用户确认及 Independent Architecture / IMPLEMENTATION_READY、Issue/PR Ready 读回后，同一 Writer/PR 整批实施。先按 §6.1 扩展原 guard，在五工件仍存在时记录准确 RED；再删除五工件和转换原强制读/KEEP 断言，补全部窄顶注至 GREEN。当前有效身份没有新业务缺陷，不伪造 RED 或重建旧测试；保留现役拒绝/隔离回归。

必要开发自检：原四个修改 guard/八命令依赖检查、原 Architecture README 四项检查；shared/aiidentity 的 context、authidentity 的原 effective-role/不完整身份拒绝、当前 ImageAgent/Temporal Organization mode/完整身份隔离、Sourcing 原四槽位/规范化/hash/读拒绝与 SHEIN/Store 原适用合成 fixture。只运行直接相关选定原用例，不建 runner/故障矩阵或重复未变代码的完整 CI。稳定候选运行原完整 CI。

仅使用任务自有 temp/合成值、原内存/fake/loopback fixture；不执行旧 API/constructor、身份/恢复或业务 CLI，不加载既有 .env/有效配置/凭据，不新建 PG/Redis/browser/provider 环境。现役入口仅编译：current-application、image-agent-temporal-worker、shein-login-worker、source-account-registry-schema-init、product-listing-api-schema-migrate、1688-local-agent、1688-batch-import、1688-public-browser-collector；hack/debug test-amazon 独立模块仅编译，不执行产物。完整原 CI 内既有隔离 fixture 不等于真实实例验收。

### 6.1 原检查转换与三十一工件准确边界

本轮实施 **31 工件**＝5 整退源码/专属测试＋1 原 guard 文件＋22 窄顶注旧文档＋本文/Architecture README/Legacy Register 三当前文档。设计阶段仅最后三文档；用户确认及准入读回后，已按下表转换原四个函数，其它检查不变。

`tests/listingkit_backend_retirement_test.go` **仅下列四个函数可变**，其余函数/global 变量、原声明清单、helper、全部其它检查逐字不变；不新增独立测试/检查框架：

| 原函数 | 唯一转换 | 原检查保持 |
| --- | --- | --- |
| TestLegacyRuntimeSkeletonAndIdleConfigClosureIsRetired | 原 fail-closed Lstat 集合加四个 aiidentity 退休文件；沿原 owner-qualified FuncDecl/TypeSpec/ValueSpec 扫描加入 shared/aiidentity owner 与 §5.1 声明禁入；正向 KEEP Identity 五字段、WithIdentity/FromContext/request roles 四函数 | 原 config/kernel/http/logging/timeout/goroutine/json/appenv/httpclient/imagex 的禁入/KEEP、解析/读取失败、JPEG/PNG/WebP 注册断言不变；不得对其它 owner 同名符号禁入 |
| TestNumericAcquisitionAndPlatformTailClosureIsRetired | 将 types.go 的 StoreInfo 字符串禁入项转换成 listingruntime 整根 fail-closed Lstat 缺席；在本函数复用 trackedFiles/parser/decodeGoImportPath/importMatchesPrefix 扫描全部 tracked Go，拒绝该根及子路径 decoded import，含 tests/build tags/nested module | 当前 Browser/Amazon/SHEIN/API/SourceRequest/配置各旧禁入、所有其它文件强制读和失败语义保留；不加 missing-skip |
| TestListingKitTaskBackendSourcesAreRetired | 将 types.go 的 ImportTask/ScheduledTaskConfig/TaskStatusUpdate 强制读转换成整 listingruntime 根缺席，原根缺席检查覆盖旧 local 子目录；其余 map/循环保持 | 原全部文件/目录退出、当前平台 API/规则/字段/资源 KEEP、schema/worker/Store/JSON 等所有断言不减；不为类型退出删除其它检查 |
| TestCurrentCommandsDoNotCompileLegacyBusinessBackends | types.go 从必须存在转换为该文件 Lstat 必须缺席；原八命令依赖禁入增加 listingruntime 整根（保留 local 含义），其它依赖/旧根扫描原样 | 原全部平台 API/marketing/pricing/config KEEP、两套 workflow 必需/race/安全断言、八命令及其 go list 错误均保留，不能减命令或跳失败 |

aiidentity 完整包仍有现役 importer，故不能整体禁止 import；解析和正向 KEEP 用来防止误退当前上下文。listingruntime 无保留消费者，整根缺席与 decoded import 禁入共同覆盖旧四个类型、映射及改名文件复活。物理删除后先 stage 准确删除再运行依赖 trackedFiles 的原 guard；不能通过弱化读取失败换绿。

**workflow/完整 inventory（含 CURRENT）/历史/计数/ceilings/depguard/其它检查/依赖与锁/保护规则零改**。aiidentity 当前 import edges 保持，listingruntime 本身无外部依赖，无 inventory 行校准。若原 CI 实证发现表外孤立尾部或冻结 KEEP 冲突，先列失败、实际消费者与准确最小修订，取得用户决定后再应用，不把完整闭包当扩大修改范围的授权。

## 7. 准入、复核、交接与停止条件

### 7.1 设计交付

三文档设计完成适用原检查、准确设计 HEAD 原 CI 与一次适用只读独立设计复核，记录实际 SHA/结论及 finding 分类后交用户确认 §1–§7（含 §6.1）。31 工件是完整删除闭包，不按风险提示线形式拆 PR。本轮设计已获用户确认并完成 IMPLEMENTATION_READY/Ready 读回；设计证据仍不替代源码最终 CI 或复核。

### 7.2 实施交付

确认后沿同一主要 PR 连续实施、必要自检、提交/推送、范围内 finding/CI 修复。最终一次只读检查完整真实 diff、实际消费者/兄弟路径、当前身份/JSON/独立 worker 保留和 §6.1/历史后缀；修复仅复核相应增量。只有符合项目 BLOCKER 条件的真实问题重开设计，不以理论 hardening 扩建 IAM/恢复或验收体系；实现者自检、CI 或另一 AI 均不等于用户验收。

### 7.3 权限与评审

同一 HEAD 的 GitHub 评审运行时不重复触发；仅明确启动失败后一次受控重试，空 reviews/标签/摘要布尔值不当实际 PASS。**未经用户本轮授权不默认新增 AI 互验线程**；#917 设计/源码子代理授权不覆盖本轮。备用设计或源码复核须提供准确 HEAD、只读范围和失败证据，先获用户决定；不得从旧 plan 的派工文字推定授权。

设计确认、正式实施、合并、部署、运行/产品验收各自记录。本轮用户已授权冻结范围源码/测试实施、必要隔离自检、提交/推送与同一 PR 维护；合并/部署/关单及既有实例/数据/凭据/进程操作 NOT_AUTHORIZED，源码最终复核子代理须另获授权，运行/产品验收 NOT_RUN；#301 保持 OPEN，无新试用前置。未来获准确 HEAD 正常合并授权后，只用实际 merge SHA 的 event=push/head_branch=main 整 run completed/SUCCESS确认，不用 PR CI、单 Gate 或其它 main 提交替代。
