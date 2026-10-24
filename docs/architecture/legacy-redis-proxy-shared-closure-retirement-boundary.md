> #301 / #937 **USER_CONFIRMED**：[五组闲置支持门面完整退休闭包](legacy-idle-support-facade-closure-retirement-boundary.md) §1–§7（含§6.1）已获用户确认，Reuse Existing Architecture / Ready与Issue/PR读回6386195192/6386203704先于源码修改。仅收窄ptr/DB的广泛KEEP：六闲置指针变体、两无context DB入口及专属private helper退出；原DB安全测试只改接现有Context参数化接口，全部断言保留。IntPtr/Float32Ptr、Context DB/Open/共享池/关闭/迁移和其它共享基础层保持。只替代准确旧KEEP；以下完整原Git正文/既有顶注/数字/勾选/批准/历史证据逐字保留。workflow/完整inventory/依赖/保护与既有实例/数据/凭据/进程零改，不增试用前置，保留独立SHEIN登录与Store同步，#301 OPEN。

> #301 / #934 **USER_CONFIRMED**：[旧闲置 adapter/helper 完整退休闭包](legacy-idle-adapter-helper-closure-retirement-boundary.md) §1–§7（含 §6.1）已获用户确认，Reuse Existing Architecture / Ready 与 Issue/PR 读回先于源码修改。仅收窄 shared/resilience 的广泛 KEEP：无消费者的 limiter/breaker 及两专属测试整退；包根与 Retry/RetryConfig/RetryAttempt、ptr、实际 DB/Redis/共享能力保持，原有效重试行为不变。只按准确矩阵替代本项旧 KEEP；以下原文/已有顶注/数字/勾选/批准/历史证据完整逐字保留。workflow/完整 inventory/依赖/保护及既有实例/数据/凭据/进程零改，保留独立 SHEIN 登录与 Store 同步，不增试用前置，#301 OPEN。

> #301 / #923 **SUPERSEDED，仅旧租户 Prompt 闭包**：用户已确认[完整退休设计 §1–§7（含 §4.2/§6.1）](legacy-tenant-prompt-closure-retirement-boundary.md)。本文涉及旧 `internal/prompt`、`internal/promptmgmt`、`shared/tenantctx`、五 YAML 及其现役 KEEP/复用建议仅在该准确矩阵内被退休决定替代；如涉及镜像打包，仅退出两条 `prompts` COPY。当前 AI/图像/采集、身份/有效规则、独立 SHEIN 登录与 Store 同步及其它 KEEP 不变。停止全新 schema 创建旧 Prompt 表，不改 Goose 登记或已有表/行，不新增替代 Store/迁移/兼容。以下原正文、历史数字/勾选/证据完整保留，不扩大实施、合并或真实资源权限。

> #301 / #921 当前适用边界：用户已确认并按[旧任务身份与数字映射合同完整闭包 §1–§7/§6.1](legacy-task-identity-mapping-closure-retirement-boundary.md)整批实施。仅替代旧 shared/aiidentity ExecutionEnvelope/PersistedExecutionEnvelope、capture/restore/context/分类/来源白名单、Task tenant 辅助和两专属测试的保留/实施含义；五工件闭包已在同一 PR 源码退休。当前 Identity/WithIdentity/FromContext/request roles、authidentity 与现役 ImageAgent/Commercetool 授权/隔离/恢复完整保留；不复建旧无 scope 放行或旧信封，不执行历史 plan 的派工/迁移/运行步骤。 当前能力/采集/图像、独立 SHEIN 登录与 Store 同步及原 owner/fence/UNKNOWN 保留；原历史正文/计数/批准/勾选完整保留为后缀，workflow/inventory 不改，不操作既有实例/数据/凭据/进程，不增加试用前置。

# #301 旧 DB 代理、Redis 封装与 shared helper 完整退休设计

Status: **USER_CONFIRMED / SOURCE_IMPLEMENTED**。用户已确认冻结设计 `023567fe47f0262e23b56b58f02b34c195921bea` §1–§7（含 §6.1），在 #880 整批实施。[用户确认与实施准入](https://github.com/qq550723504/task-processor/pull/880#issuecomment-6333413341)及 #301/PR 的 Ready 读回先于正式源码修改；设计阶段 N/A，本轮 **Design Basis: Reuse Existing Architecture / Ready**。调查与开工同步 main 同为 `6266629b79a7ac100a6b007f77eadacefc46ec5e`，消费者刷新不变；以下冻结矩阵与停止条件保持，仅校准当前状态。一个 Writer、一个分支、一个主要 PR，三组完整闭包不机械拆批。#301 OPEN，#29 不重开；准确实现 HEAD、RED/GREEN、原 CI/最终复核留 PR，不把源码完成当作运行或用户验收。

## 1. 用户结果、范围与权限

使用者是维护当前应用、采集、图像、Store 与独立 SHEIN worker 的维护者。预期源码结果是一起退出无现役调用的 ConnectionProxy、旧 platform Redis/唯一闲置 adapter，以及 hashx/mathx/strx 三组专属闭包，避免把已无消费者的技术包装误认为当前可用产品能力。没有证据说明这些残留正在阻塞核心用户操作；不新增业务功能、替代基础设施或内部试用前置。

本轮已按用户确认的 §5/§6.1 同批源码退休 22 个完整工件及原检查/当前文档尾部；不为包、文件数、测试或审核拆分 PR。当前 DB 共享池与 schema owner、原生 Redis 和现役行为均留在原 owner。

不执行旧 constructor、ConnectionProxy.Execute/Close、Redis New/Ping/锁/Lua/Scan/Delete、MemoryLock ticker 或当前业务命令；不运行应用/worker/dry-run，不读有效运行配置、既有凭据、Profile/binding/Secret/token、进程环境、日志或既有报告。不操作既有实例、数据、凭据或进程，不启动 PG/Redis/browser/provider，不使用 Docker/Compose/kubectl/SQL。只读 Git 追踪的源码/文档与 `go list` 依赖元数据；物理状态/外部人工使用 UNKNOWN，运行与用户验收 NOT_RUN。

## 2. Design Basis、Authority 与不变量

设计阶段 **N/A**；本轮用户确认和 Ready 读回后以 **Reuse Existing Architecture** 实施，直接复用：

- [Greenfield 产品决定](../product/greenfield-no-legacy-migration.md)、[Hard-Cut Policy](../refactoring/legacy-hard-cut-policy.md)、[Legacy Register](../refactoring/legacy-register.md)、[项目边界](project-boundaries.md)和[Issue 驱动规则](../engineering/issue-driven-development.md)。仅 EXTRACT/RETIRE，无兼容层、旧 Service 包装、迁移、双读/双写或第二事实源。
- 当前 `platform/database.OpenShared/CloseShared`、GORM/Goose、`app/configadapter.Database`；当前 `core/config.RedisConfig` 与各 owner 的原生 `github.com/redis/go-redis/v9` 使用；不改变连接、关闭、schema 或资源 owner。
- [独立 SHEIN 当前身份与持久化边界](shein-worker-numeric-persistence-retirement-v1.md)、[#868 数字值合同退休边界](legacy-numeric-value-contract-closure-retirement-boundary.md)、[#878 配置/开关/tracing 边界](legacy-host-config-featureflag-tracing-retirement-boundary.md)的有效保留合同。当前身份/组织隔离、计费、采集、图像、Store、独立登录/同步及 UNKNOWN/fence 均保持。

本批无新产品/公共领域合同、表/schema、事实 owner、事务、状态机、权限/租户、幂等、retry/UNKNOWN/recovery、计费或跨系统安全/副作用边界。没有有用行为必须重新归属，不新增 EXTRACT/替代通用库；复用当前 owner 与现有成熟组件。UI/IA/Figma N/A。按完整范围处理风险，而不以行数/目录规避准入；若真实消费者刷新要求改变上述边界，按 §7 停止对应部分并升级准入。

Must：完整闭包退出，现役能力/共享合同不变，原必需与安全检查有效；workflow/完整 inventory/依赖/锁/ceilings/保护零改，无真实状态操作或新试用前置。Should：复用原 AST/import/退休 guard 和标准库/当前 SDK。Out of Scope：数据库/Redis 替换或调优、新锁/限流/缓存/清理系统、迁移/旧值接管、全仓 dead-code 工程、依赖清理和运行验收。Accepted Risks：无新增；未读取的真实状态不判为空。

**精确取代条件**：用户确认本文，仅在准确矩阵上取代 #868 对 MD5/HashString/ConsistentHash 和三旧 shared 目录的暂留、旧 Phase 2 对 platform Redis 包和 shared 搬迁目标的保留要求。§6.1 当前文档按新状态校准；旧冻结设计、dated inventory/计划/计数保留为历史，不授权复活旧包。其它当前 SDK/架构/业务要求不变。

## 3. contract → implementation → injection → consumer

基线使用全仓 `go list` 的 Imports/TestImports/XTestImports、受追踪 Go 引用、实际入口依赖及 adapter 调用核对；声明、包内自测或 dated 迁移目标不等于现役消费者。

| 对象 | 实际消费者/接线 | 设计结论 |
| --- | --- | --- |
| database.ConnectionProxy / ConnectionProxyConfig / ProxyLogger | NewConnectionProxy 只有专属 connection_proxy_test 调用；生产声明/方法只在同一源文件，没有当前 app/cmd 构造或注入 | 同批 RETIRE 两文件；不把父 database 包判为闲置 |
| platform/redis.Client / Config | 唯一包外生产 importer 为 app/configadapter/redis.go，只做 RedisConfig → platformredis.Config 值转换；包外测试 importer 是 adapter 自测 | adapter.Redis 没有生产调用，唯一包外依赖随 adapter 同退；Config 不留空壳/alias |
| platform/redis.RedisLock / MemoryLock / LockOptions / Logger | constructor 与操作只在本包代码/专属自测；没有 app/cmd 注册或业务调用 | 完整包连同锁/内存锁/README 同退，不新增替代锁或 lifecycle owner |
| shared/hashx | MD5/HashString/ConsistentHash 无包外生产或测试 importer；SimpleHash/ShardForTenantShop 已在 #868 退出 | 整包 RETIRE；当前 SourceIdentity 规范化/hash 等合同仍由原当前 owner 保留 |
| shared/mathx / shared/strx | 两包均无包外生产/测试 importer；Abs/Min/Max、清理/拼接/解析/大小写/截断等只剩声明与自测 | 整包及自测同退；不为理论可复用性搬家或新增工具层 |
| 当前 DB/adapter | commercial_read_module、storecenter_module、imageagent/dependencies 等仍调用 OpenShared/CloseShared；configadapter.Database 有当前调用 | 保留 postgres.go/config.go/README/migration、共享引用计数与关闭 owner、整个有效 adapter 其余部分 |
| 当前 Redis/SHEIN | sheinlogin/store.go 的 NewRedisStore 直接消费 core/config.RedisConfig 并构造原生 redis.Client；各当前 Redis/SDS 配置和入口仍有效 | 保留原配置/默认/env/Build/校验、原生客户端、key/TTL/owner/fence/UNKNOWN 及当前注入，不改为旧 platform 包 |
| 其它 shared/基础层 | ptr、resilience、aiidentity、tenantctx 及 logger/workerpool 等仍有现役消费者 | 保留；不按 shared/platform 名字或零默认入口误退显式保留的平台 API/规则 |

`app/configadapter` 的包级 import 在冻结基线使入口编译依赖 platform Redis，但不能证明 Redis adapter 函数被执行。本轮删除这条闲置编译依赖，不重新注入它。三个当前入口 `current-application`、`shein-login-worker`、`image-agent-temporal-worker` 的实际依赖须退出四个旧包；当前 DB/core-config/原生 Redis 依赖不退出。

## 4. 配置、持久化与副作用 owner

| 边界 | 旧实现可产生的效果 | 删除与保留边界 |
| --- | --- | --- |
| ConnectionProxy | OpenShared 得到 GORM 共享句柄；semaphore/maxOps、activeOps/WaitGroup、mutex/closeOnce 控制 callback 与关闭；Close 等待 activeWG 后通过 master.DB() 直接 sqlDB.Close()，不调用 CloseShared、不减少共享登记引用，可能留下指向已关闭池的登记 | 无当前生产构造、无独立 durable 业务事实；旧关闭缺陷不扩成代理修复，本轮整代理源码退休且不运行 Close/清理真实登记。现役 OpenShared/CloseShared/sharedDatabases/sharedDatabaseKey 及当前业务事务/关闭协议原样保留，不继承旧代理关闭路径、不引入另一连接池 |
| Redis Client | NewClient + 5 秒 Ping，RPush/Get/Set/SetNX/Del/Scan/集合及 TxPipelined ReplaceSet、Close | 删除无调用封装及专属 Config；不连接/扫描/删除任何 key，不把源码退出记成数据库清理或连接关闭 |
| RedisLock | UUID owner、SETNX、Lua compare-owner unlock/pexpire、TTL/Close | 删除闲置 owner 实现，不重新归属现有 Redis 锁或当前执行 fence；不调用锁/脚本、不自动接管 UNKNOWN |
| MemoryLock | 进程内 map/mutex/expiry、10 秒清理 ticker/goroutine 与 Close | 删除源码不等于停止既有进程/goroutine；不运行 constructor、自测或清理 |
| 当前业务/原生 Redis | 当前组织/身份/Store/独立登录 owner、installation/key 前缀/TTL、可选 Store execution fence、原持久化与恢复合同 | 无表/schema/迁移、owner marker、key/TTL、credential/Profile、SQL、队列或缓存操作；原 owner/幂等/授权/重试语义原样 |
| 纯值 helper | hashx/mathx/strx 只计算值，无本包持久化 owner | 当前来源规范化/hash、品牌/文本清理、价格/SKU/平台规则留原 owner，不删除同名现役行为，不新抽取闲置算法 |

物理旧二进制、连接、key/锁/配置、外部人工消费者均 UNKNOWN；本轮不探测，也不由仓库零调用推断为空。若发现具体当前外部可观察契约或保留消费者，停止对应实施并报告，不添加 compatibility/fallback 或自行迁移。

## 5. RETIRE / KEEP / EXTRACT 完整矩阵

### 5.1 同批 RETIRE 的 22 个完整工件

12 个生产 Go、9 个专属测试 Go、1 个 README，按准确文件/旧包根操作，不删除混合父目录：

```text
internal/platform/database/connection_proxy.go
internal/platform/database/connection_proxy_test.go
internal/platform/redis/client.go
internal/platform/redis/client_test.go
internal/platform/redis/config.go
internal/platform/redis/lock.go
internal/platform/redis/lock_test.go
internal/platform/redis/memory_lock.go
internal/platform/redis/memory_lock_test.go
internal/platform/redis/README.md
internal/app/configadapter/redis.go
internal/app/configadapter/redis_test.go
internal/shared/hashx/hashutil.go
internal/shared/mathx/math.go
internal/shared/mathx/math_test.go
internal/shared/strx/cleaner.go
internal/shared/strx/join.go
internal/shared/strx/join_test.go
internal/shared/strx/parse.go
internal/shared/strx/string.go
internal/shared/strx/string_test.go
internal/shared/strx/strings_test.go
```

不删除 `platform/database`、`app/configadapter`、`internal/shared` 父根；不留 dummy 文件、原包 alias/re-export、替代 constructor 或不再有 owner 的测试。

### 5.2 KEEP 与无新 EXTRACT

- `platform/database` 的 config/postgres/共享引用/CloseShared/Goose/migration/有效测试；`app/configadapter/database.go` 及测试；当前 HTTP/worker DB 接线和事务。不能从 ConnectionProxy 无调用推导共享池无用。
- 当前 `core/config.RedisConfig`、Config.Redis、SDS AuthRedis、env/default/Build/校验和样例；现役原生 go-redis，独立 SHEIN 的认证/installation/Store/登录/同步/TTL/UNKNOWN/fence。只删闲置 adapter 输出类型，不删配置源，不引入新 adapter。
- `shared/ptr`、`shared/resilience`、`shared/aiidentity`、`shared/tenantctx`，有效 logger、workerpool/通用队列/Retry、Temporal、appenv/downloader/httpclient/imagex/jsonx/goroutine/recovery/timeout/runtimepath 等当前基础层。
- 当前身份与计费、采集、图像/百炼图片参数/风险规则、AI/prompt/资产/预算、Store 观察同步、独立 SHEIN 登录与其 Store 同步；平台 API/DTO/规则/共享合同、MappingInfo、两种 SHEIN 属性、PriceManager/PriceCalculator、原来源规范化/hash/四个 JSON 零值槽位等此前全部保留能力。
- 全部成熟 SDK/依赖/锁保持；不 tidy/升级或改安全配置。不因无默认入口删显式保留的平台 API，不复制这些 helper 到新位置。

```text
Legacy decision: RETIRE（用户确认，已整批源码实施）
Reusable behavior: 无新 EXTRACT；有当前使用价值的 DB、Redis、纯规则/值合同已有保留 owner
Current owner: platform/database + app/configadapter.Database；core/config + 各现役原生 Redis/业务 owner；当前共享/规则 owner
Cutover/deletion condition: 已获用户确认 §1–§7（含 §6.1）、Ready 读回和消费者刷新无冲突，22 工件/原检查/直接文档尾部同退，无物理状态处置
```

## 6. 必要验证与证据边界

设计阶段只做受追踪源码/调用/依赖映射和原适用文档检查，不修改生产/测试/config，不执行旧包自测来证明保留必要，也不为纯文档制造 RED。原设计 CI/复核仅证明文档候选，不代替未来实现/合并后 main CI/运行/用户验收。

确认实施后复用原退休/AST/decoded-import 设施：准确缺席/禁入断言先 RED，退出旧闭包后 GREEN；读取、解析、依赖加载失败必须报错，不 missing-skip。必要开发自检包括原 tests 架构/退休/文档检查、保留 DB 共享引用与配置/adapter 合同自测、三个当前入口的真实 `go list -deps` 退出和适用编译；仅使用原已有的合成 fixture/t.TempDir，不新建 PG/Redis/容器、专项 runner 或故障矩阵。稳定候选跑原 CI，独立最终检查检查完整真实 diff；未变化代码证据明确引用，不冒充新 SHA 运行。

### 6.1 原检查、当前文档、inventory 与 workflow 的精确处置

下表为用户确认的同批实施范围，本轮已据此精确校准，没有扩大到其它源/配置/检查：

| 对象 | 同批最小校准 | 必须保留 |
| --- | --- | --- |
| 9 个旧包/adapter/proxy 自测 | 与准确旧 owner 同退，不把旧锁/代理/字符串实现的细节测试提升为当前需求 | 当前 DB 共享池、Database adapter、原生 Redis/SHEIN、当前规则及安全/租户/持久化测试 |
| tests/listingkit_backend_retirement_test.go | 复用已有设施增加本完整闭包缺席/禁入 guard：22 准确工件、platform/redis 和 hashx/mathx/strx 四旧包根；decoded imports 包括测试均禁入；database 根中旧 ConnectionProxy/ConnectionProxyConfig/ProxyLogger/NewConnectionProxy 与 configadapter.Redis 的 owner 专属顶层声明不再出现；三个当前入口真实依赖退出。删除对应旧 hashutil.go 字符串读取项，转为更强的整包缺席保护 | TestNumericContextAndValueContractsAreRetired 的其它 map 项、5/8 有效字段和原 hash/规范化合同等不变；不得删整个测试或把文件读取错误忽略；不按通用方法/局部变量同名误杀 |
| 同文件旧 TEMU 文案 guard 的 strx 专属 AST 循环 | CleanProductTitle/CleanWhitespace 在该旧包的禁止声明检查归入整包缺席/禁入 guard，移除只服务该包的 Glob/AST 循环 | TEMU 其它产品/规则/属性/AI/prompt/handlerbase/共享合同的原检查保持；当前 SHEIN 同名 receiver/field/local 不误杀 |
| tests/target_architecture_phase2_test.go / TestSharedPackageTargetsExist | 仅 hashx/mathx/strx 从必须存在转为旧包缺席/禁入保护，可复用同批 guard；ptr/resilience 必须存在及原错误处理保留 | 原共享依赖方向、resilience 无 internal 依赖、literal decoder/alias/dot-import、其它 Phase2/SDK/安全边界不变；不增加 missing-skip |
| 同文件 provider adapter matrix fixtures | 四处 shared/strx 示例路径精确替换为仍有效 shared/ptr，包括三个 allowed 和 httpimage forbidden 示例 | isAllowedIntegrationProviderAdapterImport 的现行共享/产品/平台允许拒绝策略、fixture 期望/解析/所有其它样例保持，不扩大 allowlist |
| 原 phase2 ceilings / inventory accuracy / depguard | **零改**：platform/redis max=8 是 dated ceiling，不要求包存在；app/configadapter 是原精确豁免的 intentional wiring，没有 CURRENT consumer 行需删。旧 internal/pkg/hashx/mathx/strx 的 depguard 禁入条款也保留 | 完整 phase2-runtime-inventory.md 的 CURRENT、历史/计数/ceilings/映射及所有准确性/解析检查逻辑、.golangci.yml、test-baseline.txt 原样；不新申请不存在的 stale 行例外 |
| project-boundaries.md §3.6 / internal/platform/README.md | 仅当前 runtime owner 列表退 platform Redis 项，增加本准确退休注记：原生 Redis/core/config/现役消费者及 DB bootstrap/共享池仍有效 | dated Phase2 原文及旧 Config/flags/tracing/RabbitMQ 退役说明、Goose/workerpool/Temporal/其它 integration owner 和必需文档词句原样；不宣称 Redis 能力退役 |
| Architecture README / Legacy Register / 本文 | 设计阶段只加 PROPOSED 引用，本轮确认实施后校准为确认/源码退出和实际 guard 证据 | 其它 owner、旧正文/计数/历史失败和保留矩阵保持；不把全仓/shared/database 宣称清零 |
| 全部 workflow / 依赖与锁 / 保护 | **零改**；无本闭包独立 workflow 目标需要精确替换，原 CI 正常执行 | 原触发/分类/权限/并发/Required CI Gate、必需和安全检查、SDK 约束与 ceilings 不变 |

若原 CI/实际消费者与矩阵冲突，给准确失败及最小处置后报告；不先改 inventory/workflow/ceilings/安全检查求绿，不扩大到其它旧包、配置、依赖或真实状态。用户确认的精确范围不能由未来 Reviewer 的理论 hardening 扩展。

## 7. 设计确认、交接与停止点

1. 在同一主要 PR 提交本设计及引用，记录准确设计 HEAD、基线、原文档检查/CI；一次有界原 GitHub 设计复核仅审本闭包与保留合同。同 SHA Running 不重复触发，不默认新增 AI 互验线程；无新高风险边界不重开全局架构。
2. 交用户确认 §1–§7（含 §6.1）后，在 #301/PR 记录冻结设计 HEAD、唯一 Writer 与 **Reuse Existing Architecture / Ready** 并读回，才开始生产源码。本轮已取得明确确认并在源码修改前读回；N/A/设计复核完成本身都不授权实施。
3. 三组 22 工件及上述专属尾部一起实施；原必要测试、提交/推送、范围内 CI/review 修复留同一 PR。开工同步一次实际 main，最终候选再同步一次；仅验证真实组合风险，不持续 merge-main 或重复审核未变化代码。
4. 发现具体现役消费者/外部可观察义务、新事实/权限/状态/事务/恢复/副作用边界、需兼容/迁移或额外 workflow/inventory/源删除时，停止对应部分并报告受影响 Must、阻塞层级和最小差异。只有真实 BLOCKER 重开冻结设计；非阻塞建议按原分类，不新增试用前置。
5. 实施后一次集中独立最终检查核对真实 diff、保留能力和有效证据；修复仅审增量。交付准确实现 HEAD、原 CI、复核与限制供用户准确 HEAD 合并授权，之后才确认实际 merge SHA 匹配的 completed/SUCCESS push/main 整 run；PR CI/Gate 单项不能替代。
6. 不以源码完成/CI/AI评审签发产品验收，不操作既有实例/数据/凭据/进程，不增加内部试用前置。#301 OPEN，#29 不重开；合并/关闭/部署/真实环境动作均单独授权。Issue 只改当前段，原历史后缀/勾选逐字保留；滚动证据留 PR，不为证据产生重复源码提交。
